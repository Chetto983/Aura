# Alcatraz Italian fiscal code correction and VM latency

Date: 2026-10-12. Follow-up to the
[PII candidate probe](2026-10-12-pii-alcatraz-rizzo-lab-vm.md).
The operator rejected the measured 4.76 s Rizzo CPU latency for the synchronous chat
path and requested an Alcatraz fork with the CF defect corrected.

**Result:** correction implemented, committed and tested locally. GitHub publication
is deferred while the operator's account restriction is resolved. No GitHub fork was
created and no further GitHub requests were made after the operator confirmed the block.
No PII gateway was enabled in Aura.

## Reproducible correction

- Upstream: `hoophq/alcatraz`, `9a49e558c80fc54d7936fcedf2cf39cbce795eba`.
- Local corrected commit: `666d46451e154fdef4b13e56f99e3d7cd75366c4`.
- WSL working tree: `/home/davide/src/alcatraz`, branch `main`, clean after commit.
  The original repository is named `upstream`; no publication remote has been added.
- [Git patch](2026-10-12-alcatraz-cf-fix/alcatraz-italian-fiscal-code.patch), verified
  with `git apply --check` against the unmodified upstream commit.
- [Git bundle](2026-10-12-alcatraz-cf-fix/alcatraz-cf-fix.bundle), verified with
  `git bundle verify`. It contains the correction commit and requires the upstream
  commit/history as its prerequisite; it is not a complete standalone clone.
- [Artifact hashes](2026-10-12-alcatraz-cf-fix/sha256.json).

The faulty odd-position **digit** conversion was replaced with explicit numeric
conversion values shared with letters A–J, as specified by
[Finance Ministry decree, Article 7](https://def.finanze.it/DocTribFrontend/executePrintArticolo.do?articolo=Articolo+7&codiceOrdinamento=200000700000000&id=%7B8D93A450-E5FE-47FC-A6F0-F938769051BF%7D).
The previous odd-position letter lookup was already correct; the numerical string
and the addition of its individual decimal characters caused the checksum defect.
Unsupported input bytes now fail validation instead of implicitly contributing zero.

The recognizer also accepts ASCII lowercase/mixed case and omocodic digit substitutions
from [Article 6](https://def.finanze.it/DocTribFrontend/executePrintArticolo.do?articolo=Articolo+6&codiceOrdinamento=200000600000000&id=%7B8D93A450-E5FE-47FC-A6F0-F938769051BF%7D),
while preserving the original byte span and spelling. The public API and module path
are unchanged. No inference model or dependency was added.

Changed files in the external Alcatraz checkout: `recognizers/italy.go`,
`recognizers/italy_test.go`, `recognizers/validators_test.go`. This is a code correction
in the requested dependency, not copied product code inside Aura.

## Validation

Regression tests failed on the original source before the fix. The corrected version:

- Detects valid `RSSMRA80A01H501U` and `RSSMRA85T10A562S`.
- Rejects the old upstream fixture `RSSMRA85T10A562C`, whose correct check letter is S.
- Covers all 128 combinations of the seven omocodia positions, in upper/lower case,
  and rejects an incorrect check character for each combination.
- Exercises every letter and digit in both checksum position classes using an
  independently represented inverse conversion table.
- Checks exact recognition spans in prose, JSON and after a UTF-8 prefix, malformed
  inputs, default-engine registration and entity filtering.

`go vet ./...`, `go build ./...` and `go test -race -coverprofile=... ./...` passed for
the root module (seven packages). `go vet ./...` and `go test -race ./...` also passed
in all three additional modules: `lookaround`, `ner` and `pfilter`. These additional
module tests do not load live models. `git diff --check` passed.

## VM benchmark

Commands, builds, SSH and verification ran through WSL. Both upstream and corrected
binaries were compiled from the respective revisions using the same probe, then run
on the operator's VM in disposable containers: `--cpus 2`, `GOMAXPROCS=2`, 256 MiB
memory limit, read-only filesystem, no network and no access to Aura data. The active
Aura image supplied the container filesystem. Production services were not restarted.

Three runs per revision, with original/corrected execution order alternated. Each run
warmed the detector and measured 50 iterations at 2,248 bytes, 20 at 22,480 bytes and
5 at 112,400 bytes. Inputs repeat the same nine synthetic sentences from the earlier
probe. The VM continued serving its other workloads; these are lab observations, not
a latency guarantee or a statistically established optimization result.

Corrected version, **median of the three run medians**:

| Input bytes | All default recognizers | Italian + generic selection |
|---|---:|---:|
| 2,248 | 14.64 ms | 4.83 ms |
| 22,480 | 210.66 ms | 61.50 ms |
| 112,400 | 1,163.45 ms | 436.75 ms |

The 2,248-byte all-default run medians varied from 12.98 to 19.11 ms. The narrower
selection includes email, phone, IBAN, card, IP, URL, Italian fiscal code, VAT,
identity card, driving licence and passport. It **excludes other countries' specific
identifiers**, so its lower cost does not represent equal coverage. No selection was
installed in Aura; a production coverage policy remains undecided.

Every corrected VM run detected the valid CF and rejected the invalid fixture; every
upstream run showed the original inverse behaviour. The pattern-only engines still
missed the synthetic name, street address and organization. These benchmarks contain
**no NER inference, vault, restoration, network call or full Aura agent loop**.

Evidence: [summary](2026-10-12-alcatraz-cf-fix/summary.json),
[corrected run 1](2026-10-12-alcatraz-cf-fix/fixed-1.json),
[run 2](2026-10-12-alcatraz-cf-fix/fixed-2.json),
[run 3](2026-10-12-alcatraz-cf-fix/fixed-3.json),
[upstream run 1](2026-10-12-alcatraz-cf-fix/upstream-1.json),
[run 2](2026-10-12-alcatraz-cf-fix/upstream-2.json),
[run 3](2026-10-12-alcatraz-cf-fix/upstream-3.json).

## Consequence for the central gateway

The measured Rizzo CPU configuration is not acceptable on every synchronous chat
request. Corrected Alcatraz is a fast candidate for structured identifiers, but does
not establish fast, complete Italian PII detection. Names/addresses still require a
measured solution. The known NLP error-swallowing behaviour in Alcatraz was not changed
by this bounded CF fix and must be addressed before using its NLP engine as a mandatory
privacy gate.

Even the pattern-only path becomes noticeable on full conversation histories: the
112 KB fixture takes over one second with all defaults. Caching unchanged content and
processing only new input are candidates for the gateway design; neither is implemented
or measured here. The detector must finish successfully before sending the associated
text to the cloud. Deferring detection until after transmission would not meet the goal.

[Post-run check](2026-10-12-alcatraz-cf-fix/runtime.json): Aura remained healthy at
`6a3290e2f84dfcbc85b7dd5d4eb71f4b86e1a8cc`; no benchmark container remained running.
Spike 109 remains PENDING for its wider real-corpus and end-to-end gates.
