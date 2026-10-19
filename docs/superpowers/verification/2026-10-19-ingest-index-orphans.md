# Index rows whose object is gone leave the index: verification

Date: 2026-10-19. Spec `docs/superpowers/specs/2026-10-19-ingest-index-orphans-design.md`, fix
`4c552b994`, and the ingest's host mapping `af5f02281`. Both were published as edge and applied on
the lab VM `192.168.101.158` by its updater.

**Verdict:** the nine documents Documenti listed without a file left the index on the first live
cycles of the new ingest, and every real file stayed. Once the host mapping reached the ingest, the
image upload that had failed was indexed. The index now holds one row per object in the bucket.

## Before

Identity `448ddbe1-…`, after `6a1847515` put back its bucket:
- **The bucket** held 0 objects. The **index** held 10 `IndexedDocument` rows: nine dated
  2026-10-09 to 2026-10-11, restored from the 2026-10-13 backup, plus `esempio.md` (18:56 today).
- **CocoIndex's state** `/state/448ddbe1-…/coco.db` sat on a volume created 2026-10-14 09:10, the
  wipe, so no recorded write named the nine rows.
- At 19:20 the operator uploaded six files from the web into `prove-craft/`: a 47-page PDF, a docx,
  an xlsx, a pptx, a JPEG and an MP4. The bucket then held seven objects.

## After the new ingest (applied 20:06:30)

- **The sweep:** nine lines `[audit] ORPHAN <key> removed`, one per restored row: seven under
  `chat/`, plus `media/…ogg`, and the three sandbox reports among the chat keys.
- **What stayed:** `esempio.md` and every upload except the image, with their passages: the PDF 26,
  the xlsx 71, the docx 2, the pptx 1 and the MP4 0. Nothing the bucket listed was removed.
- **Still missing:** `prove-craft/Colosseo.jpg` failed at every cycle with
  `media indexer exited 1: Post "http://aura-ocr-vl:8082/…": lookup aura-ocr-vl … server misbehaving`.
  The audit named it `MISSING` from the second cycle on.

## The image, and the host mapping

- **Why it fell back:** the media indexer asks the chat model whether it reads images. On this VM
  that model is `gemma4:31b-cloud` on Ollama at `host.docker.internal:11434`. From `aura-ingest` the
  name did not resolve (`Name or service not known`), while `aura` mapped it with `extra_hosts`. The
  probe therefore failed, and the indexer fell back to `aura-ocr-vl`, which the VM does not run.
  `scripts/ingest_media_e2e.sh` had passed only because it adds the same host itself.
- **When the fix arrived:** the mapping travels in the payload of the aura image. The operator's
  update at 20:04 brought the new ingest image, but the payload stayed as it was: the publisher moves
  `aura:edge` last, and it had not moved yet. The next update put the payload in place, and
  `aura-ingest` was recreated at 20:19:16 with `ExtraHosts=[host.docker.internal:host-gateway]`.
- **The result:** `Colosseo.jpg` was indexed after that recreate, with one passage, which is the
  model's description. The index then held 7 rows for the bucket's 7 objects.

## Tests

- Unit: the sweep's cycle rules (`test_audit_cycle.py`) and the order of its two deletes
  (`test_audit.py`).
- Integration, against a real ArcadeDB: rows written in CocoIndex's own `MERGE` shape for two keys;
  removing one key left only the other key's rows (`test_arcade_integration.py`).
- The ingest suite inside `aura-ingest:local`: 268 passed. `make ingest-test` had lacked the `/eval`
  mount since 2026-10-17 (`39b20735d`). Ruff was clean, and so were the pre-commit and pre-push hooks.

## Not shown here

- **A catch-up run** does not sweep (spec decision 3), and none ran here.
- **The cost of a re-index after a lost `coco.db`** was not measured, since the bucket was empty
  when the state was lost.
- **Postgres:** the 26 `aura.assets` rows of the lost files are still there, and their downloads
  still answer 404.
- **The xlsx after GridCraft:** once GridCraft had saved the xlsx, it re-indexed with 55 passages
  instead of 71. The 11,216 cell values and their number formats are unchanged; the cause was not
  looked for.
- **CI:** none ran. GitHub Actions is off while the account is flagged, and GitLab's only runner is
  offline.
