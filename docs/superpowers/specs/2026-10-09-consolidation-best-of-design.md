# The best of a year, brought into Aura

Decided by the operator on 2026-10-09: "all this work taught me a lot; now take the best of
everything and bring it into Aura". This document is the inventory that decision needs,
measured on the repositories themselves on the same day, and the order in which the pieces
land. Every landing is gated by a measurement on the live stack, per the PRD-first rule;
nothing here authorizes code before its measurement.

## What was measured

Window: 2025-10-10 to 2026-10-09. Every count is from a git history deepened past the
shallow clone, at the commit named.

| Source | Commit | Commits | Period | What it is |
|---|---|---|---|---|
| Aura | `3e4e65a90` | 6,780 + 1,463 pre-rewrite | 2026-04-28 → today | Go agent appliance, 212k LOC Go, 313k LOC Go tests, 73k LOC web, 114 migrations, PRD 3,474 lines |
| PMSync | `042cc04c` | 4,366 | 2025-10-30 → 2026-04-24 | NestJS/Next mail client with AI triage and CRM, 91 Prisma models, 273k LOC |
| StickyFlow | `e36c4eb` | 1,479 | 2026-02-02 → 2026-03-03 | React/Supabase Kanban with AI copilot, 78k LOC |
| wpt-iot | `48f0e37` | 765 | 2026-04-02 → 2026-10-01 | Industrial IoT edge stack, PLC over UDP, TimescaleDB, installer wizard |
| turing_AgentMemory_MCP | `dacb77f` | 457 | 2026-07 | ArcadeDB memory MCP, absorbed into `cmd/arcadedb-mcp` |
| sacchi_agent | `181a388` | 193 | 2026-09-17 → today | Python desktop app on Aura's method, PRD rewritten after measures |
| Market_MCP | `900c9a3` | 106 | 2026-09-24 → 2026-10-02 | Quant research blueprint, Python 3.14, coverage floor 90 |
| Plotter-Pen | `1755b60` | 227 | bursts in 2026-01 and 2026-09 | CAD for pen plotters, Go + Three.js, OPC UA |
| OpenDots, SVAR | web, 2026-10-09 | | | Read the same day: prd.md §5, §12, §16 |

One human author on every repository. Claude signs 362 commits on Aura and 136 on PMSync.

## The rule

Behaviours transfer; code does not. Every source is a different stack from Aura's, and two
of them, PMSync and StickyFlow, were built at a quality bar Aura no longer accepts (PMSync's
README badge: tests 17.45%; three production wipes in April 2026). What transfers is a
model, a pipeline, a rule or a surface, re-specified against Aura's tree and gated by a
measurement on the lab VM. Aura is the host; the other repositories feed it or are archived.

## The inventory, ranked

Rank is by value to the operator divided by the cost of landing it, with the state of
Aura's tree as the second column. "Spec" means a design exists in this directory.

