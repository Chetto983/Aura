# Coding-agent questions over Aura's own sources

Date: 2026-10-16. Asked by the operator: put all of Aura's source through the ingest pipeline on
this workstation's Docker, then use it to ask the questions a coding agent really asks, and see
whether it works.

**Verdict:**
- **It works for "where is X" questions.** In English, the file that answers is among the 8
  documents `document_search` returns for 19 of 24 questions, and first for 11. The same
  retriever without the dense leg finds it first for 6.
- **Asked in Italian, it still answers 15 of 24.** The full-text leg alone answers 2, because
  Italian words do not match English code.
- **Indexing the repository found a defect that lost files, now fixed.** Dense cards took 93 files
  out of the index (fixed in `1ab54ef31`, see below).
- **It also found a gap in the upload path.** `supportedDocumentExt` accepts no source
  extension, so a `.go` uploaded from the cockpit is refused before it reaches the reader.

## Setup

- **Corpus:** every file tracked at `ed4126f86`, 5,983 files (48.8 MB), uploaded to a disposable
  Garage bucket under `aura/`. Left out: 728 files of build output or lockfiles
  (`internal/webui/dist/`, `package-lock.json`, `go.sum`), and 49 images, audio, video and PDFs,
  which could reach a paid vision or speech route.
- **Text:** 5,796 files had text, led by `.go` 3,091, `.md` 679, `.ts` 573 and `.tsx` 548.
  - 187 got a row by name only. Among them `.mjs` 52, `.css` 22, `.jsx` 9 and `.cypher` 2: those
    extensions are not in `SOURCE_EXTENSIONS`.
- **Pipeline:** `aura-ingest:edge` at `426753ef9`, with the working tree's `services/ingest`
  mounted over `/app/ingest`.
  - The embedder was the local `aura-llama-embed` on the GPU, EmbeddingGemma in space
    `es1-e0aa6accf0b79c6b`, the same space as the lab VM.
  - The database was a disposable ArcadeDB `26.10.1`, Aura's pin, with compose's memory, GC and
    Bolt settings.
- **Index:** 5,983 documents and 11,771 passages, every vector in that one space. 218 documents
  have no passage.
- **Before any run:** the suite `pytest ingest/tests` passed, 214 of 214, against a throwaway
  ArcadeDB and the live tokenizer.

## Found while indexing: dense cards took their batch down

The first catch-up pass ended with 5,890 of 5,983 objects indexed. All 93 missing objects failed
the same way: `HTTP 500 for a 2459/2580/2600-token input (model ceiling 2048)`, each retried
about 30 times.

**The cause.** `_fit_for_embedding` measured with the tokenizer only when `ceil(chars / 3)`
passed the ceiling. It counted on 3 chars/token always overshooting, but that ratio was
measured on prose (about 5.3).

The `aura-filecard` cards of four files ran at 1.3-1.5 chars/token, measured with `/tokenize`:
three spike `results.json` files and `scripts/payload_manifest.txt`. A card is the one input no
chunker measures:

| Card | Real tokens | Estimate |
|---|---|---|
| `090-turingdb-runtime-durability-fit/results.json` | 2,600 | 1,159 |
| `091-turingdb-aura-cypher-compat/results.json` | 2,598 | 1,147 |
| `093-turingdb-llm-graph-access-path/results.json` | 2,580 | 1,148 |
| `scripts/payload_manifest.txt` | 2,459 | 1,275 |

**Why 93 files and not 4.** Batching groups chunks from different files in one request. A 500
is not one of the statuses CocoIndex splits a batch on, so every file in the request failed with
the card.

**The fix (`1ab54ef31`).** The measurement is now skipped only when the text's UTF-8 byte count
fits the ceiling; anything longer is measured.

**After the fix.** A second pass took 29 s, cut the four cards to the head that fits, and the
audit reported 5,983 indexed and 0 missing.

## The questions

- **24 questions** that come up while working on this repository: where something is
  implemented, where an error message comes from, which script or component does something.
