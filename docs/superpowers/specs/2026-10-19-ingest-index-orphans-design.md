# The ingest removes index rows whose object is gone

Date: 2026-10-19. The operator's report: the file manager is empty, *"qui però risultano"*, with
Documenti listing nine documents, then *"così non va"*. Asked *"ma perché è successo questo disastro?"*,
the operator chose this fix first and a complete backup second (*"2 e poi 1"*).

## What was measured

On the lab VM `192.168.101.158`, identity `448ddbe1-…`, after `6a1847515` had put back its bucket:
- **The bucket** `aura-448ddbe1-…` holds 0 objects. It was created at 18:52:03 by the restore.
- **The index** `mem_448ddbe1_…` holds 10 `IndexedDocument` rows and 8 `Passage` rows. Nine rows date from
  2026-10-09 to 2026-10-11, and the tenth is `esempio.md`, indexed at 18:56:16 today. Documenti lists
  `IndexedDocument` (`internal/docfields/library.go`), and the file manager lists the bucket.
- **CocoIndex's state** `/state/448ddbe1-…/coco.db` sits on the volume `aura_aura-ingest-state`, created
  2026-10-14 09:10:10. That is the wipe of `2026-10-14-gitlab-image-bridge.md`, whose restore brought back
  Postgres and ArcadeDB but neither Garage nor that volume.

Read from the installed cocoindex 1.0.25 (`connectors/neo4j/_target.py`):
- `_RecordHandler.reconcile` returns no action for a row that is not desired and has no previous
  tracking record (`if not prev_possible_records and not prev_may_be_missing: return None`). A row
  that another state wrote is therefore never deleted.
- `_TableHandler` drops only the table's constraint on teardown, never its rows. `managed_by` decides
  who owns that DDL, not which rows are tracked.
- A tracked row leaves with `MATCH (n:<Label> {pk: $key_0}) DETACH DELETE n` (`_cypher.py`).

From those measurements:
- **The audit looks one way.** `app.audit_cycle` names an object that has no row, and
  `test_an_index_ahead_of_the_bucket_reports_nothing` pinned the converse as "the delete path's
  business". When CocoIndex has lost its state, nothing deletes those rows.
- **No edge reaches either type.** The only edge types are the memory's (`memory*.go`), and none ends on
  `Passage` or `IndexedDocument`. A SQL `DELETE` therefore removes exactly what `DETACH DELETE` would.

## What changes

1. **A live cycle removes an index row whose object stayed out of the bucket for two consecutive
   cycles.** At the end of each cycle, `audit_cycle` already compares the bucket with the index. Keys that
   are indexed but were absent from the bucket listing in this cycle and in the previous one have their
   `Passage` rows deleted first, then their `IndexedDocument` row. Aura logs `[audit] ORPHAN <key> removed`.
2. **The rule is CocoIndex's own, applied to rows it does not remember.** A row stays only while its
   object is in the bucket. The same bucket, prefix and path matcher feed both sides of the comparison
   (`source.expected_keys`).

## Decisions

1. **Two consecutive cycles, as for MISSING.** An object deleted after the walker listed the bucket
   leaves a row that CocoIndex still tracks and will remove itself on the next cycle. Waiting a second
   cycle means the sweep never deletes a row CocoIndex is about to delete, so CocoIndex's tracking never
   points at a row that no longer exists.
2. **Passages first, then the document row.** `IndexedDocument` is what the audit reads. If a delete
   fails between the two statements, the row is still there and the next cycle sweeps again. The reverse
   order would leave passages that no audit can find.
3. **Live mode only.** Appliances run live (`AURA_INGEST_LIVE` defaults to true in `compose.yaml`).
   Catch-up is one pass, so the two-cycle rule has no second cycle there, and the catch-up audit is
   unchanged.
4. **A failed delete is printed and retried.** Like `MISSING`, the sweep never raises out of a cycle,
   since a cycle that fails on one stale row would stop indexing every other file. Aura prints
   `[audit] ORPHAN <key> not removed: <error>` on stderr, and the key is tried again on the next cycle.
5. **Operator fields survive.** The labels an operator set live in `DocumentFields`, keyed by
   `source_key`, and are only projected onto the indexed rows (`document_fields_triggers.go`). The
   projection comes back if the object is uploaded again under the same key.

## Acceptance

1. **Unit tests:**
   - an orphan seen for one cycle is not removed;
   - an orphan seen for two consecutive cycles is removed, passages before document;
   - a key that returns to the bucket between the two cycles is kept;
   - a failed removal is printed, does not raise, and is retried on the next cycle;
   - a key in both the bucket and the index is never touched.
2. **Integration test (real ArcadeDB):** with `Passage` and `IndexedDocument` rows for two keys, removing
   one key leaves only the other key's rows.
3. **Lab VM:** after the updater applies the new `aura-ingest` edge image, the nine rows whose objects are
   gone leave the index within two cycles. Documenti then shows only `esempio.md`, which is still searchable.

## What this does not cover

- **The 26 lost files** stay lost. Their `aura.assets` rows in Postgres still point at missing objects,
  so a chat attachment among them still answers 404 on download. The object-lost marker designed on
  2026-10-09 is still deferred.
- **The backup**, the cause, is the next spec.
- **A catch-up run** still leaves orphans in place (decision 3).