| # | Behaviour | From | Aura today | Lands in | Gate | State |
|---|---|---|---|---|---|---|
| 1 | Per-identity tool policy, `ask` and `deny` | OpenDots | scopes widen only | gateway, `approvalpolicies` | lab-VM acceptance in spec | spec, PRD §5 |
| 2 | Browser takeover with stale-reference refusal | OpenDots | no control state | `browsercontrol`, relay, live view | lab-VM acceptance in spec | spec, PRD §12 |
| 3 | Work board with a `board` tool | StickyFlow, PMSync Tasks pillar, OpenDots | `todo` is a scratchpad | `internal/board`, SVAR Kanban mode | lab-VM acceptance in spec | spec, PRD §16 |
| 3a | `source` on a card and saved smart views | PMSync `ManualTask`, `TaskSmartView` | none | the board spec | with 3 | add to spec |
| 4 | Inbox triage as a scheduled job feeding the board | PMSync triage pipeline | PIM reads live, no per-email state | `agent_job` kind, PIM, board | triage of a real mailbox through the PIM: calls, latency, cost | blocked on the mail-table decision |
| 5 | Daily briefing | PMSync `BriefingService`, Smart Today | scheduler, Telegram, `memory_digest` | one skill plus one scheduled task | five working days on the lab VM against a no-model baseline | spec |
| 6 | Command palette | StickyFlow, PMSync v12 Phase 136 | `cmdk` used by two pickers only | `web/src/shell`, one `CommandDialog` | measured use over a week | to spec |
| 7 | PII tokenization before every cloud model, ingestion included (widened 2026-10-10) | PMSync chat pipeline | `redact` covers logs only | an `llm.Client` wrapper plus the embedding, vision, speech, media and CocoIndex exits | spike 109 (`.planning/spikes/109-pii-pseudonymization`): ten measurements, rizzo-pii under baseline-or-drop | spike planned |
| 8 | Air-gapped appliance install | wpt-iot `build-bundle.sh`, `install-offline.sh` | wizard carries its payload, needs GHCR | `packages/create-aura` | one install on a machine with no Internet | to measure demand |
| 9 | Writing style profile for drafts | PMSync `WritingStyleProfile` | `messagedrafts` reviews, learns nothing | memory facts about the operator's style | draft acceptance rate before and after | later |
| 10 | Strategy lenses for task generation | StickyFlow | none | a skill, used by the board | with 3, after the board is in use | later |
| 11 | A member administers nothing that is not theirs (added 2026-10-10) | Aura's own measurement, 2026-10-10; Open WebUI and LibreChat as reference | `governance.write`, which every identity holds, gated deployment administration, every identity's scheduled tasks, the house skills and the skill install | `identity.create` on administration, an owner rule on the scheduler board (prd.md §3) | the member calls of the two `verification/2026-10-10-member-*` ledgers, repeated on the lab VM | on master; measured on the lab VM with a provisioned member (`verification/2026-10-10-lab-vm-member-governance.md`), which also found and fixed the unmounted pause and resume; background shells and the install running as root still open |

