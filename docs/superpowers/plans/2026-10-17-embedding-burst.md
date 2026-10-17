# Ingest embedding bound implementation plan

**Status (2026-10-17):** NOT STARTED. The spec was approved by the operator on 2026-10-17
("Approvata, scrivi il piano e procedi con subagent").

> **For agentic workers:** Use superpowers:subagent-driven-development with
> test-driven-development. This plan runs on branch `claude/github-abuse-review-0o3kqw`, on top
> of the workbook-row slice, which is paused at its Task 5 and resumes after Task 6 here.

**Goal:** An ingest never has more embedding requests at the local sidecar than the sidecar has
slots, and a catch-up that CocoIndex reports as failed somewhere exits non-zero.

**Architecture:**
- **The bound.** `embed._embed_batch` is the one place a request is sent. It acquires a
  process-wide bounded semaphore before `urlopen`, and releases it when the response is read. The
  semaphore is sized once from llama.cpp's `GET /props` `total_slots`, falling back to 1.
  - A hosted route (`embed.MODEL` set) skips it.
  - `_embed_texts` is a sync function that CocoIndex runs off the event loop, so the semaphore is
    a `threading.BoundedSemaphore`. Task 2's RED proves that before the GREEN relies on it.
- **The `[embed]` line.** It carries a timestamp and the number of requests in flight, so the
  burst can be seen.
- **The exit code.** `__main__` keeps the last snapshot and exits 1 when its
  `stats.total.num_errors` is above 0, or when the audit finds an object missing.

**Tech Stack:** Python 3.12 with cocoindex 1.0.25 (`services/ingest`), llama.cpp server
`server-v0.5.0` (build b11146), ArcadeDB 26.10.1, Garage, Docker.

**Spec:** docs/superpowers/specs/2026-10-17-embedding-burst-design.md (approved 2026-10-17).

## Global Constraints

- **No new environment variable.** The bound comes from `/props`, and the fallback is the
  constant 1.
- **No change to the batch bounds** (`EMBED_MAX_BATCH = 32`, `EMBED_REQUEST_TOKEN_BUDGET =
  4096`), to the 120 s request timeout, or to any vector.
- **A hosted route is not bounded,** so its behaviour is unchanged.
- **No file over 600 lines.** `app.py` is at 516 lines, `embed.py` at about 300.
- **No paid run.** Every acceptance run uses the local CPU sidecar.
- **Commands:**
  - the ingest suite through
    `/tmp/claude-0/-home-user-Aura/49bdb6e4-4c0e-5ab1-bd90-48a588ea50e8/scratchpad/harness/ingest-pytest.sh`,
    which stands in for `make ingest-test` on this host;
  - `ruff check` on the files this plan touches.
- **The `/props` contract is documented.** llama.cpp `tools/server/README.md` at b11146 says:
  "`total_slots` - the total number of slots for process requests (defined by `--parallel`
  option)". Cite it in the code comment.

## Review Focus

1. **The semaphore is released on every path:** an HTTPError, a timeout, a malformed response,
   and `RetryWithSmallerBatch` raised from `_embed_texts`. A leaked permit deadlocks the next
   ingest silently.
2. **The wait is outside the deadline.** `urlopen(timeout=120)` starts after the acquire, never
   before.
3. **`/props` is read once per process,** even under concurrent first calls, and its failure
   (refused, 401 because the server has an API key, malformed JSON, no `total_slots`, 0 or a
   negative value) gives 1, never an unbounded semaphore or a crash.
4. **The hosted route never reads `/props`** and never waits on the semaphore.
5. **Exit code.** The last snapshot's `num_errors` decides it, not the first one's. Live mode
   never exits.

## Decisions taken while writing this plan

- **The open question on `/props` is closed by documentation.** At b11146 the server README
  documents `total_slots` (quoted above), and the bench sidecar answers `"total_slots": 1`.
- **Ruling: the semaphore is a `threading.BoundedSemaphore`,** not an `asyncio` one.
  - `_embed_texts` is a plain `def` wrapped by `coco.fn.as_async`, and `_execute` runs a sync
    function through `asyncio.to_thread` when it is not scheduled.
  - Task 2's RED test runs concurrent callers and must see more than one request in flight
    before the GREEN; otherwise the premise is wrong and the task stops.
  - Cost if wrong: one rewrite of the primitive.
- **Ruling: the `/props` read uses the same base URL and API-key header as the embedding request,
  with a 2 s timeout.** That is the tokenizer probe's timeout, `_TOKENIZE_TIMEOUT_S` in
  `chunk.py`. Cost if wrong: one more second at the first request.
- **Ruling: open question 2 (a bound of 1 or of 2) is measured in Task 4 with a scratch image
  only.** A bound larger than the slots is never committed unless it is faster without a timeout.
  Then the spec is amended, and still no env var is added.

## File structure

| Files | Responsibility |
|---|---|
| `services/ingest/embed.py` | the `[embed]` line's time and in-flight count; `_inflight_bound()` from `/props`; the semaphore around `urlopen` |
| `services/ingest/tests/fake_embedder.py` | the peak number of requests served at once; an optional `/props` answer |
| `services/ingest/tests/test_embed_inflight.py` (new) | the bound, the fallback, the hosted route, release on failure |
| `services/ingest/app.py` | `__main__` keeps the last snapshot; the exit code and the audit line read `num_errors` |
| `services/ingest/tests/test_audit.py` or `test_audit_cycle.py` | a catch-up with CocoIndex errors exits 1 |
| `prd.md` §11 | before the code |

