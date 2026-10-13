# PII gateway: synthetic VM probe, Alcatraz and Rizzo

Date: 2026-10-12. Scope: partial evidence for spike 109. **Not a production privacy
feature, not a completed spike, and not a measurement of recall on the operator's data.**

Follow-up: the operator rejected Rizzo's measured latency on the synchronous chat
path. The [Alcatraz CF correction and expanded VM measurements](2026-10-12-alcatraz-cf-fix-lab-vm.md)
record the local corrected commit, passing checks and publication deferred while
GitHub access is restricted. The initial observations below retain their original scope.

The subsequent [cache experiment](2026-10-12-pii-cache-lab-vm.md) measures reuse per
complete message. Saving already pseudonymized Alcatraz messages reduced the growing
history median from 314 to 20 ms; Rizzo reused a document and processed a new short
question in 278–293 ms, while a new document still cost 6.77–7.76 s. This does not
validate cloud prompt caching or enable privacy filtering on normal Aura traffic.

## Environment and isolation

- Operator-provided Ubuntu VM: `192.168.101.158`. Commands, builds and SSH after the
  operator's WSL instruction ran through WSL.
- Running Aura binary: `6a3290e2f84dfcbc85b7dd5d4eb71f4b86e1a8cc`, Go 1.27.2,
  image `aura:lab-browser-keepalive-20261012`. The image's older source label does not
  identify the running binary; `aura version` does.
- Active database-selected route: provider `chatgpt`, model `gpt-6.1-sol`,
  `https://api.openai.com/v1`. Owner-scoped existing authentication was reused through
  Aura's `chatgptplan.Service`; credentials were not written into this evidence.
- Detached disposable worktree `.planning/tmp/pii-spike-20261012` uses the exact binary
  revision. Product sources, routing settings and Compose files were not modified.
- Synthetic fixtures only. The tool was a declared `prepare_contact` function; no tool
  executed and no mail, calendar event or other communication was sent.
- The cloud probe ran in a temporary `--rm` container with the same image and network as
  Aura. Rizzo ran separately with a two-CPU/four-GiB limit, no Aura data or credentials,
  and no published HTTP port. Production services were not restarted.

## Central interception point

The requested behaviour fits one privacy policy at the **outbound model client**:

```text
User + context + history + tool results
                |
                v
       local detection and replacement <--> local scoped, encrypted vault
                |
       model request with placeholders
                |
                v
             cloud LLM
                |
        response with placeholders
                |
       local scoped restoration
          /                \
   streamed UI text    tool arguments, before validation/execution
```

