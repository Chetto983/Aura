# ArcadeDB native against JVM: memory, and what the native image breaks

Date: 2026-10-17. The operator proposed switching to the native image ("Io passerei a native: è
più lenta ma di poco, ma consuma molta meno RAM"). This file measures both halves of that sentence
before any spec is written.

**Verdict:**
- **The native image uses far less memory.** On the 7,798-passage library it holds 30 MiB idle
  and peaks at 610 MiB under the variant bench. Aura's compose JVM holds 2.10 GiB idle and peaks
  at 2.18 GiB.
- **Most of the JVM's figure is compose's fixed heap, not the JVM.**
  - With `-Xms512M -Xmx512M` the JVM holds 605 MiB idle and peaks at 685 MiB.
  - It still runs the statements faster than the native image: post-filter 17.3 against 25.0 ms.
- **The native image is slower by 50 to 65%** on the statements that matter: about 10 ms on
  post-filter and on `where`, and about 27 ms on the shipped statement.
- **26.10.1-native, Aura's pin, is unusable and destroys data.**
  - Its binary lacks Lucene's `EnglishAnalyzer`, the analyzer every memory full-text index uses
    (`internal/arcadedb/memory.go`, `memory_conversation.go`, `memory_reasoning.go`).
  - The schema fails to load. The server logs one SEVERE line, keeps 49 of the identity
    database's 86 indexes, and rewrites `schema.json` on disk without the rest.
  - 26.10.2-native and the 26.11.1 snapshot load all 86. The fix is ArcadeDB #9495 (`e6d80b831`,
    2026-10-09).
- **The native image is "an experimental add-on" by ArcadeDB's own documentation.** The JVM
  "remains the default, fully supported way to run ArcadeDB" (`docs/native-image.md` at
  `15958b48f`). The same page says an unknown plugin is skipped without a log line.

## Memory, 7,798 passages

Each configuration started on a fresh copy of the library, with a 3 GB container limit and the
Bolt plugin enlisted. Idle is 20 s after `/api/v1/ready`, before any query. Peak is the highest
`docker stats` sample while `TestFusedQueryVariants` ran: 13 statements, 16 questions, 7 runs
each.

| Image and heap | idle | peak | shipped | post-filter | three edits |
|---|---|---|---|---|---|
| JVM 26.10.1, `-Xms2G -Xmx2G` (compose today) | 2.10 GiB | 2.18 GiB | 48.4 ms | 17.6 ms | 14.8 ms |
| JVM 26.10.1, `-Xms256M -Xmx2G` | 348 MiB | 2.19 GiB | 50.4 ms | 17.2 ms | 13.7 ms |
| JVM 26.10.1, `-Xms512M -Xmx512M` | 605 MiB | 685 MiB | 63.8 ms | 17.3 ms | 14.9 ms |
| JVM 26.11.1-SNAPSHOT, `-Xms2G -Xmx2G` | 2.10 GiB | 2.18 GiB | 46.6 ms | 15.4 ms | 11.1 ms |
| native 26.11.1-SNAPSHOT, default heap | 30 MiB | 610 MiB | 75.0 ms | 25.0 ms | 19.7 ms |
| native 26.11.1-SNAPSHOT, `-Xmx2g` | 30 MiB | 618 MiB | 85.5 ms | 26.8 ms | 20.1 ms |
| native 26.11.1-SNAPSHOT, `-Xmx512m` | 30 MiB | 305 MiB | 89.2 ms | 24.6 ms | 16.4 ms |

- **A JVM given 2 GB of heap fills it under load,** whatever its `-Xms`. Only a smaller `-Xmx`
  lowers its peak.
- **No configuration hit an `OutOfMemoryError` on this bench.** The bench is reads only.
- **The two shipped-statement figures of the JVM rows** (48.4 and 46.6 ms) match the latency
  files of the same day within one run's noise.

## What else differs in the native image

Measured or read at `15958b48f`, unless a version is named.
- **Bolt works.** The plugin is in the image, listens on 7687, and loads from
  `-Darcadedb.server.plugins=Bolt:com.arcadedb.bolt.BoltProtocolPlugin`. CocoIndex writes through
  it (compose's comment on the service).
- **There is no shell and no `wget`.** The image holds the binary and its `config/` only, so
  compose's healthcheck (`wget … /api/v1/ready`) cannot run. Three services wait on
  `service_healthy` for ArcadeDB (`compose.yaml` lines 49, 748 and 948).
- **No `JAVA_OPTS` and no `ARCADEDB_OPTS_MEMORY`.** Every `-D` setting and `-Xmx` must be a
  trailing argument to the entrypoint.
- **The linux/amd64 image is `FROM scratch` and runs as UID 0.** The JVM image runs as
  `arcadedb`.
  - The files the native image writes are root-owned with mode 600, so a JVM started on the same
    volume afterwards cannot write `schema.json` ("AccessDeniedException").
  - Switching back to the JVM is therefore not a plain image swap. Compose would need `user:` on
    the native service.
- **Gremlin is not in the native image.** Aura does not use it.
- **26.10.1-native also failed `SELECT … FROM (SELECT … FROM schema:types)`** with a
  `NoSuchMethodException` (missing reflection metadata). 26.10.2-native and the snapshot answer it.

## 26.10.1-native rewrites the schema, reproduced

1. Copy the library. Start 26.10.1-native on it. The log has `SEVERE [LocalSchema] Error on
   loading schema. The schema will be reset`, caused by `ClassNotFoundException:
   org.apache.lucene.analysis.en.EnglishAnalyzer`. `schema:indexes` lists 49.
2. Stop it with `docker stop -t 30`. `schema.json` goes from 18,047 bytes to 8,739, owned by root.
3. Start the 26.10.1 JVM on the same copy. It cannot open the root-owned file, falls back to
   `schema.prev.json`, and lists 86 indexes.

The JVM's recovery here came from the permissions accident. With the same user on both sides,
the 8,739-byte schema would have been the one loaded.

## What this does not prove

- **Memory under ingest.** Every run read only. Writing, the vector-graph rebuild, and the
  embedding sweep may need more heap than 512 MB on either runtime.
- **More than one identity.** Each identity is its own database, so memory grows with tenants.
  One was opened here.
- **The appliance.** It is a different CPU, and its RAM budget is the one that matters.
- **26.10.2-native under the bench.** Only its schema load and the FROM subquery were checked.
- **Every Aura code path on native.** Only the document read, the schema load and Bolt's
  listener were exercised: not memory writes, `cmd/arcadedb-mcp`, backup or restore.
- **One run per row, on one 4-core host.**
