package objectbackup

import (
	"context"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/chetto1983/aura/internal/objectstore"
)

func restore(t *testing.T, dir string, st objectstore.Store, asOf time.Time) RestoreResult {
	t.Helper()
	res, err := Restore(context.Background(), dir, []Bucket{{Name: bucket, Store: st}}, asOf)
	if err != nil {
		t.Fatalf("restore as of %s: %v", asOf, err)
	}
	return res
}

func read(t *testing.T, st objectstore.Store, key string) (string, objectstore.Attrs) {
	t.Helper()
	body, attrs, err := st.Get(context.Background(), objectstore.ObjectRef{Bucket: bucket, Key: key})
	if err != nil {
		t.Fatalf("get %s: %v", key, err)
	}
	defer func() { _ = body.Close() }()
	raw, err := io.ReadAll(body)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw), attrs
}

func TestRestorePutsBackAMissingObjectWithItsNameAndType(t *testing.T) {
	dir, st := t.TempDir(), newStore()
	put(t, st, "identity/x/asset/1", "Colosseo bytes", "Colosseo.jpg")
	backup(t, dir, st, t0)
	remove(t, st, "identity/x/asset/1")

	if res := restore(t, dir, st, t0); res.Restored != 1 || res.Present != 0 || res.Buckets != 1 {
		t.Fatalf("restore = %+v, want 1 restored", res)
	}
	body, attrs := read(t, st, "identity/x/asset/1")
	if body != "Colosseo bytes" || attrs.MIMEType != "text/plain" ||
		attrs.Metadata[objectstore.MetadataFileName] != "Colosseo.jpg" {
		t.Fatalf("restored %q %+v", body, attrs)
	}
}

func TestRestoreLeavesAKeyTheBucketHoldsAlone(t *testing.T) {
	dir, st := t.TempDir(), newStore()
	put(t, st, "a", "backed up", "a.txt")
	backup(t, dir, st, t0)
	put(t, st, "a", "edited since", "a.txt")

	if res := restore(t, dir, st, t0); res.Restored != 0 || res.Present != 1 {
		t.Fatalf("restore = %+v, want the present key left alone", res)
	}
	if body, _ := read(t, st, "a"); body != "edited since" {
		t.Fatalf("restore overwrote the bucket's file with %q", body)
	}
}

func TestRestoreAsOfBeforeADeleteBringsTheFileBack(t *testing.T) {
	dir, st := t.TempDir(), newStore()
	put(t, st, "kept", "kept body", "kept.txt")
	put(t, st, "gone", "gone body", "gone.txt")
	backup(t, dir, st, t0)
	remove(t, st, "gone")
	t1 := t0.Add(24 * time.Hour)
	backup(t, dir, st, t1)

	if res := restore(t, dir, st, t1); res.Restored != 0 {
		t.Fatalf("restore as of after the delete = %+v, want the deleted file left deleted", res)
	}
	if res := restore(t, dir, st, t0.Add(time.Hour)); res.Restored != 1 || res.Present != 1 {
		t.Fatalf("restore as of before the delete = %+v, want gone restored, kept present", res)
	}
	if body, _ := read(t, st, "gone"); body != "gone body" {
		t.Fatalf("restored %q", body)
	}
}

func TestRestoreAsOfBeforeTheFirstBackupRestoresNothing(t *testing.T) {
	dir, st := t.TempDir(), newStore()
	put(t, st, "a", "body", "a.txt")
	backup(t, dir, st, t0)
	remove(t, st, "a")

	if res := restore(t, dir, st, t0.Add(-time.Minute)); res.Restored != 0 {
		t.Fatalf("restore = %+v, want nothing from before the first backup", res)
	}
}

func TestRestoreRefusesABlobWhoseBytesChanged(t *testing.T) {
	dir, st := t.TempDir(), newStore()
	put(t, st, "a", "original", "a.txt")
	backup(t, dir, st, t0)
	remove(t, st, "a")
	if err := os.WriteFile(blobFile(dir, "original"), []byte("tampered"), 0o600); err != nil {
		t.Fatal(err)
	}

	res, err := Restore(context.Background(), dir, []Bucket{{Name: bucket, Store: st}}, t0)
	if err == nil || !strings.Contains(err.Error(), "1 object(s) not restored") || res.Failed != 1 {
		t.Fatalf("restore = %+v, %v, want the tampered blob refused", res, err)
	}
	if _, err := st.Head(context.Background(), objectstore.ObjectRef{Bucket: bucket, Key: "a"}); !objectstore.IsNotFound(err) {
		t.Fatalf("a tampered blob was put back: %v", err)
	}
}

func TestRestoreOfABucketNeverBackedUpIsAnError(t *testing.T) {
	if _, err := Restore(context.Background(), t.TempDir(), []Bucket{{Name: bucket, Store: newStore()}}, t0); err == nil ||
		!strings.Contains(err.Error(), "no backup") {
		t.Fatalf("err = %v, want no backup", err)
	}
}

func TestBackedUpNamesEachBucketWithAManifest(t *testing.T) {
	dir, a, b := t.TempDir(), newStore(), newStore()
	if _, err := Backup(context.Background(), dir,
		[]Bucket{{Name: bucket, Store: a}, {Name: "aura-assets", Store: b}}, t0); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(dir+"/stray", 0o750); err != nil {
		t.Fatal(err)
	}
	got, err := BackedUp(dir)
	if err != nil || strings.Join(got, ",") != "aura-3f2a9c1e-0b7d-4e55-9a61-2c8f0d4b7e19,aura-assets" {
		t.Fatalf("BackedUp = %v, %v", got, err)
	}
	if none, err := BackedUp(dir + "/missing"); err != nil || len(none) != 0 {
		t.Fatalf("BackedUp of a missing dir = %v, %v, want none", none, err)
	}
}
