# CocoIndex's Neo4j connector on ArcadeDB: JVM against native image

Date: 2026-10-15. Follows [2026-10-14-arcadedb-native-image-and-go-driver.md](2026-10-14-arcadedb-native-image-and-go-driver.md),
which measured the native image idle. This run measures it under Aura's document ingest, for
Roberto Franchini (ArcadeDB maintainer).

**Verdict:**
- **Memory.** The native image held half the memory of the JVM image during the load: 1.2 GiB at
  peak against 2.4 GiB.
- **Writes.** They were slower on native: the initial load ran at 139 rows/s against 212-217.
- **Vector reads.** These are the gap that matters. On native, `vector.neighbors` took 3.7-6.6
  times as long, and Aura's fused retrieval query 6-8 times as long.
- **Full text.** It was the one read faster on native.
- **Correctness.** All three images returned the same counts and the same top hit.

Aura stays on the JVM image. The native image is worth re-measuring once its vector path closes
the gap.

## Setup (lab VM, 192.168.101.158)

**The writer.** The `aura-ingest:edge` image the VM runs, which carries cocoindex 1.0.24, the
neo4j Python driver 5.28.1 and Aura's `ingest` package. It ran a benchmark script beside the
stack, on a private Docker network:
- **Schema:** Aura's own `arcade.ensure_schema` with 768 dimensions. It creates the
  `Passage` and `IndexedDocument` types, `LSM_VECTOR` with COSINE and no quantization,
  `FULL_TEXT` with StandardAnalyzer, and the unique keys.
- **Targets:** both mounted with `neo4j.mount_table_target`, exactly as
  `services/ingest/app.py` mounts them, and the same dataclasses field for field.
- **Source:** synthetic and deterministic, with no S3, extraction or embedder, so the timings
  are the connector and the server, not a model. Passages are 150 random words (about 1 KB).
  Vectors are seeded unit vectors, all computed before any pass was timed.

**The servers.** Three ArcadeDB containers, one after the other, each with `--memory 3g` as in
compose and the Bolt plugin enlisted. The two JVM images ran with Aura's `-Xms2G -Xmx2G`:

| Label | Image | Build |
|---|---|---|
| JVM 26.10.1 | `arcadedb:26.10.1@sha256:01506549…` (Aura's pin) | release |
| JVM latest | `arcadedb:latest@sha256:811ec98b…` | 26.11.1-SNAPSHOT, 2026-10-15 |
| native latest | `arcadedb:latest-native@sha256:3fb9cf4b…` | 26.11.1-SNAPSHOT, 2026-10-10 |

`26.10.1-native` was left out because it breaks FULL_TEXT and CREATE EDGE (see the 10-14 file).
Roberto confirmed the same day that it is broken, that `latest` carries the fixes, and that
26.10.2 is due shortly.

**Passes.** 1,000 documents with 10 passages each, so 10,000 `Passage` rows plus 1,000
`IndexedDocument` rows. Each pass is one timed `app.update()`:
- **load:** every document;
- **noop:** the same corpus again, with nothing to write;
- **update:** 10% of the documents get new text and vectors;
- **delete:** 10% of the documents leave the corpus.

**Reads.** After the passes, each query ran 3 times to warm up and then 30 times measured,
over HTTP from the writer's container:
- `vector.neighbors` with k=10;
- a `SEARCH_INDEX` count;
- Aura's `fusedStatement` from `internal/arcadedb/document_retrieval.go`:
  `vector.rerank(vector.fuse(vector.neighbors {filter, maxDistance}, SEARCH_INDEX), RRF, groupBy)`,
  with permissive floors (maxDistance 2.0, minRelevance 0);
- a Cypher `MATCH` by key.

## Results

| | JVM 26.10.1 | JVM latest | native latest |
|---|---|---|---|
| `ensure_schema` (HTTP DDL) | 1.01 s | 1.40 s | 0.22 s |
| **load**, 11,000 rows | 50.65 s, 217 rows/s | 51.99 s, 212 rows/s | 78.93 s, **139 rows/s** |
| noop, nothing to write | 0.29 s | 0.27 s | 0.25 s |
| update, 1,100 rows | 5.26 s, 209 rows/s | 5.52 s, 199 rows/s | 5.87 s, 187 rows/s |
| delete, 1,100 rows | 1.98 s, 555 rows/s | 2.08 s, 529 rows/s | 2.31 s, 476 rows/s |
| Row counts after each pass | as expected | as expected | as expected |
| `vector.neighbors` k=10, median / p95 | 38.7 / 60.6 ms | 69.2 / 96.6 ms | **256.0 / 300.1 ms** |
| `SEARCH_INDEX` count, median / p95 | 18.8 / 36.3 ms | 22.9 / 60.6 ms | 15.0 / 24.4 ms |
| Aura fused query, median / p95 | 63.1 / 128.4 ms | 80.5 / 97.4 ms | **499.2 / 593.7 ms** |
| Cypher `MATCH` by key, median / p95 | 1.8 / 2.5 ms | 1.7 / 2.2 ms | 2.4 / 5.8 ms |
| Top hit of the vector and fused queries | the probe | the probe | the probe |
| RAM idle, before the bench | 2.087 GiB | 2.081 GiB | 16.67 MiB |
| RAM peak during the bench | 2,418 MiB | 2,407 MiB | **1,204 MiB** |
| RAM 10 s after the bench | 2.261 GiB | 2.214 GiB | 777.6 MiB |
| `databases` volume | 122 MB | 122 MB | 119 MB |
| Server log lines with SEVERE or exception | 0 | 0 | 0 |

RAM figures come from `docker stats`, sampled about every 2 s, so a short peak can fall between
samples.

## How the connector writes

From `cocoindex/connectors/neo4j/_target.py` at v1.0.24:
- **Transactions.** Each batch of actions opens one Bolt transaction.
- **Statements.** Inside it, every row is its own `await tx.run(...)`:
  - an upsert is `MERGE (n:Label {pk: $key_0}) SET n += $props`;
  - a delete is `DETACH DELETE`.
- **Ordering.** The statements run in sequence, with no `UNWIND`.

The write rates above are therefore one round trip per row.

What Aura has to do outside the connector, because ArcadeDB's Bolt endpoint is Cypher-only
(`services/ingest/arcade.py`):
- the `LSM_VECTOR` and `FULL_TEXT` indexes with their METADATA, which go over HTTP SQL;
- the typed `ARRAY_OF_FLOATS` property, which has to exist before the first Bolt write. Without
  it the connector's `MERGE` stores an untyped list that `LSM_VECTOR` refuses to index.

CocoIndex has no ArcadeDB connector, either at v1.0.24 or on `main` as of today. It ships 20
connectors.

## Not shown here

- **Repeats.** There was one run per image, so the write rates are single measurements. Each
  query latency is the median of 30 samples within that one run.
- **The snapshot builds differ.** The two snapshots are five days apart, so "JVM latest against
  native latest" mixes runtime and build. The 26.10.1 JVM and the JVM latest differ only by
  version, and their vector reads differ too: 38.7 against 69.2 ms.
- **The cause of the native image's vector latency** is not measured: SIMD, GC or something
  else.
- **Scale.** Real embeddings, real text, and corpora far above or below 10,000 vectors were not
  tested.
- **Concurrency.** There were no concurrent writers or readers.
- **Platform.** arm64 was not tested.

## Re-run with the scalar jvector provider (afternoon)

**Why.** Roberto Franchini traced the native image's vector latency to jvector's provider choice,
starting from this file's numbers ([ArcadeDB #10072](https://github.com/ArcadeData/arcadedb/issues/10072)):
- the native image is built with `jdk.incubator.vector`, so jvector picks
  `PanamaVectorizationProvider`;
- GraalVM CE's Substrate VM does not compile the Vector API to SIMD, so every distance runs the
  incubator's lane-by-lane Java fallback.

He suggested `-Djvector.vectorization_provider=default`, the scalar provider. #10072 was closed
the same day by [#10077](https://github.com/ArcadeData/arcadedb/pull/10077), in 26.11.1, which
picks that provider by itself under native.

**Setup.** The same script and corpus as above, on the same VM, with one change. The native
digest from the morning (`3fb9cf4b…`, built 2026-10-10, before the fix) ran twice: once without
the flag, and once with it as a trailing argument. The JVM image at Aura's pin ran first as the
same-session reference.

A first attempt ran across a VM suspend and was discarded: its wall time was 1555 s for about
250 s of work, and the session that carried its last run dropped.

| | JVM 26.10.1 | native, no flag | native, `-Djvector.vectorization_provider=default` |
|---|---|---|---|
| Provider in the server log | Panama, 512-bit species | Panama, 256-bit species | scalar (no Panama line) |
| **load**, 11,000 rows | 48.35 s, 228 rows/s | 63.33 s, 174 rows/s | 52.54 s, 209 rows/s |
| update, 1,100 rows | 5.53 s, 199 rows/s | 5.83 s, 189 rows/s | 7.00 s, 157 rows/s |
| delete, 1,100 rows | 1.88 s, 586 rows/s | 2.23 s, 493 rows/s | 2.01 s, 546 rows/s |
| `vector.neighbors` k=10, median / p95 | 38.0 / 49.8 ms | **first query timed out at 120 s** | **22.2 / 38.4 ms** |
| `SEARCH_INDEX` count, median / p95 | 19.2 / 38.8 ms | — | 4.7 / 7.2 ms |
| Aura fused query, median / p95 | 62.9 / 91.6 ms | — | 160.2 / 296.5 ms |
| Cypher `MATCH` by key, median / p95 | 1.5 / 2.6 ms | — | 1.5 / 2.9 ms |
| Top hit of the vector and fused queries | the probe | — | the probe |
| RAM idle / peak during the bench | 2.102 GiB / 2,436 MiB | 16.43 MiB / 939 MiB | 16.68 MiB / 576 MiB |
| Bench wall time | 64 s | 194 s | 72 s |
| Server log lines with SEVERE or exception | 0 | 0 | 0 |

Without the flag, the first `vector.neighbors` waited on a jvector graph build that the server
log shows starting before the queries; the client gave up after its 120 s timeout. In the
morning's run the same image answered in 256 ms. When the graph build happens relative to the
first query is therefore not stable between runs. #10072 measured that build at 178 s, against
6.5 s with the scalar provider.

**Verdict.** The flag closes the vector gap on this workload:
- `vector.neighbors` on native is faster than on the JVM: 22.2 against 38.0 ms;
- writes come within 8% of the JVM;
- the peak memory is a quarter of the JVM's.

Aura's fused query is still 2.5 times the JVM's. That query also runs `vector.rerank`. #10077
moves ArcadeDB's own Vector API paths to scalar code too, and the flag does not reach them. That
they are the remaining cost is a hypothesis, not a measurement.

**Not shown:**
- the 26.11.1 build that carries #10077: none was published when this ran, since the latest
  `latest-native` dates from 12:19 UTC and the PR closed at 14:20;
- why the fused query still lags;
- repeats: one run per configuration;
- arm64, real embeddings, other corpus sizes and concurrency, as above.

Aura stays on the JVM image until a native release carries the fix and its fused query is
measured again.
