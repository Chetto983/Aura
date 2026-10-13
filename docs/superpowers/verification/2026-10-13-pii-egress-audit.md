# PII boundary audit: ingestion, retrieval and outbound model requests

Date: 2026-10-13. Requested audit, revised by the operator after identifying direct
Excel/PDF opening as a bypass of ingestion-only masking.

**Verdict:** ingestion-only protection is insufficient. Keep original documents and
the original representation for local retrieval. Apply the shared PII policy to the
complete outgoing model request before network transmission/TLS, and restore scoped
references locally on the response path. Ingestion preprocessing can populate a
cache but must not authorize bypassing that final boundary.

This is a source audit plus read-only runtime inspection, **not an implemented privacy
feature or a new end-to-end acceptance run**. Findings below are blockers for the
proposed design, not regressions introduced by a production PII change: none exists.

## Evidence and scope

- WSL used for inspection and SSH. Workspace revision:
  `deffda241333cf3e1fc07378fef6e21ea678d457`.
- VM `192.168.101.158` is healthy, deployed Aura revision
  `6a3290e2f84dfcbc85b7dd5d4eb71f4b86e1a8cc`.
- The reviewed ingestion, document retrieval/opening, embeddings and LLM provider
  files have no source diff between those revisions. `llm_agent_retry.go` differs by
  eight lines; execution-order references below refer to the workspace revision.
- Actual embedding process: local `embeddinggemma-300M-Q8_0.gguf`, CPU only, four
  threads, one slot, context 2048, bound to VM loopback port 8081. This observation
  identifies this process; it is not an attestation of every configured model route.
- Reused evidence: [synthetic transport/restoration](2026-10-12-pii-alcatraz-rizzo-lab-vm.md)
  and [component cache measurements](2026-10-12-pii-cache-lab-vm.md). Those fixtures
  do not prove that normal Aura requests are currently protected.
- No product code, routing, index, document, provider account or service was changed.
  No new cloud inference, parallelism benchmark or retrieval-quality benchmark ran.

## Findings, ordered by impact

### A1 — High: direct document opening bypasses an ingestion-only filter

`OpenService.OpenDocument` resolves a document to `source_key` and streams its
original object bytes. The agent can process those bytes locally using its existing
file tools. Their output joins model history as tool content.

This is particularly significant for spreadsheets: ingestion deliberately returns no
text passages for them. Filtering only `process_chunk` would never inspect their rows.

Evidence:
- [original object read](../../../internal/documents/open.go:69), especially line 92;
- [spreadsheet path](../../../services/ingest/media.py:123), especially lines 131–132;
- [tool result enters history](../../../internal/agent/llm_agent_dispatch.go:143).

Required change in the design: inspect outgoing model content regardless of whether
it originated in retrieval, an opened file, shell output, an MCP result or chat.
Individual tools need no separate detector. A cache entry for an ingested chunk does
not cover a newly formatted table or a different excerpt from that same document.

### A2 — High: a text-only request wrapper misses media added by provider serialization

Both provider clients resolve media while constructing their SDK request, after the
provider-neutral `llm.Request` was assembled. ChatGPT can add original image bytes as
base64; OpenAI-compatible routes also support audio/video depending on capabilities.
In the inspected code, ChatGPT rejects native non-image content: this audit does not
claim that it directly uploads Excel/PDF files through that path.

Separately, a scanned PDF can be rendered and sent through the configured vision
client during extraction, before there is text for the PII detector to inspect.

Evidence:
- [provider-neutral media references](../../../internal/llm/client.go:147);
- [ChatGPT media projection](../../../internal/llm/chatgpt/request.go:24);
- [image bytes serialized](../../../internal/llm/chatgpt/media.go:74);
- [OpenAI-compatible media construction](../../../internal/llm/openai_compat/request.go:22)
  and [native content](../../../internal/llm/openai_compat/request.go:170);
- [scanned PDF vision calls](../../../cmd/aura-media-index/main.go:97).

Required change in the design: enforce the policy on the final payload after all
content projection. A text recognizer cannot sanitize pixels, audio or a base64 blob
merely because it sees their JSON field. Unsupported raw media must use an appropriate
local extraction/redaction route or be refused on the protected cloud path. A final
transport check is enforcement, not a replacement for understanding those formats.

### A3 — High: restoring only display text breaks name-based tool queries

The local retriever passes the incoming query unchanged to query embedding and lexical
search. If the cloud sees `[[PII_PERSON_…]]` and returns that token inside a search or
file-processing tool call, an index containing the original name cannot be relied on
to find the same document without local resolution first.

Restoration must precede consumers of tool arguments, including deduplication,
validation, approval fingerprints and execution. Placing it only in a tool's Execute
method is too late. Placing it only in `BeforeTool` also misses special paths such as
`ask_user`. Names, document IDs and function names are distinct protocol concepts;
the gateway must preserve routing/schema identifiers.

Evidence:
- [query used by both retrieval legs](../../../internal/documents/retrieval.go:313);
- [query embedding](../../../internal/documents/retrieval_space.go:45);
- [lexical/dense passage routing](../../../internal/documents/retrieval_lexical.go:25);
- [argument canonicalization](../../../internal/agent/llm_agent_dispatch.go:76);
- [operation identity and policy decision](../../../internal/agent/llm_agent_retry.go:105);
- [special direct execution](../../../internal/agent/llm_agent_pause.go:116).