- **Gold:** the files that answer each question, picked by reading the code before any run.
  - The questions are in `scripts/fixtures/document_retrieval_eval/coding_agent_questions_en.json`.
  - The same questions translated, with the same gold, are in `..._it.json`.
- **The test:** `TestCodingAgentQuestions` in `internal/documents` (`retrieval_eval` tag).
  - It calls `HostRetriever.Retrieve`, the call behind `document_search` and `aura docs
    search`, at the tool's default limit of 8.
  - The baseline arm is the same retriever with no embedder. Both legs are then lexical (spec §8).

| Arm | r@1 | r@3 | r@8 | MRR |
|---|---|---|---|---|
| production, English | 0.458 | 0.542 | 0.792 | 0.533 |
| full-text only, English | 0.250 | 0.417 | 0.750 | 0.362 |
| production, Italian | 0.375 | 0.458 | 0.625 | 0.434 |
| full-text only, Italian | 0.000 | 0.042 | 0.083 | 0.027 |

The rank of the first gold file in each answer; – means it is not among the 8:

| Question | Prod. EN | Full-text EN | Prod. IT | Full-text IT |
|---|---|---|---|---|
| dense-gate | 7 | 3 | – | – |
| tenant-password | 1 | 3 | 1 | – |
| secure-version | – | 1 | – | – |
| lucene-escape | 1 | – | 1 | – |
| fused-statement | 6 | 1 | – | – |
| relevance-floors | 3 | 7 | – | – |
| extractable | – | – | – | – |
| no-vision-route | – | 3 | – | – |
| chunk-ceiling | 1 | 3 | 1 | – |
| embed-env | 1 | 4 | 3 | 7 |
| error-control-plane | 1 | 8 | 3 | – |
| keylock | 1 | 1 | 1 | – |
| log-redaction | 1 | 1 | 1 | – |
| host-update-request | 1 | 5 | 1 | – |
| document-open | 1 | 1 | 1 | – |
| degradation-log | 5 | 1 | – | – |
| abstention | – | – | 6 | – |
| tool-search-rank | 4 | – | – | – |
| tool-promotion | 8 | 7 | 1 | – |
| telegram-docwait | 1 | – | 1 | – |
| mcp-auth-panel | 3 | 8 | 6 | – |
| publish-images | 1 | 5 | 6 | – |
| coverage-policy | 4 | 6 | 4 | 2 |
| embed-space-stamp | – | – | – | – |

The full-text leg alone ranks the gold higher than production on 6 questions:
- `secure-version` and `fused-statement`, where it is first;
- `dense-gate`, `no-vision-route`, `degradation-log` and `tool-promotion`.

The final `vector.rerank` orders by cosine alone, so a lexical first place does not survive it.
That is the next lever to measure, against this same set.

## What the agent reads

- **Names flood the card leg.** 27 of the 192 documents returned in English were card-only, that
  is, ranked by their name with no passage.
  - For "how does the ingest sidecar decide whether a file has text", 5 of 8 results were files
    named `*sidecar*`, and `services/ingest/extract.py` was not among them.
- **A file's first chunk is often only its package line and imports.** The chunker splits by
  size, so for a short file the agent gets about 2,000 tokens that say nothing about it.
- **A miss is not always useless.** For `secure-version`, the first result is a plan that quotes
  the `minSecureVersion` code verbatim. An agent reading it learns the identifier and can grep for
  it. The gold metric counts that as a miss.

## Four questions with no answer known beforehand

Each answer below was verified against the code afterwards.

| Question | Where the answer ranked | Verified |
|---|---|---|
| Which file types does the cockpit upload accept? | 1: `internal/documents/extensions.go` | yes, and it is the gap above |
| Where does aura-filecard card a file it cannot parse? | 3: `internal/documents/filecard/card.go` | right package; the routing is `build.go:128` |
| How does the updater restart edge after aura? | 6: `deploy/aura-image-update.sh` | yes, its line 49 |
| Which env var sets the retrieval candidates? | 5: `internal/config/config_document_retrieval.go` | yes: `AURA_DOCUMENT_RETRIEVAL_CANDIDATES`, 200 |