For Aura's text chat and side calls the existing seam is `llm.Client.Stream`, with the
wrapper installed consistently in the client factories. It sees tool results after
they have joined the model request, so individual tools do not need PII filters.
Restoration must also cover the central tool execution path, including approvals and
the execution paths which bypass `BeforeTool` (see spike 109's existing inventory).

Filtering belongs before TLS encryption. A network firewall alone sees encrypted
traffic and cannot replace individual fields. Network restrictions can enforce that
cloud calls use the approved gateway, but no such restriction was installed here.

This seam covers E1/E2, not every model exit. Embeddings in Go and CocoIndex, vision,
STT, TTS and media generation have separate clients. They need the same policy or an
explicit local-only route. Pixels/audio are not protected by a text detector. A LAN
URL is not proof of local inference: a local service can itself forward to a cloud
provider. Tool destinations also matter: restoring an email address for a local
contact operation and putting a restored name in a web search have different effects.

## Live cloud transport and restoration

Evidence: [cloud.json](2026-10-12-pii-lab/cloud.json).

The disposable probe uses the production SDK serialization and stream decoder from
the deployed commit. A recording `http.RoundTripper` examines the serialized request
immediately before TLS/network transmission; it blocks if a known fixture is present.
It does not record authentication headers. The clear baseline is intercepted locally
and never transmitted.

Six synthetic values cover a name, email, phone, address containing double quotes,
codice fiscale and IBAN. Detection in this part is **known-value matching**, deliberately
separate from evaluating Alcatraz/Rizzo. The probe is not a production detector.

- Clear baseline: **20 value occurrences** across profile/system text, user content,
  previous tool arguments, JSON tool output and a tool description. Masked baseline:
  **zero**. The scanner parses JSON strings containing nested JSON; a raw substring
  search misses escaped address quotes.
- Two placeholder formats, `[[PII_TYPE_16hex]]` and `⟦PII_TYPE_16hex⟧`, each tested twice.
  Each case requests tool arguments and then final text: **8 cloud requests per run**.
- The synthetic mapping is HMAC-based with an in-memory key. Identical value/type/scope
  produced identical tokens; the two tested scopes produced disjoint tokens. This is
  not a test of durable encrypted storage or cross-user authorization.
- All four cases preserved six placeholders in tool JSON and restored the exact
  original fields, including quotes. The schema declares the email as `format: email`;
  local restoration must precede validating such fields.
- All four final responses restored exactly relative to whole-string replacement.
  All **24 placeholder occurrences** crossed streaming delta boundaries. Replacing
  each delta independently failed in every case; the bounded pending-prefix buffer
  succeeded in all four.
- Zero known clear fixture occurrences were seen in the outbound cloud bodies.

Final run: `2026-10-12T10:47:02Z`, test duration 55.08 s, process exit **0**, including
the package's goroutine leak check. In this small run, individual tool requests took
5.34–8.54 s and final text requests 5.57–7.68 s. These are observed model timings, not
the overhead of a detector: this probe only matched known synthetic values. The three
development runs made 24 cloud requests in total; `cloud.json` contains the final eight.

Probe corrections are part of the evidence: the first run counted 18 occurrences
because its scanner did not decode JSON embedded in strings. The scanner and masker
were extended and rerun. That rerun passed the transport/restoration assertions but
the package's leak check found an idle HTTP/2 connection from the OAuth service's
default HTTP pool. The disposable harness now closes that pool before the leak check;
it does not suppress leak detection. The final report is from the corrected run.

This is one model and four synthetic cases per run. It does not establish the proposed
99% preservation gate, task success on real tools, the full agent/approval/UI loop,
cross-provider behaviour, cache preservation, retrieval quality or a fail-closed policy.

## Alcatraz

Source: [hoophq/alcatraz](https://github.com/hoophq/alcatraz), revision
`9a49e558c80fc54d7936fcedf2cf39cbce795eba` (2026-09-21).
Evidence: [alcatraz.json](2026-10-12-pii-lab/alcatraz.json).

The Go core already provides span detection, overlap handling and a custom
`anonymizer.Operator`. A prototype operator produced distinct placeholders for two
email values, reused the placeholder for a repeated email and restored the input
exactly. Those capabilities should be reused if this candidate is selected.

`go test ./...` passed all seven core packages in WSL. Pattern-only detection on the
VM took **12.36 ms p50 / 14.96 ms p95** on a 2,248-byte repeated synthetic corpus,
100 warm iterations. This is a small fixture benchmark, not throughput under load.

Findings on nine fixtures:

- Email, Italian phone, IBAN and partita IVA were detected. The defaults also identify
  part of the phone as `UK_NHS` and the partita IVA as `US_BANK_NUMBER`; Italian
  language selection does not limit the registry to Italian entity types.
- `RSSMRA80A01H501U`, which has a valid CF checksum, was missed. The upstream test's
  `RSSMRA85T10A562C` was accepted although its correct check character is `S`.
  `recognizers/italy.go` computes the odd-position digit conversion incorrectly. This
  was checked independently against the conversion tables in
  [Finance Ministry, Article 7](https://def.finanze.it/DocTribFrontend/executePrintArticolo.do?articolo=Articolo+7&codiceOrdinamento=200000700000000&id=%7B8D93A450-E5FE-47FC-A6F0-F938769051BF%7D).
- Names, streets and organizations were not found by the pattern-only engine. Optional
  `ner` and `pfilter` integrations were inspected but **not executed**.
- Static inspection of `analyzer/engine.go` found that `ProcessText` errors are ignored;
  `Analyze` continues with patterns. An NLP failure therefore does not itself block
  transmission. A required privacy gate must surface failure instead of treating
  incomplete detection as success. Fault injection remains untested.
- Default replacements such as `<EMAIL_ADDRESS>` do not provide unique reversible
  mapping. The core is a detector/anonymizer, not an LLM proxy, scoped vault or streaming
  restoration layer.

Alcatraz is a possible Go component, but its CF validator and NLP failure behaviour
must be addressed before it is accepted as the mandatory Italian privacy boundary.

## Rizzo

Source: [Rizzo-AI-Academy/rizzo-pii](https://github.com/Rizzo-AI-Academy/rizzo-pii),
revision `30d6f9c675e245bc8494df959adaeaa9d8de35c7` (2026-09-25).
Model: [rizzoaiacademy/rizzo-pii-0.3B](https://huggingface.co/rizzoaiacademy/rizzo-pii-0.3B),
release `v1.5.0`, resolved commit `a1c3c83827eca22e9675e30c1111c4641caf5901`.

This candidate already combines Italian token classification and structured-data
patterns. `src/app/detectors.py` uses only the Python standard library and includes CF,
IBAN, P.IVA and card checksums. The CF detector masks a canonical CF-shaped string even
if its checksum fails, while marking it as unvalidated; omocodic forms require a valid
checksum. All **10 upstream CF/omocodia tests** passed in WSL.

The desktop application's `analyze` function creates `counters`, `seen` and `mapping`
anew per call. It is suitable for a document-oriented copy/restore workflow, but those
short counters are not a conversation vault and must not be shared across identities.
The returned mapping and preview segments contain originals and must remain local.
The application is not an automatic LLM gateway and provides no LLM streaming adapter.

The detector/model should be evaluated separately from the desktop/PDF application.
The source and model cards declare MIT; `THIRD_PARTY_LICENSES.md` identifies PyMuPDF in
the desktop bundle as AGPL. This probe does not import that bundle or use its PDF code.

### Actual CPU inference on the VM

Evidence: [rizzo.json](2026-10-12-pii-lab/rizzo.json). The existing TTS image supplied
Python 3.12, `torch 2.8.0+cpu`, `transformers 5.14.1` and `huggingface-hub 1.6.0` in a
separate container. No packages were installed into a production container. The
1,230,273,700-byte safetensors artifact and tokenizer were downloaded first; the final
inference run used `--network none` and offline model-loading flags.

The harness executes the upstream `chunk_text`, `detect_model`, `_merge`, `_norm` and
`analyze` function definitions unchanged, extracted with Python's AST, plus its actual
`detectors` module. This avoids importing the Flask/PDF desktop application. It is a
component test, not a test of the application's HTTP API or a deployed Aura sidecar.
An initial harness invocation passed `local_files_only` twice through the Transformers
pipeline API; that failed before inference. The corrected offline run exited **0**.

| Measurement | Result |
|---|---|
| Structured-data patterns | Exact matches for all six structured fixtures, including masking an invalid-checksum CF as unvalidated |
| Model added coverage | Person name; street, building number and city; organization, all absent from pattern-only results |
| Combined detection/restoration | All nine synthetic examples masked their expected values; 9/9 restored exactly |
| Warm combined benchmark | Same 2,248-byte corpus as Alcatraz, 3 chunks, 10 iterations: **4,755.6 ms median**, 5,096.8 ms maximum |
| CPU settings | Two PyTorch compute threads, one interop thread, Docker two-CPU limit |
| Peak process RSS | **1,310.7 MiB** (about 1.28 GiB), not whole-container/VM memory |
| Model/pipeline load | 8.73 s after artifacts were cached |
| Mapping scope | Repeated name within a call reused one token; separate calls assigned `[FULLNAME_1]` to different people |

The model examples are simple and synthetic, not unseen labelled user documents. They
demonstrate added coverage, **not** 100% recall or precision. Confidence scores are
model outputs and are not measured probabilities. The Alcatraz process did not have
the same CPU quota, so these timings are operational observations, not a controlled
performance comparison. Ten Rizzo iterations do not justify a reliable p95 estimate.

Rizzo is the stronger next candidate for Italian detection from this smoke test. It
already includes structured patterns; adding a second complete detector by default
would need evidence of improved coverage. Its measured CPU cost also makes repeated
rescanning of a growing prompt a concern. Caching unchanged inputs or optimizing the
model are candidates for the next measurement, not features demonstrated here.

## What remains before a production decision

Spike 109 stays **PENDING**. Its original gates still require a labelled real Italian
corpus, comparison of pattern/model/union recall and precision by type, a second cloud
model, real tool/approval/UI flows, stable conversation scope and vault persistence,
failure injection, cache/retrieval measurements, and the other model exits. No automatic
PII protection has been enabled in the normal Aura process by this work.

Temporary probe sources remain in `.planning/tmp/pii-spike-20261012`; their hashes are
recorded in [probe-sha256.json](2026-10-12-pii-lab/probe-sha256.json). The JSON evidence
contains only synthetic fixture values and measurements.

Post-run [runtime check](2026-10-12-pii-lab/runtime.json): Aura health remained `ok`,
the active `chatgpt` / `gpt-6.1-sol` route was unchanged, and neither temporary probe
container remained running. The pinned Rizzo model cache remains under
`/home/aura/rizzo-pii-spike-20261012` for repeatability; no PII service was left listening.
