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