## Ranking levers, measured the same day on the same index

**Two levels.** The passage order was measured with `TestFusedQueryVariants` in
`internal/arcadedb`: each variant is the shipped statement with one clause replaced, scored on 20
passages, one per file. The document order was measured with `TestCodingAgentQuestions`'
ranking-rule arms.
- Those arms re-order the production retriever's whole ranking from the evidence each document
  carries.
- The empty rule must reproduce production. It did on every question of every set.

### Passage order

| Variant | EN r@1 | EN r@10 | EN MRR | IT r@1 | IT r@10 | IT MRR | Same answers EN/IT |
|---|---|---|---|---|---|---|---|
| shipped | 0.458 | 0.875 | 0.583 | 0.417 | 0.750 | 0.500 | 24/24 |
| full-text: stopwords out | 0.458 | 0.875 | 0.588 | 0.417 | 0.750 | 0.510 | 10 / 5 |
| full-text as memory recall's (stopwords out, BM25 floor 2, top 80) | 0.458 | 0.917 | 0.591 | 0.375 | 0.750 | 0.471 | 6 / 7 |
| fusion order, cosine floor | 0.333 | 0.917 | 0.547 | 0.000 | 0.583 | 0.124 | 0 / 0 |
| rerank only the fusion's top 80 | 0.458 | 0.875 | 0.583 | 0.417 | 0.750 | 0.501 | 24 / 23 |

**The filter variants,** which only change how the dense candidates are filtered:
- these all gave identical answers in both languages:
  - the filter applied after the rerank;
  - the filter read from the index;
  - the filter as one list;
- with fetch 100, the answers changed on 3 English and 2 Italian questions.

**Ordering by the fusion is rejected.** It loses the first place in English and collapses in
Italian. Memory recall measured the same on 2026-09-02 (`memory_vector.go`): the full-text leg
belongs in the candidate pool, not in the order.

**The full-text variants move MRR by less than one question** in each direction. They are not a
result.

### Document order

Four question sets:
- the 24 coding questions in English and in Italian;
- six find-by-name questions over the same sources
  (`scripts/fixtures/document_retrieval_eval/coding_agent_questions_names.json`);
- the office fixture's 20 Italian questions (`qrels.json`) over its 22 files, ingested as a
  second identity. Its scanned PDF was left out, because it would reach the vision route.

| Rule | EN r@8 / MRR | IT r@8 / MRR | names r@8 / MRR | office r@8 / MRR |
|---|---|---|---|---|
| production | 0.792 / 0.533 | 0.625 / 0.434 | 1.000 / 0.667 | 0.800 / 0.650 |
| B: the lane only for a file with no passage at all | 0.792 / 0.552 | 0.625 / 0.453 | 1.000 / 0.750 | 0.800 / 0.650 |
| C: B, and a card scores only such a file | 0.875 / 0.569 | 0.708 / 0.485 | 0.833 / 0.521 | 0.800 / 0.650 |
| D: memory's quota, cards at places 2 and 4 | 0.833 / 0.555 | 0.625 / 0.470 | 1.000 / 0.667 | 0.800 / 0.625 |

**B** is `reserveCardLane` doing what its own comment says it is for: a spreadsheet, a file with no
passages by design. Today it gives the place to any document this query found no passage of, so a
file matched only by its name is promoted to second.
- Over the 74 questions B moved 14 golds higher and none lower.

**C** improves the topical questions in both languages, but it loses find-by-name lookups:
- `compose.yaml` drops from first to out of the 8;
- `retrieval_rank.go` drops from third to eighth.

A file with text is then found only by its passages, and a passage does not carry its own file
name.

**D** applies memory recall's lesson (`memory_recall_quota.go`): two record types embedded apart
have no common cosine scale, so each gets its own places. It sits between B and C on the code, and
costs one first place on the office set.

