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

// RestoreResult counts one restore. Present is the versions whose key the bucket already held.
type RestoreResult struct {
	Buckets  int
	Restored int
	Present  int
	Failed   int
}

// Restore puts back, in each bucket, every version live at asOf whose key the bucket does not
// hold. It never overwrites a key, so it cannot roll a changed file back; it only refills what is
// missing.
func Restore(ctx context.Context, root string, buckets []Bucket, asOf time.Time) (RestoreResult, error) {
	var total RestoreResult
	var errs []error
	for _, b := range buckets {
		res, err := restoreBucket(ctx, root, b, asOf)
		total.Restored += res.Restored
		total.Present += res.Present
		total.Failed += res.Failed
		total.Buckets++
		if err != nil {
			errs = append(errs, fmt.Errorf("bucket %s: %w", b.Name, err))
		}
	}
	return total, errors.Join(errs...)
}

func restoreBucket(ctx context.Context, root string, b Bucket, asOf time.Time) (RestoreResult, error) {
	var res RestoreResult
	dir, err := bucketDir(root, b.Name)
	if err != nil {
		return res, err
	}
	m, err := readManifest(dir, b.Name)
	if errors.Is(err, fs.ErrNotExist) {
		return res, fmt.Errorf("no backup in %s", dir)
	}
	if err != nil {
		return res, err
	}
	for _, v := range m.LiveAt(asOf) {
		ref := objectstore.ObjectRef{Bucket: b.Name, Key: v.Key}
		present, err := holds(ctx, b.Store, ref)
		if err == nil && present {
			res.Present++
			continue
		}
		if err == nil {
			err = putBack(ctx, dir, b.Store, ref, v)
		}
		if err != nil {
			res.Failed++
			slog.Warn("object restore: put back failed", "bucket", b.Name, "key", v.Key, "err", err)
			continue
		}
		res.Restored++
	}
	if res.Failed > 0 {
		return res, fmt.Errorf("%d object(s) not restored", res.Failed)
	}
	return res, nil
}

func holds(ctx context.Context, st objectstore.Store, ref objectstore.ObjectRef) (bool, error) {
	_, err := st.Head(ctx, ref)
	if objectstore.IsNotFound(err) {
		return false, nil
	}
	return err == nil, err
}

func putBack(ctx context.Context, dir string, st objectstore.Store, ref objectstore.ObjectRef, v Version) error {
	path := blobPath(dir, v.SHA256)
	if err := verifyBlob(path, v.SHA256); err != nil {
		return err
	}
	blob, err := os.Open(path) //nolint:gosec // the path is built from a validated bucket name and sha256.
	if err != nil {
		return err
	}
	defer func() { _ = blob.Close() }()
	_, err = st.Put(ctx, ref, blob, objectstore.PutOptions{MIMEType: v.MIMEType, Size: v.Size, Metadata: v.Metadata})
	return err
}

func verifyBlob(path, want string) error {
	blob, err := os.Open(path) //nolint:gosec // the path is built from a validated bucket name and sha256.
	if err != nil {
		return err
	}
	defer func() { _ = blob.Close() }()
	hash := sha256.New()
	if _, err := io.Copy(hash, blob); err != nil {
		return err
	}
	if got := hex.EncodeToString(hash.Sum(nil)); got != want {
		return fmt.Errorf("blob %s holds bytes hashing to %s", want, got)
	}
	return nil
}

// BackedUp names, sorted, the buckets root holds a backup of. A missing root holds none.
func BackedUp(root string) ([]string, error) {
	entries, err := os.ReadDir(root)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []string
	for _, e := range entries {
		if _, err := os.Stat(filepath.Join(root, e.Name(), manifestName)); e.IsDir() && err == nil {
			out = append(out, e.Name())
		}
	}
	return out, nil
}