Already in Aura, no action: HITL with tiers, reservations and durable grants (PMSync's
`needsApproval` and rate tiers are weaker); bitemporal memory with supersede authority
(PMSync's `AIMemory` and `GraphEdge` decay are the Postgres version); per-identity RLS and a
database per identity (PMSync's `tenantId` with coarse RBAC), though the capability gates in
front of them were not: item 11; hybrid retrieval in one engine
(PMSync's RRF function); the memory MCP (turing, absorbed); faster-whisper STT and TTS
(StickyFlow's client-side Whisper); the per-file mutation gate
(`scripts/critical_mutation_gate.py`, which sacchi_agent's `tools/mutation_score.py` copies);
the installer wizard with local and remote SSH modes (wpt-iot's `create-wpt-iot` is the same
shape); an OAuth relay for changing addresses (`aura-connect`).

Not taken, with the reason:

- **CRM Pro 360** (accounts, pipelines, deals, tickets, SLA, knowledge base, scoring,
  forecast, churn): a product, outside prd.md §1. If ever, an MCP server Aura mounts.
- **PMSync's mail sync engine and embeddings**: a local copy of the mailbox. Item 4 decides
  whether Aura stores a per-email analysis; it never decides to store the mail.
- **Socket.IO, Redis, BullMQ, PWA offline reading**: Aura has SSE, Postgres advisory locks
  and a cockpit that is a home-screen app.
- **StickyFlow's calendar and Gantt views, presence, encryption, billing**: a product's
  surfaces; the SVAR Calendar stays a candidate measured on its own (prd.md §16 reading).
- **Plotter-Pen, SacchiBot's scraping pipeline, Market_MCP's models**: other domains. The
  two live siblings, sacchi_agent and Market_MCP, keep Aura's method and stay separate.
- **PMSync's web push**: Telegram carries every notification today; measure a need first.

## Read 2026-10-16: Rag_Full_Version

Asked by the operator: "take the best techniques" of `github.com/Souheil007/Rag_Full_Version`, an
outside Python RAG engine, read at `e32534a`: 107 files, about 6,200 lines. Its own evidence is
thin: a 25-question chunking benchmark, where recall@5 moves by one question, and a 10-question
end-to-end run. What it offers is techniques, measured here on Aura's own sets
(`verification/2026-10-16-rag-full-version-techniques-spike.md`). The same rule holds:
behaviours, never code.

| # | Behaviour | From | Aura today | Lands in | Gate | State |
|---|---|---|---|---|---|---|
| R1 | Workbook rows as key-value passages: 15-row groups, 2 rows of overlap, the header repeated in every row | `BlockChunker`, `TableSerializer.to_kv_sentences` | a workbook has no passage; the card and `document_open` answer for it (prd.md §11) | the ingest's spreadsheet route and `passage_count` | the code sets and the arms of `2026-10-17-workbook-row-passages-design.md` | spiked: MRR 0.812 → 0.893 over 54 questions, every gold within 8 |
| R2 | A real title in EmbeddingGemma's `title:` slot, which the model card says improves document embeddings | the section-path prefix on every chunk | `title: none` | `embeddings.UntitledDocumentPrefix`, `chunk.EMBED_DOC_PREFIX`, a `RecipeVersion` bump and a full re-embed | the code sets, where a file path is the most telling title | spiked: nothing alone; beside R1 MRR 0.893 → 0.900, complex up and office down |
| R3 | Cross-encoder rerank of the fused candidates | `Reranker`, English `ms-marco-MiniLM-L-6-v2` | cosine `vector.rerank`; `internal/rerank` and the `aura-rerank` sidecar were deleted for having no caller | a multilingual reranker on llama.cpp's rerank endpoint, after the fused query | none: ruled out by the operator | measured: spike 070, 23 s per 15 documents on CPU, and the appliance has no GPU (`2026-10-16-card-description-vector-design.md`) |
| R4 | Large PDF tables in row groups with the header repeated | `BlockChunker` on Docling's table nodes | PDF text from pypdf, chunked by size | a table extractor Aura does not have since the Docling era | a measured loss first: the complex manuals' table questions are already first | not now |
| R5 | Claim-level answer check against the retrieved passages | `CitationVerifier`, `HallucinationDetector` | citation tokens and locators on every passage, no answer-level check | the agent's answer path | an answer-level eval; `complex_qrels.json` carries each expected answer | later |

Already in Aura, no action: hybrid dense and BM25 retrieval fused by RRF, done inside ArcadeDB in
one query rather than in Python; adjacent-passage expansion (`RetrievalRequest.Neighbours`, which
`HitCollapser`'s row window and sibling merge resemble); a `heading_path` field on passages, often
empty today where Rag_Full_Version always fills its section path; a golden
question set with gold documents per corpus.

Not taken, with the reason:

- **VLM table repair with Mistral Pixtral**: a paid cloud model reading the operator's
  documents, against local-first and item 7.
- **Docling**: Aura left the Docling era for CocoIndex with its own extractors.
- **Semantic answer cache**: it serves stale answers on a corpus that changes, and no latency
  problem is measured.
- **Query router and contextual compression**: no measured need on Aura's sets.
- **Langfuse and Sentry**: Aura has its own observability.

## Read 2026-10-17: two r/Rag field reports

The operator shared r/Rag on 2026-10-17. Two posts reach Aura's measured losses.
- **"1.5 years of RAG in fintech"** (u/Klutzy_Newspaper3196, 2026-09-17, 368 votes). These are
  lessons from production, with no benchmark.
- **"We ran 4 RAG setups over all 34,735 FDA drug labels"** (u/International_Elk_33,
  2026-10-17). A vendor's own evaluation:
  - 100 questions, with the answer keys written before any run;
  - a blind grader;
  - the setup at `doi.org/10.5281/zenodo.23403889`, which was not read here.

The comments were not read, because Reddit serves automated browsers a challenge. The rule is
the same as above: behaviours, never code.

| # | Behaviour | From | Aura today | Lands in | Gate | State |
|---|---|---|---|---|---|---|
| D1 | Read what the question names first, rank second. When a question's identifier names a document (its title or card), search that document's own passages for the rest of the question | FDA post: crowding by related labels hid the named drug's own boxed warning; "look up the names first" took false reassurances from 5 to 0 | Identifiers first ranks a document only if its title, card or *returned* passages carry every identifier. For "FX300 … A119" no document qualifies, because the FX300's A119 passage is not returned (`2026-10-17-answers-read-by-a-model.md`) | `rankDocuments` plus one full-text query restricted to the named document | the PDF look-alikes `fx300-alarm-A119` and `fx300-chains`, then every set | not measured |
| D2 | Code chunked by its syntax | fintech post: "AST worked better than plain text chunking" | `chunk.chunk` calls CocoIndex's `RecursiveSplitter` with no `language`, so source files split as prose. The installed 1.0.25 already takes `language=` (tree-sitter) and offers `detect_code_language(filename=)` | `services/ingest/chunk.py`, from the file name | the English, Italian and find-by-name code sets, the lowest today (r@8 0.833 English, 0.667 Italian, prd.md §11 "The card's vector") | not measured |
| D4 | Questions over every document ("which contracts have a change-of-control clause", "which expire in two months") | FDA post: every search-based setup listed under 1 in 10 of 282 contracts; looking at every document found 88% | no sweep; the agent can only open documents one at a time | outside retrieval: an agent or batch path over the card catalog | a question set of that kind | not measured |

Already in Aura, no action:
- the calculation goes to code, and the model only explains it (`document_open` with pandas);
- memory, documents, the database and the sandbox are separate routes;
- passages carry citation tokens, so a quoted sentence can be checked against its source.

## Read 2026-10-17: Paperless-ngx

Shared by the operator on 2026-10-17: `github.com/paperless-ngx/paperless-ngx`, GPL-3.0.
- **Read from the `dev` branch:** `docs/usage.md`, `docs/advanced_usage.md`, and
  `src/documents/` `matching.py`, `classifier.py`, `models.py`.
- **It is a document archive with its own interface.** It is a source of behaviours for Aura's
  documents, not a dependency. The agent's search, with passages and citations, stays Aura's.

**What it does, as read:**
- **Each document carries fields the operator owns:**
  - a correspondent;
  - a document type (invoice, contract, statement);
  - nested tags;
  - a created date, apart from the date added;
  - an archive serial number;
  - typed custom fields: Date, Monetary with an ISO 4217 code, Integer, Select, Document Link.
- **Assignment starts deterministic:** Any, All, Exact, Regex and Fuzzy rules on each tag, type
  or correspondent.
- **"Auto" is not an LLM.**
  - It is scikit-learn's `MLPClassifier` over a bag of words: `CountVectorizer`, 1–2-grams,
    `min_df=0.01`.
  - It is trained on the operator's own past assignments, and retrains hourly.
  - It learns only from documents out of the inbox, which have been checked.
  - Its own documentation says it works only where the text correlates with the label, and that
    a tag like "TODO" cannot be learned.
- **LLM suggestions are opt-in.** The model is Ollama locally or any OpenAI-compatible API, and
  the documentation carries a privacy warning. Suggestions are accepted or rejected by the user.
- **Versions.** A root document keeps the metadata, and file versions (`_v1`, `_v2`) keep the
  file and its text. Search reads the latest version.

| # | Behaviour | From | Aura today | Lands in | Gate | State |
|---|---|---|---|---|---|---|
| D5 | Document fields the operator owns (type, counterparty, dates, typed custom fields), used as **filters** by `document_search`. They are assigned by rules first, then by a classifier trained on the operator's own corrections, and by an LLM only as a last resort | Paperless's data model, matching rules and "Auto" classifier | `CandidateFilter` filters by document, key or prefix only. filecard drops an invoice's header rows ("Data emissione", "Cliente"), which are exactly those fields. Free LLM tags would add little: the words are already in the text, the name and the card | `IndexedDocument` properties, a filter in `document_search`, extraction in filecard/the ingest | a question set of "which documents…" (office q09 and q18 are two of them today) and BASELINE OR DROP for the classifier against the rules alone | not measured |
| D6 | An LLM summary per file, beside the card, never in its place | discussed with the operator 2026-10-17; Paperless's AI suggestions | the card is deterministic and its description is the document vector (prd.md §11, "The card's vector") | the ingest, memoized on the file's bytes like the billed media calls | the English and Italian code sets, against the card alone, with summaries for every file of the library and the cost per file on the appliance's local LLM | not measured |

Not taken, with the reason:

- **Document versioning (formerly D3), and Paperless's root-plus-versions model.** The operator
  dropped it on 2026-10-17: it brings only complexity.
  - A changed file is already reconciled by CocoIndex over its old passages, and a removed file
    takes its passages with it.
  - What stays unhandled is two files that are two versions of one document, such as a v3 and a
    v4 of the same policy. They are two keys, and both are searched. No failure of that kind has
    been measured on Aura's corpora; the 2025 and 2026 budgets are both valid documents.

## Process, taken now

Three rules from the siblings are worth a line in `CLAUDE.md`, because Aura already lives by
them without saying so:

- **A learned component must beat its dumb baseline out of sample, or it is dropped**
  (Market_MCP, guiding principle 3). Aura's turn-recall gate already measures seeds against
  memory against teacher; the rule makes the comparison mandatory for every learned piece,
  items 4, 5 and 9 above included.
- **Spike verdicts gate specs**: `VALIDATED` freely, `PARTIAL` with its constraints,
  `INVALIDATED` never (Market_MCP). Aura's `spike-findings` skill carries the vocabulary;
  the gate is not written down.
- **Validations are a dated ledger** (sacchi_agent `docs/superpowers/validazioni/`, one file
  per measurement). Aura's `verification/` holds reviews; the lab-VM acceptance runs of the
  three specs above should land there one file each, named by date and scenario.

The anti-pattern the year measured, also worth a line: PMSync's three wipes came from
schema changes pushed without migration files. Aura's disposable-stack rule and the
migrate-only discipline are the answer and are already in `CLAUDE.md`.

## Order of landing

1. **Wave 1, specs exist**: items 1, 2, 3 with 3a. Each closes on its lab-VM acceptance.
   Item 3 is the largest and the one the others feed; items 1 and 2 are a few days each.
   Implemented on 2026-10-09; not closed. None of the three lab-VM acceptances has run: the
   only live run so far had the reviewer in place of the model, in a cloud container
   (`docs/superpowers/verification/2026-10-09-wave1-model-in-the-loop.md`).
2. **Decisions before wave 2.** First, the six gateway and cockpit findings that run left to
   the operator (a delete prompt that names no card, a second prompt after a restart,
   policies inert under `dev` and `local_trusted` with no notice, a denied call shown as
   completed, the approval request HTML-escaped, a slow SIGTERM): the wave-2 triage runs
   through the same gateway. Then, does Aura keep a per-email analysis table (a mail index
   light)? Measured first by running one triage over a real mailbox through the PIM on the
   lab VM and recording calls, latency and cost; the amendment to prd.md §11 and §15 follows
   the number, then the spec for item 4.
3. **Wave 2**: items 4 and 5, which share the scheduler and the board. Item 5 does not wait
   on the mail-table decision: `2026-10-09-daily-briefing-design.md`.
4. **Wave 3**: items 6, 7, 8, each with its own small spec after its measurement.
5. **Later**: items 9 and 10, once the board has a week of real cards.

Each wave follows the superpowers workflow (CLAUDE.md): one spec per decision in
`docs/superpowers/specs/`, one plan per implementation in `docs/superpowers/plans/`, and one
dated file in `docs/superpowers/verification/` per acceptance run. The live run closes a
wave's item, never the unit suite (CLAUDE.md, Definition of Done).

## What this document does not establish

It does not measure items 1 to 10 on the live stack, and item 11 only on a cloud stack, not
on the lab VM; every row's gate is still to run. It does not decide what happens to PMSync and StickyFlow, both idle since spring with
a milestone half done: archive or cantiere is the operator's call, and nothing here depends
on it. It does not resolve the fifteen open questions in the three specs of the same day.
