// Package objectbackup copies the object store's buckets into a local directory, keeping
// Retention of history, and puts them back as of a moment in it
// (docs/superpowers/specs/2026-10-19-object-backup-design.md). Garage has no snapshot to take:
// its own guidance is to copy the objects out through S3, which is what this does, one bucket at
// a time with that bucket's own key.
//
// Each bucket's backup is root/<bucket>/manifest.json plus root/<bucket>/blobs/<sha[:2]>/<sha>.
// Blobs are named by content and never rewritten, so no object key becomes a path and the
// directory can be carried off-site by restic or rsync as it is.
package objectbackup

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/chetto1983/aura/internal/objectstore"
)

// Retention is how long a version stays restorable after a run found it deleted or replaced.
// It is the Postgres dumps' window, and it is what makes a wiped bucket recoverable: the next
// run sees a wipe as every file deleted, and the versions stay for this long.
const Retention = 14 * 24 * time.Hour

// Bucket is one bucket and a store holding a key that can read it.
type Bucket struct {
	Name  string
	Store objectstore.Store
}

// BackupResult counts one run. Objects is the live versions after it; Fetched, the versions it
// downloaded; Purged, the versions dropped past Retention.
type BackupResult struct {
	Buckets int
	Objects int
	Fetched int
	Retired int
	Purged  int
	Failed  int
}

func (r *BackupResult) add(o BackupResult) {
	r.Objects += o.Objects
	r.Fetched += o.Fetched
	r.Retired += o.Retired
	r.Purged += o.Purged
	r.Failed += o.Failed
}

// Backup brings each bucket's backup under root up to date at now. A bucket that fails does not
// stop the next; the error names every bucket that failed.
func Backup(ctx context.Context, root string, buckets []Bucket, now time.Time) (BackupResult, error) {
	var total BackupResult
	var errs []error
	for _, b := range buckets {
		res, err := backupBucket(ctx, root, b, now)
		total.add(res)
		total.Buckets++
		if err != nil {
			errs = append(errs, fmt.Errorf("bucket %s: %w", b.Name, err))
		}
	}
	return total, errors.Join(errs...)
}

// backupBucket leaves the manifest as it was when the listing fails: an unreadable bucket is not
// an empty one. An object it cannot download keeps its previous version live.
func backupBucket(ctx context.Context, root string, b Bucket, now time.Time) (BackupResult, error) {
	var res BackupResult
	dir, err := bucketDir(root, b.Name)
	if err != nil {
		return res, err
	}
	m, err := readManifest(dir, b.Name)
	if errors.Is(err, fs.ErrNotExist) {
		m, err = Manifest{SchemaVersion: schemaVersion, Bucket: b.Name}, nil
	}
	if err != nil {
		return res, err
	}
	listed, err := b.Store.List(ctx, objectstore.ListRequest{Bucket: b.Name})
	if err != nil {
		return res, fmt.Errorf("list: %w", err)
	}
	if err := os.MkdirAll(filepath.Join(dir, blobsDir), 0o750); err != nil {
		return res, err
	}

	live := m.live()
	listedKeys := make(map[string]bool, len(listed))
	for _, obj := range listed {
		listedKeys[obj.Ref.Key] = true
		i, known := live[obj.Ref.Key]
		if known && m.Versions[i].matches(obj.Attrs) && blobExists(dir, m.Versions[i].SHA256) {
			continue
		}
		v, err := fetch(ctx, dir, b.Store, obj, now)
		if err != nil {
			res.Failed++
			slog.Warn("object backup: copy failed", "bucket", b.Name, "key", obj.Ref.Key, "err", err)
			continue
		}
		if known && m.Versions[i].SHA256 == v.SHA256 {
			v.SeenAt = m.Versions[i].SeenAt
			m.Versions[i] = v
			continue
		}
		if known {
			m.Versions[i].retire(now)
			res.Retired++
		}
		m.Versions = append(m.Versions, v)
		res.Fetched++
	}
	for key, i := range live {
		if !listedKeys[key] {
			m.Versions[i].retire(now)
			res.Retired++
		}
	}

	res.Purged = m.purge(now.Add(-Retention))
	res.Objects = len(m.live())
	m.UpdatedAt = now
	if err := m.save(dir); err != nil {
		return res, err
	}
	if err := collectBlobs(dir, m); err != nil {
		return res, err
	}
	if res.Failed > 0 {
		return res, fmt.Errorf("%d object(s) not copied", res.Failed)
	}
	return res, nil
}

// matches compares what a listing reports. The ETag is the listing's own, so the next listing
// compares like with like.
func (v Version) matches(attrs objectstore.Attrs) bool {
	return v.Size == attrs.SizeBytes && v.ETag == attrs.ETag
}

func fetch(ctx context.Context, dir string, st objectstore.Store, obj objectstore.ObjectInfo, now time.Time) (Version, error) {
	body, attrs, err := st.Get(ctx, obj.Ref)
	if err != nil {
		return Version{}, err
	}
	defer func() { _ = body.Close() }()
	sha, size, err := writeBlob(dir, body)
	if err != nil {
		return Version{}, err
	}
	return Version{
		Key:        obj.Ref.Key,
		SHA256:     sha,
		Size:       size,
		ETag:       obj.Attrs.ETag,
		MIMEType:   attrs.MIMEType,
		Metadata:   attrs.Metadata,
		ModifiedAt: obj.Attrs.ModifiedAt,
		SeenAt:     now,
	}, nil
}

func writeBlob(dir string, body io.Reader) (string, int64, error) {
	tmp, err := os.CreateTemp(filepath.Join(dir, blobsDir), ".tmp-*")
	if err != nil {
		return "", 0, err
	}
	defer func() { _ = os.Remove(tmp.Name()) }()
	hash := sha256.New()
	size, err := io.Copy(io.MultiWriter(tmp, hash), body)
	if err == nil {
		err = tmp.Sync()
	}
	if closeErr := tmp.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return "", 0, err
	}
	sha := hex.EncodeToString(hash.Sum(nil))
	if blobExists(dir, sha) {
		return sha, size, nil
	}
	final := blobPath(dir, sha)
	if err := os.MkdirAll(filepath.Dir(final), 0o750); err != nil {
		return "", 0, err
	}
	return sha, size, os.Rename(tmp.Name(), final)
}

func blobExists(dir, sha string) bool {
	_, err := os.Stat(blobPath(dir, sha))
	return err == nil
}

// collectBlobs deletes every file under blobs/ no version names: a purged version's blob, and the
// temporary file of a run that died mid-copy. It walks and removes through an os.Root, so a
// symlink planted under blobs/ cannot point a removal outside it.
func collectBlobs(dir string, m Manifest) error {
	keep := make(map[string]bool, len(m.Versions))
	for _, v := range m.Versions {
		keep[v.SHA256] = true
	}
	blobs, err := os.OpenRoot(filepath.Join(dir, blobsDir))
	if err != nil {
		return err
	}
	defer func() { _ = blobs.Close() }()
	return fs.WalkDir(blobs.FS(), ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || keep[d.Name()] {
			return err
		}
		return blobs.Remove(path)
	})
}
