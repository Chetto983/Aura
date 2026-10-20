# Hosted embedding models on the lab VM's documents

Measures dense passage recall of hosted embedding models against the local EmbeddingGemma, on the
passages the lab VM's ingest already wrote. Findings: `FINDINGS.md`.

The inputs and the cache are not committed: they hold the operator's documents and their vectors.

1. **The passages.** Copy `dump_passages.py` into `aura-ingest` on the VM and run it with the
   tenant database name, for example `python3 dump_passages.py mem_<identity>`. Without an
   argument it lists the databases. It reads Passage and IndexedDocument with the container's own
   ArcadeDB settings. Save its output next to `measure.py` as `dump-<database>.json`; `DUMP` in
   `measure.py` names the file.
2. **The local baseline.** Write `samples.json`, the texts of three passages, and
   `samples-idx.json`, their indices in the dump. Copy them with `questions.json` and
   `baseline_vm.py` into `aura-ingest`, run `python3 baseline_vm.py questions.json samples.json`,
   and save its output as `baseline.json`.
3. **The hosted models.** Run `OPENROUTER_API_KEY=… python3 -I measure.py [model …]`. Without
   models it runs the list in `MODELS`. Vectors are cached under `cache/`, so a rerun costs nothing.
   It writes `results.json` and `results.txt`.

`questions.json` withholds one answer, the one on `esempio.md`, which is personal data. Restore it
locally to rerun that question.
