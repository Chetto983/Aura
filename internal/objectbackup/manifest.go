package objectbackup

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

const (
	manifestName  = "manifest.json"
	blobsDir      = "blobs"
	schemaVersion = 1
)

var (
	// S3's bucket naming rule. A bucket name becomes a directory, so anything else is refused
	// rather than cleaned.
	bucketName = regexp.MustCompile(`^[a-z0-9][a-z0-9.-]{1,61}[a-z0-9]$`)
	sha256Hex  = regexp.MustCompile(`^[0-9a-f]{64}$`)
)

// Manifest is one bucket's backup: every version of every object captured in the last
// Retention, by key.
type Manifest struct {
	SchemaVersion int       `json:"schema_version"`
	Bucket        string    `json:"bucket"`
	UpdatedAt     time.Time `json:"updated_at"`
	Versions      []Version `json:"versions"`
}

// Version is one object's bytes as a run captured them. SeenAt is the run that first captured
// it; RetiredAt, the first run that found the key gone or holding other bytes. A version
// without RetiredAt is live.
type Version struct {
	Key        string            `json:"key"`
	SHA256     string            `json:"sha256"`
	Size       int64             `json:"size"`
	ETag       string            `json:"etag,omitempty"`
	MIMEType   string            `json:"mime_type,omitempty"`
	Metadata   map[string]string `json:"metadata,omitempty"`
	ModifiedAt time.Time         `json:"modified_at"`
	SeenAt     time.Time         `json:"seen_at"`
	RetiredAt  *time.Time        `json:"retired_at,omitempty"`
}

func bucketDir(root, name string) (string, error) {
	if !bucketName.MatchString(name) || strings.Contains(name, "..") {
		return "", fmt.Errorf("objectbackup: %q is not an S3 bucket name", name)
	}
	return filepath.Join(root, name), nil
}

func blobPath(dir, sha string) string {
	return filepath.Join(dir, blobsDir, sha[:2], sha)
}

// readManifest returns fs.ErrNotExist, unwrapped, for a bucket never backed up.
func readManifest(dir, bucket string) (Manifest, error) {
	raw, err := os.ReadFile(filepath.Join(dir, manifestName)) //nolint:gosec // dir is built from a validated bucket name.
	if err != nil {
		return Manifest{}, err
	}
	var m Manifest
	if err := json.Unmarshal(raw, &m); err != nil {
		return Manifest{}, fmt.Errorf("objectbackup: %s manifest: %w", bucket, err)
	}
	if m.SchemaVersion != schemaVersion || m.Bucket != bucket {
		return Manifest{}, fmt.Errorf("objectbackup: %s manifest is schema %d of bucket %q", bucket, m.SchemaVersion, m.Bucket)
	}
	for _, v := range m.Versions {
		if !sha256Hex.MatchString(v.SHA256) {
			return Manifest{}, fmt.Errorf("objectbackup: %s manifest names blob %q for %q", bucket, v.SHA256, v.Key)
		}
	}
	return m, nil
}

func (m Manifest) save(dir string) error {
	sort.SliceStable(m.Versions, func(i, j int) bool {
		if m.Versions[i].Key != m.Versions[j].Key {
			return m.Versions[i].Key < m.Versions[j].Key
		}
		return m.Versions[i].SeenAt.Before(m.Versions[j].SeenAt)
	})
	raw, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	return writeAtomic(filepath.Join(dir, manifestName), raw)
}

func writeAtomic(path string, raw []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".tmp-*")
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(tmp.Name()) }()
	if _, err := tmp.Write(raw); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}

// live maps each key to the index of its live version.
func (m Manifest) live() map[string]int {
	out := make(map[string]int)
	for i, v := range m.Versions {
		if v.RetiredAt == nil {
			out[v.Key] = i
		}
	}
	return out
}

// LiveAt returns the versions the bucket held at t, as the runs saw it: first captured at or
// before t, and not retired by then.
func (m Manifest) LiveAt(t time.Time) []Version {
	var out []Version
	for _, v := range m.Versions {
		if !v.SeenAt.After(t) && (v.RetiredAt == nil || v.RetiredAt.After(t)) {
			out = append(out, v)
		}
	}
	return out
}

// purge drops the versions retired before cutoff and returns how many.
func (m *Manifest) purge(cutoff time.Time) int {
	kept := m.Versions[:0]
	for _, v := range m.Versions {
		if v.RetiredAt == nil || !v.RetiredAt.Before(cutoff) {
			kept = append(kept, v)
		}
	}
	purged := len(m.Versions) - len(kept)
	m.Versions = kept
	return purged
}

func (v *Version) retire(at time.Time) { v.RetiredAt = &at }