Required change in the design: resolve authorized PII tokens at the common inbound
model/tool-call boundary, before dispatch, and again protect tool results outbound.
The mapping must be durable and scoped to the caller. Unknown or wrong-scope tokens
must not resolve to another user's values. Streamed display restoration requires
buffering split tokens; the earlier synthetic transport probe demonstrated that need.

### A4 — High: the chat client is not every outbound model connection

Embedding requests have independent Go and Python clients. Document extraction uses
separate vision/STT clients. A wrapper installed only at `llm.Client.Stream` cannot
enforce policy over those other connections.

Evidence:
- [chat client factory](../../../cmd/aura/llm_client.go:47);
- [Go embedding client](../../../internal/embeddings/client.go:102);
- [Python embedding HTTP request](../../../services/ingest/embed.py:219);
- [extraction clients](../../../cmd/aura-media-index/main.go:57).

Required change in the design: one shared model-egress policy, connected to all model
clients or an explicit local model gateway with controlled direct egress. This is a
small set of protocol/client boundaries, not one implementation per tool. The request
body is inspected while still plaintext, before TLS. A firewall looking at encrypted
HTTPS cannot perform field replacement. Ordinary tool APIs and arbitrary network calls
are a separate egress scope; protecting model calls does not silently protect those.

### A5 — Medium: changing the indexed representation creates avoidable retrieval drift

Current ingestion stores original passage text and embeds that same text. Retrieval
searches a lexical index over `Passage.text` and combines it with vector candidates.
Cards and filenames are independently searchable. The current query contains real
names. Masking only documents or only queries creates inconsistent representations;
opaque PII IDs are not an exact-person lookup feature of an embedding model.

Evidence:
- [passage text and embedding input](../../../services/ingest/app.py:177);
- [card and filename indexing](../../../services/ingest/app.py:291);
- [lexical query binding](../../../internal/arcadedb/document_retrieval.go:143);
- [raw text hashes used for deduplication](../../../internal/documents/retrieval_rank.go:173).

Recommended direction for the observed local embedder: keep original local text,
vectors, lexical indexes and citation coordinates. Store pseudonymized output as an
additional representation/cache. Do not overwrite `text`, original hashes or offsets.
Do not replace identity with per-document counters such as `[FULLNAME_1]`: different
documents could become identical after masking and be collapsed if their canonical
hashes were recomputed from that form.

Embedding the original text and PII preprocessing may run in parallel locally because
neither depends on the other's result. The acceleration has **not been measured**.
If embeddings move to a cloud route, preprocessing must precede that outbound call;
document/query representation and retrieval quality then need a separate measured
decision. The final boundary does not make cloud embedding original text private.

### A6 — Medium: cache and detector limits remain at the final boundary

An exact-ingested-text cache is not an exact-outbound-payload cache. Tool results can
be HTML-escaped, Unicode-normalized and wrapped with a new nonce. JSON strings can
contain nested JSON. A raw substring replacement is not a complete parser for this
content, and an unrelated nonce must not be removed to improve cache hits.

Evidence: [tool framing and escaping](../../../internal/agent/trust.go:69) and the
earlier transport probe's correction from 18 to 20 detected baseline occurrences
after decoding nested JSON strings.

Required change in the design: cache exact complete blocks/fields with identity and
policy/token generations; preserve trust framing and protocol structure. Reuse a
prepared block only when its inputs and policy still match. A miss requires analysis;
an unavailable mandatory detector must not cause an unprotected fallback. The local
Alcatraz CF fix does not address the separately observed silent NLP-error behavior.

The cache benchmark still supports this optimization: Alcatraz's already masked
message cache measured 20 ms for growing history and 0.80 ms for an identical request.
Those are pattern-only component results. Rizzo still required 6.77–7.76 seconds for
a new tool block in its two trials. Moving the boundary does not remove cold latency
or fix missed names; ingestion preprocessing remains useful but insufficient alone.

## Positive observations and dimension assessment

| Dimension | Assessment |
|---|---|
| Security/privacy | Final model-egress enforcement is the necessary direction; ingestion-only and text-only coverage are insufficient. No production assurance yet. |
| Correctness | Preserve local retrieval inputs and restore incoming tool arguments before dispatch. Existing identity-scoped retrieval and original-file provenance are reusable. |
| Performance | Component cache benefit measured previously. Parallel embedding/PII and complete gateway overhead remain unmeasured. |
| Maintainability | Reuse the existing provider-neutral client, serializer boundaries, per-identity key/sealing primitives and ingestion memoization. Avoid a detector implementation in every tool. |

The existing architecture already gathers tool outputs into a common model history,
retains source coordinates for original documents, isolates document lookup by identity
and separates query/document embedding prefixes. These are useful integration points;
none requires replacing the local search index to add outbound protection.

## Acceptance needed for the implementation

Use synthetic PII to open an original XLSX, read a PDF, retrieve by a real name, then
search again using a model-returned token. Inspect the serialized outbound requests:
all expected fixture values must be absent, the local tools must receive the correct
original values, and retrieval/citation results must match the baseline. Include
nested JSON, escaped names, file metadata, repeated history, provider media projection,
missing mappings, wrong identity, detector failure and token splits across streaming
deltas. Observe all model clients, not just chat. Raw unsupported media must not leave
through a side path. These are future acceptance tests, not checks passed by this audit.

Spike 109 remains PENDING. The audited direction supersedes ingestion-only masking as
a privacy boundary; ingestion remains an optional cache producer.
