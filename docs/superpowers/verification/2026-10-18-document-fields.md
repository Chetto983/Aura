# Document fields: the live verification

Date: 2026-10-18. Task 13 of `docs/superpowers/plans/2026-10-18-document-fields.md`, against the
spec's acceptance (`docs/superpowers/specs/2026-10-18-document-fields-design.md`). Run on the
operator's Windows dev PC, on the dev stack built from this checkout (`aura:local` and
`aura-ingest:local` rebuilt at each fix, ArcadeDB 26.10.1, EmbeddingGemma-300M Q8_0 on CUDA), as
the operator identity `15d43203-…` holding the 4,045-document bench library of Task 0.

**Verdict:**
- **Filters raise set recall from 0.141 to 0.968** on the 25 "which documents" questions, and no
  gold document is lost to a filter. The gate holds.
- **The extractor proposed the counterparty and the date of all 14 office invoices**, each equal to
  the generator's values.
- **An operator value and its history survive a rename, a move and new bytes** made through the
  file manager's routes. After each, the detail, the list filter and the agent's filtered search
  find the document under its new key.
- **The Documenti view works end to end on a desktop and on a phone**: list, filter, bulk edit with
  its confirmation, review with "Rivisto, salva e avanti", the lists and the custom fields.
- **The live run found six defects the unit suites could not**, each fixed test-first and
  re-verified live (below).

## The which-documents set (acceptance 2)

The office library was reviewed as the question set declares it, through the documents API as the
signed-in operator: the test harness standing in for the operator's review in the view. Each
question then went to the agent's own `document_search` through `aura docs mcp`, once with no
filter (today's search) and once with the filters the question implies, at the tool's default
limit of 8. An answer over the 30,000-byte preview cap was paged back with `read_tool_output`, as
the agent does. Spec: `web/e2e/documents-filter-eval-live.spec.ts`; 15.8 s.

| qid | question | gold | today | filtered | admitted by each filter |
|---|---|---|---|---|---|
| q03f | fattura di febbraio emessa a Bianchi | 1 | 0.00 | 1.00 | type 14, counterparty 3, date 38* |
| q09f | tutte le fatture del cliente Gialli Import | 2 | 0.00 | 1.00 | type 14, counterparty 2 |
| q18f | fatture emesse a marzo 2026 | 3 | 0.00 | 1.00 | type 14, date 39* |
| f01 | le fatture di ACME SpA | 4 | 0.00 | 1.00 | type 14, counterparty 5 |
| f02 | le fatture intestate a Rossi SRL | 3 | 0.00 | 1.00 | type 14, counterparty 3 |
| f03 | fatture emesse a Bianchi & Figli | 3 | 0.00 | 1.00 | type 14, counterparty 3 |
| f04 | fatture di Verdi Costruzioni | 2 | 0.00 | 1.00 | type 14, counterparty 2 |
| f05 | fatture del primo trimestre 2026 | 7 | 0.00 | 1.00 | type 14, date 113* |
| f06 | fatture del secondo trimestre 2026 | 7 | 0.00 | 1.00 | type 14, date 114* |
| f07 | fatture emesse a gennaio 2026 | 2 | 0.00 | 1.00 | type 14, date 36* |
| f08 | fatture di febbraio 2026 | 2 | 0.00 | 1.00 | type 14, date 38* |
| f09 | fatture di aprile 2026 | 3 | 0.00 | 1.00 | type 14, date 39* |
| f10 | fatture di maggio 2026 | 2 | 0.00 | 1.00 | type 14, date 38* |
| f11 | fatture di giugno 2026 | 2 | 0.00 | 1.00 | type 14, date 37* |
| f12 | fatture ad ACME nel secondo trimestre | 2 | 0.00 | 1.00 | type 14, counterparty 5, date 114* |
| f13 | fatture a Rossi SRL da aprile in poi | 2 | 0.00 | 1.00 | type 14, counterparty 3, date 321* |
| f14 | fatture di Verdi Costruzioni del primo trimestre | 1 | 0.00 | 1.00 | type 14, counterparty 2, date 113* |
| f15 | la fattura di maggio a Gialli Import | 1 | 0.00 | 1.00 | type 14, counterparty 2, date 38* |
| f16 | fatture a Bianchi & Figli emesse a marzo | 1 | 0.00 | 1.00 | type 14, counterparty 3, date 39* |
| f17 | fatture emesse tra il 15 e il 30 aprile | 2 | 0.00 | 1.00 | type 14, date 18* |
| f18 | il contratto con ACME SpA | 1 | 1.00 | 1.00 | type 1, counterparty 5 |
| f19 | tutti i documenti di ACME SpA | 5 | 0.20 | 0.20 | counterparty 5 |
| f20 | i documenti interni dell'amministrazione | 3 | 0.33 | 1.00 | tags 3 |
| f21 | la bozza interna | 1 | 1.00 | 1.00 | tags 1 |
| f22 | il registro contabile del 2026 | 1 | 1.00 | 1.00 | type 2 |

\* The count rests on suggestions too: the date filter, with `include_unreviewed` at its default
`true`, admits the bench invoices whose dates the extractor proposed and nobody reviewed.

- **Mean set recall: 0.141 today, 0.968 with the filters.** No question lost a gold document that
  today's search found.
- **Today's search answers a "which invoices" question with the documents that list invoices**:
  the ledger, the payment notes and the schedule rank above the invoices themselves, which are
  spreadsheets with one card each.
- **f19 is the one question the filters do not answer:** the counterparty filter admits exactly the
  5 gold documents, but the search returns 1 of them. The four ACME invoices miss with and without
  the filter: a filter narrows the candidates, and the ranking still needs a leg to score each one.
  The filter report tells the agent `admitted_documents: 5`, so the gap is visible to it. Returning
  every admitted document when a filter admits no more than the limit is a candidate change, not
  made here.

## The extractor (acceptance 3)

After the header-pairs re-ingest, the pass proposed a counterparty and a date for each of the 14
office invoices: 14 of 14 equal the generator's values (read from each record's history before the
review replaced them). The lists were empty, so each counterparty came as a proposed name
(`new:<name>`). The number reaches a custom field only once the operator defines one. That case is
not run live; `TestPassExtractsOfficeInvoiceHeaders` covers it.

