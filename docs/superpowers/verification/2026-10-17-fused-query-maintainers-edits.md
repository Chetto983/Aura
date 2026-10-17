# The fused query with the maintainers' edits, measured

Date: 2026-10-17. Spec `docs/superpowers/specs/2026-10-17-fused-query-filter-after-rerank-design.md`,
queued after the workbook-row slice and measured on the operator's request ("Fai le misure sulla
query di Franchini"). No code changed: the existing `TestFusedQueryVariants` already carries both
arms.
- **Arm P** is the variant "post-filter". The dense filter leaves `vector.neighbors`, and
  `embed_space = :space` moves to the outer clause after the rerank.
- **Arm R** is Roberto Franchini's statement: `LIMIT :fetch` on the full-text leg, `limit:
  :candidates` in `vector.fuse`, and no filter in `vector.neighbors`. The spec adds the space
  filter after the rerank. On an index with one embedding space that filter drops nothing, so the
  variant "maintainers' three edits" is arm R row for row.

**Verdict:**
- **Both arms keep every judged answer.**
  - On the 108 questions with gold (office 20, complex 34, code English 24, Italian 24,
    find-by-name 6), P and R give exactly the shipped statement's r@1, r@3, r@10 and MRR on every
    set.
  - P returns identical answers on all 108. R changes one Italian code answer with no metric
    moving.
- **Both cut most of the latency at Aura's pin.** On ArcadeDB 26.10.1 over 7,798 real passages:
  - shipped 44.3 ms;
  - P 15.3 ms;
  - R 12.3 ms.
- **R is the faster arm and changes 2 of 16 ungolded answers there.** By the spec's rule R
  qualifies: as good as P on every gold set, and faster on the largest index measured. The binding
  latency is still the operator's full code index of 11,771 passages.

## Setup

- **Engine.** ArcadeDB 26.10.1, Aura's pin, with the bench's memory settings. EmbeddingGemma-300M
  Q8_0 on CPU (llama.cpp `server-v0.5.0`).
- **Indexes:**
  - the 7,798-passage library of 2026-10-16: real text, real embeddings, the 16 questions of
    `docs/document-retrieval-order-pilot.json`. Its gold documents are not in this library, so
    recall reads 0 and only "identical" and latency count;
  - the office and complex identities after the workbook-row upgrade (arm B), which is master's
    state today;
  - the 62-file code subset of `2026-10-17-workbook-row-passages.md`. It has the gold files of
    the coding questions plus the repository's workbooks.
- **Latency.** One run per question per variant, median over the questions. The large library ran
  twice, and the second run is the table; the first gave shipped 47.6, P 19.0 and R 13.7 ms.

## Latency and answers, 7,798 passages

| Variant | median | identical to shipped |
|---|---|---|
| shipped | 44.3 ms | 16/16 |
| no filter | 15.5 ms | 16/16 |
| **post-filter (P)** | **15.3 ms** | **16/16** |
| full-text `LIMIT :fetch` | 38.6 ms | 14/16 |
| no filter, rerank top 80 | 13.9 ms | 16/16 |
| **maintainers' three edits (R)** | **12.3 ms** | **14/16** |
| filter from the index | 26.2 ms | 16/16 |

- **The filter is most of the cost.** Removing it, before or after the rerank, takes the query
  from 44 to 15 ms.
- **R's two changed answers come with the full-text `LIMIT`.**
  - "full-text LIMIT :fetch" alone changes the same 2 of 16.
  - Fuse's `limit` alone ("no filter, rerank top 80") changes none.
  - So the reasoning in the spec, that fuse's `limit` makes the full-text `LIMIT` inert, does not
    hold on this library.
  - Whether those two answers got better or worse cannot be judged here: no gold.

## Gold sets

| Set | Questions | shipped r@1 / r@3 / r@10 / MRR | P | R | P identical | R identical |
|---|---|---|---|---|---|---|
| office | 20 | 0.700 / 0.850 / 1.000 / 0.804 | same | same | 20/20 | 20/20 |
| complex | 34 | 0.882 / 1.000 / 1.000 / 0.941 | same | same | 34/34 | 34/34 |
| code, English | 24 | 0.917 / 0.958 / 0.958 / 0.938 | same | same | 24/24 | 24/24 |
| code, Italian | 24 | 0.792 / 0.875 / 0.917 / 0.840 | same | same | 24/24 | 23/24 |
| code, find-by-name | 6 | 0.833 / 1.000 / 1.000 / 0.889 | same | same | 6/6 | 6/6 |

- **These are passage-level metrics** (`TestFusedQueryVariants`), not the document ranking
  `TestCodingAgentQuestions` reports. Both P and R change only the candidate pool of one
  statement, which is what this bench isolates.
- **Latencies on these small indexes are 8 to 15 ms for every variant.** The filter's cost scales
  with the passages of the space, so these sets cannot rank the arms by speed.

## What this does not prove

- **The full code index.** Its 11,771 passages on the operator's workstation are the spec's
  binding latency (acceptance 1). The largest index here is 7,798 passages.
- **Scoped searches.** Both arms apply only to an unscoped search (spec decision 2), and nothing
  was measured with a caller's document scope. Acceptance 3 stays open.
- **Two embedding spaces at once.** On an index with one space, P and R drop nothing after the
  rerank. Whether a search returns fewer results during a model change is acceptance 4, not run.
- **ArcadeDB 26.11.1.** PR #10181, which stops the double record loads, is not released. It may
  change the gap between the statements.
- **R's two ungolded answer changes.** They are neither a gain nor a loss until a set with gold
  shows them.
- **One run per variant on the gold sets,** on one host.
