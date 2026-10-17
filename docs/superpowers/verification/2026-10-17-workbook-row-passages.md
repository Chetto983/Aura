# Workbook rows as passages, through the real ingest

Date: 2026-10-17. Plan `docs/superpowers/plans/2026-10-17-workbook-row-passages.md`, spec
`docs/superpowers/specs/2026-10-17-workbook-row-passages-design.md`. It continues
`2026-10-16-rag-full-version-techniques-spike.md`, which measured the same idea over copies of the
indexes.

**Verdict:**
- **Arm B ships.** Workbook rows become passages in groups of 15 sharing 2. Each passage opens
  with `File: <name> — Foglio: <sheet>`.
  - It beats as-shipped and both other arms on both sets: complex MRR 0.882 → 0.985, office
    0.692 → 0.804.
  - It loses nothing on a code subset.
- **Arm C is reverted.** Dropping the rows of sheets the card shows whole lost to B on both sets.
- **The run needed the embedding bound of `2026-10-17-embedding-burst.md`.** Without it, the first
  complex upgrade lost two large workbooks to timeouts and still exited 0.
- **The binding code-set gate is still the operator's workstation run.** Here the code sets ran
  on a 62-file subset, because the operator capped the run at 20 minutes.

## Setup

- **Corpora and questions.** The same as on 2026-10-16:
  - office: 22 files, 20 questions in `qrels.json`;
  - complex: 33 files, 34 questions in `complex_qrels.json`.
- **Stack.** ArcadeDB 26.10.1, EmbeddingGemma-300M Q8_0 on a 4-core CPU (llama.cpp
  `server-v0.5.0`, one slot), Garage v2.4.1, CocoIndex 1.0.25.
- **The upgrade path.** Every arm is one catch-up pass of the real ingest
  (`python -m ingest.app`) over fresh copies of the as-shipped index and its CocoIndex state. The
  image is built from the arm's commit.
- **Retrieval.** `TestCodingAgentQuestions`, production arm at limit 8.

| Arm | Commit | What it adds |
|---|---|---|
| A | 5d0a81a (with the bound, c1b30d5) | row passages headed `Foglio: <sheet>` |
| B | 51d19fc | the header becomes `File: <name> — Foglio: <sheet>` |
| C | 3969d7c, reverted in 1bab6f4 and 797ea8d | no rows for a sheet its card shows whole |

## Retrieval

| Set | Arm | r@1 | r@3 | r@8 | MRR |
|---|---|---|---|---|---|
| complex | as shipped | 0.794 | 0.971 | 0.971 | 0.882 |
| complex | A | 0.941 | 1.000 | 1.000 | 0.971 |
| complex | **B** | **0.971** | **1.000** | **1.000** | **0.985** |
| complex | C | 0.971 | 0.971 | 1.000 | 0.978 |
| office | as shipped | 0.500 | 0.850 | 0.950 | 0.692 |
| office | A | 0.600 | 0.850 | 1.000 | 0.746 |
| office | **B** | **0.700** | **0.850** | **1.000** | **0.804** |
| office | C | 0.550 | 0.900 | 1.000 | 0.739 |

The spike's "rows as passages" was complex 0.912/0.951 and office 0.700/0.794. Arm A reproduced
it within one question on complex. Office stayed two short until B (see below).

**What rows fixed.**
- **The store's part numbers:** `parts-location` and `parts-supplier` moved 2 → 1. The
  identifiers-first rule now finds `RIC-10959` in a row passage.
- **`rota-legend`** abstained with zero documents and is now first.
- **The plan legend** is first.

**What the file name added (B over A).**
- **Office q01 and q02 moved to first.** filecard leaves an invoice's header rows (number, client,
  date) out of its rows, because it reads them as a banner. The invoice number is in the file
  name, so B recovers it.
- **The budgets:** three of the four questions are first. `budget-centro-2026` is second.
- **q12, q13 and q15 stay first.** The spike's title-slot variant had lost q13.

**Why C loses.** With no rows for the invoices' sheets, the invoices fall back to their cards:
q01, q02 and q15 go 1 → 2. On complex, `budget-ebitda` falls to fourth.

