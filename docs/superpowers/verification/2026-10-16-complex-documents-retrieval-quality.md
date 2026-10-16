# Retrieval quality on complex PDFs and workbooks

Date: 2026-10-16. Asked by the operator: beyond speed, guarantee quality. Measure the fused
query's variants and the production retriever with gold, and add complex PDFs and Excel files.

**Verdict:**
- **On complex documents the full-text leg alone ranks better than production at first place.**
  Over 34 questions production finds the gold among the first 8 as often (0.971), but first for
  25, against 31 for full-text alone.
- **The losses are exact identifiers and workbooks.** Production puts the gold first for 1 of 5
  exact-code questions and 1 of 4 matrix questions. Full-text alone puts it first for 4 and 3.
- **The query variants change nothing measurable on these sets.** Every variant, the maintainers'
  three edits included, gives answers identical to the shipped statement on all 54 office and
  complex questions. These sets hold too few passages to tell the variants apart.
- **The code sets are not measured yet.** Their ingest needs about ten hours on this CPU.

## Setup

- **Corpus:** `scripts/fixtures/document_retrieval_eval/complex_corpus/`, 11 files built by
  `spikes/document-routing-benchmark/generate_complex_corpus.py` (3efe1f3).
  - Two 15-page packaging-machine manuals, FX300 and FX200, with the same layout and different
    alarm causes. Each holds three tables that run over several pages: 120 alarms, 150 spare
    parts, 80 panel parameters. They also have two-column pages and footnotes.
  - A 4-page supply contract, a quality report with a landscape page, an audit report and a
    16-section safety data sheet. Every PDF has a text layer.
  - Five workbooks: two budgets (2025 and 2026, same layout), a 1,500-row spare-parts store with
    a VLOOKUP and data validation, a machine-by-week plan with merged headers, and a shift rota.
    LibreOffice recalculated and re-saved each one, so every formula carries its value.
- **Questions:** `complex_qrels.json`, 34 questions built from the generator's facts. Each gold
  document and expected answer is therefore known by construction, not judged.
- **Identities:**
  - **office:** the office fixture without its scanned PDF, 22 files, `qrels.json`'s 20 questions;
  - **complex:** the 11 complex files plus the same 22 office files as distractors, 34 questions.
- **Pipeline:** the real ingest, `python -m ingest.app`, one catch-up pass per identity.
  - It ran in an image built from `docker/aura-ingest/Dockerfile`'s steps. The base images came
    from Google's Docker Hub mirror, and the proxy's CA was installed. The Go binaries were built
    on the host from the same source, because the build container could not reach the proxy.
  - Its source was `services/ingest` at 70487a7, with CocoIndex 1.0.25.
  - Garage v2.4.1 with `docker/garage/garage.ipv4.toml`, because this host has no IPv6.
  - ArcadeDB 26.10.1, Aura's pin, with compose's memory and Bolt settings.
  - EmbeddingGemma-300M Q8_0 on CPU, compose's `-np 1 -c 2048 -ub 2048`, in space
    `es1-e0aa6accf0b79c6b`.
- **Audit:** office 22 of 22 objects indexed, complex 33 of 33. The complex identity holds 27
  passages, all from its PDFs and text files: a workbook has no passages by design.

## Production retriever

`TestCodingAgentQuestions`, limit 8:

| Set | Arm | r@1 | r@3 | r@8 | MRR |
|---|---|---|---|---|---|
| office | production | 0.500 | 0.850 | 0.950 | 0.692 |
| office | full-text only | 0.600 | 0.700 | 0.800 | 0.662 |
| complex | production | 0.735 | 0.971 | 0.971 | 0.848 |
| complex | full-text only | 0.912 | 0.971 | 0.971 | 0.941 |

By category on the complex set, the count of questions answered first:

| Category | Questions | Production | Full-text only |
|---|---|---|---|
| EXACT (a code: complaint, non-conformity, spare part, UN number) | 5 | 1 | 4 |
| MATRIX (plan and rota workbooks) | 4 | 1 | 3 |
| LOOKALIKE (FX300 or FX200, 2025 or 2026) | 4 | 3 | 4 |
| AGG | 6 | 5 | 5 |
| PROSE, TABLE, TABLE_DEEP, PARAPHRASE, CROSS_DOC | 15 | 15 | 15 |

**Where production loses first place,** the document above the gold is one with passages:
- `report-complaint`, "che difetto riguardava il reclamo RC-2026-040", ranks
  `note_pagamenti.txt` first and the quality report third. Only the report holds that number.
- `parts-location` and `parts-supplier` rank the FX200 manual first. The codes they ask for are
  `RIC-10959`, which appears only in the FX300 manual, and `RIC-16377`, which no PDF holds. The
  FX200's spare-parts table wins by similarity, not by the code.
- The budget, plan and rota questions get their workbook second, through the reserved place.

This is the same lever the coding-question run of the same day named: the final `vector.rerank`
orders by cosine alone, so a lexical first place does not survive it.

**The ranking-rule arms** move nothing on the complex set except memory's quota, which loses
r@1, from 0.735 to 0.647. Two or three reserved places lift office r@3 from 0.850 to 0.950.

## Query variants

`TestFusedQueryVariants` on both identities, every variant of 70487a7:
- **identical answers on all 20 office and all 34 complex questions:** no filter, full-text
  LIMIT :fetch, the dropped filter with the rerank limited to 80, all three maintainers' edits,
  the rerank limited to 80, the post-filter, the filter read from the index, the filter as one
  list, and the post-filter with fetch 100;
- **different answers:** fusion order with the cosine floor, on 10 office and 13 complex
  questions; the full-text leg without stopwords and in memory's shape, on 12 and 13 complex
  questions. None moved recall or MRR by more than one question.

The passage-level numbers stop at r@10 0.647 on the complex set because 12 of its 34 golds are
workbooks, which have no passages to rank. Latencies are 2-3 ms on corpora this small and are not
a measurement of the variants: the 7,798-passage run of the same day is.

## Found along the way

- **A ranking read seconds after a re-ingest can differ.** Right after a pass rewrote the
  workbooks' cards, `TestCodingAgentQuestions` stopped on its own check: production at limit 8
  returned `piano_manutenzione_2026.xlsx` second, and the whole ranking did not return it. Minutes
  later both limits agreed, three calls out of three. Not explained, and not reproduced.
- **An ArcadeDB call ended with EOF under load.** With the embedder saturated by the code ingest,
  one card query of `q03` failed with `EOF` and no error in the server log. Alone and idle, the
  same question passed. Not reproduced.
- **The office fixture no longer regenerates byte for byte.** `generate_corpus.py` writes files
  that differ from `corpus.sha256`, before and after this session's refactor alike. The
  generator's output is stable between runs, so the drift comes from the environment, most likely
  the openpyxl version.
- **The code ingest is CPU-bound on embedding:** about 28 inputs a minute. A real ingest chunk
  is about 2,000 tokens, against the 1,000 characters of the earlier synthetic load.

## What this does not prove

- **The code sets are missing.** The 24 English, 24 Italian and 6 find-by-name questions over
  Aura's sources were not run: their ingest was still embedding when this was written.
- **One author wrote the corpus and its questions,** and knew the documents. The gold is
  objective, but the wording leans toward the documents' own words, which favours full-text.
- **Small sets.** One question moves r@1 by 0.029 on the complex set and 0.050 on office.
- **Synthetic content.** The documents are generated: their prose repeats a few sentences, and
  real manuals are longer and less regular.
- **No language model read the answers.** Whether an agent answers correctly from what it gets
  is not measured, although `complex_qrels.json` carries each expected answer for that.
- **No scanned PDF, image or recording went through the ingest.**
