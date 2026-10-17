# Ingest embeddings: no more requests than the embedder has slots

Proposed on 2026-10-17 at `abc97e7`.
- **Requested by the operator,** after the workbook-row slice stopped on its own gate
  ("Prima la raffica"). That slice is `2026-10-17-workbook-row-passages.md`, at Task 5.
- **Approved by the operator on 2026-10-17** ("Approvata, scrivi il piano e procedi con
  subagent"); the plan is `docs/superpowers/plans/2026-10-17-embedding-burst.md`.
- **The decisions below shipped,** as measured in
  [the verification](../verification/2026-10-17-embedding-burst.md).

Measured in `.superpowers/sdd/2026-10-17-workbook-row-passages/task-4-5-report.md` and in
`docs/superpowers/verification/2026-10-16-complex-documents-retrieval-quality.md`. The
verification file this slice writes carries both forward.

Risk tier: **medium.**
- It changes how every ingest embeds, on every route.
- It changes the catch-up's exit code.
- No schema, no migration, no change to any vector.

## The problem, measured

**1. Requests queue at a one-slot embedder until they time out.**
- **The appliance's sidecar** runs llama.cpp with `-np 1`. Its `GET /props` reports
  `total_slots: 1`, measured on the bench embedder on 2026-10-17.
- **The ingest's deadline.** `embed._embed_batch` sends each request with
  `urllib.request.urlopen(timeout=120)`, and the 120 s count from the send, queueing at the server
  included.
- **One large workbook passes.** The 5,000-row ledger alone ingested cleanly: 385 row passages,
  about 2 requests in flight, 410 s, no timeout.
- **Two large workbooks fail.** The arm-A upgrade of the complex library embedded two of them in
  one pass:
  - about 17 requests (136 inputs) were queued at the first `TimeoutError`;
  - a second pass, retrying only those two files, reached about 37 requests (291 inputs) and
    timed out again;
  - both files kept their old documents with no row passages.
- **The code ingest of 2026-10-16 failed the same way.** 5,918 requests waited past 120 s, and 20
  of 6,001 objects were indexed.

**2. The source of the concurrency is not known.**
- `embed_text` is `coco.fn.as_async(batching=True, max_batch_size=32)`.
- Reading cocoindex 1.0.25 (`_internal/function.py`, `_create_batcher`) shows the facts below.
  The Rust core's scheduling was not read.
  - With batching and no runner, each batcher gets its own `BatchQueue`.
  - A batcher is dropped when it goes idle.
  - The app runs up to 1,024 components in flight (`_DEFAULT_MAX_INFLIGHT_COMPONENTS`).
- Why one workbook keeps about 2 requests in flight and two keep 37 was not measured.

**3. A failed rewrite passes the catch-up.**
- The complex upgrade's audit printed "33 indexed, 0 missing" and the process exited 0.
- CocoIndex had counted the two failures. The library's `IngestStatus` row says `errors: 2`,
  read on 2026-10-17.
- `audit_pass` compares the bucket's keys with the index's. A document whose rewrite failed
  keeps its old row, so it is not missing, and the exit code (`app.py`, `__main__`) looks at
  nothing else.

## Goal

An ingest never has more embedding requests at the local sidecar than the sidecar has slots, so
none waits past its deadline in the server's queue. A catch-up that CocoIndex reports as having
failed somewhere exits non-zero.

## Decisions (shipped)

### 1. A bound on requests in flight, sized from the sidecar

- **Where.** `embed._embed_batch` takes a process-wide semaphore before it sends a request, and
  releases it when the response arrives.
- **The deadline counts only the request itself.** The wait for the semaphore happens before
  `urlopen` and is not part of its 120 s.
- **The size is the local sidecar's `total_slots`.**
  - It is read once, from `GET /props` on `AURA_EMBED_BASE_URL`, when the first request goes out.
  - Go already reads llama.cpp's `/props` the same best-effort way (`internal/llm/llamacpp_caps.go`).
  - If `/props` fails or has no `total_slots`, the bound is 1. A one-slot sidecar is the
    appliance's shape, and it is never wrong for it.
- **A hosted route is not bounded.** That route is the one with `AURA_EMBED_MODEL` set
  (`embed.MODEL`).
  - Batching measured 25.9× there because each request was a network round trip.
  - Its provider sets its own limits, and a 429 is not a queue timeout.
- **The same shape exists in the library.** CocoIndex's own LiteLLM embedder offers
  `max_inflight_requests`, an `asyncio.Semaphore` (`ops/litellm.py`). Aura does not embed through
  LiteLLM, so the bound is Aura's, in the same form.
- **No new env var.** The size comes from the server, and the fallback is a constant.

### 2. The catch-up fails when CocoIndex reports errors

- **The exit code.** `__main__` exits 1 when the last snapshot's `stats.total.num_errors` is
  above 0, as well as when the audit finds a missing object.
- **The log.** The audit prints the error count beside "N objects …, M indexed, K missing".
- **Live mode** keeps publishing the count to `IngestStatus`, as it does today.

### 3. What is not done

- **No retry on timeout.** With the bound, the queue is client-side and nothing should wait past
  120 s. A split on timeout (`RetryWithSmallerBatch`) would only make more requests.
- **No change to the batch bounds:** 32 inputs, 4,096 tokens per request.
- **No `RateLimiter`.** CocoIndex's limiter bounds rows per second, not requests in flight.

## Shape

| File | Change |
|---|---|
| `services/ingest/embed.py` | the semaphore, its size from `/props`, the hosted route left unbounded |
| `services/ingest/app.py` | the exit code reads `num_errors`; the audit line prints it |
| `services/ingest/tests/fake_embedder.py` | reports the most requests it ever served at once, and can answer `/props` |
| `services/ingest/tests/test_embed_inflight.py` (new) | the bound, the fallback, the hosted route |
| `services/ingest/tests/test_audit*.py` | a pass with CocoIndex errors exits 1 |
| `prd.md` §11 | the amendment, before the code |

### New env vars

None.

## Acceptance

1. **Reproduce first, with time.**
   - Re-run the failing arm-A complex upgrade at `5d0a81a`, with a timestamp and the in-flight
     count on every `[embed]` line.
   - It must time out again.
   - Record the peak in flight.
2. **Unit, Python.**
   - Against `fake_embedder` answering `total_slots: 1`, a pass over two large workbooks never
     has more than one request at the server.
   - The bound falls back to 1 when `/props` fails.
   - A hosted route stays unbounded.
   - A pass with a CocoIndex error exits 1.
   - The suite stays green: 228 at `5d0a81a`.
3. **The same upgrade, fixed.** The same volumes and the same two workbooks must pass with:
   - zero timeouts;
   - every row passage written;
   - the audit at 33 of 33 with 0 errors;
   - exit code 0.

   Wall time is recorded beside the failed run's.
4. **The code library.** One catch-up of Aura's own sources, the 6,001 objects of 2026-10-16, on
   this CPU. It must end with every object indexed and 0 errors, however long it takes, and that
   time is recorded.
5. **The workbook-row slice resumes** at its Task 5 on top of this.

## Open questions

1. **Does llama.cpp document `total_slots` on `GET /props`** in the release Aura pins? Read the
   server README for that tag before the plan. The bench build `b11146` serves it.
2. **Is a bound of `total_slots` the fastest?** One request queued behind the running one might
   keep the slot busier. Acceptance 3 can compare a bound of 1 with one of 2, and the plan keeps
   whichever is faster without a timeout.

## What this spec does not prove

- **Why CocoIndex keeps that many requests in flight.** The bound makes the cause irrelevant to
  correctness, but not to speed.
- **The appliance.** Its sidecar is the same one-slot llama.cpp, but its CPU and corpus are not
  this host's.
- **A hosted route under load.** It stays unbounded, as it is today.
