# The ingest removes index rows whose object is gone: implementation plan

**Status (2026-10-19):** Tasks 1–2 done; Task 3 (lab VM) open. The operator chose this fix (*"2 e poi 1"*).

> **For agentic workers:** REQUIRED SUB-SKILL: superpowers:executing-plans, task by task, with
> test-driven-development.

**Goal:** a live ingest cycle removes the `Passage` and `IndexedDocument` rows of a key that stayed out
of the bucket for two consecutive cycles, so Documenti never lists a file the bucket does not hold.

**Architecture:**
- `arcade.remove_document(base_url, database, auth, source_key, timeout_s)` deletes the key's passages,
  then its document row, over `POST /api/v1/command/<db>` with a bound parameter.
- `app.audit_cycle` keeps `_orphaned_previous_cycle` beside `_missing_previous_cycle` and removes
  `orphaned & _orphaned_previous_cycle`, catching and printing a failed removal.

**Tech Stack:** Python 3.12 in `aura-ingest:local`; cocoindex 1.0.25; ArcadeDB 26.10.1; pytest.

**Spec:** `docs/superpowers/specs/2026-10-19-ingest-index-orphans-design.md`.

## Global Constraints

- Every change test-first. The suite runs in the image: `make ingest-test` (needs `memory-up`).
- `ruff check services/ingest` stays clean (`ruff.toml`).
- `app.py` stays under 600 lines.

## Review Focus

1. Only keys that are absent from the bucket in two consecutive cycles are deleted, never a key the
   bucket lists.
2. Passages go before the document row.
3. No failure of the sweep escapes `audit_cycle`.

## Decisions taken while writing this plan

- **`remove_document` lives in `arcade.py`**, beside `indexed_source_keys`, which reads the same rows.
  `app.py` only decides which keys to remove.
- **One key per request pair.** Orphans are rare, and one key per call lets one bad key fail without
  holding back the others. The lab VM has nine.
- **`scripts/ingest_reconcile_e2e.sh` is not extended.** It is 599 lines, and a lost-state phase would
  mean splitting it first. The real scenario is the lab VM, where the state was actually lost.

## File structure

- `services/ingest/arcade.py`: `remove_document`.
- `services/ingest/app.py`: `_orphaned_previous_cycle`, `_remove_orphans`, `audit_cycle`.
- `services/ingest/tests/test_audit_cycle.py`: the sweep's cycle rules.
- `services/ingest/tests/test_arcade_integration.py`: `remove_document` against ArcadeDB.

## Tasks

### Task 1: `remove_document`

- [x] Failing integration test `test_remove_document_leaves_only_the_other_keys_rows`: `ensure_schema`
  on the disposable database, two passages and one document row for each of `gone.md` and `kept.md`,
  then `remove_document(..., "gone.md")`. The test then counts each type's rows by key. A unit test
  pins the order and the bound parameter.
- [x] Implement `remove_document`: `DELETE FROM Passage WHERE source_key = :key`, then
  `DELETE FROM IndexedDocument WHERE source_key = :key`.
- [x] `make ingest-test`. It had no `/eval` mount, so `test_header_pairs.py` and
  `test_row_passages.py` failed at setup since they landed; the mount is added.

### Task 2: the sweep in `audit_cycle`

- [x] Failing tests in `test_audit_cycle.py`:
  - one cycle removes nothing;
  - two cycles remove the key;
  - a key that returns between the two cycles is kept;
  - a failed removal is printed, does not raise, and is tried again on the next cycle;
  - a key in both the bucket and the index is never removed.
- [x] Implement `_orphaned_previous_cycle` and `_remove_orphans`; update the stale comment in
  `test_an_index_ahead_of_the_bucket_reports_nothing` (it still reports no MISSING).
- [x] `ruff check services/ingest && make ingest-test` (268 passed); commit.

### Task 3: acceptance on the lab VM

- [ ] Publish edge (`scripts/publish_images.sh`); the updater applies it.
- [ ] Within two cycles of the new child starting, `IndexedDocument` in `mem_448ddbe1_…` holds only
  `esempio.md`, its passage is still there, and the ingest log carries nine `[audit] ORPHAN … removed`.
- [ ] Documenti in the cockpit lists one document.
- [ ] `docs/superpowers/verification/2026-10-19-ingest-index-orphans.md`.
