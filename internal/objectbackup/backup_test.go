package objectbackup

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/chetto1983/aura/internal/objectstore"
	"go.uber.org/goleak"
)

func TestMain(m *testing.M) { goleak.VerifyTestMain(m) }

const bucket = "aura-3f2a9c1e-0b7d-4e55-9a61-2c8f0d4b7e19"

var t0 = time.Date(2026, 10, 19, 23, 30, 0, 0, time.UTC)

// recordingStore is a real fake store whose reads and listings a test can count or break.
type recordingStore struct {
	*objectstore.FakeStore
	listErr    error
	getErr     map[string]error
	etagSuffix string
	gets       int
}

func newStore() *recordingStore {
	return &recordingStore{FakeStore: objectstore.NewFake(), getErr: map[string]error{}}
}

func (s *recordingStore) List(ctx context.Context, req objectstore.ListRequest) ([]objectstore.ObjectInfo, error) {
	if s.listErr != nil {
		return nil, s.listErr
	}
	listed, err := s.FakeStore.List(ctx, req)
	for i := range listed {
		listed[i].Attrs.ETag += s.etagSuffix
	}
	return listed, err
}

func (s *recordingStore) Get(ctx context.Context, ref objectstore.ObjectRef) (io.ReadCloser, objectstore.Attrs, error) {
	s.gets++
	if err := s.getErr[ref.Key]; err != nil {
		return nil, objectstore.Attrs{}, err
	}
	return s.FakeStore.Get(ctx, ref)
}

func put(t *testing.T, st objectstore.Store, key, body, name string) {
	t.Helper()
	_, err := st.Put(context.Background(), objectstore.ObjectRef{Bucket: bucket, Key: key},
		strings.NewReader(body), objectstore.PutOptions{
			MIMEType: "text/plain",
			Size:     int64(len(body)),
			Metadata: map[string]string{objectstore.MetadataFileName: name},
		})
	if err != nil {
		t.Fatalf("put %s: %v", key, err)
	}
}

func remove(t *testing.T, st objectstore.Store, key string) {
	t.Helper()
	if err := st.Delete(context.Background(), objectstore.ObjectRef{Bucket: bucket, Key: key}); err != nil {
		t.Fatalf("delete %s: %v", key, err)
	}
}

func backup(t *testing.T, dir string, st objectstore.Store, now time.Time) BackupResult {
	t.Helper()
	res, err := Backup(context.Background(), dir, []Bucket{{Name: bucket, Store: st}}, now)
	if err != nil {
		t.Fatalf("backup at %s: %v", now, err)
	}
	return res
}

func manifest(t *testing.T, dir string) Manifest {
	t.Helper()
	m, err := readManifest(filepath.Join(dir, bucket), bucket)
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	return m
}

func versionsOf(m Manifest, key string) []Version {
	var out []Version
	for _, v := range m.Versions {
		if v.Key == key {
			out = append(out, v)
		}
	}
	return out
}

func sum(body string) string {
	s := sha256.Sum256([]byte(body))
	return hex.EncodeToString(s[:])
}

func blobFile(dir, body string) string {
	s := sum(body)
	return filepath.Join(dir, bucket, "blobs", s[:2], s)
}

func TestBackupCopiesEachObjectOnceWithItsNameAndType(t *testing.T) {
	dir, st := t.TempDir(), newStore()
	put(t, st, "identity/x/asset/1", "Costituzione", "Costituzione_Italiana.pdf")
	put(t, st, "identity/x/asset/2", "Colosseo", "Colosseo.jpg")

	first := backup(t, dir, st, t0)
	if first.Fetched != 2 || first.Objects != 2 || first.Buckets != 1 {
		t.Fatalf("first run = %+v, want 2 fetched, 2 objects, 1 bucket", first)
	}
	got, err := os.ReadFile(blobFile(dir, "Costituzione"))
	if err != nil || string(got) != "Costituzione" {
		t.Fatalf("blob = %q, %v", got, err)
	}
	v := versionsOf(manifest(t, dir), "identity/x/asset/1")
	if len(v) != 1 || v[0].SHA256 != sum("Costituzione") || v[0].MIMEType != "text/plain" ||
		v[0].Metadata[objectstore.MetadataFileName] != "Costituzione_Italiana.pdf" ||
		!v[0].SeenAt.Equal(t0) || v[0].RetiredAt != nil || v[0].Size != int64(len("Costituzione")) {
		t.Fatalf("version = %+v", v)
	}

	reads := st.gets
	second := backup(t, dir, st, t0.Add(24*time.Hour))
	if second.Fetched != 0 || second.Objects != 2 || st.gets != reads {
		t.Fatalf("second run = %+v after %d new reads, want nothing fetched", second, st.gets-reads)
	}
}

