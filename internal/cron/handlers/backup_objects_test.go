package handlers

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/chetto1983/aura/internal/objectbackup"
)

func TestObjectBackupMeta(t *testing.T) {
	t.Parallel()
	meta := NewObjectBackupHandler(nil).Meta()
	if meta.Kind != KindBackupObjects || meta.MaxDuration != backupMaxDuration || !meta.ReschedulesOnRecovery {
		t.Fatalf("meta = %+v, want backup_objects with the backup budget, fired on recovery", meta)
	}
}

func TestObjectBackupRunCopiesUnderTheBackupDir(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("AURA_BACKUP_DIR", dir)
	var root string
	h := NewObjectBackupHandler(func(_ context.Context, r string, now time.Time) (objectbackup.BackupResult, error) {
		root = r
		if time.Since(now) > time.Minute || now.Location() != time.UTC {
			t.Errorf("now = %s, want the current UTC time", now)
		}
		return objectbackup.BackupResult{Buckets: 2, Objects: 9, Fetched: 1, Retired: 1, Purged: 3}, nil
	})

	summary, err := h.Run(context.Background(), Job{})
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(dir, "objects"); root != want {
		t.Fatalf("root = %q, want %q", root, want)
	}
	if want := "backup objects ok -> " + filepath.Join(dir, "objects") +
		": 2 bucket(s), 9 object(s), 1 copied, 1 retired, 3 purged"; summary != want {
		t.Fatalf("summary = %q, want %q", summary, want)
	}
}

func TestObjectBackupRunFailsWithTheCountsAndTheCause(t *testing.T) {
	t.Setenv("AURA_BACKUP_DIR", t.TempDir())
	h := NewObjectBackupHandler(func(context.Context, string, time.Time) (objectbackup.BackupResult, error) {
		return objectbackup.BackupResult{Buckets: 2, Objects: 7, Failed: 1},
			errors.New("bucket aura-x: 1 object(s) not copied")
	})

	summary, err := h.Run(context.Background(), Job{})
	if err == nil || summary != "" || !strings.Contains(err.Error(), "7 object(s)") ||
		!strings.Contains(err.Error(), "1 failed") || !strings.Contains(err.Error(), "aura-x") {
		t.Fatalf("Run = %q, %v, want an error carrying the counts and the bucket", summary, err)
	}
}

func TestObjectBackupIsDisabledWithoutAnObjectStore(t *testing.T) {
	t.Parallel()
	summary, err := NewObjectBackupHandler(nil).Run(context.Background(), Job{})
	if err != nil || summary != "backup objects: disabled (no object store)" {
		t.Fatalf("Run = %q, %v", summary, err)
	}
}
