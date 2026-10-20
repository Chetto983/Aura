# Spike: hosted embedding models against EmbeddingGemma on the lab VM's documents — findings

Date: 2026-10-20. The operator, after a route change to `mistralai/mistral-embed-2312` left every
re-embed failing on the lab VM: "possiamo adeguare lo spazio in base al modello?", then
"procedi", and "usa i documenti già presenti su garage".

**Corpus.** The 86 passages the lab VM's ingest wrote for identity `448ddbe1-…`, with the
EmbeddingGemma-300M Q8_0 vectors it stored (local route, space `es1-e0aa6accf0b79c6b`):

| file | passages |
|---|---|
| `prove-craft/Financial_Sample.xlsx` (row passages) | 54 |
| `prove-craft/Costituzione_Italiana.pdf` | 26 |
| `prove-craft/calibre_demo.docx` | 2 |
| `prove-craft/SampleShow.pptx`, `prove-craft/Colosseo.jpg` (its visual description), and two chat files, `esempio.md` and `esempio_excel.xlsx` | 1 each |

Re-embedding three of them through the VM's sidecar gave the stored vectors back (cosine 1.0).

**Questions.** 35 in `questions.json`: 27 on the Constitution and 8 on the other files. They are
paraphrases written after reading the passages. Each has a string that a correct passage contains,
checked against all 86 passages; three were sharpened until they matched only the passages that
answer.

**Requests.** Each hosted model got the request Aura sends on a hosted route: the EmbeddingGemma
prefixes (`title: none | text: `, `task: search result | query: `) and `dimensions: 768`. A wider
answer is cut and renormalized by `TruncateMRL`'s rule. A model that refuses `dimensions` was
measured at its native width, which is what a space that followed the model's width would store.
The ranking is exact cosine over the 86 passages.

**Latency and cost.** Latency is the median of three single-question requests: from the
operator's PC (WSL) to OpenRouter for the hosted models, and from inside the VM to the sidecar for
the local one. Cost is OpenRouter's `usage.cost`. The whole spike cost $0.115.

## Results

| model | width | r@1 | r@3 | r@5 | MRR | Constitution r@1 | others r@1 | one question | corpus + questions |
|---|---|---|---|---|---|---|---|---|---|
| `google/gemini-embedding-2` | 768 | **32**/35 | **35** | 35 | **0.952** | 24/27 | 8/8 | 0.55 s | $0.0266 |
| `voyageai/voyage-4-large` | 1024 native | **32** | **35** | 35 | **0.952** | 25/27 | 7/8 | 0.61 s | $0.0165 |
| `perplexity/pplx-embed-v1-4b` | 768 | 31 | 35 | 35 | 0.938 | 23/27 | 8/8 | 0.81 s | $0.0041 |
| `openai/text-embedding-3-large` | 768 | 30 | 35 | 35 | 0.924 | 22/27 | 8/8 | 0.85 s | $0.0147 |
| `qwen/qwen3-embedding-8b` | 768 | 30 | 34 | 34 | 0.914 | 23/27 | 7/8 | 0.81 s | $0.0014 |
| `voyageai/voyage-4` | 1024 native | 30 | 35 | 35 | 0.914 | 22/27 | 8/8 | 0.57 s | $0.0082 |
| `nvidia/llama-nemotron-embed-vl-1b-v2:free` | 768 | 30 | 33 | 34 | 0.903 | 22/27 | 8/8 | 0.26 s | free |
| `perplexity/pplx-embed-v1-0.6b` | 768 | 29 | 34 | 35 | 0.902 | 22/27 | 7/8 | 0.65 s | $0.0006 |
| `voyageai/voyage-4-lite` | 1024 native | 29 | 34 | 35 | 0.888 | 21/27 | 8/8 | 0.59 s | $0.0028 |
| `google/gemini-embedding-001` | 768 | 29 | 34 | 34 | 0.888 | 23/27 | 6/8 | 0.71 s | $0.0199 |
| `nvidia/nemotron-3-embed-1b:free` | 2048 native | 28 | 33 | 35 | 0.886 | 21/27 | 7/8 | 0.25 s | free |
| `qwen/qwen3-embedding-4b` | 768 | 29 | 32 | 33 | 0.880 | 23/27 | 6/8 | 0.64 s | $0.0028 |
| **local EmbeddingGemma-300M Q8_0** | 768 | 29 | 32 | 32 | 0.879 | 21/27 | 8/8 | 0.05 s | — |
| `baai/bge-m3` | 768 | 21 | 27 | 29 | 0.698 | 16/27 | 5/8 | 0.63 s | $0.0009 |
| `openai/text-embedding-3-small` | 768 | 20 | 27 | 31 | 0.707 | 14/27 | 6/8 | 0.91 s | $0.0023 |
| `mistralai/mistral-embed-2312` | 1024 native | 13 | 20 | 21 | 0.491 | 11/27 | 2/8 | 0.54 s | $0.0146 |