func TestBackupRetiresTheOldVersionOfAChangedObject(t *testing.T) {
	dir, st := t.TempDir(), newStore()
	put(t, st, "a", "first", "a.txt")
	backup(t, dir, st, t0)
	put(t, st, "a", "second", "a.txt")
	t1 := t0.Add(24 * time.Hour)

	res := backup(t, dir, st, t1)
	if res.Fetched != 1 || res.Retired != 1 || res.Objects != 1 {
		t.Fatalf("run = %+v, want 1 fetched, 1 retired, 1 object", res)
	}
	v := versionsOf(manifest(t, dir), "a")
	if len(v) != 2 || v[0].SHA256 != sum("first") || v[0].RetiredAt == nil || !v[0].RetiredAt.Equal(t1) ||
		v[1].SHA256 != sum("second") || v[1].RetiredAt != nil || !v[1].SeenAt.Equal(t1) {
		t.Fatalf("versions = %+v", v)
	}
	if _, err := os.Stat(blobFile(dir, "first")); err != nil {
		t.Fatalf("the retired version's blob is gone inside the window: %v", err)
	}
}

func TestBackupRetiresADeletedObjectAndPurgesItAfterTheWindow(t *testing.T) {
	dir, st := t.TempDir(), newStore()
	put(t, st, "kept", "kept body", "kept.txt")
	put(t, st, "gone", "gone body", "gone.txt")
	backup(t, dir, st, t0)
	remove(t, st, "gone")
	t1 := t0.Add(24 * time.Hour)

	if res := backup(t, dir, st, t1); res.Retired != 1 || res.Objects != 1 {
		t.Fatalf("run after the delete = %+v, want 1 retired, 1 object", res)
	}
	if res := backup(t, dir, st, t1.Add(13*24*time.Hour)); res.Purged != 0 {
		t.Fatalf("run 13 days later purged %d version(s)", res.Purged)
	}
	if _, err := os.Stat(blobFile(dir, "gone body")); err != nil {
		t.Fatalf("blob gone 13 days after the delete: %v", err)
	}

	res := backup(t, dir, st, t1.Add(15*24*time.Hour))
	if res.Purged != 1 || res.Objects != 1 {
		t.Fatalf("run 15 days later = %+v, want 1 purged, 1 object", res)
	}
	if v := versionsOf(manifest(t, dir), "gone"); len(v) != 0 {
		t.Fatalf("purged version still listed: %+v", v)
	}
	if _, err := os.Stat(blobFile(dir, "gone body")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("purged blob still on disk: %v", err)
	}
	if _, err := os.Stat(blobFile(dir, "kept body")); err != nil {
		t.Fatalf("live blob deleted by the purge: %v", err)
	}
}

func TestBackupUpdatesInPlaceWhenOnlyTheETagChanged(t *testing.T) {
	dir, st := t.TempDir(), newStore()
	put(t, st, "a", "same bytes", "a.txt")
	backup(t, dir, st, t0)
	st.etagSuffix = "-2"

	res := backup(t, dir, st, t0.Add(time.Hour))
	if res.Fetched != 0 || res.Retired != 0 {
		t.Fatalf("run = %+v, want no new version for the same bytes", res)
	}
	v := versionsOf(manifest(t, dir), "a")
	if len(v) != 1 || !strings.HasSuffix(v[0].ETag, "-2") || !v[0].SeenAt.Equal(t0) {
		t.Fatalf("versions = %+v, want one version carrying the new ETag", v)
	}
	if again := backup(t, dir, st, t0.Add(2*time.Hour)); st.gets != 2 || again.Fetched != 0 {
		t.Fatalf("the refreshed ETag is read again: %d reads", st.gets)
	}
}

func TestBackupLeavesTheManifestAloneWhenTheListingFails(t *testing.T) {
	dir, st := t.TempDir(), newStore()
	put(t, st, "a", "body", "a.txt")
	backup(t, dir, st, t0)
	path := filepath.Join(dir, bucket, manifestName)
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	st.listErr = errors.New("NoSuchBucket")

	if _, err := Backup(context.Background(), dir, []Bucket{{Name: bucket, Store: st}}, t0.Add(time.Hour)); err == nil ||
		!strings.Contains(err.Error(), "NoSuchBucket") {
		t.Fatalf("err = %v, want the listing's error", err)
	}
	after, _ := os.ReadFile(path)
	if !bytes.Equal(before, after) {
		t.Fatal("a failed listing rewrote the manifest")
	}
}

