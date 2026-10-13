# PII cache: measured reuse on the lab VM

Date: 2026-10-12. Follow-up to the operator's request to reduce PII latency with
caching. **Component experiment, synthetic inputs, no production gateway enabled.**
Spike 109 remains PENDING. No GitHub requests or publication were attempted during
this experiment; the account restriction does not affect local builds or VM tests.

## Result

Cache each complete message/document block **after detection and replacement**.
Reuse it while its exact input, identity, detector configuration and vault scope
remain valid. An entire-prompt cache misses whenever history grows. Caching detection
offsets alone helps, but still repeats potentially expensive replacement work.

On this VM, Alcatraz's growing-history median fell from **313.73 to 19.82 ms** with
the final-message cache; completely repeated requests took **0.80 ms**. With Rizzo,
a cached document plus a new short question took **278–293 ms**, versus **6.88–7.56 s**
without reuse. A new document still took **6.77–7.76 s** with Rizzo. Caching does not
solve first analysis latency, increase detector recall, or protect unrecognized PII.

Evidence: [summary](2026-10-12-pii-cache/summary.json),
[Rizzo trials](2026-10-12-pii-cache/rizzo-cache.json),
[runtime](2026-10-12-pii-cache/runtime.json).

## Environment and method

- WSL executed all commands, Go builds and SSH. VM: `192.168.101.158`; running Aura
  commit `6a3290e2f84dfcbc85b7dd5d4eb71f4b86e1a8cc` remained healthy.
- Separate temporary containers, no published ports, `--network none`, two CPU quota.
  Go probe: 256 MiB limit, read-only root filesystem, `GOMAXPROCS=2`. Rizzo: four GiB
  limit, two PyTorch compute threads, one interop thread, offline cached weights.
- Alcatraz source is the local CF-corrected commit
  `666d46451e154fdef4b13e56f99e3d7cd75366c4`. All default pattern recognizers, language
  `it`, custom deterministic synthetic HMAC replacements. **No NER.** This misses
  names/addresses/organizations as already recorded in the initial detector probe.
- Rizzo source `30d6f9c675e245bc8494df959adaeaa9d8de35c7`; model
  `a1c3c83827eca22e9675e30c1111c4641caf5901`; `torch 2.8.0+cpu`, Transformers 5.14.1.
  The upstream detection, merge and analyze functions run unchanged through the
  same AST extraction used by the earlier probe. The desktop/PDF app is not imported.
- Caching is compared with uncached analysis of **the same complete blocks**. It is
  not compared with a single joined prompt: changing NER boundaries can change
  recognition. Every cached output was compared with its uncached reference.
- Timings cover component processing and cache lookup, excluding network, cloud
  model generation, durable vault I/O, restoration streaming and Aura's agent loop.
- The VM also runs normal services. Strategies ran in a fixed order, not randomized;
  there are only three Go trials and two Rizzo trials. These are operational
  observations, not controlled load capacity, p95 estimates or latency guarantees.

## Alcatraz: growing history

The workload starts with five blocks, then adds one user message, one approximately
2.3 KB tool result and one assistant message per turn. Twenty turns finish at
**65 blocks / 53,952 input bytes**. Each strategy then repeats that final request
twenty times. The corpus is deliberately repetitive and densely populated with
synthetic identifiers; it is not a representative real-user corpus.

The first series measured offset caching. Its later-turn median was 216.28 ms
uncached, 211.28 ms for whole-request caching and 21.33 ms for message offset caching.
A 112,400-byte document still took 987–1,072 ms on an offset-cache hit, including
replacement. That observation motivated the second series below, which reran the
same controls and added the final-message cache.

| Strategy, second series | Median turns 2–20 | Median identical final request | Detector calls across 20 growing turns |
|---|---:|---:|---:|
| Uncached | 313.73 ms | 507.72 ms | 730 |
| Whole-request output cache | 281.13 ms | 0.79 ms | 730 |
| Detection offsets per message | 35.65 ms | 20.93 ms | 65 |
| Already pseudonymized message | **19.82 ms** | **0.80 ms** | **65** |

The medians pool turns 2–20 across three trials and sixty final-request repetitions,
respectively. The first turn is a cache miss: 52.74–95.51 ms for the final-message
strategy. A growing turn still analyzes its new blocks; it does not cost 0.80 ms.

The final-message cache held 65 entries, accounting for 84,654 bytes; the offsets
cache accounted for 167,003 bytes. These are admission weights, **not measured RSS**.
The whole-request strategy is a bounded-workload comparison using a simple map;
its report's zero cache-byte field describes the unused offsets cache, not that map.

For a new 112,400-byte dense document, the final-message cache measured **2.57–3.37 s
on first use** and **0.11–1.15 ms on subsequent hits** (five hits per trial).
Caching does not remove that initial cost. Source inspection identifies why saving
only offsets leaves work behind: `anonymizer.resolve` compares candidates with
already-kept spans, and `AnonymizeWith` repeatedly copies text suffixes. No anonymizer
rewrite was made here.

