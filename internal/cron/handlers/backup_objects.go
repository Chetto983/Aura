package handlers

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"github.com/chetto1983/aura/internal/objectbackup"
)

// KindBackupObjects is the nightly copy of the object store into AURA_BACKUP_DIR/objects
// (docs/superpowers/specs/2026-10-19-object-backup-design.md). It is seeded (seedObjectBackup),
// not model-schedulable, and like the Postgres backup it can be run now but never cancelled; the
// 0142 migration widened the scheduler_tasks.kind CHECK to admit the row.
//
// It exists because the 2026-10-14 reinstall of the lab VM brought back Postgres and ArcadeDB
// from their backups and nothing brought back Garage: every uploaded file was lost.
const KindBackupObjects TaskKind = "backup_objects"

// BackupObjects names the object-store backup in the missed-backup alert.
const BackupObjects BackupVariant = "objects"

// ObjectBackupFn backs up every bucket the caller holds a key for into root, at now.
type ObjectBackupFn func(ctx context.Context, root string, now time.Time) (objectbackup.BackupResult, error)

type objectBackupHandler struct {
	backup ObjectBackupFn
}

// NewObjectBackupHandler builds the backup_objects handler. A nil backup is the disabled no-op
// of a deployment without an object store.
func NewObjectBackupHandler(backup ObjectBackupFn) Handler {
	return objectBackupHandler{backup: backup}
}

// Meta is the Postgres backup's contract: a missed night fires on recovery, and the budget is
// generous because the first run downloads every object.
func (h objectBackupHandler) Meta() HandlerMeta {
	return HandlerMeta{Kind: KindBackupObjects, MaxDuration: backupMaxDuration, ReschedulesOnRecovery: true}
}

func (h objectBackupHandler) Run(ctx context.Context, job Job) (string, error) {
	now := time.Now().UTC()
	MissedBackupAlert(BackupObjects, job.MissedSince, now)
	if h.backup == nil {
		return "backup objects: disabled (no object store)", nil
	}
	root, err := ObjectBackupRoot()
	if err != nil {
		return "", err
	}
	res, err := h.backup(ctx, root, now)
	counts := fmt.Sprintf("%d bucket(s), %d object(s), %d copied, %d retired, %d purged",
		res.Buckets, res.Objects, res.Fetched, res.Retired, res.Purged)
	if err != nil {
		return "", fmt.Errorf("backup objects: %s, %d failed: %w", counts, res.Failed, err)
	}
	return fmt.Sprintf("backup objects ok -> %s: %s", root, counts), nil
}

// ObjectBackupRoot is where the object backup lives: AURA_BACKUP_DIR/objects, beside the Postgres
// dumps. `aura objectstore restore` reads it from here too.
func ObjectBackupRoot() (string, error) {
	dir, err := backupDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "objects"), nil
}
