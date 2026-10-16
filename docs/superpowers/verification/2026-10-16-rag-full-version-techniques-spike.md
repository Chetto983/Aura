# Rag_Full_Version's techniques, measured on Aura

Date: 2026-10-16. Asked by the operator: take the best techniques of
`github.com/Souheil007/Rag_Full_Version`. That repository was read at `e32534a`, 107 files and
about 6,200 lines of Python. The two techniques that reach Aura's measured losses were spiked on
copies of the indexes of `2026-10-16-complex-documents-retrieval-quality.md`. The inventory and
what was not taken are in the consolidation spec
(`docs/superpowers/specs/2026-10-09-consolidation-best-of-design.md`, "Read 2026-10-16").

**Verdict:**
- **Workbook rows as key-value passages is the largest retrieval gain measured today.** Over the
  54 complex and office questions MRR goes from 0.812 to 0.893, and every gold is then within the
  first 8 of both sets.
- **A file title in EmbeddingGemma's title slot does nothing alone and helps only beside those
  rows,** where it separates look-alike workbooks. It lifts the complex set and costs the office
  set, so its net is small: MRR 0.893 → 0.900.
- **Neither shipped.** Rows as passages reverses a deliberate design, so it needs a spec and the
  code sets first.

## The two techniques

- **Rows as passages.** Rag_Full_Version's `BlockChunker` keeps a small table whole and splits a
  large one into groups of 15 rows with 2 of overlap. `TableSerializer.to_kv_sentences` repeats
  the header in every row, as "Header: value; Header: value". Aura gives a workbook no passages at
  all: the card answers for it and `document_open` reads it (prd.md §11, the reserved place). The
  spike gave each sheet groups of 15 rows with 2 of overlap, headed by the sheet name, with its
  first all-text row as header and values as LibreOffice cached them. It wrote them as `Passage`
  rows with the real embedder, prefix `title: none`, and set `passage_count`.
  `spikes/rag-full-version-techniques/spike_kv_rows.py`.
- **Title slot.** Rag_Full_Version prefixes every embedded chunk with its section path. The
  EmbeddingGemma model card defines the document prompt as `title: {title | "none"} | text:`,
  and says a real title "will improve model performance". Aura sends `title: none`
  (`embeddings.UntitledDocumentPrefix`, `chunk.EMBED_DOC_PREFIX`). The spike re-embedded every
  passage in place with its file name as title. Text and full-text index were unchanged.
  `spikes/rag-full-version-techniques/spike_title_slot.py`.

Rows written: 559 passages for the complex identity's 23 workbooks, and 403 for the office
identity's 18. A 5,000-row ledger alone became 385 passages and a 1,500-row store 120. On this
CPU that took about 10 minutes of embedding.

## Results

`TestCodingAgentQuestions`, production arm, at b0c264a (identifiers first included):

| Configuration | complex r@1 | r@3 | r@8 | MRR | office r@1 | r@3 | r@8 | MRR |
|---|---|---|---|---|---|---|---|---|
| as shipped | 0.794 | 0.971 | 0.971 | 0.882 | 0.500 | 0.850 | 0.950 | 0.692 |
| rows as passages | 0.912 | 1.000 | 1.000 | 0.951 | 0.700 | 0.800 | 1.000 | 0.794 |
| rows and title | 0.971 | 1.000 | 1.000 | 0.985 | 0.650 | 0.800 | 1.000 | 0.757 |
| title only | 0.794 | 0.971 | 0.971 | 0.882 | 0.500 | 0.850 | 0.950 | 0.692 |

Over all 54 questions, MRR is 0.812 as shipped, 0.893 with rows, 0.900 with rows and title, and
0.812 with the title alone.

**What rows fixed.**
- On the complex set, the store's part numbers (`parts-location`, `parts-supplier`), the
  maintenance plan (`plan-pz8`, `plan-legend`) and the rota's legend moved to first. The rota's
  legend had not been found at all.
- On the office set, q01, q02, q03, q07, q09 and q18 moved to first, and q04 entered the 8.

**What rows broke.**
- Look-alike workbooks crowd each other, because a row's text does not say which file it is in.
  The 2026 budget lost places to the 2025 one: `budget-centro-2026` 2 → 3, `budget-top-centre` 1
  → 2.
- Fourteen near-identical invoices, each with a row naming its client, pushed the answering
  files from first to fourth on q12 ("perche Bianchi ha contestato una fattura") and q15 ("la
  partita IVA di Rossi SRL").

**What the title added on top of rows.**
- It fixed all three budget questions, which went first.
- It cost `fx300-alarm-A119` one place, 1 → 2.
- On office, q13 fell 1 → 6. Its question names "la fattura 2026_006", and every invoice's title
  now carries "fattura". The first answer became `fattura_2026_006.xlsx`, which is the file asked
  about, while the gold names only the note that explains it. Part of that loss is the gold's
  strictness.

**The title alone moved nothing.** These corpora's 31 passages come from distinct PDFs and text
files, so a name adds nothing a passage did not already say.

## What this does not prove

- **The code sets were not run.** Their ingest failed on this host
  (`2026-10-16-complex-documents-retrieval-quality.md`). The title slot could matter most there,
  where a file path is the most telling title there is.
- **Rows were written by a spike, not by the ingest.** Header detection is a heuristic: a sheet
  whose first all-text row is not its header gets wrong labels. A merged title row was skipped
  only because it has one cell.
- **Aggregates are still `document_open`'s.** A row passage answers a lookup, not "how many items
  are below minimum stock". The retrieval gold counts both the same.
- **Cost.** The index grows by about one passage per 13 rows: 385 passages for one 5,000-row
  ledger. Its embedding time and storage on the appliance were not measured.
- **The title changes the embedding space.** Shipping it means bumping `RecipeVersion` and
  re-embedding every vector of every identity.
- **54 questions, written by the corpora's own generator** and the office fixture's author, one
  run per configuration.