## Survival (acceptance 4)

Spec: `web/e2e/documents-survival-live.spec.ts`; 4.3 min. A text file is uploaded through the file
manager, then given a type, a date and a tag as the operator. After each operation below, the
values and the history hold, and the list filtered by the type and the agent's search filtered by
it return the document under its new name and key:

| operation | the index held only the new key after |
|---|---|
| rename | 67.1 s |
| move into a subfolder | 65.1 s |
| new bytes | 49 ms after the re-index |

- **Each wait is one ingest scan:** `AURA_INGEST_INTERVAL_SEC` is 60 s on the dev stack. Until the
  scan, the old key's row stays in the index, holding no record.
- **The same bytes uploaded again are not read again by the ingest** (measured: no re-index within
  90 s). The step was dropped from the spec.
- **A rename keeps a `moved` event in the history.**

## The Documenti view (Task 13 step 3)

Spec: `web/e2e/documents-live.spec.ts`, at 1440×900 and at 390×844 with touch; 6.1 s and 5.7 s.
It takes the run's own rule label through the list, a filter, a bulk edit of two documents with
its confirmation, the detail with a suggestion taken and "Rivisto, salva e avanti", and the lists
page. It takes back what it wrote. It asserts the two layout defects below on both viewports. The 24
screenshots are attached to the run.

## Found live, fixed test-first

| defect | test that failed first |
|---|---|
| `/api/documents` was never mounted on the cockpit's mux: the view got 404 | `TestServeWebui` routing table |
| the pass skipped for good the documents indexed in one millisecond across a batch boundary (`fattura_2026_0291.xlsx`) | `TestPassReadsEveryDocumentOfAnInstantABatchSplits` (200 of 202) |
| "Rivisto" accepted a proposed name as it stood, a name on no list | `TestReviewAdoptsAProposedNameAsALabel`, `TestSettingAProposedNameAdoptsIt`, `TestAdoptReplacesAProposedNameByItsLabel` |
| the grid grew past its view (1,889 px in 438 px), hiding rows and the pager | the live spec's grid-fit assertion |
| the name column shrank to 17 px on a phone | `documentsColumns.test.ts`, and the live spec's width assertion |
| the lists page showed "0 documenti" while the counts loaded | `LabelsAdmin.test.tsx` |

The 4,027 `DocumentFields` rows Task 0's spike left in the identity's database (source names, no
`record_id`) would have refused every operator write on their documents. They were deleted with the
operator's approval before the run.

## Asked by the operator during the run

- **The columns resize, and keep their width.** Every column but the selection has a resize grip,
  and a dragged width is kept in the browser. The live spec drags the name column's grip on the
  desktop viewport. The date column starts wider, so its source mark fits.
- **The xlsx preview is GridCraft** (`docs/superpowers/specs/2026-10-18-xlsx-viewer-gridcraft-design.md`).
  - The image serves GridCraft 0.8.0 at `/apps/gridcraft/`: the page answers 200 with its own
    policy, and the wasm answers as `application/wasm`, 7.05 MB gzipped against 18.9 MB.
  - In the detail an invoice opens in it on both viewports, with no console error.
  - The Documenti view now loads as its own chunk: the shell's test lost a race with it.

## Isolation (acceptance 5)

- **Measured at the database level only.** The stack's other tenant database holds none of the
  document types, and Postgres holds no document-field tables: the fields live only in each
  identity's ArcadeDB database.
- **A second identity's sign-in was not run.** Creating one through the identities wizard
  provisions OpenRouter credit. The API's not-found answer for another identity's document is
  covered by the agui tests.

## Gates (Task 13 step 4)

- `make quality` green in WSL: deadcode, vet, build, file size, contracts, lint, test-race, vuln.
- **Coverage, on the disposable full matrix (`scripts/coverage_docker.sh`): 47,579 of 54,461
  owned statements, 87.4%.** The package policy is green.
  - `internal/docfields/rules` is at 98.6% and `internal/staticapp` at 94.4%, both at target.
  - `internal/docfields` reaches 217 of 876 statements in the gate's tier, pinned as a baseline.
    Its ArcadeDB tier measures 90.0% and now runs in CI's Agent Memory job.
  - `internal/objectstore` moved to an exact 467/596 (from 436/576).
- The web suite: 424 files, 3,875 tests. The three live specs pass on the final image.

## What this run does not prove

- **The agent chose no filter here:** each question was searched with the filters it implies,
  written in the set. Whether the real agent picks them from the operator's words is Step 7, on the
  lab VM, which the operator runs.
- **The review was the harness's, through the API**, not an operator working through the view.
- **The office library is 23 documents inside a 4,045-document bench library.** The measured
  recall is for these questions, on this library, at the tool's default limit.
- **One machine, one run.** No latency claim is made here; Task 0 measured the filter's cost.