**Outcome: B shipped, keyed on the card's passage count (PRD §11, 2026-10-16).**
- Rebuilt and re-run on all four sets, the production arm now gives B's numbers, and the
  ranking-rule arms' empty rule reproduces it.
- The change also reaches lexical-only mode, the degraded path. There:
  - English moved 5 golds up and none down;
  - Italian moved one up, and `coverage-policy` down from 2 to 4;
  - one find-by-name gold moved down one place.
- **The 2026-09-09 pilot could not be re-run.** `docs/document-retrieval-order-pilot.json` is the
  pilot that first measured `reserveCardLane`. Its 118-document corpus is gone from the lab VM: the
  VM holds one tenant with 9 documents and 7 passages, and no `ArcadeDB-Manual.pdf`.

## The documents: the office fixture

All numbers below are after B.

**By question category:**

| Category | Questions | Production |
|---|---|---|
| PROSE and AGING | 8 | all first |
| EXACT ("fattura numero 011") | 3 | gold second, through the reserved place; full text alone ranks 2 of them first |
| AGG and CROSSREF | 4 of 6 | the gold spreadsheet is missing |

Two more experiments, neither of which helped:
- **Two or three reserved places** changed nothing in any set. The missing spreadsheet is not a
  candidate at all. For q04 the answer holds three documents, and `contabilita_2026.xlsx` is
  not one of them. Full text alone ranks it first, at BM25 4.34.
- **The floor is why it is missing.** `documentCardStatement` applies the passage floor, 0.32,
  to cards. That floor was measured on 2026-09-09 over passages.

**`TestCardCosineBand`** measures every card's cosine against each question
(`scripts/fixtures/document_retrieval_eval/card_band_questions.json`: the 20 questions plus the
five out-of-corpus questions of 2026-09-09):
- **The four gold cards that go missing** sit just under the floor: `contabilita_2026.xlsx` at
  0.300, 0.301 and 0.316, and `anagrafica_clienti.xlsx` at 0.298.
- **The best card for an out-of-corpus question** scores 0.135-0.202. Cards therefore have a band
  of their own, from 0.202 to 0.298, with a midpoint near 0.25.

**A card floor of 0.25** was measured once, as a local change that was never committed:

| Set | r@8 | MRR | golds moved |
|---|---|---|---|
| office | 0.800 → 1.000 | 0.650 → 0.722 | all four spreadsheets come back |
| English code | — | 0.552 → 0.537 | 6 down one place each |
| Italian code | — | 0.453 → 0.444 | 2 down one place each |
| find-by-name | — | 0.750 → 0.722 | 1 down one place |

**What moved the code results down.** The reserved place went to a newly qualifying file Aura
could not read, whose card is its name and "Aura did not read this file at ingest: nothing here
describes its contents" (`run-speed.mjs`, `*.sse` test data). Such a file has no passage, like a
spreadsheet. The index cannot tell the two apart: `IndexedDocument` stores the card's text, not
filecard's kind.

**Not shipped.** A floor of the cards' own needs the index to know which cards describe their
file.

## What a card's vector is made of

The ingest embeds the whole rendered card (`services/ingest/app.py`, `index_object`). That card
carries fixed sentences of filecard's:
- **every card** ends with `cardTrailer`: "Written by Aura at ingest from the file itself: it
  describes the file and is not the file — open it with document_open to read or compute on it.";
- **a file filecard has no handler for** also says "Aura did not read this file at ingest: nothing
  here describes its contents." (`filecard/build.go`).

**Census of the stored cards:**

| Sentence | Code corpus (5,983 cards) | Office (22) |
|---|---|---|
| the trailer | 5,983 | 22 |
| "Aura did not read this file" | 5,046 | 0 |
| "The description was cut to fit" | 2 | 0 |
| "Aura could not read this file", the PDF lines, the media line | 0 | 0 |

