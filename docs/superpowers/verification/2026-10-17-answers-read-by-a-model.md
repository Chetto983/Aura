# Answers read by a model, over the complex and office sets

Date: 2026-10-17. The operator asked: act as Aura's model and test retrieval and answers. The
retrieval verifications of 2026-10-16 measured where the gold document ranked. They also said
"no language model read the answers". This run reads them.

**Verdict:**
- **Complex set: 33 of 34 correct on the first pass.**
  - The miss is a look-alike. For "FX300 alarm A119", search ranks the FX300 manual first, but
    with its opening passage only. The passage holding A119 belongs to the FX200, and the model
    answered from it: "inceppamento sul nastro di uscita" instead of "riparo aperto".
  - Retrieval counts that question as a first-place success.
- **Office set: all 20 reachable, two only after a second search.**
  - q04 "quanto abbiamo fatturato ad ACME" returns no ledger. q18 "fatture emesse a marzo"
    returns two of the three March invoices among April and May ones.
  - In both cases the ledger was reached by searching for the name that `riepilogo_annuale.txt`
    gives it.
- **A first-place document is not an answer.** On 2 of the 22 complex PDF questions, the gold
  document ranked first without the passage that answers. In both cases the look-alike manual's
  passage held a different value:
  - `fx300-chains`: 40 h is the FX200's; the FX300's is 160;
  - `fx300-alarm-A119`.

  The metric of 2026-10-16 counts both as r@1.

## Setup

- **Index.** The production indexes of
  `2026-10-16-complex-documents-retrieval-quality.md`, volume `cb-arcade`, ArcadeDB 26.10.1,
  EmbeddingGemma-300M Q8_0 on CPU. The stack was restarted after the container was recycled.
  `TestCodingAgentQuestions` reproduced every row of that file:
  - complex: r@1 0.794, MRR 0.882;
  - office: r@1 0.500, MRR 0.692.
- **document_search** is the production arm of that run: `HostRetriever.Retrieve` at limit 8,
  each response saved whole. A second search reused the same arm on new queries.
- **document_open** was emulated by reading the fixture file. Its bytes are what the index's
  `raw_sha256` pins. PDFs were read with `pdftotext`, workbooks with openpyxl's cached values,
  as the agent's sandbox would.
- **Policy:**
  - answer from passages when they hold the answer;
  - otherwise from the card;
  - open the file when the card says `requires_open` and does not hold the answer, or when a
    count or sum is asked;
  - search again when the response abstains or points at a document it did not return.
- **Blindness.**
  - Complex: answers were written to a file before `complex_qrels.json`'s answers were read.
  - Office: `qrels.json` carries no answers, so the ground truth is the files themselves. They
    were opened before the answers were written. The office half therefore measures whether the
    search path reaches the answer, not whether a blind model finds it.

## Complex set, 34 questions

| How it was answered | Questions | Correct |
|---|---|---|
| a returned passage | 21 | 20 |
| the card alone (budgets, plan legend) | 5 | 5 |
| `document_open` after the card (store, plan, rota, the FX300 manual) | 7 | 7 |
| a second search, then `document_open` (rota legend) | 1 | 1 |

- **Look-alikes are where a model goes wrong.**
  - Nine questions name the FX300 or the FX200. On six of them, the other manual's returned
    passage holds the same row, with the same layout and a different value.
  - Each table row ("A119 Arresto … riparo aperto") names no machine. Only the document it sits
    under does.
  - On `fx300-alarm-A119` and `fx300-chains`, the asked machine's passages lack the row, and the
    other machine's passages carry it.
- **Workbook questions need the file.** Seven of the 12 workbook golds were answered only by
  opening the file:
  - the store, for a location, a supplier and a lead time;
  - the plan, for its R count;
  - the rota, for nights and the legend;
  - the FX300, whose maintenance section was not returned.

  The four budget questions and the plan legend were answered by the card alone.
- **`rota-legend` returns nothing.** "a che ora inizia il turno di notte secondo il foglio
  turni" abstains with `no_qualified_passage` and zero documents. "orario turno di notte"
  abstains too. "turni reparto ottobre legenda" ranks the rota first, and its file says
  "N notte 22-6".

## Office set, 20 questions

| How it was answered | Questions |
|---|---|
| returned passages (notes, contract, archive, summary) | 6 |
| a workbook's card (invoices, registry rows, client list, payables) | 5 |
| card and passage together | 3 |
| `document_open` (Q2 VAT, the year's total, every Gialli invoice) | 3 |
| passage, then `document_open` (q19, the year's summary) | 1 |
| a second search (q04 card, q18 open) | 2 |

**What would mislead a less careful model:**
- **The registry's card misreads the summary sheet's second table.**
  - The sheet `riepilogo` holds a per-client table, then a per-quarter one. The card labels the
    quarter row with the client table's headers: "cliente: Q1 | num_fatture: 79350 |
    imponibile_totale: 17457 | iva_totale: 96807".
  - Read literally, Q1's VAT is 96,807. It is 17,457.
  - The card shows no Q2 row at all, so q05 needs the file.
- **q18 looks answered when it is not.** Search returns `fattura_2026_005` and `_006`, both
  March, beside four invoices from April and May. A model that answers from those cards gives two
  invoices. The third, `fattura_2026_007`, comes only from the ledger.
- **The plan's legend card** takes the legend's first entry, "L | Lubrificazione", as its header.
  So T reads "L: T | Lubrificazione: Taratura sensori", which is answerable but mislabelled.

## What this does not prove

- **The model was not Aura's configured model.** The model that read these responses was not
  driven by Aura's runtime, its system prompt or its tool loop. It worked through a harness of
  scripts, and emulated `document_open` without the sandbox.
- **One reader, one run, judged by the same model.** A model that does not check which document
  a passage sits under will lose more look-alike questions than this one did.
- **The office half was not blind.** Its files were read before the answers were written.
- **Synthetic corpora** written by one generator. The office fixture has no expected answers;
  its ground truth here is the files' content.
- **No code set.** The 54 questions over Aura's sources were not run here, as on 2026-10-16.
