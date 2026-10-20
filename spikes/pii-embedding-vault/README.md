# Pseudonymized embedding input with an ArcadeDB vault over Redis

Measures what pseudonymizing the text Aura sends to a cloud embedder costs document retrieval,
with every replacement going through a per-identity vault in ArcadeDB spoken over the Redis wire
protocol, and checks that the vault rebuilds every original exactly. Findings: `FINDINGS.md`.

The corpus mixes the lab VM's passages (the operator's Garage files, not committed) with 33
synthetic Italian documents (`corpus.py`); every personal value in the synthetic part is invented.

1. **The vault.** `./up.sh` starts a throwaway ArcadeDB 26.10.1 (the compose pin) with the Redis
   plugin on loopback, no volume. It writes a random root password to `root.pw`.
2. **The VM's passages and their vectors.** `../embedding-cloud-models` must hold its dump
   (`dump-mem_<identity>.json`), `questions.json` with the withheld answer restored, and its
   `cache/`; its README says how to produce them. The hosted vectors in that cache are reused.
3. **The measurement.** `./run.sh [model ...]` builds `corpus.json` and runs `measure.py`. Without
   models it runs the local EmbeddingGemma sidecar (`127.0.0.1:8081`) and two OpenRouter models;
   the OpenRouter key comes from the repo's `.env.google` (`OPEN_ROUTE_API_KEY`). Vectors are cached
   under `cache/`, so a rerun costs nothing. It writes `results.json` and `results.txt`.
4. **The vault's guarantees.** `./probe.sh` runs `probe.py`, restarts the container and checks
   what survived.

`docker rm -f pii-spike-arcadedb` removes the vault.