**The 5,046 include the source files.** filecard has no handler for `.go`, `.ts` and the other
extensions the ingest has read directly since ed4126f86. So a source file indexed as passages
carries a card saying nothing describes its contents.

**`TestCardTextVariants`** (`internal/arcadedb/document_card_floor_bench_test.go`) re-embeds
each office card with some lines dropped. It then scores the 22 golds that only a card can
answer against the 0.32 floor (`card_band_questions.json`):

| Card text | Gold cards ≥ 0.32 | Lowest gold | Best out-of-corpus card |
|---|---|---|---|
| as stored (re-embedding reproduces the stored vectors, cosine 1.0000) | 17/22 | 0.295 | 0.202 |
| without the trailer and the "did not read" caveat | 19/22 | 0.307 | 0.217 |
| also without the sample rows | 16/22 | 0.228 | 0.200 |
| schema lines only | 15/22 | 0.214 | 0.180 |

- **Sample rows help.** Dropping them loses golds, so they do not dilute the card.
- **The fixed sentences shift the band, not its width.** The gap between the lowest gold and the
  best out-of-corpus card is 0.093 as stored and 0.090 without them. Two golds lose a little:
  `fattura_2026_011.xlsx` falls from 0.419 to 0.397 for q09, still over the floor.

**End to end.** For the end-to-end run, every card vector of both identities in `cb-arcade` was
rewritten from its own text without those two sentences. The card text was left unchanged, so the
full-text leg saw what it saw before. The rewrite took 115 s for the 5,983 code cards.

Two checks before measuring:
- **The index held the new vectors.** Every rewritten record was its own nearest neighbour, at
  distance 0.000000, except two groups:
  - 134 records whose vector another record shares exactly;
  - 11 where the approximate search returned a different record.
- **The ranking was otherwise unchanged.** The lexical-only arm was identical in all four sets.

| Set | r@8 | MRR | Golds better | Golds worse |
|---|---|---|---|---|
| English code | 0.792 → 0.833 | 0.552 → 0.559 | `extractable` missing → 5 | `degradation-log` 4 → 5 |
| Italian code | 0.625 → 0.667 | 0.453 → 0.462 | `extractable` missing → 7; `abstention`, `mcp-auth-panel` 6 → 5 | — |
| find-by-name | 1.000 | 0.750 → 1.000 | `name-rank-go`, `name-publish`, `name-keylock` 2 → 1 | — |
| office | 0.800 → 0.900 | 0.650 → 0.652 | q07 and q14 missing → 4 | q09 2 → 3; q18 2 → 5 |

**The losses are near-ties:**
- **q18:** the reserved place went to `fattura_2026_009.xlsx` instead of `fattura_2026_005.xlsx`,
  0.478 against 0.476. Both are invoice cards.
- **q09:** `contabilita_2026.xlsx` and `anagrafica_clienti.xlsx` swapped places, 0.003 apart.

**Fewer cards answered alone.** Documents that entered the top 8 with no passage at all:

| Set | Before | After |
|---|---|---|
| English | 15 | 5 |
| Italian | 18 | 5 |
| find-by-name, where a card is the right answer | 4 | 7 |

**Run-to-run noise.** The vectors were restored from a snapshot and the four sets re-run. Every
answer matched the fresh baseline except one: Italian `abstention` moved 6 → 5. One gain above is
therefore within noise. The restored corpus is the one the earlier sections measured.

**Not shipped.** The trailer and caveats are fixed sentences about Aura, not about the file.
Embedding the card without them is a filecard and ingest change. It waits on approval, together
with the false caveat on source files.

### Every caveat, and PDFs

The run above dropped two sentences and kept the other caveats: "Only the opening of the file is
described here.", the sheet-sampling lines, and every PDF line. The change proposed drops them
all. A second instrument measured exactly that:
- it rebuilt each card from its file with filecard itself;
- it checked the rebuild against the stored card, modulo invalid UTF-8;
- it rendered the card again with no caveat, no cut notice and no trailer.

