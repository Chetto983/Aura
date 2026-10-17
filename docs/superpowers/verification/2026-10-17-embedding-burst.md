# Ingest embeddings bounded to the sidecar's slots

Date: 2026-10-17. Plan `docs/superpowers/plans/2026-10-17-embedding-burst.md`, spec
`docs/superpowers/specs/2026-10-17-embedding-burst-design.md`.

The workbook-row upgrade stopped on embedding timeouts. The operator chose to fix the burst
first ("Prima la raffica").

**Verdict:**
- **The burst is reproduced and fixed.**
  - The same complex upgrade that lost two workbooks to 19 timeouts now runs with one request in
    flight, no timeout, and every row passage written.
- **A catch-up that CocoIndex saw fail no longer exits 0.**
  - Before, the audit said "33 indexed, 0 missing" over 19 CocoIndex errors.
  - Now the audit line carries the error count, and any error exits 1.
- **Concurrency buys no throughput on this CPU.**
  - Sending more requests at once to a one-slot sidecar only lengthens each request.
  - That longer wait is the timeout mechanism.

## Setup

- **Host.** A 4-core cloud container. EmbeddingGemma-300M Q8_0 runs on
  `ghcr.io/ggml-org/llama.cpp:server-v0.5.0`, build b11146, with `-t 4 -np 1 -c 2048 -ub 2048`.
- **`total_slots`.** `GET /props` answers `"total_slots": 1`. The server README at b11146
  documents the field: "the total number of slots for process requests (defined by `--parallel`
  option)".
- **Storage.** ArcadeDB 26.10.1 and Garage v2.4.1.
- **Upgrade runs.** Each starts from fresh copies of the as-shipped complex index (`cb-arcade`)
  and its CocoIndex state. The ingest code is the workbook-row slice's arm A (`5d0a81a`) plus
  this plan's commits.

## Reproduced first (Task 1)

Image `aura-ingest:burst-1` from `c233147`, whose `[embed]` line carries the time and the number
of requests in flight.

| Time | `inflight=` |
|---|---|
| 09:07:01.8, first request of the batch | 1 |
| 09:07:03.8 | 16 |
| the next ~2 minutes | 16 |

- **Requests completed one every ~9 s at the single slot**, so the 16th waited past 120 s.
- **The run ended in a false clean.**
  - 19 `TimeoutError`, and two "component build failed: timed out".
  - `IngestStatus.errors = 19`.
  - The audit printed "33 objects in aura-complex, 33 indexed, 0 missing", and the process
    exited 0 after 357 s.
- **The threading-locked counter reached 16,** so CocoIndex runs `_embed_texts` in real threads.
  The plateau at exactly 16 points to a fixed cap upstream, which was not identified.

## Fixed (Task 4)

Image `aura-ingest:burst-2` from `c1b30d5`:
- `_embed_batch` holds a `threading.BoundedSemaphore` sized from `/props`, falling back to 1;
- the 120 s count only after the wait;
- the hosted route is unbounded.

| | failed run (`burst-1`) | bounded (`burst-2`) |
|---|---|---|
| peak in flight | 16 | 1, on 80 of 80 lines |
| `TimeoutError` | 19 | 0 |
| `magazzino_ricambi_2026.xlsx` row passages | 0 | 120 |
| `movimenti_analitici_5000.xlsx` row passages | 0 | 385 |
| CocoIndex errors | 19 | 0 |
| audit | "0 missing", errors not shown | "33 indexed, 0 missing, 0 errors" |
| exit | 0 (false) | 0 |
| wall time | 357 s, incomplete | 768 s, complete (534 row passages) |

Every later upgrade of the day, arms B and C of the workbook-row slice, also ran without a single
timeout: office in 550 s and 468 s, complex in 752 s and 748 s, every audit at 0 errors.

## Is parallel faster? (operator's question)

The setup was 32 inputs of ~500 tokens on the same host, measured when no other job was running.

| Configuration | inputs/min | slowest request |
|---|---|---|
| 1 slot, sequential, 8 per request | 47–50 | 11 s |
| 1 slot, one request of 32 | 50 | 39 s |
| 1 slot, 4 concurrent | 48 | 40 s |
| 4 slots (`-np 4 -c 8192`), 4 concurrent | 50 | 38 s |
| 4 slots, one request of 32 | 54 | 35 s |

- **Throughput is compute-bound.** One request already uses the four threads.
- **Concurrency against one slot multiplies each request's wait.**
- The bound of 2 that Task 4 had planned was not run, because this table already answers it.

## What was not done

- **Task 5 (all 6,027 objects of Aura's sources on this CPU, about 7 h) was cut** by the operator
  ("7 ore sono troppe … al massimo 20 minuti").
  - A 62-file subset ran instead, under 20 minutes, with the same bound and zero timeouts.
  - It holds the 36 gold files of the coding questions and the repository's 26 workbooks. Its
    result is in `2026-10-17-workbook-row-passages.md`.
- **The full code library's catch-up is unproved here.** It is acceptance 4 of the spec and stays
  open.

## What this does not prove

- **The appliance.** Its sidecar runs on the Intel iGPU through Vulkan (`compose.vulkan.yaml`),
  and no ingest rate was measured there today.
- **Why CocoIndex keeps 16 requests in flight.** The bound makes the cause irrelevant to
  correctness, not to speed.
- **A hosted route under load.** It stays unbounded, as before.
- **A multi-slot sidecar.** The bound reads `total_slots`, but only 1 was run through an ingest.
- **One run per configuration, on one host.**
