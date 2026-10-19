# An identity's lost Garage bucket and key are put back: verification

Date: 2026-10-19. Fix `6a1847515` (`fix(objectstore): put back an identity's bucket and key Garage
lost`), published as edge and run on the lab VM `192.168.101.158`.

**Verdict:** the lab VM's file manager answers again. The identity's bucket and key are back in
Garage, and every request with the identity's key since then has succeeded. The files the bucket held
before 2026-10-14 are lost: they were in no backup, and this fix cannot bring objects back.

## What was broken

After the operator's Craft web-apps update, the file manager's listing (`GET /api/filemanager/files`)
answered 400.
- **The cause was older than that update.** On 2026-10-14 the lab VM was wiped and reinstalled, and
  the 2026-10-13 backup restored Postgres and ArcadeDB but not Garage
  (`2026-10-14-gitlab-image-bridge.md`, "The databases come back").
- **Postgres** still named the bucket `aura-448ddbe1-…` and the key `GK1bb7…` in
  `aura.identity_object_store`, since 2026-09-21, and held 26 asset rows up to 2026-10-11.
- **Garage**, a volume created on 2026-10-14 09:19, held only `aura-assets`, with 0 objects, and its
  key `aura-assets-local`.
- **Every read with the identity's key** answered `403 No such key`: the first at 13:52:04, a minute
  after the VM booted and an hour before the update. The listing turned it into a 400.
- **The resolve path** only logged `browser-upload CORS not configured … no bucket for global alias`.

## What was measured before writing the fix

On a throwaway Garage v2.4.1 (`dxflrs/garage:v2.4.1`, single node, layout applied):

| call | answer |
|---|---|
| `GET /v2/GetKeyInfo?id=<unknown>` | 404 `NoSuchAccessKey` |
| `GET /v2/GetBucketInfo?globalAlias=<unknown>` | 404 `NoSuchBucket` |
| `POST /v2/ImportKey` (new id) | 200 |
| `POST /v2/ImportKey` (same id again) | 409 `KeyAlreadyExists`, "even if it is deleted" |
| S3 PUT and GET signed with the imported key and its secret | 200, the bytes read back |
| S3 GET signed with a wrong secret | 403 |

Garage's admin API documents `ImportKey` as reserved for "migrations and backup restore".

## Tests

- **Unit tests:**
  - garageadmin: `KeyExists` reports 200 as present and 404 as absent, and any other status as an
    error. `ImportKey` sends the stored id, secret and name, and fails on a refusal.
  - The adapter puts back both a lost bucket and a lost key, or only the key. It leaves an intact
    bucket alone, with one check per process, and tolerates a 409 from a concurrent import.
  - The adapter survives a failed restore, does not import a key Garage could not check, and runs
    nothing on the mint or shared paths.
- **`garage_integration` tier** against the throwaway Garage:
  - `TestGarageAdminImportsALostKey` passes;
  - `TestEnsureForIdentityRestoresAgainstALiveGarage` passes: a row naming a bucket and a key Garage
    never held resolves, and the identity's stored key then writes and reads its bucket.
- **Gates:** `go vet` (with every tier tag), `go test -race` on `cmd/aura` and `internal/objectstore/...`,
  golangci-lint 0 issues, pre-commit and pre-push hooks green.

## On the lab VM

- **The update:** edge `6a1847515` was applied by the updater; aura was healthy at 18:49:10Z.
- **The restore:** it ran at the first request that resolved the identity, when the operator opened
  the file manager. Aura logged at 18:52:03Z `put back what Garage had lost … restored=[bucket key]`.
- **What Garage holds now:**
  - the bucket `aura-448ddbe1-…`, created 18:52:03, 0 objects;
  - the key `GK1bb7…` imported under the name `aura-448ddbe1-…`;
  - grants: `RW` for the identity's key, `RWO` for Aura's own key;
  - the CORS rule written by Aura's key at 18:52:03.7 (`PUT /aura-448ddbe1-…/?cors=`).
- **Since 18:52:04:**
  - six requests with the identity's key, none refused;
  - no object-store warning in aura's log but the restore line;
  - no error line in the ingest log.
- **What the operator sees:** the file manager opens, empty, with no error.

## Not shown here

- **Lost files:** the 26 files uploaded between 2026-09-21 and 2026-10-11 are lost. Their asset rows
  are still in Postgres and their downloads will answer 404. The Documenti page, which lists those
  rows, still shows 9 documents, and every one of them points into a bucket that holds 0 objects.
  Nothing was cleaned up; that is the operator's call. No page marks a row whose object is gone.
- **Backups:** the backup procedure still does not carry Garage's objects, so a restore loses files
  every time.
- **When the restore runs:** it is lazy, at the first resolve of each identity in a process, not at
  boot. Until then a sidecar that reads with the identity's key directly (the ingest) keeps answering
  403.
- **A deleted key cannot be imported again:** Garage refuses an id it has ever held. That case is
  logged as a failed restore and not repaired.
- **CI:** none ran. GitHub Actions is off while the account is flagged, and GitLab's only runner is
  offline.