`results.txt` has the rank of every question under every model.

## Verdicts

| # | Finding | Verdict |
|---|---|---|
| 1 | **The best hosted models beat EmbeddingGemma by a few questions on this set.** `gemini-embedding-2` and `voyage-4-large`: 32/35 at rank 1, 35/35 within 3, MRR 0.952. The local model scores 29, 32 and 0.879. Its three misses within 3 (sovereignty of the people, the repudiation of war, the reasonable length of a trial) are at rank 0 or 1 for both. | VALIDATED on this set |
| 2 | **`text-embedding-3-small`, `bge-m3` and `mistral-embed-2312` are clearly worse than the local model**: 20, 21 and 13 of 35 at rank 1. `text-embedding-3-small` misses 7 more Constitution questions than EmbeddingGemma. | VALIDATED |
| 3 | **A width that followed the model would add no recall here.** The best native-width model (`voyage-4-large`, 1024) ties the best model at 768 (`gemini-embedding-2`), and Mistral at its own width is the worst of all. | VALIDATED on this set |
| 4 | **Five models refuse `dimensions`**: `mistral-embed-2312`, `voyage-4-lite`, `voyage-4`, `voyage-4-large` and `nemotron-3-embed-1b:free`. Aura sends the field on every hosted request, so every request to these models fails, while the route preview probes without the field and accepts them. Mistral did exactly this on the VM on 2026-10-20: applied at 09:37:01, then HTTP 422 `extra_forbidden` on `dimensions` for the memory pass and every ingest file. | VALIDATED live for Mistral; for the other four, measured here only |
| 5 | **One question costs 0.25–0.91 s on a hosted route, against 0.05 s locally.** | VALIDATED, measured from the operator's PC |
| 6 | **Cost does not decide.** Re-embedding this corpus and the questions costs at most $0.027 per model. | VALIDATED |
| 7 | **A hosted route re-cuts the passages without losing text.** After the operator applied `pplx-embed-v1-4b` (10:03:57), both families were dense with 0 ingest errors. The Constitution came back as 22 passages instead of 26, with the same 109.5k characters, and every expected answer still appears. | VALIDATED (observed once) |
| 8 | **`bge-m3` takes `dimensions: 768` and returns 768 values, then ranks poorly.** Whether the provider shortens a vector the model was not trained to shorten is not traced. | PARTIAL, cause not traced |

## What this spike does not show

- **The question set is small and written by the measurer.** The 35 questions were written after
  reading the passages. That can favour phrasings a passage contains. A difference of one to three
  questions is within what another set could reverse.
- **Only the dense leg.** Aura's document retrieval adds the lexical leg and the reranker. The
  card vectors (`IndexedDocument`, embedded from a description the store does not keep) and the
  memory family were not measured.
- **Relevance floors.** They are calibrated for EmbeddingGemma, and the cockpit says so for any
  other model. This spike does not test what they cut or let through with a hosted model.
- **The hosted route's own passages.** The local passages were measured for every model, but a
  hosted route cuts the documents differently (verdict 7). Recall on those passages is not
  measured.
- **One run per model, one tenant.** Latency was measured from the PC, not from the VM, and a
  provider can route requests differently from one call to the next.
- **The free NVIDIA endpoints may keep their inputs.** The corpus sent there was the operator's
  test files.