Every rebuild matched except seven LibreOffice-converted fixtures (`sample.doc`, `.xls`, `.ppt`,
`.rtf`, `.odt`, `.ods`, `.odp`). The ingest cards the converted file, so those seven kept the
narrow filter.

**A PDF identity** was added to `cb-arcade`, with 30 files:
- the office fixture's 18 spreadsheets;
- its four prose files, converted to PDF by LibreOffice;
- six real PDFs: SICK, Siemens and AECO datasheets, the 53-page SIMATIC S7-1200 G2 migration
  overview, `sample.pdf`, and a TIA Selection Tool report;
- two TIA Selection Tool order lists.

The report and the order lists are the operator's own files, not in the repo. Every PDF was
extracted offline first, and all ten carry a text layer. The ingest log shows no vision call.

**The questions** are 38. Twenty are the office questions, with the prose gold moved to the PDF.
The other 18 were written by the same agent from the extracted text: 12 have a PDF gold, 4 an
order list, and 2 `sample.pdf`.

| Card text | r@1 | r@3 | r@8 | MRR | Golds better | Golds worse |
|---|---|---|---|---|---|---|
| as stored | 0.632 | 0.868 | 0.895 | 0.757 | | |
| without the trailer and "did not read" | 0.632 | 0.868 | 0.947 | 0.760 | q07, q14, p14 | q09, q18, p08 |
| without the trailer, every caveat and the cut notice | 0.632 | 0.895 | 0.947 | 0.768 | q07, q14, p14 | q09, p08 |

**PDFs gain nothing and lose nothing of their own:**
- **As stored, 21 of the 22 questions with a PDF gold** rank it first.
- **The exception is p08**, "connettore M12 PROFINET 4 poli schermato: grado di protezione". It
  ranks the AECO M12 sensor datasheet above the Siemens connector datasheet. With the change it
  moves 2 → 3 because `Configurazione.xlsx` now clears the floor and takes the reserved place.
- **The gains are spreadsheets.** q07 and q14 are the office ones, and p14 asks for an article
  number in an order list (4 → 2).

**The full variant on the other sets:**

| Set | As stored (r@8 / MRR) | Two sentences dropped | Every caveat dropped |
|---|---|---|---|
| English code | 0.792 / 0.552 | 0.833 / 0.559 | 0.833 / 0.557 |
| Italian code | 0.625 / 0.453 | 0.667 / 0.462 | 0.667 / 0.462 |
| find-by-name | 1.000 / 0.750 | 1.000 / 1.000 | 1.000 / 1.000 |
| office | 0.800 / 0.650 | 0.900 / 0.652 | 0.900 / 0.667 |

**The full variant keeps every gain.** Office r@3 stays 0.800, where the narrow one fell to
0.750, because q18 is no longer lost. English `degradation-log` goes 4 → 6 instead of 4 → 5.

**Restored and re-measured:** after restoring, all five sets matched their baseline exactly.

**Found in the PDF corpus: encrypted metadata.**
- `scheda_sick.pdf` is RC4-encrypted with an owner password only, so it opens without one.
  `pdfinfo` reads its title, "W100 WT100-P1432, Data sheet".
- filecard's metadata reader reads the raw bytes, so the card's Title, Subject and Author lines
  are ciphertext. That text is embedded.

## Through the ingest: spec acceptance 3

The change shipped as code (97fec747e: the ingest stores the card and embeds its description)
and ran as the ingest itself. One catch-up pass of an image built from master went over each
`cb-arcade` library, on that library's existing CocoIndex 1.0.25 state.

| Library | Files | `[extract]` | Embedding inputs | Of which passages | Time |
|---|---|---|---|---|---|
| code | 5,983 | 5,983 | 6,080 | 97 | 356 s |
| office | 22 | 22 | 22 | 0 | 4 s |
| pdf | 30 | 30 | 30 | 0 | 5 s |

