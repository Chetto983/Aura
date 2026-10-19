# The nightly backup covers the object store

**Status (2026-10-19):** approved by the operator (*"1 ma elimina le copie vecchie se no hhd
esplode"*): Aura copies the objects itself, through S3, into `AURA_BACKUP_DIR`, and deletes old
copies so the disk does not fill. Implemented and verified on the lab VM the same day
(`docs/superpowers/verification/2026-10-19-object-backup.md`).

## Why

On 2026-10-14 the lab VM was wiped and reinstalled. The 2026-10-13 backup restored Postgres
(`backup_postgres`) and ArcadeDB (native scheduler), and nothing restored Garage: every file the
operator had uploaded was gone, while Documenti still listed nine of them. The bucket and its key
are back since `6a1847515` (rebuilt from the credential row Postgres keeps), but the row holds no
objects. `docs/BACKUP-RESTORE.md` already said so under *Recovery scope*: "Garage object data and
metadata" must be preserved separately, and nothing did.

Garage's own guidance ([Backups](https://garagehq.deuxfleurs.fr/documentation/connect/backup/)) is
to copy the objects out through the S3 API with a backup tool. Garage has no snapshot to take.

## Decisions

1. **A seeded job, `backup_objects`**, beside `backup_postgres`. It is seeded daily at
   `30 1 * * *` in `Europe/Rome`, half an hour after the Postgres dump. It keeps the Postgres
   backup's contract:
   - it shows on the cockpit's scheduler board and to the agent's task list;
   - it can be run now;
   - it can never be cancelled or paused (operator's decision of 2026-10-05 for the database
     backup);
   - a run missed during downtime fires on recovery.

   The task tool's `kind` enum is unchanged, so no one creates a second one. Adding the kind takes
   migration `0142` to widen `scheduler_tasks_kind_check`.
2. **Every bucket Aura holds a key for:**
   - the shared bucket (`AURA_OBJECTSTORE_BUCKET`, which also holds the shares);
   - each identity's own bucket, from `aura.identity_object_store`.

   Each bucket is read with its own key (F-007), never with a key granted on every bucket. An
   identity without a row has no bucket and is skipped. The backup uses the plain
   `IdentityStore`, not the minting resolver, so a backup never creates a bucket.
3. **Content-addressed layout.** Each bucket has `AURA_BACKUP_DIR/objects/<bucket>/manifest.json`
   and `blobs/<sha256[:2]>/<sha256>`. A key never becomes a path, so a key such as `../x` cannot
   escape the directory. A bucket name is accepted only if it matches S3's naming rule. Blobs never
   change once written, so restic or rsync can carry the directory off-site unchanged (the
   off-site leg stays its own design).
4. **Incremental.** An object whose size and ETag match the manifest's live version, and whose
   blob is on disk, is not downloaded again. Each run downloads only what changed.
5. **Fourteen days of history.** Each version records:
   - `seen_at`: the first run that captured it;
   - `retired_at`: the first run that listed it no more, or listed a different version under its
     key.

   A version without `retired_at` is live. *"Elimina le copie vecchie"*: a version retired more
   than 14 days ago is dropped, and in the same run every blob no version references is deleted.
   The disk holds the live set plus fourteen days of changes, the same window as the Postgres
   dumps.
6. **Restore picks a moment.** `aura objectstore restore [--as-of RFC3339] [--bucket name]` puts
   back every version live at `--as-of` (default: the latest backup) whose key the bucket does not
   hold. The history in decision 5 makes this necessary. A wiped Garage looks, to the next nightly
   run, like an operator who deleted every file: the run retires every version. Restoring "as of"
   the night before the wipe brings them back, within the 14 days.

   Restore rules:
   - It never overwrites a key the bucket holds.
   - It checks each blob's sha256 before the Put.
   - It writes the version's MIME type and user metadata (the file name), so Documenti and the file
     manager read the same names they read before.
   - It goes through the minting resolver, so a bucket or key Garage lost is put back first.
7. **A failure stays local.** A bucket whose listing fails keeps its manifest untouched. An object
   whose download fails keeps its previous version live. The run backs up everything else,
   then returns an error that names the counts, so the scheduler records a failed run.

## What this does not do

- It copies nothing off the host. A disk loss still takes the backup with it. Off-site restic is
  `project_dr_offsite_backup_design`.
- The four stores are still not one consistent snapshot. Objects are captured half an hour after
  Postgres, and a file uploaded in between is in one and not the other.
- It encrypts nothing. `AURA_BACKUP_DIR` holds the objects in clear, as it holds the Postgres dumps.
- Restore puts back missing keys only, so it cannot roll a changed file back. The older blob stays
  on disk for 14 days, and its manifest entry says which key it was.

## Measure that closes it

On the lab VM:
- the job's run is recorded `succeeded` with its counts;
- the manifest lists every object the file manager shows, with sha256 equal to the file's;
- a sample deleted through the cockpit is `retired` by the next run;
- `aura objectstore restore --as-of <before the delete>` puts it back with the same sha256 and
  name;
- a second run downloads nothing.

The result lands in `docs/superpowers/verification/`.
