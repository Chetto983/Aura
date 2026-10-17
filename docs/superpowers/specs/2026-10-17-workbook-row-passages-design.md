# Workbook rows as passages

Proposed on 2026-10-17 at `e8c2937`. The operator asked for this spec ("Scrivi la spec per le
righe Excel come passage"). They then set its constraint: every row passage names its file and
its sheet in its own text. Approved by the operator on 2026-10-17 ("Approvata, scrivi il
piano"); the plan is `docs/superpowers/plans/2026-10-17-workbook-row-passages.md`.

The measurements behind it are in three files under `docs/superpowers/verification/`:
- `2026-10-16-rag-full-version-techniques-spike.md` (the spike);
- `2026-10-16-complex-documents-retrieval-quality.md`;
- `2026-10-17-answers-read-by-a-model.md`.

Every number below comes from those three unless it says otherwise.

Risk tier: **medium.**
- It reverses the 2026-09-09 decision that a spreadsheet gets no passages (`media.py`, prd.md §11).
- It grows every library's passage index.
- It re-runs `_extract` once for every file.
- No Postgres migration. No new ArcadeDB property: `char_start` and `char_end` become nullable in
  the ingest's contract only, and Aura's reader already accepts them absent.

## The problem, measured

**1. A value that lives only in a workbook's rows cannot be searched.**
- A workbook has no passages. The card answers for it, and `document_open` reads it.
- 12 of the complex set's 34 golds are workbooks:
  - `parts-location` and `parts-supplier` ask for a part number that is in no card and no
    passage, and their workbook ranks second;
  - `rota-legend` abstains with zero documents.
- Read by a model, 7 of the 12 workbook questions needed `document_open`. One of them needed a
  second search first.

**2. Rows as passages is the largest retrieval gain measured.** The spike gave each sheet groups
of 15 rows, 2 of them shared with the next group, headed by the sheet's name. Each row was
written as "Riga N: Header: value; …":

| Configuration | complex r@1 | MRR | office r@1 | MRR | MRR, all 54 |
|---|---|---|---|---|---|
| as shipped (b0c264a) | 0.794 | 0.882 | 0.500 | 0.692 | 0.812 |
| rows as passages | 0.912 | 0.951 | 0.700 | 0.794 | 0.893 |
| rows and file name in the title slot | 0.971 | 0.985 | 0.650 | 0.757 | 0.900 |

With rows, every gold of both sets is within the first 8.

**3. What rows broke.**
- **Look-alike workbooks.** The 2026 budget lost places to the 2025 one, because a row's text
  does not say which file it is in.
- **Fourteen near-identical invoices**, each with a row naming its client. They pushed the
  answering files from first to fourth on q12 and q15.
- **The file name in EmbeddingGemma's title slot** fixed the budgets but cost office q13
  (1 → 6), and alone it moved nothing.

**4. A passage that does not say whose it is gets misread.** This is the operator's constraint.
- A table row such as "A119 Arresto … riparo aperto" names no machine; only the document above
  it does.
- On 2 of 22 PDF questions, the asked manual ranked first without its answering passage, and the
  look-alike manual's passage carried the row:
  - read as the model, `fx300-alarm-A119` was answered from the FX200's row;
  - retrieval counts it as a first-place success.
- Workbook rows are the same shape, multiplied: budgets 2025 and 2026, 14 invoices.

**5. Why the 2026-09-09 measurement does not settle this.**
- That measurement chunked eight reference tables as plain text. The Caraglio row landed in a
  2,114-character chunk beside fourteen other municipalities, with no header, so no value said
  which column it was.
- An aggregate scored 0% at every k when asked of passages (`internal/documents/open.go`).
- Rows as key-value passages repeat the header in every row, and leave aggregates to
  `document_open`. Nothing in this spec asks a passage to answer a count or a sum.
- **What stays true:** a 15-row group still embeds as one averaged vector. Exact lookups ride
  on the full-text leg and on identifiers-first (prd.md §11, "Exact identifiers rank first").
  The Caraglio case itself was not re-run. Acceptance 4 runs it if the lab VM still holds those
  tables.

## Goal

A value that lives only in a workbook's row is found by `document_search`. The passage that
returns it names its file, its sheet and its row number.
- The card, its description vector and `document_open` stay as they are.
- Aggregates stay on `document_open`.
- No question set loses beyond one question of run-to-run noise.

## Decisions (proposed)

### 1. filecard serializes the rows

There is one implementation of reading a workbook, and it is filecard's. `filecard.Card` gains
`Rows []Row`, which is not rendered. `aura-filecard -json` emits them beside the card and the
description:

```json
{"card": "…", "description": "…",
 "rows": [{"sheet": "Giacenze", "row": 742, "text": "Codice: RIC-10959; Ubicazione: S6-F13; …"}]}
```

- **`row` is the worksheet's own row number**, the one the operator sees in Excel. filecard
  already tracks it (`worksheetRowPosition`, `addRowAt`). The spike counted body rows from one,
  which no one can find in the file.
- **`text` is "Header: value; Header: value".**
  - The values are the ones LibreOffice cached, as the card reads them.
  - Values are not truncated. The card's 72-rune cut is for a summary, and a lookup needs the
    whole value.
  - Empty cells and empty rows are skipped.
- **Headers are the card's headers.** The probe looks at 8 rows, needs a 70% text share, and
  leaves a banner as preamble.
  - A headerless sheet labels its cells with column letters (`A`, `B`), which is what the
    operator sees.
  - The spike took the first all-text row as header. That heuristic is weaker than the one
    filecard already ships, and the card and its passages must agree on what a column is called.
- **The caps are the card's caps:** 8 sheets, 10,000 rows per sheet, 64 columns, 8 MiB of
  shared strings. A capped sheet's rows stop where its card says it stopped.
- **Row text has its own budget: 16 MiB per workbook** (`maxRowTextBytes`, no env var).
  - The caps alone did not bound it. A cell holds an index into the string table, so one long
    string can be referenced by every cell of every row.
  - Past the budget no further row is kept, in that sheet or the next. A row is checked before
    it is built.
  - The card says where keeping stopped, and only then, so no card of the corpus changes.
- **Formats.**
  - `.xlsx` and `.xlsm` are read directly.
  - `.xls` and `.ods` are read through the LibreOffice normalisation that already feeds their
    card (`extract.prepared`).
  - CSV is out: the ingest extracts it as text today.

Not openpyxl in the ingest. A Python reader would be a second implementation of headers, caps
and cached values to keep in step. `_card` already shells out to `aura-filecard` for exactly that
reason.

### 2. The ingest groups rows into passages, and each one names its file and sheet

`services/ingest/rows.py` groups a sheet's rows the way the spike measured: 15 rows to a group,
2 of them shared with the next.

Every passage opens with one line that names the file and the sheet:

```text
File: budget_consolidato_2026.xlsx — Foglio: Ricavi
Riga 4: Area: Centro; T1: 1972; T2: 1921; T3: 2525; T4: 2296; Totale anno: 8714
```

- **The constraint.**
  - The file line is in the passage's text, so both legs see it. The full-text leg matches
    "2026", and the dense vector carries the name.
  - The title slot was measured on the dense leg alone, and it cost office q13. A name in the
    text is a different lever and has not been measured. Acceptance 3 measures it as arm B.
- **A group longer than `chunk.document_budget()`** is split by the existing `chunk.chunk`. Each
  piece keeps the file-and-sheet line.
- **The embedding prefix is `title: none`,** as for every passage. `RecipeVersion` does not move.

### 3. A sheet the card already shows whole gets no row passages

**This is arm C of acceptance 3,** not yet a decision.
- **The rule.** A sheet whose data rows all appear in its card gets no row passages: it has at
  most `sampleRows` (5) data rows, its card was not cut, and none of its sample values was cut.
  A sheet whose card abbreviated a value at 72 runes, or whose rows the row budget left out,
  has not been shown whole.
- **Why.** Such sheets caused the look-alike losses of problem 3, while their cards already
  answered for them. All the invoices' sheets qualify, and three of the four sheets of each
  budget. A budget's `Costi` has 6 data rows, one more than the card shows.
- **The workbook keeps the reserved place.** A workbook all of whose sheets are like that keeps
  `passage_count == 0`.
- **The cost.** Office q01 and q02 each ask for one invoice, and both rose to first in the
  spike. Whether the invoices' own rows lifted them was not isolated, and this rule takes those
  rows away.

### 4. The locator is the sheet; the citation stays by ordinal

- **`heading_path`** is `[sheet]`.
- **`char_start` and `char_end` are null.**
  - `PassageLocator` anchors in the plain text the extractor produced, and a workbook has none.
  - A character span counted inside the passage would claim an anchor that does not exist.
- **Aura's reader already accepts the span absent** (`optionalCharacterSpan`). `citationLocator`
  then falls back to `ordinal=N`, and the passage's own text carries the real row numbers.
- **The ingest's `Passage` contract** makes the two fields `int | None`.

### 5. `passage_count` counts row passages; the reserved place keeps its rule

- **A workbook with row passages leaves the reserved place** (`passage_count == 0`) and competes
  on its passages. That is the configuration the spike measured, since it set `passage_count`.
- **`IndexedDocument.normalized_text_sha256` stays empty** for a workbook. Its rows are not the
  extractor's text, so retrieval's same-text collapse is unaffected.

### 6. What does not change

- The card, its description vector, `document_open`, the identifiers-first rule.
- **`media.index_text` still returns "" for a spreadsheet.**
  - Rows arrive through filecard, not through text extraction.
  - `test_index_text_leaves_spreadsheets_to_document_open` stays true.
  - Its comment and `media.py`'s routing comment are amended to say where rows come from.

### 7. The PRD is amended first

One commit before the code amends prd.md §11. It records:
- the spike's numbers;
- why the 2026-09-09 decision is narrowed rather than contradicted;
- the constraint;
- what is not proved.

The reserved-place paragraph gets one sentence: a workbook with row passages no longer takes the
place.

### 8. The upgrade's cost, and a prerequisite

**What the upgrade costs.**
- Rows are read from the converted file, so they are produced inside `_extract`, whose body
  changes. `_extract` re-runs once for every file of every library.
- No billed call repeats: they are memoized on content since 2026-10-16. Passages whose text is
  unchanged are not re-embedded.
- **The new cost is one embedding per row passage.**
  - The spike embedded 962 row passages in about 10 minutes on this host's CPU, about 96 a
    minute.
  - The 5,000-row ledger alone made 385 passages, about 4 minutes.
  - The cap allows 8 sheets of 10,000 rows, about 6,150 passages in one workbook, or an hour at
    that rate.
- **No new env var.** The bounds are filecard's.

**The prerequisite.** The code ingest of 2026-10-16 failed on this host:
- CocoIndex sent concurrent requests to a one-slot embedder;
- 5,918 of them waited past the ingest's 120 s timeout;
- the audit found 20 of 6,001 objects indexed.

A large workbook produces the same burst. So the plan's first task ingests the 5,000-row ledger
through the real pipeline on CPU:
- **if it times out,** that defect gets its own spec and is fixed first;
- **a lower row cap would not fix it,** only hide it.

## Shape

| File | Change |
|---|---|
| `internal/documents/filecard/rows.go` (new) | `Row`; collecting rows with their worksheet position once the header is resolved; the whole-sheet test of decision 3 |
| `internal/documents/filecard/table.go` | the probe's post-header rows and every consumed row also become `Row`s |
| `internal/documents/filecard/xlsx.go` | no reading change; the positions it already passes reach `rows.go` |
| `internal/documents/filecard/card.go` | `Card.Rows`, not rendered |
| `cmd/aura-filecard/main.go`, `main_test.go` | `-json` emits `rows` |
| `services/ingest/rows.py` (new) | grouping, the file-and-sheet line, the split over budget |
| `services/ingest/app.py` | `Extracted.rows`; `_card` parses them; `index_object` declares row passages for a workbook; `Passage.char_start`/`char_end` optional |
| `services/ingest/media.py` | the routing comment |
| `services/ingest/tests/test_rows.py` (new), `test_card_*.py` | the Python acceptance |
| `prd.md` | §11, decision 7 |

**File sizes, against the 600-line limit:**
- `table.go` is at 452 lines, which is why the row collection lives in its own file.
- `app.py` is at 498, which is why the grouping lives in `rows.py`.
- No file crosses 600.

### New env vars

None.

## Acceptance

1. **Unit, Go** (`internal/documents/filecard`, `cmd/aura-filecard`). Rows from the complex
   corpus's workbooks. Each case checks a specific value:
   - **headers:** the header under a banner; column letters for a headerless sheet (`Riordino`);
     the merged two-row header of `piano_manutenzione_2026.xlsx`;
   - **values:** cached formula values, the VLOOKUP's supplier and the IF's `SI`;
   - **layout:** real row numbers after a banner; empty rows skipped; the row cap; the
     whole-sheet test;
   - **output:** the `-json` shape.

   The package stays at its coverage policy (`scripts/coverage_package_policy.json`).
2. **Unit, Python.** The cases below, with `ruff` clean:
   - groups of 15 with 2 shared;
   - the file-and-sheet line on every passage, including the pieces of a split group;
   - no passage for a sheet under the whole-sheet rule, in arm C;
   - null spans;
   - `passage_count`.
3. **Through the real ingest,** on the complex and office identities here and on the code sets
   in `cb-arcade` on the workstation. Four configurations, `TestCodingAgentQuestions` on each:

   | Arm | What it is | What it must show |
   |---|---|---|
   | as shipped | today | the baseline |
   | A | rows with the sheet line only | the spike reproduced through the ingest, within one question per set. If not, stop and explain |
   | B | A plus the file name in the line | the operator's constraint |
   | C | B plus decision 3 | the look-alike fix |

   - **The arm that ships** beats as shipped on MRR on both complex and office, and loses nothing
     beyond one-question noise on the English, Italian and find-by-name code sets. If no arm
     does, nothing ships (BASELINE OR DROP).
   - **Reported one by one:** q12, q13, q15, the four budget questions, and `rota-legend`.
   - **Answers read** with the harness of 2026-10-17, for the shipping arm:
     - the office qrels first get expected answers, so that half is blind;
     - no more wrong answers than today (1 of 34 complex, 0 of 20 office);
     - the count of answers that needed `document_open` is reported.
   - **Cost:** passages added per identity, embedding time, and the ArcadeDB database's size
     before and after.
4. **The lab VM, through the updater.** One ingest pass over its library, with its time, the
   passages added, and the Caraglio lookup if those tables are still there. Recorded in a dated
   file in `docs/superpowers/verification/`.

## Open questions

1. **Does `TableSchema.from_class` accept `int | None`?** Read the installed CocoIndex before the
   plan is written. The rule is documentation first.
2. **A row-range locator** (`sheet=Giacenze;rows=740-754`) would let `document_open` go straight
   to the rows.
   - It needs two ArcadeDB properties and `PassageLocator` fields.
   - It is not in this spec unless the operator wants it.
3. **The labels** "File", "Foglio" and "Riga" mix the card's English with the spike's Italian.
   Only the Italian ones were measured, on an Italian corpus.
4. **Are the 2026-09-09 reference tables** (comuni, CAP) still on the lab VM? If so, Caraglio is
   part of acceptance 4.

## Later levers, not in this spec

- **The file name in every PDF and text passage.**
  - The A119 and chain-lubrication misses are PDF look-alikes, and this spec does not touch
    them.
  - Changing every passage's text re-embeds every passage of every library, so it needs its own
    measurement.
- **The title slot** (consolidation R2) sits beside rows: MRR +0.007, at office's cost. It needs
  a `RecipeVersion` bump and the code sets.
- **Answering aggregates from rows.** `document_open` with pandas already does this, and a
  passage cannot.
- **A cross-encoder** is ruled out: spike 070 measured 23 s per 15 documents on CPU, and the
  appliance has no GPU.

## What this spec does not prove

- **The gain comes from a spike over copies of the indexes,** not from the ingest: 54 synthetic
  questions, one run per configuration.
- **The file-and-sheet line (arm B) and the whole-sheet rule (arm C) are unmeasured.**
- **The code sets are unmeasured,** as on 2026-10-16.
- **The appliance's embedding rate and storage are unmeasured,** and so is whether the
  ingest's burst defect hits there.
- **A 15-row group embeds as one averaged vector.** An exact lookup depends on the full-text leg
  and on identifiers-first.
- **Rows do not answer aggregates.** The retrieval gold counts a lookup and an aggregate the same.
