---
spike: 109
idea: pii-pseudonymization
name: pii-pseudonymization
type: comparison
validates: "Given an operator's real Italian mail, calendar, documents and conversations on the lab VM, when every request Aura sends to a cloud model carries keyed placeholders instead of personal data, and the placeholders are restored only in tool arguments and in what the operator sees, then the measurements show how much personal data leaves Aura today, whether rizzo-pii adds recall over checksum-validated patterns, whether real cloud models carry placeholders into tool calls intact, and what it costs in task success, latency, KV-cache reuse and retrieval quality"
verdict: PENDING
related: []
tags: [pii, gdpr, pseudonymization, llm, embeddings, ingestion, cocoindex, multimodal, rizzo-pii, kv-cache]
---

# Spike 109: Personal data before every cloud model

## Design handoff — 2026-10-13

The operator requested a spec without a branch. The
[model-egress design](../../../docs/superpowers/specs/2026-10-13-pii-model-egress-design.md)
records the measured findings and the remaining acceptance gates. It is proposed,
not an implementation or a completed spike. Its final-payload boundary and restoration
before tool-call consumers supersede the exploratory wrapper/`execTool` seams below.
The historical prototype-branch instructions below do not apply to this handoff.

## Partial measurement — 2026-10-12

[VM evidence and candidate comparison](../../../docs/superpowers/verification/2026-10-12-pii-alcatraz-rizzo-lab-vm.md).
Synthetic transport/restoration on the deployed Aura client and active `gpt-6.1-sol`
route passed: 20 clear baseline occurrences intercepted before network, zero after
masking, 4/4 tool argument and text roundtrips; all 24 output placeholders crossed
stream deltas and required buffering. Alcatraz's pattern-only VM probe found a CF
checksum defect and confirmed its native custom replacement operator is reusable.
Rizzo's model plus existing patterns covered all nine synthetic examples, including
names/addresses/organizations missed by patterns alone; all nine restored exactly.
On two VM CPUs, 2,248 bytes took 4.76 s median with a 1,310.7 MiB peak process RSS.
The operator rejected that Rizzo latency for synchronous chat. A
[follow-up local Alcatraz correction](../../../docs/superpowers/verification/2026-10-12-alcatraz-cf-fix-lab-vm.md)
fixes CF validation, adds lowercase/omocodia coverage, and passes root/submodule race
tests. Pattern-only VM medians are 14.64 ms for 2,248 bytes with all defaults and
1.16 s for 112,400 bytes; full-history rescanning remains a concern. Publishing the
requested GitHub fork is deferred while the operator's account is restricted.
The [cache follow-up](../../../docs/superpowers/verification/2026-10-12-pii-cache-lab-vm.md)
measured 730 → 65 detector calls across twenty growing turns. Alcatraz's already
pseudonymized message cache reduced the later-turn median from 314 to 20 ms, with
0.80 ms for an entirely repeated request. Rizzo processed a new short question with
cached history in 278–293 ms; a new document still required 6.77–7.76 s. Output
equivalence passed; this is local processing reuse, not the cloud KV-cache gate.
These measurements do **not** close the real-corpus, second-model, full-agent,
durable-vault, failure-mode, cache/retrieval or non-text-exit gates below. Verdict
remains **PENDING**; normal Aura traffic has not been switched to a PII gateway.

## Q6 measured — 2026-10-20

[Embedding-input findings](../../../spikes/pii-embedding-vault/FINDINGS.md): 33 synthetic Italian
documents mixed with the lab VM's 86 passages, 98 questions, perfect detection, every replacement
through a per-identity ArcadeDB 26.10.1 vault over RESP. Under Aura's cosine re-rank the spec's
opaque references cost 6–20 of 98 questions at rank 1; consistent surrogates cost nothing on
`gemini-embedding-2` (83.0 against 82 in clear) and 1.6–2.6 on EmbeddingGemma and `pplx-embed-v1-4b`;
OpenRouter's type-only Sensitive Info redaction is the worst and cannot be restored. The vault
rebuilt 217 of 217 texts byte-exact and refused cross-identity reads. Q6's gate now has numbers;
the choice between pseudonymized cloud embeddings and local-only is the operator's. Verdict stays
**PENDING**: detection on real data, the chat path and the other questions are untouched.

## What this validates

Operator clarification, 2026-10-13: opening original Excel/PDF files bypasses
ingestion-only masking. The [egress audit](../../../docs/superpowers/verification/2026-10-13-pii-egress-audit.md)
therefore retains the final outgoing model request, before TLS/network transmission,
as the enforcement boundary. Keep original local retrieval representations and resolve
model-returned PII tokens before tool dispatch. Ingestion can prepare cached masked
blocks alongside local embeddings; that parallelism is not yet measured and cannot
replace the final check. Media projection and non-chat model clients must be covered.

