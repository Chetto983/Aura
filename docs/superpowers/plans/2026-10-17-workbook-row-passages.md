# Workbook row passages implementation plan

**Status (2026-10-17):** DONE except Task 10. Task 9: final review "with fixes", no Critical; one fix wave
(e3801ae..b2696c1) and its scoped re-review cleared every finding; merged to master on the operator's
instruction.
- **Landed:** Tasks 0 to 8.
  - PRD: e9d1afb.
  - filecard rows: 93589f7, 2793a47 and ee075a0.
  - Arm A: 5d0a81a.
  - Arm B: 51d19fc.
  - Arm C: 3969d7c, reverted in 1bab6f4 with its Go side removed in 797ea8d.
- **The embedding bound.** Task 5 stopped on embedding timeouts. The fix landed through
  `2026-10-17-embedding-burst.md`, then the arms ran.
- **Measured:**
  - B ships: complex MRR 0.882 → 0.985, office 0.692 → 0.804;
  - the code subset is unchanged or better;
  - answers read blind: 54 of 54.
  - The record is `docs/superpowers/verification/2026-10-17-workbook-row-passages.md`.
- **Arm C lost and is gone.** Every mention below of `ShownWhole`, `shown_whole` and decision 3
  describes what was measured and then removed.
- **Open:**
  - the operator's full code-index gate on the workstation;
  - Task 9's final review;
  - Task 10 (publish and the lab VM).

> **For agentic workers:** Use superpowers:executing-plans with test-driven-development. The
> operator's convention is to work in D:/Aura on master, with no worktree. The acceptance runs
> of Tasks 4 to 8 need the stack described under "Setup" in
> `docs/superpowers/verification/2026-10-16-complex-documents-retrieval-quality.md`.

**Goal:** A value that lives only in a workbook's row is found by `document_search`. The passage
that returns it names its file, its sheet and its row number. Cards, `document_open` and
aggregates stay as they are.

**Architecture:**
- **filecard** already reads every workbook for its card. It now also keeps each data row as
  "Header: value; …", with its worksheet row number, under the card's own headers and caps.
- **`aura-filecard -json`** emits those rows beside the card and the description.
- **The ingest** reads them in `_extract` from the same converted file. `rows.py` groups them 15
  at a time with 2 in common, under a line naming the file and the sheet. `index_object` declares
  the groups as the workbook's passages: no character span, `heading_path` = `[sheet]`.
- **Aura's Go reader does not change.** It already accepts a passage without a span, and
  identifiers-first already reads passage text.
- **The three arms** of the spec land one commit at a time and are measured between commits.
  The losers are reverted rather than kept behind a switch.

**Tech Stack:** Go (filecard, aura-filecard), Python 3.12 with cocoindex 1.0.25 (the ingest),
ArcadeDB 26.10.1, EmbeddingGemma-300M Q8_0 through llama.cpp, Garage, Docker.

**Spec:** docs/superpowers/specs/2026-10-17-workbook-row-passages-design.md (approved
2026-10-17).

## Global Constraints

- **No Postgres migration and no new ArcadeDB property.** `Passage.char_start` and `char_end`
  become nullable in the ingest's contract only.
- **No new environment variable.** Every bound on rows is filecard's: 8 sheets, 10,000 rows per
  sheet, 64 columns, 8 MiB of shared strings.
- **No embedding recipe change.** Row passages embed with `title: none`, so `RecipeVersion` and
  the embedding space stay as they are.
- **`media.index_text` still returns "" for a spreadsheet,** and `IndexedDocument.
  normalized_text_sha256` stays empty for a workbook.
- **No file over 600 lines.** `table.go` is at 452 and `app.py` at 498, so new logic goes to
  `rows.go` and `rows.py`. Stage explicit paths only.