- **`_extract` ran once for every file**, as its changed body requires. The log's line-start
  count read 5,885 and 29 because concurrent prints tear lines ("[embed] 1 chunk(s) in one
  request[extract] fattura_2026_004.xlsx"). Counting every record gives the table.
- **One embedding per card, and no existing passage re-embedded.** The code library's 97 extra
  inputs are the passages of its 85 `.css`, `.cypher`, `.jsx` and `.mjs` files, which the
  source list gained in 39a9f7290; ArcadeDB holds exactly 97 passages for those extensions.
  Every other passage kept its text and hit `embed_text`'s memo.
- **No paid call.** None of the three libraries holds an image, a recording or a video, and
  all ten PDFs carry a text layer, checked offline with the ingest's own reader first.
- **The audit** found every object indexed in all three libraries.

The five sets, over those rows:

| Set | Before (r@8 / MRR) | Through the ingest | Rewritten in place |
|---|---|---|---|
| English code | 0.792 / 0.552 | 0.833 / 0.557 | 0.833 / 0.557 |
| Italian code | 0.625 / 0.453 | 0.667 / 0.462 | 0.667 / 0.462 |
| find-by-name | 1.000 / 0.750 | 1.000 / 1.000 | 1.000 / 1.000 |
| office | 0.800 / 0.650 | 0.900 / 0.667 | 0.900 / 0.667 |
| PDF corpus | 0.895 / 0.757 | 0.947 / 0.768 | 0.947 / 0.768 |

The ingest reproduces the in-place measurement exactly, on every set. That also answers the
spec's open question 2: the source card's new head line ("nastro.go — source file, 55 bytes.")
and the 97 new passages move no production number. The lexical-only variant does move,
English MRR 0.385 → 0.401, because the cards' text changed.

## CocoIndex Code, read 2026-10-16

`cocoindex-io/cocoindex-code` (v1 source, pushed 2026-10-06) chunks code with the same
`cocoindex.ops.text.RecursiveSplitter` as `services/ingest/chunk.py`. Its settings:
- `language=detect_code_language(filename=...)`, which gives tree-sitter boundaries;
- `CHUNK_SIZE = 1000`, `MIN_CHUNK_SIZE = 250` and `CHUNK_OVERLAP = 150`.

Aura's chunker passes no `language` and budgets about 8,000 bytes, so it splits code the way it
splits prose.

Search there is vector-only, a KNN over sqlite-vec. The default model is
`Snowflake/snowflake-arctic-embed-xs`, with `nomic-embed-code` and `CodeRankEmbed` curated as
alternatives. A structural `ccc grep` sits beside the search.

## What this does not prove

- **One corpus, and an unusual one.** Its documents describe its own code, and plans often quote
  it.
- **Gold is one author's.** The same agent wrote the English questions and picked the gold, and it
  knows the code's vocabulary. That leans toward lexical overlap. The Italian questions are
  translations of the English ones.
- **24 questions.** One question moves recall by 0.042.
- **No language model read the answers.** Whether an agent then answers correctly from these 8
  documents is not measured.
- **Latency is not a measurement here.** The median was 642 ms, on a workstation running other
  work.
- **The cockpit upload path was not exercised.** The extension gate above is read from the code.
- **The ranking rules were measured on small, local sets.** The office fixture has 20 questions.
  The live 118-document corpus behind `reserveCardLane`'s original measurement
  (`docs/document-retrieval-order-pilot.json`, on the lab VM) was not re-run. The find-by-name
  questions are six, written by the same agent.
- **The card rewrite was measured on text, code and ten PDFs, not on media.** The PDF corpus is 30
  files of short, distinct documents. It does not show how a PDF card behaves among hundreds of
  manuals on one topic. The media line and the "could not read" caveat were dropped from no
  vector.
- **The levers were measured by rewriting vectors in place.** The card-vector change alone was
  then run through the ingest, which reproduced its numbers exactly. The other levers were not.
- **No media went through the new ingest**, and no route reads images in these libraries. The
  billed-call memo is proven with a stub bridge only (`test_billed_media_memo.py`).
