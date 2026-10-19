# The nightly backup covers the object store: implementation plan

**Status (2026-10-19):** done; verified on the lab VM (`docs/superpowers/verification/2026-10-19-object-backup.md`). The operator chose this backup (*"1 ma elimina le copie vecchie se no hhd esplode"*).

> **For agentic workers:** REQUIRED SUB-SKILL: superpowers:executing-plans, task by task, with
> test-driven-development.

**Goal:** a nightly `backup_objects` job copies every bucket Aura holds a key for into
`AURA_BACKUP_DIR/objects/`, keeps fourteen days of history and deletes older copies, and
`aura objectstore restore --as-of` puts back what was live at a moment within those days.

**Architecture:**
- `internal/objectbackup` owns the copy and the restore over `objectstore.Store`, so the logic is
  tested against `objectstore.NewFake()`, with no Garage:
  - `Backup(ctx, dir, buckets, now)` brings each bucket's `manifest.json` and `blobs/` up to date;
  - `Restore(ctx, dir, buckets, asOf)` puts back each missing key.
- `internal/cron/handlers/backup_objects.go`: `KindBackupObjects` and a handler over a
  `func(ctx, dir, now) (objectbackup.Result, error)` seam. The dispatcher routes the kind there,
  and migration `0142` admits it.
- `cmd/aura/serve_object_backup.go` holds the walk over buckets:
  - the shared bucket, plus every identity's own bucket from `objectstore.IdentityStore`;
  - the backup uses the plain store, and the restore uses the minting resolver;
  - the same file holds the seed (`30 1 * * *`) and `aura objectstore restore`.

**Tech Stack:** Go 1.26; Garage v2.4.1 through `objectstore.S3Store`; Postgres scheduler.

**Spec:** `docs/superpowers/specs/2026-10-19-object-backup-design.md`.

## Global Constraints

- Every change test-first; `go vet`, `go build`, `go test -race` on each touched package, in WSL.
- Every file under 600 lines. `serve_provisioning.go` is at 591, so nothing lands there.
- The new package enters `scripts/coverage_package_policy.json` as `target` (≥85%).
- Migration number from `ls internal/db/migrations/ | tail -1`: 0141 is the head, so this is 0142,
  and `db_unit_test.go` moves its pin from 141 to 142.

## Review Focus

1. No key ever becomes a path, and a bucket name that is not a valid S3 name is refused.
2. A failed listing leaves the manifest as it was. A failed download keeps the previous version live.
3. Restore never overwrites a key the bucket holds, and it checks the sha256 before the Put.
4. A version retired more than 14 days ago is dropped. Every blob that no version references is
   deleted in the same run.
5. The backup never mints a bucket. The restore may.

## Decisions taken while writing this plan

- **Blobs per bucket, not shared across buckets.** Each bucket's garbage collection then reads only
  its own manifest. The same file in two identities' buckets is stored twice; isolation matters
  more than that saving.
- **No new env var.** The 14 days are a constant, as `backupRetentionPostgres` is. The objects land
  under the `AURA_BACKUP_DIR` both services already mount at `/backups`.
- **The version's MIME type and user metadata come from `Get`.** `List` returns neither. An
  unchanged object keeps what the manifest stored.
- **A changed ETag with the same sha256 updates the live version in place.** Same bytes are not a
  new version.
- **No live Garage test in CI.** `garage_integration` runs only in `scripts/musr_e2e.sh`. The live
  leg is the lab VM run in Task 5, and it has its own verification file.

## File structure

- `internal/objectbackup/manifest.go`: `Manifest`, `Version`, load and atomic save, bucket-name check.
- `internal/objectbackup/backup.go`: `Bucket`, `Result`, `Backup`, retention and blob GC.
- `internal/objectbackup/restore.go`: `Restore`, `LiveAt`.
- `internal/objectbackup/*_test.go`: over `objectstore.NewFake()`.
- `internal/cron/handlers/backup_objects.go` and its test.
- `internal/cron/store.go`: `KindBackupObjects`.
- `internal/db/migrations/0142_scheduler_backup_objects_kind.{up,down}.sql`.
- `cmd/aura/serve_object_backup.go` and its test: the walk, the seed, the handler's seam.
- `cmd/aura/objectstore.go`: `restore`.
- `cmd/aura/serve_dispatch.go`, `cmd/aura/serve.go`: wiring.
- `docs/BACKUP-RESTORE.md`, `prd.md`, `scripts/coverage_package_policy.json`.

## Tasks

### Task 1: the copy

- [x] Failing tests for `Backup` over a fake store:
  - first run: every object gets a blob and a live version;
  - second run: nothing is downloaded;
  - a changed object: the old version is retired and a new one is live;
  - a deleted object: its version is retired;
  - a version retired 15 days ago is dropped and its blob deleted, while one retired 13 days ago is
    kept;
  - a failed listing: the manifest is untouched;
  - a failed download: the old version stays live and the error names the count;
  - a bucket name `../x` is refused.
- [x] Implement `manifest.go` and `backup.go`.
- [x] `go test -race ./internal/objectbackup/`.

### Task 2: the restore

- [x] Failing tests for `Restore`:
  - a missing key comes back with its bytes, MIME type and metadata;
  - a key the bucket holds is not touched;
  - `--as-of` before a delete brings the deleted version back;
  - `--as-of` before a change brings back nothing for that key, since it is present;
  - a blob whose sha256 does not match is refused;
  - a bucket without a manifest is an error.
- [x] Implement `restore.go`.

### Task 3: the job

- [x] Failing handler tests: `Meta` (kind, budget, fires on recovery), the summary, the error.
- [x] `backup_objects.go`, `KindBackupObjects`, migration 0142 up/down, `db_unit_test.go` pin.
- [x] Wire `serve_dispatch.go`, the seed in `serve.go`.
- [x] Found while wiring:
  - `backup_objects` is managed like `backup_postgres`: `IsUserManageableKind` is true and
    `IsCancellableKind` false. As a system kind it would have been missing from the cockpit board.
  - `display.parseTaskList` refuses a whole list on a kind it does not know, so it learns
    `backup_objects`. Otherwise the agent's task list would have fallen back to raw text.
  - `collectBlobs` removes through an `os.Root` (gosec G122), with a symlink test.

### Task 4: the walk and the restore command

- [x] Failing tests for the walk: the shared bucket plus each provisioned identity, unprovisioned
  identities skipped, a bucket listed once, a resolve error kept and the walk continuing.
- [x] `serve_object_backup.go`; `aura objectstore restore [--as-of] [--bucket]`.
- [x] `newObjectIdentityStore` folds the shared-credential `IdentityStore` construction, which
  `buildObjectResolverBundle` and `buildProvisioningPorts` each carried, into one helper.

### Task 5: docs, push, lab VM

- [x] `docs/BACKUP-RESTORE.md` (the table row, the restore procedure, recovery scope), `prd.md`,
  the coverage policy entry.
- [x] Push from WSL, publish, and let the updater bring the VM to the build (`d0a2d1efe`, then `a5d75c2d8`).
- [x] On the VM:
  - run the job;
  - compare the manifest with the bucket and each sha256;
  - delete one sample through the cockpit and run again: the sample is retired;
  - `aura objectstore restore --as-of <before>`: it is back with its sha256 and name;
  - a third run downloads nothing.
- [x] `docs/superpowers/verification/2026-10-19-object-backup.md`.