- **No paid run.** The corpora carry no image or scan, and nothing reaches a vision route.
- **Commands:**
  - Go: `go vet ./... && go build ./... && go test -race` on the touched packages, from WSL.
  - The ingest: `make ingest-test` and `ruff check services/ingest`.
- **BASELINE OR DROP.** An arm ships only if it beats as-shipped (spec, acceptance 3). If none
  does, Tasks 3 to 9 are reverted and the PRD records why.

## Review Focus

1. **A banner over the header.** The 2026 budget's `Ricavi` has a title on row 1, "Trimestre"
   on row 2 and its header on row 3. Neither banner row may become a header or a data row, and
   Centro must be row 6, as in Excel.
2. **A headerless sheet** (`Riordino`, the plan's `Legenda`): its cells are labelled `A`, `B`.
   A first data row must never become a header.
3. **The probe window.** Rows read before the header is resolved, the first 8, are kept with
   their own positions once it is. A file shorter than the probe resolves in `done()` and keeps
   its rows too.
4. **A capped sheet** emits rows only up to the cap, and its card still says it was sampled.
5. **A null span through ArcadeDB's Cypher.** `SET n += $props` with `char_start: null` must
   leave no property, or a null one. It must never leave a stale value from an earlier version
   of the row. Aura's reader must then return no `CharacterSpan`, and the citation must read
   `ordinal=N`.
6. **A group over `document_budget()`**, a wide sheet with long text cells: every piece keeps
   the file-and-sheet line, and no piece exceeds the budget.
7. **A workbook all of whose sheets are shown whole** (arm C): `passage_count` is 0, and the
   reserved place still reaches it.

## Decisions taken while writing this plan

- **The open question on nullable fields is closed by reading cocoindex 1.0.25.**
  - `analyze_type_info` strips `None` from a union and sets `nullable=True`, and the neo4j
    connector keeps the column's type.
  - A nullability change is a column action that is "tracked for fingerprint stability but not
    applied" (`_target.py`): no DDL, and no table replace.
  - It is read, not measured. Task 3 measures the null write against ArcadeDB, and Task 5 the
    upgrade of an existing library.
- **Ruling: the prerequisite runs in the first task that can measure it** (Task 4), not in Task
  0.
  - The burst it guards against needs row passages to exist, and Tasks 1 to 3 are needed
    whatever it finds.
  - Cost if wrong: three tasks written before knowing whether a separate spec must land first.
- **Ruling: rows live on `Card`, as `Card.Rows []Row`.** `Sheet.Rows` is already the declared
  row count. `Row` is `{Sheet string; Number int64; Text string}`, and JSON names the number
  `row`.
- **Ruling: only `buildXLSX` collects rows.**
  - A CSV is extracted as text by the ingest and must not get row passages twice.
  - `.xls` and `.ods` reach `buildXLSX` after LibreOffice's conversion, as their card does.
- **Ruling: the whole-sheet test is `ShownWhole(sheet)`.** It holds when every scanned row is
  one of the card's samples and the rendered card carries no cut notice. `-json` emits the names
  of the sheets that pass it as `shown_whole`. The ingest uses it in arm C only.
- **Ruling: ordinals of row passages start at 0.** A workbook has no text passages, so
  `passage_key` stays `<document>:<ordinal>`. The spike's `1000 + ordinal` was there only to sit
  beside nothing.
- **Ruling: labels are "File", "Foglio" and "Riga", as measured.** The spike measured the
  Italian ones on an Italian corpus (spec, open question 3).
- **Ruling: no row-range locator** (spec, open question 2, not requested).

## File structure

| Files | Responsibility |
|---|---|
| `internal/documents/filecard/rows.go`, `rows_test.go` (new) | `Row`; serializing a row under its headers or column letters; `ShownWhole` |
| `internal/documents/filecard/table.go` | positions travel with consumed rows; the builder collects rows when asked |
| `internal/documents/filecard/xlsx.go` | `readWorksheet` asks for rows and hands them to the card |
| `internal/documents/filecard/card.go` | `Card.Rows`, not rendered |
| `cmd/aura-filecard/main.go`, `main_test.go` | `-json` emits `rows` and `shown_whole` |
| `services/ingest/rows.py`, `tests/test_rows.py` (new) | grouping, the file-and-sheet line, the split over budget |
| `services/ingest/app.py` | `Card.rows`; `Extracted.rows`; row passages in `index_object`; nullable spans |
| `services/ingest/media.py` | the routing comment |
| `services/ingest/tests/space_driver.py`, `tests/test_row_passages.py` (new) | a file through the production path; row passages and the null span through ArcadeDB |
| `prd.md` | §11, before the code |

## Tasks

### Task 0: PRD amendment

- [x] Amend prd.md §11 after "The card's vector". The new paragraph records the spike's numbers
  (spec, problem 2), the operator's constraint, the three arms, and what is not proved.
- [x] In the reserved-place paragraph, add one sentence: a workbook with row passages leaves the
  place.
- [x] Narrow the 2026-09-09 decision ("a spreadsheet is queried, not read") rather than
  contradict it. Generic chunking still does not apply to a workbook, and aggregates still go to
  `document_open`.
- [x] Commit: `docs(prd): workbook rows become passages that name their file and sheet`.

### Task 1: filecard keeps the rows

- [x] RED, in `rows_test.go`, over workbooks built in the test with the helpers `card_test.go`
  already uses:
  - `TestRowsCarryTheWorksheetRowNumber`: in `budget_consolidato_2026.xlsx`, `Ricavi`, Centro is
    row 6 under two banner rows (Review Focus 1);
  - `TestRowsUseTheCardHeaders`: "Codice: RIC-10000; Descrizione: …", empty cells skipped, and
    values longer than 72 runes kept whole;
  - `TestAHeaderlessSheetLabelsCellsByColumnLetter`: `A: RIC-10175`, and a gap in the middle
    of a row keeps its letter;
  - `TestRowsKeepCachedFormulaValues`: the store's VLOOKUP gives "Meccanica Bassi S.r.l." and
    the IF gives `SI`, read from `scripts/fixtures/document_retrieval_eval/complex_corpus/
    magazzino_ricambi_2026.xlsx`;
  - `TestProbeRowsKeepTheirPositions`: a sheet shorter than 8 rows (Review Focus 3);
  - `TestRowsStopAtTheRowCap`: rows end at `maxRowsScanned`, and the card still says sampled;
  - `TestShownWhole`: true for the 2026 budget's `Ricavi` (5 data rows); false for its
    `Costi` (6 rows, 5 samples), for the store's `Giacenze`, and for a card that was cut.
- [x] Run `go test ./internal/documents/filecard/ -run 'Rows|ShownWhole|Probe'`. Expect FAIL:
  `Row` is undefined.
- [x] GREEN:
  - `table.go`: `consume(values, position)`; `resolveHeader` passes `probePositions[i]`; a
    `keepRows` flag appends `rowText(values)` with its position.
  - `rows.go`: `Row`, `rowText`, `ShownWhole`.
  - `xlsx.go`: `readWorksheet` sets `keepRows` and returns the rows; `buildXLSX` appends them
    to `card.Rows` in sheet order.
- [x] `go test -race ./internal/documents/filecard/`. The package stays at its policy in
  `scripts/coverage_package_policy.json`; check with `bash scripts/coverage_gate.sh`.
- [x] Commit: `feat(filecard): keep a workbook's rows under the card's headers`.

### Task 2: `aura-filecard -json` emits the rows

- [x] RED in `main_test.go`:
  - `TestJSONCarriesRowsAndShownWhole` on the budget 2026: `rows` holds
    `{"sheet":"Ricavi","row":6,"text":"Area: Centro; T1: 1972; …"}`, and `shown_whole` lists
    `Ricavi` and not `Costi`;
  - `TestJSONOmitsRowsForAPDF`: a PDF has `rows: []`.
- [x] GREEN: the encoder map gains `rows` and `shown_whole`, both as empty lists, never null.
- [x] `go test -race ./cmd/aura-filecard/ ./internal/documents/filecard/`.
- [x] Commit: `feat(aura-filecard): emit rows and the sheets the card shows whole`.

### Task 3: the ingest declares row passages (arm A: sheet line only)

- [x] RED in `tests/test_rows.py`, with `count_tokens` as `test_chunk.py` uses it:
  - `test_rows_group_fifteen_at_a_time_sharing_two`: 40 rows give groups starting at rows
    1, 14 and 27 (and so on);
  - `test_every_group_opens_with_its_sheet`: `Foglio: Giacenze`;
  - `test_a_group_over_budget_splits_and_every_piece_keeps_the_header`;
  - `test_sheets_never_share_a_group`.
- [x] Extend `tests/space_driver.py` so an argument `name=@path` passes a file's bytes, with its
  suffix and content type, through `_extract` and `index_object`. Today it takes text only.
- [x] RED in `tests/test_row_passages.py`, through that driver against a disposable ArcadeDB
  database with `fake_embedder`. Ingest `magazzino_ricambi_2026.xlsx` and read it back with the
  SQL Aura's reader uses (`internal/arcadedb/document_retrieval.go`):
  - `test_a_workbook_gets_row_passages_without_a_span`:
    - `passage_count` equals the number of groups;
    - `heading_path == [sheet]`;
    - no `char_start` value;
    - `normalized_text_sha256 == ""` on the document;
  - `test_a_span_does_not_survive_a_rewrite_without_one`: the same `passage_key` written once
    with a span, then without (Review Focus 5);
  - `test_a_part_number_in_a_row_is_found_by_full_text`: `RIC-10959` through the passage
    full-text index.
- [x] Run `make ingest-test`. Expect FAIL: there is no `rows` module.
- [x] GREEN:
  - `rows.py`: `group(rows, file_name)` returns `chunk.Chunk`s. In arm A it does not use
    `file_name`.
  - `app.py`: `Card` and `Extracted` gain `rows`. `_card` parses `rendered["rows"]`.
    `index_object` uses `rows.group` when `extracted.rows` is non-empty, and `chunk.chunk`
    otherwise. `Passage.char_start` and `char_end` become `int | None`.
  - `media.py`: the comment says rows come from filecard.
- [x] `make ingest-test` and `ruff check services/ingest` both green.
- [x] Commit: `feat(ingest): give workbooks row passages headed by their sheet`.

### Task 4: the prerequisite — a large workbook through the real ingest

- [x] Build `aura-ingest:rows-a` from Task 3's commit.
- [x] One catch-up pass, on CPU, of a fresh identity whose bucket holds only
  `movimenti_analitici_5000.xlsx` from the office fixture.
- [x] Expect:
  - about 385 row passages;
  - zero `EmbedRequestError` and zero timeouts in the log;
  - an audit with 1 of 1 objects indexed.

  Record the time.
- [x] **If it times out: STOP.** Write the measured burst (requests in flight, the embedder's
  slot count, the 120 s deadline) and bring it to the operator as its own spec. Nothing after
  this task runs until that one lands.
- [x] Record the result in the verification file of Task 8. This task makes no commit.

### Task 5: arm A through the ingest

- [x] Copy `cb-arcade` and the `ingest-state-complex` and `ingest-state-office` volumes to an
  arm-A set. Run one catch-up pass per identity with `aura-ingest:rows-a`. This is the upgrade
  path, not a fresh index.
- [x] Measure the upgrade:
  - `[extract]` once per file;
  - one embedding per row passage;
  - no PDF or text passage re-embedded;
  - no table replace in the log.
- [x] Measure `TestCodingAgentQuestions` on complex and office. They must reproduce the spike's
  row (complex MRR 0.951, office 0.794) within one question per set. **If not, stop and explain**
  before Task 6.
- [x] Report q12, q13, q15, the four budget questions and `rota-legend` one by one. Then
  report the passages added, the embedding time, and the database size before and after.

### Task 6: arm B — the file name in the line

- [x] RED in `test_rows.py`: `test_every_group_names_its_file_and_sheet`, for
  `File: budget_consolidato_2026.xlsx — Foglio: Ricavi`. Also extend the split test so every
  piece carries both names.
- [x] GREEN: `rows.group` writes the file line.
- [x] Run `make ingest-test` and ruff.
- [x] Commit: `feat(ingest): row passages name their file beside their sheet`.
- [x] Measure as in Task 5, on a fresh copy of the as-shipped volumes, with
  `aura-ingest:rows-b`.

### Task 7: arm C — no rows for a sheet the card shows whole

- [x] RED in `test_row_passages.py`:
  - `test_a_sheet_the_card_shows_whole_gets_no_row_passages`;
  - `test_a_workbook_shown_whole_keeps_passage_count_zero` (Review Focus 7).
- [x] GREEN: `index_object` drops the rows of every sheet in `shown_whole`.
- [x] Run `make ingest-test` and ruff.
- [x] Commit: `feat(ingest): a sheet its card shows whole needs no row passages`.
- [x] Measure as in Task 5 with `aura-ingest:rows-c`.

### Task 8: choose the arm, read the answers, record

- [x] Choose the arm with the spec's rule:
  - it beats as-shipped on MRR on complex and office;
  - it loses nothing beyond one question on the English, Italian and find-by-name code sets,
    measured on the workstation's GPU index (`cb-arcade`) for the chosen arm and as-shipped.

  Then:
  - **if B or C wins,** revert the arms after it;
  - **if A beats B,** stop and bring it to the operator: B is the operator's constraint;
  - **if no arm beats as-shipped,** revert Tasks 3, 6 and 7 and record the drop in the PRD.
- [x] Add each question's expected answer to `qrels.json`, read from the files, so the office
  half can be judged blind.
- [x] Read the answers for the chosen arm with the 2026-10-17 harness, blind on both sets.
  There must be no more wrong answers than today (1 of 34 complex, 0 of 20 office). Report how
  many answers needed `document_open`.
- [x] Write `docs/superpowers/verification/2026-10-17-workbook-row-passages.md`. It holds
  Tasks 4 to 8, every arm's table, and what the run does not prove.
- [x] Update prd.md §11 with the measured result, and add the citation.
- [x] Commit the verification and the PRD together.

- [ ] The binding code-set gate: the full index on the operator's workstation. A 62-file subset
  stood in here, under the operator's 20-minute cap.

### Task 9: review and fixes

- [x] Give one fresh reviewer the range from Task 0 to Task 8, this plan, the spec and the
  Review Focus.
- [x] Fix every finding test-first.
- [x] Run Go `-race` on the touched packages, `make ingest-test`, `make quality`, and the
  coverage gate. What ran here: `go vet ./...`, `go build ./...`, `-race` on the four touched
  packages, `golangci-lint` (0 issues), `check-file-size.sh`, filecard coverage 90.3%, and the
  ingest suite (259 passed) through the scratch harness. `make quality` and
  `scripts/coverage_gate.sh` need the WSL stack and were not run; CI or the operator runs them.

### Task 10: push, publish and the lab VM

- [ ] Push with the WSL recipe, then read CI for that SHA.
- [ ] Publish edge, and update the lab VM through the updater.
- [ ] Run one ingest pass over the VM's library, recording its time and the passages added.
  If the 2026-09-09 reference tables are still there, ask the agent the CAP of Caraglio.
- [ ] Write `docs/superpowers/verification/2026-10-17-workbook-row-passages-vm.md`.