func TestBackupKeepsTheLastVersionOfAnObjectItCouldNotRead(t *testing.T) {
	dir, st := t.TempDir(), newStore()
	put(t, st, "a", "old", "a.txt")
	backup(t, dir, st, t0)
	put(t, st, "a", "new", "a.txt")
	put(t, st, "b", "b body", "b.txt")
	st.getErr["a"] = errors.New("connection reset")

	res, err := Backup(context.Background(), dir, []Bucket{{Name: bucket, Store: st}}, t0.Add(time.Hour))
	if err == nil || !strings.Contains(err.Error(), "1 object(s) not copied") {
		t.Fatalf("err = %v, want the count of objects not copied", err)
	}
	if res.Failed != 1 || res.Fetched != 1 {
		t.Fatalf("run = %+v, want 1 failed, 1 fetched", res)
	}
	m := manifest(t, dir)
	if v := versionsOf(m, "a"); len(v) != 1 || v[0].RetiredAt != nil || v[0].SHA256 != sum("old") {
		t.Fatalf("a = %+v, want the old version still live", v)
	}
	if v := versionsOf(m, "b"); len(v) != 1 {
		t.Fatalf("b = %+v, want it copied despite a's failure", v)
	}
}

func TestBackupGoesOnToTheNextBucketAfterOneFails(t *testing.T) {
	dir, broken, ok := t.TempDir(), newStore(), newStore()
	broken.listErr = errors.New("AccessDenied")
	put(t, ok, "a", "body", "a.txt")

	res, err := Backup(context.Background(), dir,
		[]Bucket{{Name: "aura-assets", Store: broken}, {Name: bucket, Store: ok}}, t0)
	if err == nil || !strings.Contains(err.Error(), "aura-assets") {
		t.Fatalf("err = %v, want it to name the failed bucket", err)
	}
	if res.Buckets != 2 || res.Objects != 1 {
		t.Fatalf("run = %+v, want both buckets visited and the good one copied", res)
	}
}

func TestBackupRefusesABucketNameThatIsNotAnS3Name(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "objects")
	for _, name := range []string{"../escape", "a", "Upper", "a..b", "-lead", strings.Repeat("a", 64)} {
		_, err := Backup(context.Background(), dir, []Bucket{{Name: name, Store: newStore()}}, t0)
		if err == nil {
			t.Fatalf("bucket %q accepted", name)
		}
	}
	if _, err := os.Stat(filepath.Join(root, "escape")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("a refused name wrote outside the backup: %v", err)
	}
}

func TestBackupNeverDeletesThroughASymlinkUnderTheBlobs(t *testing.T) {
	root, st := t.TempDir(), newStore()
	dir := filepath.Join(root, "objects")
	put(t, st, "a", "body", "a.txt")
	backup(t, dir, st, t0)
	outside := filepath.Join(root, "outside")
	if err := os.MkdirAll(outside, 0o750); err != nil {
		t.Fatal(err)
	}
	victim := filepath.Join(outside, "operator.txt")
	if err := os.WriteFile(victim, []byte("keep me"), 0o600); err != nil {
		t.Fatal(err)
	}
	blobs := filepath.Join(dir, bucket, "blobs")
	if err := os.Symlink(outside, filepath.Join(blobs, "zz")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(victim, filepath.Join(blobs, "link")); err != nil {
		t.Fatal(err)
	}

	backup(t, dir, st, t0.Add(time.Hour))
	if got, err := os.ReadFile(victim); err != nil || string(got) != "keep me" {
		t.Fatalf("a file outside the backup was touched: %q, %v", got, err)
	}
	for _, link := range []string{"zz", "link"} {
		if _, err := os.Lstat(filepath.Join(blobs, link)); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("symlink %s left under blobs/: %v", link, err)
		}
	}
	if _, err := os.Stat(blobFile(dir, "body")); err != nil {
		t.Fatalf("the live blob went with the links: %v", err)
	}
}

func TestBackupRefusesAManifestOfAnotherBucket(t *testing.T) {
	dir, st := t.TempDir(), newStore()
	put(t, st, "a", "body", "a.txt")
	backup(t, dir, st, t0)
	path := filepath.Join(dir, bucket, manifestName)
	raw, _ := os.ReadFile(path)
	if err := os.WriteFile(path, bytes.Replace(raw, []byte(bucket), []byte("aura-assets"), 1), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Backup(context.Background(), dir, []Bucket{{Name: bucket, Store: st}}, t0.Add(time.Hour)); err == nil {
		t.Fatal("a manifest naming another bucket was accepted")
	}
}