All twenty turn outputs, all repeated outputs, and the final-message large-document
outputs matched the uncached references in every trial. Files:
[trial 1](2026-10-12-pii-cache/alcatraz-material-cache-1.json),
[trial 2](2026-10-12-pii-cache/alcatraz-material-cache-2.json),
[trial 3](2026-10-12-pii-cache/alcatraz-material-cache-3.json).
The first series remains in `alcatraz-cache-1.json` through `alcatraz-cache-3.json`.

## Rizzo: reuse helps history, not unseen content

Weights are loaded once and kernels warmed before measurement. This measures an
already resident model; downloading or caching model files cannot explain the gains.
The prototype caches `detect_model` results per exact complete text. Structured
patterns, merging and replacement still execute on every call.

| Request | Without cache, two trials | With model-result cache, two trials |
|---|---:|---:|
| Initial 2,248-byte document + short question; 2,321 bytes total | 7.48–9.15 s | 7.92–8.24 s |
| Same history + a new short question; 2,389 bytes total | 6.88–7.56 s | **278–293 ms** |
| Same history + new tool block; 4,663 bytes total | 15.03–15.49 s | **6.77–7.76 s** |
| Entire final request already cached | Not measured | **21.48 ms median** |

For each trial the growing requests made nine model calls uncached and four with
caching. Ten final-request repetitions per trial made zero further model calls.
The new tool block is the document with a new prefix: even that change invalidates
an exact-text cache. Preserving stable document blocks before prompt assembly is
therefore useful, but arbitrary chunking or ignoring a prefix requires separate
recognition-quality tests. Full returned analyze dictionaries matched the uncached
references, including pseudonymization and mapping data, in both trials.

This Rizzo cache is a small 32-entry LRU experiment with fixed owner/model settings.
It does not implement the full TTL/configuration/vault contract of a production
gateway. Rizzo's per-call placeholder counters also remain unsuitable as a shared
conversation vault; these measurements do not change that earlier finding.

## Cache contract exercised and proposed integration

The Go offsets cache uses a secret HMAC over length-prefixed identity, detector/
policy generation, serialized analysis options and exact text. It stores offsets,
types and scores, not raw matched values. Returned results are independent copies.
TTL is five minutes; bounds are 128 entries and eight MiB accounted weight. Errors
and oversized entries are not cached. Concurrent identical detection requests share
one in-flight computation.

The final-message prototype additionally keys on vault namespace, key version and
mapping epoch. It stores the pseudonymized complete message, which can still contain
undetected PII and must remain protected locally. A vault-availability hook rejects
serving a cached result when mappings are unavailable. **That hook is simulated**;
there is no durable vault or concurrent-revocation test. The final-message prototype
does not yet share in-flight computations, although its cache is mutex-protected.

WSL checks passed: `go vet`, `go build`, and both safety test groups under `go test
-race`. Tests exercised exact-text/identity/options/version separation, TTL, LRU,
oversized rejection, caller-copy isolation, error non-caching and concurrent reuse
for offsets. Final-message tests also exercised namespace/key/epoch changes and
vault unavailability before serving or after computing. These checks do not repair
Alcatraz's existing silent NLP-error handling: production failure propagation remains
an open gate.

For the requested central gateway, the measured direction is:

1. At the outbound model boundary, look up each complete stable block. Analyze and
   replace only a missing block, then assemble the request in its original order.
   Include every setting and context input that can change detection in the key.
2. Keep stable placeholders and a separate durable local mapping store. A cache
   eviction must not delete the mappings needed to restore old conversations. A hit
   is usable only while its mapping scope is valid and authorized.
3. Precompute document blocks during local ingestion where possible. This moves
   first-analysis cost off the chat path; it does not make the detector faster.
4. Bound and invalidate the cache when detector/model/dictionaries/policy or token
   generations change. Do not send clear text on a cache miss or detector failure.

This is **local PII processing reuse**, not the cloud model's prompt/KV cache. Stable
output equality is encouraging but does not validate Aura's `cache-audit` or provider
`cached_tokens`; those tests are still pending. Tool implementations need no individual
PII cache. Other model exits still need the shared policy as documented in the
[original exit inventory](2026-10-12-pii-alcatraz-rizzo-lab-vm.md).

## Artifacts and remaining scope

Disposable sources remain under
`.planning/tmp/pii-spike-20261012/probe-tools/cache_probe/`, plus
`rizzo_cache_probe.py`, the preparation/run scripts and collector. Their hashes and
the compiled test binary hash are in
[probe-sha256.json](2026-10-12-pii-cache/probe-sha256.json). They are experimental code,
not an Aura implementation. The local Alcatraz CF correction remains clean and
unchanged; no fork publication was attempted.

Post-run health is `ok`; no cache-probe container remains running. Normal Aura
services and routing were not changed. Real-data recognition quality, cold latency
on representative documents, concurrent load, a durable scoped vault, every outbound
model path and the full agent/tool/UI flow still require their own measurements.
