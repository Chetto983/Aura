# ArcadeDB native image and Go driver: measured against Aura's use

Date: 2026-10-14. Asked by Roberto Franchini (ArcadeDB maintainer): size, idle RAM and start time
of the native image on Aura's appliance, and whether the new Go driver fits Aura.

**Verdict:**
- **The native image.** `latest-native` runs every statement shape Aura's memory sends. It idles
  at about 17 MiB where the JVM image idles at 2.09 GiB, and it is ready in about 1 s. It is not
  smaller on disk. `26.10.1-native` breaks full-text indexes and edge creation, so the earliest
  candidate is the first stable release after 26.10.1.
- **The Go driver.** It does not replace `internal/arcadedb`'s transport today: the server
  commands Aura's per-tenant isolation depends on are not wrapped.

## Native image (lab VM, 192.168.101.158)

**Setup.** Each image ran in a throwaway container beside the running stack, with the same
`--memory 3g` as compose. The JVM image also got Aura's `-Xms2G -Xmx2G`. Each container had the
Bolt plugin enlisted and `docker/arcadedb/backup.json` mounted.
- **Start time:** from `docker run` to `/api/v1/ready` answering 204, three starts per image.
- **RAM:** `docker stats` 30 s after ready, and again 30 s after the smoke.
- **The smoke:** 12 statements using Aura's own shapes:
  - `ARRAY_OF_FLOATS`, `LSM_VECTOR` with COSINE and no quantization, and `FULL_TEXT` with
    `EnglishAnalyzer`;
  - vertices and an edge type with properties;
  - `vector.rerank(vector.fuse(vector.neighbors, SEARCH_INDEX), RRF)`;
  - a Cypher `MATCH`.

| | JVM `26.10.1` (Aura's pin) | `26.10.1-native` | `latest-native` (26.11.1-SNAPSHOT) |
|---|---|---|---|
| Image on disk (`docker image inspect`) | 1100 MB | 1134 MB | 1310 MB |
| Compressed on Docker Hub, amd64 | — | 299.6 MB | 343.3 MB |
| Start to ready, three starts | 28.3 s / 7.1 s / 4.8 s | 1.4 s / 1.0 s / 1.2 s | 2.7 s / 1.4 s / 1.0 s |
| RAM 30 s after ready | 2.086 GiB | 16.53 MiB | 16.89 MiB |
| RAM 30 s after the smoke | 2.128 GiB | 57.68 MiB | 57.29 MiB |
| Smoke statements answering 200 | 12/12 | 8/12 | 12/12 |
| Bolt on 7687 | open | open | open |
| AutoBackupSchedulerPlugin | loaded | discovered | discovered |
| A shell for compose's `wget` healthcheck | yes | no | no |

**`26.10.1-native` failures, in the smoke's order:**
- `CREATE INDEX ... FULL_TEXT METADATA {analyzer:'...EnglishAnalyzer'}` failed with
  `IndexException`, "Error on creating index on type 'FACT'".
- Both `CREATE EDGE FACT ... SET ...` failed with `java.lang.NoSuchMethodException`. The class
  name was cut off at 160 characters in the log.
- The hybrid query failed with `SchemaException`, because the full-text index did not exist.

`latest-native` passes all three.

**What adopting it would take in compose**, from `docs/native-image.md` in the ArcadeDB repo:
- **Settings.** They move from `JAVA_OPTS` to trailing `-D` arguments; the binary does not read
  `JAVA_OPTS`.
- **The healthcheck.** The amd64 image is `FROM scratch`, with no shell, so it needs another
  probe.
- **The user.** The amd64 image runs as UID 0 unless `--user` is set, while the JVM image's
  volumes are owned by its own user.
- **Gremlin.** It is excluded, and Aura does not use it.

**Not shown here:**
- recall quality or latency on a real memory (768-dimension vectors, thousands of facts);
- backup archives actually written by the native server: the plugin was discovered, and no
  backup ran;
- the volume ownership of an existing `aura-arcadedb` volume under the native image;
- arm64.

## Go driver (`github.com/ArcadeData/arcadedb-drivers/go/arcadedb`)

**Inventory.**
- **Version.** The module proxy lists only `v0.2.0`, which targets server 26.10.1. The README's
  0.3.0, which targets 26.11.1-SNAPSHOT and breaks in-band stream errors, is not published. The
  README says other server versions are "not tested or supported".
- **Dependencies.** It needs Go 1.26, `oapi-codegen/runtime`, `coder/websocket`, `google/uuid`
  and `go-jsonmerge`. Its generated client is 42,196 lines.
- **What it covers:**
  - `Query`/`Command` with bind parameters, in SQL, Cypher, Gremlin, GraphQL and Mongo;
  - transactions on `arcadedb-session-id`, streaming, `BatchLoad`, and a WebSocket insert
    session;
  - `Vector().Search/Hybrid/Fulltext`;
  - a typed `ArcadeDBError` carrying `RequestID` and `RetryAfter`;
  - `Ready`, `Health`, `Exists` and `ListDatabases`.
- **What it does not cover:**
  - **Server commands.** Create and drop database and create user are reached only through
    `Raw()`, which does not turn non-2xx answers into errors. Aura's tenancy is built on exactly
    those: one database and one derived user per identity (`admin.go`, `tenant_clients.go`).
  - **`sqlscript`.** It is not among the language constants. Aura's `Script` uses it, from two
    callers. `QueryLanguage` is a `string`, so it could be passed, but that is untested.

**What Aura would replace.** Only the transport:
- `client.go` (415 lines), `admin.go` (255 lines) and the session plumbing in `transaction.go`;
- not the recall, provenance, fusion and tenancy logic, which is the rest of `internal/arcadedb`.

The driver's `Vector().Hybrid` endpoint is not a substitute for Aura's fused SQL. Recall depends
on four things that endpoint does not offer:
- the RID filter inside `vector.neighbors`;
- `maxDistance`;
- `vector.rerank`;
- the post-rerank relevance floor.

**Decision.** Keep Aura's transport. Re-measure when three things are true:
- the driver wraps the server commands with typed errors;
- it reaches a release that promises compatibility;
- a measured need appears that its streaming or `BatchLoad` would answer.
