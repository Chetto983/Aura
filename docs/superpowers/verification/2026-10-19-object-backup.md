# The nightly backup covers the object store: verification

Date: 2026-10-19, 22:35–23:15 UTC. Spec `docs/superpowers/specs/2026-10-19-object-backup-design.md`,
plan `docs/superpowers/plans/2026-10-19-object-backup.md`, code `2689b5e02` and `cccc61240`. Build
`d0a2d1efe` was published as edge and applied on the lab VM `192.168.101.158` by its updater.

**Verdict:** each run's counts matched the bucket, and every manifest sha256 matched the bytes the
file manager serves. A file deleted through the file manager was retired by the next run, and
`aura objectstore restore --as-of` before the deletion put it back with the same bytes and the same
type. Three defects surfaced on the way and are fixed in `43bb22703`, `40b35c486` and `a5d75c2d8`.

## Setting

- **Buckets:** the operator's bucket `aura-448ddbe1-…` held 7 objects (7.7 MB): `esempio.md` from
  a chat upload, plus the six web samples in `prove-craft/` (PDF, docx, xlsx, pptx, JPEG, MP4). The
  shared bucket `aura-assets` held 0.
- **Backup directory:** `AURA_BACKUP_DIR` on the host is `/opt/aura/backups`, mounted at `/backups`
  in `aura`. It held the Postgres dumps and no `objects/`.

## Runs

The runs were driven through the cockpit's own routes, under the operator's session:
- the board: `GET /api/governance/scheduler`, then `POST …/{id}/run`;
- the file manager: `DELETE /api/filemanager/files`, then `GET /api/filemanager/direct`.

The restore ran as the appliance's CLI inside the container:
`docker exec aura aura objectstore restore`.

1. **Migration and seed:** after the update, the board listed `backup_objects` at
   `30 1 * * * Europe/Rome`, with its next run at 23:30Z and `cancellable=False`.
2. **Cancel through the board:** `403 {"error":"the database backup cannot be cancelled"}`. The
   refusal was right, but the words named the wrong backup; fixed in `40b35c486`.
3. **First run (22:35:33, 330 ms):** `2 bucket(s), 7 object(s), 7 copied, 0 retired, 0 purged`.
   `objects/` measured 7.5M.
4. **Second run:** `0 copied`, in 28 ms.
5. **Bytes:** each of the 7 files, downloaded through the file manager, had the sha256 and the size
   its manifest records. A check on the VM hashed every blob against the manifest: 0 mismatches.
   The manifest kept the MIME type of each file, and `filename` where the upload wrote one
   (`esempio.md`).
6. **Delete:** `SampleShow.pptx`, a test sample, deleted through the file manager at 22:45:24. The
   next run: `6 object(s), 0 copied, 1 retired`. Its version carries `retired_at` 22:45:33, and its
   blob stayed on disk (7 blob files, 6 objects in Garage).
7. **Restore without `--as-of`:** `0 restored, 6 already present, 0 failed`. The latest backup
   records the file as deleted.
8. **Restore `--as-of 2026-10-19T22:45:10Z`:** `1 restored, 6 already present, 0 failed`, exit 0.
   The file manager lists `SampleShow.pptx` again, serving 39083 bytes with sha256
   `bfb4b2f0…481d828`, the original's.
9. **Runs after the restore:**
   - the next run: `1 copied`. The restored key is a live version again, with its original MIME
     type, and it reuses the same blob (still 7 files);
   - the run after that: `0 copied`;
   - final state: 7 live, 1 retired within the window, Garage 7.
10. **Documenti:** 7 documents, `SampleShow.pptx` among them.

## Found and fixed

- **The empty bucket's manifest:** `aura-assets` wrote `"versions": null`. Go reads that back as
  empty, but a reader iterating the list fails on it, and the VM check did. Fixed: an empty
  manifest lists `[]` (`43bb22703`).
- **The board's refusal message** named only the database backup: see step 2 (`40b35c486`).
- **The coverage gate** (`scripts/coverage_docker.sh`, disposable Postgres) measured the owned
  surface at 87.3% (47796/54778) and refused one package. `internal/objectstore/garageadmin`
  changed its denominator from 137 to 144 in `6a1847515` without moving its baseline. The new ratio,
  113/144, is above the old one, so `a5d75c2d8` declares it.