## Tasks

### Task 0: PRD amendment

- [ ] In prd.md §11, beside the embedding-route paragraphs, record problems 1 to 3 of the spec
  with their numbers: the 37 queued requests, the 120 s deadline counted from the send, the two
  lost workbooks, `errors: 2` with exit 0, and the code ingest of 2026-10-16.
- [ ] Record decisions 1 and 2, and what is not proved.
- [ ] Commit: `docs(prd): bound ingest embeddings to the sidecar's slots`.

### Task 1: make the burst visible, then reproduce it

- [ ] RED in `tests/test_embed_inflight.py`: `test_the_embed_line_carries_time_and_inflight`.
  Two concurrent `_embed_batch` calls against `fake_embedder` print `[embed] <ISO time> inflight=N
  <k> chunk(s) in one request`, and one of the lines shows `inflight=2`.
- [ ] GREEN in `embed.py`: an in-flight counter under a lock, incremented around `urlopen`, and
  printed with the time.
- [ ] Run the suite and ruff.
- [ ] Commit: `feat(ingest): show when and how many embedding requests are in flight`.
- [ ] Reproduce. Build `aura-ingest:burst-1` from this commit
  (`scratchpad/harness/arm-image.sh`). Re-run the failing complex upgrade from fresh copies of
  `cb-arcade` and `ingest-state-complex`, the same pass that failed in the workbook-row Task 5.
  - Expect a `TimeoutError`.
  - Record the peak `inflight=` and its time course.
  - **If it does not time out, stop and report:** the problem is then not reproduced.

### Task 2: the bound

- [ ] RED in `tests/test_embed_inflight.py`. `fake_embedder` answers `/props` and counts the peak
  of requests served at once. The tests:
  - `test_requests_never_exceed_the_sidecars_slots`: `total_slots: 1`, 8 concurrent callers,
    peak 1. It must first fail with a peak above 1;
  - `test_a_failed_props_bounds_to_one`, for refused, 401, malformed, missing, 0 and negative;
  - `test_props_is_read_once`;
  - `test_a_hosted_route_is_not_bounded_and_never_reads_props`;
  - `test_the_permit_is_released_on_every_failure`, for an HTTPError, a timeout and a malformed
    response;
  - `test_the_deadline_starts_after_the_wait`: the `urlopen` timeout is 120 s from the acquire.
- [ ] GREEN in `embed.py`: `_inflight_bound()` (once, under a lock), and the semaphore around the
  request in `_embed_batch`, released in a `finally`. The comment cites the README sentence.
- [ ] Run the suite and ruff. 228 + Task 1 + these tests must pass.
- [ ] Commit: `fix(ingest): never send more embedding requests than the sidecar has slots`.

### Task 3: the exit code

- [ ] RED in `tests/test_audit.py` (or `test_audit_cycle.py`, beside the audit's own tests):
  - `test_a_catch_up_with_cocoindex_errors_exits_one`;
  - `test_the_audit_line_prints_the_error_count`.

  Drive `__main__`'s exit decision with a snapshot whose `num_errors` is 2 and an audit with
  nothing missing.
- [ ] GREEN in `app.py`:
  - `_update_publishing_status` returns the last snapshot;
  - the exit code is 1 when its `num_errors > 0` or the audit is non-zero;
  - the audit prints `[audit] … K missing, E errors`.
- [ ] Run the suite and ruff.
- [ ] Commit: `fix(ingest): a catch-up that CocoIndex saw fail exits non-zero`.

### Task 4: the same upgrade, fixed

- [ ] Build `aura-ingest:burst-2` from Task 3's commit. Re-run Task 1's upgrade on fresh copies.
  Expect:
  - zero timeouts;
  - the row passages of both workbooks written: 120 for the store, 385 for the ledger;
  - `[audit] 33 … 0 missing, 0 errors`;
  - exit 0;
  - peak `inflight=1`.

  Record the wall time beside the failed run's.
- [ ] Open question 2: build a scratch image with the bound forced to 2, never committed, and
  re-run. Keep 1 unless 2 is faster with zero timeouts. If 2 wins, amend the spec, then commit the
  change as its own task.
- [ ] Record everything in the verification file of Task 6. This task makes no commit.

### Task 5: the code library

- [ ] Run one catch-up of Aura's own sources (the 6,001 objects of 2026-10-16, bucket
  `aura-code`, re-uploaded from `git ls-files` with the 2026-10-16 exclusions) into a fresh
  identity, with `aura-ingest:burst-2`, on this CPU, in the background, however long it takes.
- [ ] Expect every object indexed, 0 errors and exit 0. Record the wall time, the passages and the
  peak in flight.
- [ ] This is acceptance 4. It also gives the workbook-row slice its code sets, whose gate
  needs them.

### Task 6: verification, PRD, review, push

- [ ] Write `docs/superpowers/verification/2026-10-17-embedding-burst.md`: Tasks 1, 4 and 5, with
  what they do not prove.
- [ ] Add the measured result and its citation to the PRD paragraph of Task 0.
- [ ] Final review by one fresh reviewer, on the range, this plan, the spec and the Review Focus.
  Fix its findings test-first.
- [ ] Push. Then resume the workbook-row plan at its Task 5.