**What stays.** Office q18 ("fatture emesse a marzo 2026") is fourth. It is a question over every
document, which no ranking answers (consolidation D4/D5).

## Code sets, indicative

The operator capped this at 20 minutes, against about 7 h for Aura's 6,027 sources on this CPU.
- **The subset.** 62 files: the 36 gold files of the English, Italian and find-by-name coding
  questions, plus the repository's 26 workbooks, which are the competitors rows add.
- **The run.** One ingest with arm B: 1,068 s, 696 chunks, no timeout, audit 62 of 62 with 0
  errors.
- **The as-shipped counterfactual.** The same index with its 537 workbook row passages deleted
  and `passage_count` set to 0 on the 26 workbooks. That is exactly the as-shipped state, since
  cards and the other passages are identical.

| Set | as shipped | B |
|---|---|---|
| English (24) | 0.875 / 0.958 / 0.958 / 0.917 | 0.917 / 0.958 / 0.958 / 0.938 |
| Italian (24) | 0.792 / 0.833 / 0.875 / 0.812 | 0.792 / 0.875 / 0.917 / 0.833 |
| find-by-name (6) | 1.000 | 1.000 |

The columns are r@1 / r@3 / r@8 / MRR. Three questions moved:
- `abstention` 2 → 1;
- `lucene-escape` from not returned to 1;
- `chunk-ceiling` 1 → 2, its two gold files swapping places.

No workbook reached the top of a moved question.

## Cost

| | office | complex |
|---|---|---|
| row passages added (A) | 403 | 534 |
| upgrade wall time, B | 550 s | 752 s |
| database, as shipped → B | 11,984 → 18,156 KB | 12,240 → 20,980 KB |

- **Arm A's office upgrade re-ran `_extract` once per file and re-embedded no PDF or text
  passage.** This was checked by record id and embedding hash. B and C were not checked that way;
  they embedded as many chunks as they added rows.
- **Every upgrade had no timeout and 0 errors,** once the bound was in.
- **The 5,000-row ledger alone** made 385 passages in 410 s and 6.3 MB.

## Answers read blind

A fresh agent played Aura's model on arm B's responses.
- **What it could read:** only the saved `document_search` responses, and the original files as
  `document_open`. It could not read the gold.
- **Expected answers.** The office qrels gained them first, read from the files, so both halves
  are blind this time.
- **No second search was available.**

| | 2026-10-17, as shipped (`answers-read-by-a-model.md`) | arm B, blind |
|---|---|---|
| correct | 33 of 34 complex, 20 of 20 office (office not blind) | **34 of 34, 20 of 20** |
| answered from the search response | 40 of 54 (passage or card) | **49 of 54** |
| needed `document_open` | 13 | 5, all complex |
| needed a second search | 3 (rota-legend, q04, q18) | 0 |

- **The five opens.**
  - Two are aggregates, as designed: `parts-below-min` and `rota-nights`.
  - One is a lead time that lives on another sheet than the one returned (`parts-lead-time`).
  - Two are the PDF look-alikes: `fx300-alarm-A119` and `fx300-chains`. The asked manual's
    passage lacked the row, and the FX200's held it. The reader attributed by document and
    opened the FX300 instead of answering from the wrong manual. The as-shipped reader of the
    same day did not.
- **Misleading spots it reported.** These are known filecard header defects, now visible inside
  passages:
  - the rota's header is its first employee row;
  - the plan legend's first entry is read as the header;
  - the registry summary's quarter rows are shifted by one column.

  It read past each one, but a less careful reader might not.

## What this does not prove

- **The full code index.** The code sets ran on 62 documents, where finding the right file is far
  easier than among 6,000. The comparison measures only whether rows take places from code files.
- **Arm B on the appliance.** Its iGPU embedding rate and its storage growth were not measured.
- **filecard's header detection.**
  - It still drops an invoice's header rows from its rows, and B recovers only the part that the
    file name carries.
  - It still takes a rota's first employee as its header.
- **Aggregates.** Rows do not answer them, and the retrieval gold counts a lookup and an
  aggregate the same.
- **Generality.** 54 questions written by the corpora's generator and the office fixture's
  author, one run per arm.