Consolidation item 7 (`docs/superpowers/specs/2026-10-09-consolidation-best-of-design.md`),
widened by the operator on 2026-10-10 from "the chat route" to **every cloud model Aura calls,
ingestion included**. The behaviour comes from PMSync's chat pipeline: pseudonymize before the
model, restore before the tool. "Industrial" means the five things PMSync does not do, each
of which this spike measures or settles before a spec is written:

1. **Every exit, not the chat path.** Inventory below: the chat model is one of eight.
2. **Keyed, stable placeholders.** PMSync's placeholder is `simpleHash(value)` with no key
   (`pii-detection-sentinel.service.ts:283-285`): a guessed value can be confirmed by hashing
   it, and equal values collide across tenants. Here: HMAC under a per-identity key
   (`secret.IdentityKey`, `internal/secret/sealer.go:101`), so the same value gives the same
   placeholder for one identity, and nothing outside Aura can test a guess.
3. **A persistent vault.** PMSync keeps the map in Redis for 30 minutes
   (`pii-resolution.service.ts:44-45`). Aura's conversations live for weeks and survive restarts,
   so a placeholder in history must stay resolvable: sealed per identity in Postgres, the
   pattern of `identitykey.Store` and `chatgptplan`.
4. **Validated detection.** PMSync's codice fiscale check is length only
   (`pii-detection-sentinel.service.ts:87-95`). Checksums for CF, P.IVA, IBAN (mod-97) and
   cards (Luhn), plus a model for what no pattern finds: names, streets, organizations.
5. **A decided failure mode.** What Aura does when the detector is down is a decision this
   spike gives evidence for, not a default.

What PMSync does and this spike carries over as behaviour (not code; PMSync is TypeScript):
redact the user message, context blocks and tool results walked field by field
(`ai-chat/utils/model-redaction.ts`), restore placeholders in tool arguments before
execution and in the stream to the client, audit every restoration.

## Exits: where Aura sends content to a model that can be cloud

Located 2026-10-10 by reading the code; Q1 measures what actually flows through each.

| # | Exit | Carries | Code | Cloud when |
|---|---|---|---|---|
| E1 | Chat completions: turns, swarm workers, `agent_job`s, finalize | text, tool results, history, profile | `llm.Client.Stream` (`internal/llm/client.go:102`), built per identity in `runner_identity_llm.go:83` | route is not `IsKeylessLocalBaseURL` (`internal/llm/keyless.go:11`); ChatGPT plan always |
| E2 | Side calls on the same client: reasoning teacher, compaction, title | transcript text | `llm_agent_reasoning.go:77`, `conversations/compaction.go:171`, `conversations/title.go:53` | as E1 |
| E3 | Embeddings, Go: conversation projection, memory, queries | text | `internal/embeddings/client.go:102`, `runner_memory_projection.go:156` | `AURA_EMBED_CLOUD_BASE_URL` / hosted embed route |
| E4 | Embeddings, ingestion (CocoIndex) | every document chunk | `services/ingest/embed.py:219-226` | `AURA_EMBED_MODEL` set (hosted route) |
| E5 | Vision: chat images, ingestion of images and scanned PDFs | **pixels** | `internal/multimodal/vision.go:110`, `cmd/aura-media-index/main.go:58-79` | primary model accepts images (`VisionConfigFrom`) |
| E6 | Speech to text | **audio** | `multimodal.STTConfigFrom` | `AURA_STT_CLOUD_MODEL` set |
| E7 | Text to speech | the reply, restored | settings `AURA_TTS_CLOUD_VOICE` | cloud voice selected |
| E8 | Image and video generation | prompts | `internal/mediagen/client*.go` | always (OpenRouter) |

E5 and E6 cannot be redacted as text, and E7 receives the values already restored. Q7 is
about them.

## Seams for a prototype

- **Pseudonymize:** a wrapper around `llm.Client` placed where clients are built
  (`newLLMClient`, `cmd/aura/llm_client.go:47-58`, and the per-identity factory) covers E1-E2
  in one place. E3-E8 each have their own client and need their own seam.
- **Restore:** `LlmAgent.execTool` (`internal/agent/llm_agent_retry.go:103`) rather than the
  `BeforeTool` hook: `ask_user` (`llm_agent_pause.go:116`) and approved drafts
  (`llm_agent_message_draft.go:101`) execute outside `dispatch`. The approval fingerprint
  (`gateway/approve.go:110`) must be computed on one form consistently, or an approval given
  on the placeholder form will not match.
- **Tool results back to the model:** `renderToolResultForPrompt` (`internal/agent/trust.go:30`);
  MCP results (PIM, WhatsApp) arrive through `bridge_call.go:124-133`.
- **Stream to the operator:** text deltas leave `consume` (`llm_agent_consume.go:60`) and fan
  out to AG-UI and Telegram through `runner.go:337-348`.
- **History** is stored in clear (`runner_persist.go:356`) and must be re-pseudonymized
  identically on every turn.
