# Document fields: where the filter can live, measured before building it

Date: 2026-10-18. Task 0 of `docs/superpowers/plans/2026-10-18-document-fields.md`. Run on the
operator's Windows dev PC (i7-9700K, RTX 3060, ArcadeDB 26.10.1, EmbeddingGemma-300M Q8_0 on
CUDA), first against a disposable ArcadeDB, then against the dev stack built from this checkout.

**Verdict:**
- **The plan's filter, `source_key IN (SELECT source_key FROM DocumentFields WHERE …)`, fails the
  2× gate.** Its cost grows with the documents it admits: 0.5× the shipped query at 12 documents,
  2.9× at 325, 5.5× at 735 and 14.6× at 4,027.
- **Two shapes pass at every width, and tie:** the fields copied onto the `Passage` rows the query
  scans (Paperless's shape: the index carries the fields), and the admitted keys computed elsewhere
  and bound as `source_key IN :keys`. They answer identically to the subquery, 16/16 at each width.
- **ArcadeDB triggers keep the copy current with no job of ours.** On a real CocoIndex ingest a
  SQL `AFTER UPDATE` trigger stamped 407 of 407 passages at no measurable ingest cost, and a
  cascade trigger carried an operator edit to the passages.
- **Decision, the operator's (2026-10-18):** document fields live in ArcadeDB, in the identity's
  database. `DocumentFields` is the truth, triggers keep `df_*` on `Passage`, and the filter is a
  `Passage` property. No Postgres tables for document fields and no Go-side mirror or stamp job.
- **Redis brings nothing here:** `HSET` cannot update by key, and bulk SQL in one script is as fast.

## Setup

**Disposable library** (the plan named `scratchpad/adb2610`, which lived in the cloud session that
wrote it; this is the local stand-in):
- 3,717 tracked repository files (2,595 `internal/*.go`, 420 `cmd/*.go`, 702 `*.md`), 8,485
  passages, ingested through the production indexing path with the real embedder;
- `DocumentFields` with one row per document and `type = 'bench-type'` on every fifth sorted key.

**Dev stack library** (identity `15d43203-…`, uploaded with `aura docs ingest`, indexed by the
stack's `aura-ingest`):

| Source | Documents | Licence | Labels taken from |
|---|---|---|---|
| `dossier-legal/italian-legal-corpus`, validation shard, stratified sample | 1,327 | CC-BY-4.0 | `doc_type`, `authority`, `date` |
| `mii-llm/gazzetta-ufficiale`, one shard, half from the 40 most frequent issuers | 1,500 | MIT | `rubrica`, issuer, heading date, series |
| `chenghao/sec-material-contracts`, one shard | 600 | CC-BY-SA-4.0 | company, filing date, form |
| `drew-ipp/invoice-extraction-benchmark`, PDF renders | 113 | CC-BY-4.0 | vendor, invoice date, document type |
| `spikes/document-routing-scale` generator, office, complex and pipeline fixtures | 512 | repo | invoice header pairs; office question-set library |

That is 4,045 documents and 7,271 passages, with 4,027 documents labelled and 1,348 distinct
counterparties. The 7 legacy and OpenDocument pipeline fixtures were refused at upload, a bug fixed
in `5d19f5240`. One scanned PDF waited for a vision route until the primary model became
`gemma4:31b-cloud`, and was then read and indexed.

**Bench:** `TestFusedQueryVariants` (`-tags retrieval_eval`) over the 16 questions of
`docs/document-retrieval-order-pilot.json`, one run per question per variant, median reported. The
gold documents are not in these libraries, so only latency and identical answers count. The added
variants were a scratch edit, not committed; their definitions are below.

## The filter shapes

Disposable library, one fifth of the documents admitted (744 keys):

| Statement | Run 1 | Run 3 |
|---|---|---|
| shipped | 79.4 ms | 66.8 ms |
| `DocumentFields` subquery | 768.9 ms | 726.9 ms |
| `Passage` property | – | 51.3 ms |
| Go keys as a parameter | – | 53.6 ms |

The three filtered forms returned identical answers on 16/16.

Dev stack, real labels:

| Filter | Admits | Subquery | `Passage` property | Go keys | Identical |
|---|---|---|---|---|---|
| shipped (run 1 / run 2) | all | 154.7 / 163.2 | | | |
| narrow, `counterparty = MINISTERO DEGLI AFFARI ESTERI` | 12 | 77.9 / 77.4 | 61.3 / 59.3 | 60.6 / 59.2 | 16/16 |
| medium, `type = ESTRATTI, SUNTI E COMUNICATI` | 325 | 452.0 / 442.8 | 74.4 / 75.7 | 74.4 / 71.6 | 16/16 |
| broad, `tags CONTAINS Serie Generale` | 735 | 847.2 / 855.9 | 90.2 / 92.5 | 92.7 / 92.4 | 16/16 |
| all labelled documents | 4,027 | – / 2,381.1 | – / 168.2 | – / 167.8 | 16/16 |

All figures are in ms. No index was declared on the `df_*` properties.

## Triggers keep the copy on CocoIndex's rows

Scratch databases on the stack's ArcadeDB, dropped after. The office corpus (23 files) was ingested
by the production path through `libdriver.py`.

**CocoIndex's write.** It is `MERGE (n:Passage {passage_key: $key_0}) SET n += $props` (cocoindex
1.0.25 `connectors/neo4j/_cypher.py:95-104`):
- the node is created with its key alone, so an `AFTER CREATE` trigger sees no `source_key`;
- the `SET` that follows is an update, and `+=` merges properties rather than replacing them;
- delete and rename go through `DETACH DELETE`.

**The triggers that work:**

```sql
CREATE TRIGGER passage_stamp AFTER UPDATE ON TYPE Passage EXECUTE SQL
 'UPDATE Passage SET df_type = first((SELECT type FROM DocumentFields WHERE source_key = $record.source_key)).type,
  df_counterparty = …, df_date = …, df_tags = …, df_stamped = true
  WHERE @rid = $record.@rid AND df_stamped IS NULL AND source_key IS NOT NULL';
CREATE TRIGGER fields_cascade_update AFTER UPDATE ON TYPE DocumentFields EXECUTE SQL
 'UPDATE Passage SET df_type = $record.type, …, df_stamped = true WHERE source_key = $record.source_key';
CREATE TRIGGER fields_cascade_create AFTER CREATE ON TYPE DocumentFields EXECUTE SQL '…the same…';
```

The sentinel stops the recursion: the stamp's own `UPDATE` fires the trigger once more and matches
no row.

| Step | Result |
|---|---|
| ingest without triggers | 407 passages, 14.8 s |
| ingest with triggers | 407 passages, **407 stamped**, 14.7 s |
| operator edit on `DocumentFields` | the document's passages carry the new counterparty |
| new bytes under the same key | the rewritten passages stay stamped |
| rename, with the row moved to the new key first | the new key's passages are stamped; the old ones are gone |

**On 26.10.1 the trigger page describes the latest build, not ours.** Each of these was measured:
- a `BEFORE CREATE` SQL trigger cannot `UPDATE $record`: "rid is null";
- in a JavaScript trigger `record.x = …` is not persisted, while `record.set("x", …)` is;
- query parameters given as a JS object are not bound;
- `database.query("sql", stmt, value)` fails as an ambiguous overload.

**Both the JavaScript and the SQL triggers fire on openCypher writes.** A failing `BEFORE` trigger
aborts the write: with the first, wrong, JavaScript body CocoIndex wrote zero passages and its pass
still exited 0.

## Redis

The `redis` query language works over HTTP without the Redis plugin.

| | Result |
|---|---|
| 4,027 `DocumentFields` records, one SQL `UPSERT` per HTTP request | 32.8 s |
| the same, one `sqlscript` of `INSERT … CONTENT` in blocks of 500 | 0.24–0.27 s |
| the same, `HSET` in blocks of 500 | 0.24 s; triggers fire; a failing block rolls back whole |
| `HSET` on a key that exists | refused, "Duplicated key": `HSET` creates and does not update by key |
| a page of 50 records by key | `HMGET` 2.9 ms, SQL `IN` 3.4 ms |

## Variant definitions

Appended to `fusedQueryVariants` in the scratch edit, for each width `{mirror predicate, Passage
predicate, value}`:

```go
fusedStatement(where+" AND source_key IN (SELECT source_key FROM DocumentFields WHERE "+mirror+")", FusionRRF, limit)
fusedStatement(where+" AND "+passage, FusionRRF, limit)            // df_counterparty = :v, df_type = :v, df_tags CONTAINS :v
fusedStatement(where+" AND source_key IN :keys", FusionRRF, limit) // keys read from DocumentFields
```

## What this does not prove

- **Search latency is all it measures here, not answer quality.** The pilot's gold documents are
  not in these libraries.
- **The library is 4,045 documents.** A personal library several times larger is not measured, and
  neither is an index on `df_*`.
- **The trigger chain was measured on 23 files through `libdriver.py`, not through the live
  `aura-ingest` loop.** Neither concurrent writers nor a backfill of an existing library were
  measured. A backfill is expected to be `UPDATE DocumentFields SET source_key = source_key`, which
  fires the cascade, but that is not measured.
- **All of it ran on one machine,** on the 26.10.1 pin. The trigger behaviours above may differ on
  a later build.
- **The cockpit's facet counts and bulk edits on ArcadeDB SQL are not measured yet.** They are Task
  8's measurement.