- **A hang in the second gate run:** `TestMountedServer_IdempotencyScopedOperationNeverRedials`
  (`internal/agent/mcptools`) waited ten minutes. The test's fixture was freed only when go-sdk's
  `ServerSession.Close` happened to cancel the client's `subscriptions/listen` first. Measured: with a
  client that opens no listen, the old fixture waited out every deadline. The new one, which pings
  until `Close` refuses and then answers on the closing connection, gets EOF in 3 ms. Fixed in
  `09a87ae38`, with a test that pins the drop without a listen. The race lives in the test, not in
  aura: a peer that dies closes its transport.

## After the fixes

Build `a5d75c2d8`, applied by the updater, at 23:14 UTC:
- **The board's refusal:** cancel answers `403 {"error":"a backup cannot be cancelled"}`.
- **A run after the update:** `2 bucket(s), 7 object(s), 0 copied, 0 retired, 0 purged`. The backup
  outlived the container's replacement, and nothing was downloaded again.
- **Manifests:** both read cleanly. `aura-assets` has 0 live versions, 0 blobs and Garage 0. The
  operator's bucket has 7 live and 1 retired version (`SampleShow.pptx`'s first copy, within the
  window), 7 blobs, 0 mismatches, and Garage 7.

## The nightly clock, observed 2026-10-20

Read the next morning from the VM's own Postgres (`aura.agent_job_runs`) and the backup directory,
with no run pressed by hand after 23:14 UTC. The VM stopped at 23:50 UTC with its host and booted
again at 08:12 UTC.
- **`backup_postgres`** started on its clock at 2026-10-19 23:00:03 UTC (01:00 Europe/Rome) and
  completed in 1.1 s: `backup postgres ok -> /backups/postgres-20261019T230003Z.dump`.
- **`backup_objects`** started on its clock at 23:30:10 UTC (01:30 Europe/Rome) and completed in
  34 ms: `2 bucket(s), 7 object(s), 0 copied, 0 retired, 0 purged`. Both manifests carry
  `updated_at` 23:30:10.77Z; the operator's bucket still has 7 live and 1 retired version.
- The board's next runs are 2026-10-20 23:00 and 23:30 UTC.

This shows the seeded cron firing on its own. Neither run was a missed-run recovery: the VM was off
only after both had fired.

## Gates

- **Unit:** `go vet`, `go test -race` and golangci-lint v2.14.0 (0 issues) on every touched package.
  `internal/objectbackup` is at 90.3%, and enters the coverage policy as a target.
- **Pre-push:** lefthook passed on each push from WSL.
- **`db_integration`:** after `09a87ae38`, `scripts/coverage_docker.sh` (disposable Postgres, all
  migrations through 0142, with `TestReset_DownUpRoundTrip` running them down and up) passed in
  552 s. The owned surface was at 87.3% (47798/54780), and the package-local policy passed.
- **GitLab CI** did not judge these commits. Its jobs ended `stuck_pending_no_matching_runners`:
  no runner picked them up.

## What this does not prove

- **Retention:** the 14-day window and the purge of a retired version's blob are proven only by
  unit tests (`TestBackupRetiresADeletedObjectAndPurgesItAfterTheWindow`). No run here was 14 days
  apart.
- **A whole wipe:** no bucket was lost. A wiped and refilled Garage is covered by the walk's unit
  test (`TestObjectBucketWalkBacksUpAndRestoresEveryBucket`), with the minting resolver putting the
  bucket back first, and by the live Garage restore of `6a1847515`. It was not run end to end here.
- **A missed night:** the scheduled run is observed above, but a run recovered after the host was
  off at 01:30 is not.
- **Scale:** 7.7 MB in 7 objects. A large bucket's first run against the 30-minute budget was not
  measured.
- **Off-host:** `AURA_BACKUP_DIR` is on the VM's own disk, so the backup does not survive the loss
  of that disk.
- **Every identity:** the VM has one identity with a bucket. Several identities, each read with its
  own key, are covered only by the walk's unit tests.