- **The KV-cache gate** (`cmd/aura cache-audit`, CI job `cache-invariant`) captures requests
  into a `FakeClient` *above* where the wrapper would sit, so it would not see the wrapper
  unless the audit wraps the fake. Q4 covers it.
- **Memory:** the model writes facts through `memory__memory_upsert_fact`, so restored
  arguments put real values into ArcadeDB. That is intended (memory is local); Q8 checks it.

Prototype code lives on a spike branch and is never merged: the spec decides the real shape.

## Questions

| Q | Question | Method | Gate (proposed, confirm before running) |
|---|---|---|---|
| 1 | How much personal data leaves today, per exit and per type? | A recording reverse proxy in front of each cloud base URL on the lab VM. A scripted week-shaped session: chat with PIM tools, one `agent_job`, a compaction, ingestion of a sample folder, one image, one voice note. Bodies scanned offline by the Q2 union detector, a sample checked by hand. | none: this is the baseline the rest is measured against |
| 2 | Does rizzo-pii add recall over checksum-validated patterns? | ≥ 200 items from the operator's real mail, calendar and documents, labelled by hand, out of rizzo's training data. Patterns alone vs rizzo-pii alone vs the union; recall and precision per type, span-exact. CPU latency p50/p95 per 1 000 tokens, RAM, image size on the lab VM; whether an ONNX export runs the same. | **baseline or drop**: the model stays only if the union beats patterns alone on FULLNAME, ORG, STREET and CITY without losing precision on the checksum types |
| 3 | Do real cloud models carry placeholders into tool calls intact? | Scripted tasks that need personal data in arguments (send a mail to X, create an event with Y at Z, record a fact about a person, find a contact), on two models of the release matrix, 10 runs each, with and without pseudonymization. Count placeholders in arguments: exact, bracket-stripped, case-changed, paraphrased, lost. Two or three placeholder formats. | placeholders intact in ≥ 99 % of argument uses; task success with pseudonymization no more than 1 in 20 below the clear run |
| 4 | Do placeholders keep the cached prefix? | `cache-audit` with a pseudonymizing wrapper around the fake; then a 10-turn live conversation on OpenRouter, `cached_tokens` with the wrapper on and off. | prefix hashes byte-identical; cached tokens not lower with the wrapper |
| 5 | Does restoration survive streaming? | Real streams on both models: how often a placeholder is split across deltas; a restore buffer bounded by the longest placeholder; check cockpit, Telegram and stored history. | no placeholder reaches the operator; history stored in clear |
| 6 | What does pseudonymizing embedding input cost retrieval? | Aura's retrieval evaluation, clear vs pseudonymized chunks and queries (same keyed placeholders), with name-centred queries added ("cosa mi ha scritto Mario Rossi"). Against keeping embeddings on the local EmbeddingGemma route. | recall@10 loss stated per query class; the spec picks between pseudonymized cloud embeddings and local-only |
| 7 | What do the non-text exits cost to keep local? | Vision on the sample images and scanned pages: cloud primary vs the local OCR-VL sidecar (quality by hand, latency). STT: cloud vs local faster-whisper on the voice notes (WER by hand). TTS: local Kokoro vs cloud on restored replies. | numbers for a per-exit policy; no gate |
| 8 | Which tools must not receive restored values? | Inventory every tool in the manifest that itself sends data out (web search, fetch, mail send, media generation) vs local ones (PIM read, memory, files). Decide restore, keep placeholder, or ask, per class. | every tool classified, none by default |
| 9 | What happens when the detector is down? | Stop the sidecar mid-session: refuse the cloud call, fall back to a local route, or send with patterns only. Measure that the fallback works end to end. | evidence for the spec's choice |
| 10 | Can the parts be shipped? | Licenses of `jhu-clsp/mmBERT-base`, the rizzo-pii weights (card says MIT) and code; nothing from rizzo-pii's AGPL desktop bundle (PyMuPDF). | a license that allows redistribution in the appliance, or the model is out |

Not covered on purpose, and recorded so the spec does not forget it: health and other GDPR
special categories have no tag in rizzo-pii nor a pattern. PMSync's own stress test
(`ai-chat/tools/pii-stress.spec.ts`, Q001: "Monica's last medical prescription") sends both
the name and the prescription in clear. Q1 counts how often this occurs in the sample.

## Order and what it needs

1. Q10, from here: licenses only.
2. Q1, on the lab VM with the operator: the recording proxy, the scripted session. Half a day.
3. Q2: labelling the sample with the operator (about two hours), then the runs. The sample
   stays on the lab VM; only counts are committed.
4. Q3, Q4, Q5 on the prototype branch, with cloud credit for about 80 runs.
5. Q6, Q7, Q8, Q9.

Results go into this file per question, a row in `MANIFEST.md`, the findings into
`Skill("spike-findings-Aura")`, and then the spec.
