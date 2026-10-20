# Spike: pseudonymized embedding input through an ArcadeDB vault — findings

Date: 2026-10-20. After the lab VM's embedding route moved to `perplexity/pplx-embed-v1-4b`, every
document passage, query and memory text goes to OpenRouter in clear. Asked whether protected mode
should keep embeddings local or pseudonymize them, the operator answered "fai delle prove", then:
"non deve perdere nulla se no serve a niente si fa una base con redis su arcadeDB con tutti i nomi
da ricostruire". Midway they pointed at OpenRouter's guardrails
(`openrouter.ai/docs/guides/features/guardrails#guardrail-settings`).

This answers question 6 of [spike 109](../../.planning/spikes/109-pii-pseudonymization/README.md)
(what pseudonymizing embedding input costs retrieval) and tests the vault the operator proposed.
It does not touch the chat path, the detector, or the product: no Aura code changed.

## Setup

**Corpus.** 119 passages: the 86 the lab VM's ingest wrote for identity `448ddbe1-…` (Financial
Sample, Constitution, calibre demo, a deck, a photo description, two chat files), which hold almost
no personal data, plus 33 synthetic Italian documents from `corpus.py`. The synthetic set has 14
people, 8 companies and 153 personal values (names and their parts, streets, emails, phones,
codici fiscali, IBANs, partite IVA), with valid checksums. Several documents share a template and
differ only by who they concern: four complaints, four invoices, three leases, three hirings, three
refunds. Two people share a surname (Mario and Marta Rossi), one list is in capitals.

**Questions.** 98: the 35 of `embedding-cloud-models` (class G) and 63 synthetic:

| class | n | what decides the answer |
|---|---|---|
| N | 27 | a full name, among documents of the same template |
| A | 7 | an attribute asked by name ("il cellulare di Roberto Esposito") |
| V | 6 | the personal value itself (a phone, an email, a street, a CF, an IBAN, a P.IVA) |
| P | 7 | part of a name ("la Ferraris", "il sig. Esposito") |
| C | 4 | a different case from the document's |
| O | 5 | a company |
| S | 7 | no personal data |

**The vault.** ArcadeDB 26.10.1 (`arcadedata/arcadedb@sha256:01506549…`, the compose pin),
throwaway, with `Redis:com.arcadedb.redis.RedisProtocolPlugin`. One database per identity and one
ArcadeDB user that may open only it, the shape of Aura's tenant databases. One document type,
`PiiEntry {token, digest, sur_digest, kind, value, sur}`, with unique indexes on `token`, `digest`
and `sur_digest`. Written with the persistent `HSET <db> PiiEntry <json>` and read with
`HGET <db>.PiiEntry[token|digest|sur_digest] <hex>` over RESP. `digest` is an HMAC of kind and
case-folded value under the identity's key, so no lookup key is a value. Every entry is stored
before any text that uses it is embedded.

**Representations**, applied to passages and queries alike, with the vault as the only source:

- `clear`: as today.
- `openrouter_redact`: what OpenRouter's Sensitive Info guardrail would send, with perfect
  detection: `[PERSON_NAME]`, `[ADDRESS]`, `[EMAIL]`, `[PHONE]`, `[REDACTED]` for CF, IBAN and P.IVA
  (custom patterns). Companies stay in clear, as no preset covers them.
- `token_whole`: the spec's form, one `[[PII_<KIND>_<32 hex>]]` per value.
- `token_parts`: a person is its given name's reference plus its surname's; a company keeps its
  sector words and references its proper words.
- `surrogate`: the same parts, each one a consistent invented value of the same kind: a given name
  of the same gender, a surname, a street, a phone, a CF spelled from the surrogate name (a real CF
  spells its owner's), an IBAN and a P.IVA with valid checks. Distinct values get distinct
  surrogates, enforced by the unique index on `sur_digest`. Measured under five identity keys.

A single-word name (a given name, a surname) is replaced only when capitalized, so "fontana" or
"gallo" as ordinary words stay. Multi-word values match in any case.

**Ranking.** `measure.py` follows Aura's fused passage read (`fusedStatement`): a dense leg
(cosine distance ≤ 0.72, 200 neighbours) and a lexical leg fused by RRF, one passage per document,
the top 32 re-ranked by cosine, kept at cosine ≥ 0.32, 8 returned. The lexical leg runs on the
original text with the original query, because Aura's full-text index is local. BM25 stands in for
ArcadeDB's Lucene index. Also reported: `rrf`, the same pool and floor ordered by the fusion
instead of the cosine, and `dense`, cosine over all 119 passages.

**Models.** The local EmbeddingGemma-300M Q8_0 sidecar, `perplexity/pplx-embed-v1-4b` (the VM's
route) and `google/gemini-embedding-2`, all at 768 with Aura's prefixes. The whole run cost $0.026
on OpenRouter.

## Results

Correct answers at rank 1 and within 3, of 98, and MRR. Surrogates: mean of five keys, with the
range at rank 1.

**Aura's ranking as deployed (cosine re-rank):**

| model | clear | OpenRouter redact | token whole | token parts | surrogates |
|---|---|---|---|---|---|
| EmbeddingGemma local | 79 / 87 / 0.848 | 66 / 82 / 0.753 | 72 / 81 / 0.782 | 67 / 80 / 0.749 | 77.4 [76–79] / 86.6 / 0.836 |
| pplx-embed-v1-4b | 83 / 88 / 0.871 | 63 / 74 / 0.699 | 63 / 74 / 0.713 | 66 / 77 / 0.737 | 80.4 [79–82] / 85.4 / 0.845 |
| gemini-embedding-2 | 82 / 90 / 0.877 | 69 / 86 / 0.788 | 76 / 84 / 0.822 | 70 / 84 / 0.788 | **83.0 [80–84] / 89.2 / 0.878** |

**The same pool ordered by the fusion (`rrf`):**

| model | clear | OpenRouter redact | token whole | token parts | surrogates |
|---|---|---|---|---|---|
| EmbeddingGemma local | 79 / 88 / 0.852 | 82 / 88 / 0.866 | 80 / 88 / 0.855 | 77 / 88 / 0.837 | 77.2 [75–79] / 87.0 / 0.838 |
| pplx-embed-v1-4b | 81 / 89 / 0.867 | 74 / 79 / 0.779 | 80 / 87 / 0.852 | 80 / 87 / 0.851 | 79.0 [78–81] / 86.4 / 0.844 |
| gemini-embedding-2 | 82 / 90 / 0.876 | 83 / 89 / 0.878 | 83 / 89 / 0.874 | 83 / 89 / 0.872 | 81.0 [79–82] / 89.6 / 0.867 |

Rank 1 by class, deployed ranking: clear / surrogates (mean) / token whole / OpenRouter redact.

| model | N 27 | A 7 | V 6 | P 7 | G 35 |
|---|---|---|---|---|---|
| EmbeddingGemma local | 21 / 20.4 / 20 / 18 | 7 / 7.0 / 6 / 2 | 4 / 3.0 / 2 / 2 | 7 / 7.0 / 6 / 6 | 24 / 24.0 / 23 / 23 |
| pplx-embed-v1-4b | 22 / 22.0 / 15 / 14 | 7 / 6.8 / 2 / 1 | 4 / 2.6 / 1 / 0 | 7 / 6.0 / 5 / 5 | 27 / 27.0 / 27 / 27 |
| gemini-embedding-2 | 25 / 24.0 / 22 / 20 | 7 / 7.0 / 5 / 1 | 2 / 3.4 / 1 / 1 | 7 / 7.0 / 6 / 6 | 25 / 25.6 / 27 / 25 |

C, O and S are equal or nearly so under every representation. `results.txt` has every question's
movement against clear, per key.

**The vault** (`probe.py`, then a container restart):

| check | result |
|---|---|
| 217 of 217 texts (passages and queries) rebuilt byte-exact from `token_whole` and from `token_parts` | PASS, 0.05–0.06 s for all |
| 33 of 33 synthetic documents rebuilt byte-exact from surrogates | PASS |
| known personal values left in the pseudonymized text | 0 for tokens and surrogates; 26 texts for OpenRouter redact (companies) |
| 12 hostile values round-trip byte-exact (accents, quotes, braces, a line break, CJK, emoji, 5,000 chars, a Redis command as a value) | PASS |
| identity 0 reading or writing identity 1's database | refused, `NOPERM` |
| a second entry with an existing token or digest | refused, `Duplicated key`; the stored value unchanged |
| entries after a restart | 171 of 171, byte-exact |
| `HGET` by token over RESP, 1,000 calls | p50 0.18–0.23 ms, p95 0.27–0.30 ms |
| pseudonymizing 217 texts against 159 entries | 0.09 s |

## Verdicts

| # | Finding | Verdict |
|---|---|---|
| 1 | **The vault loses nothing.** Every token and surrogate rebuilds its original byte for byte, a duplicate is refused rather than overwritten, the entries survive a restart, and one identity cannot read or write another's database. ArcadeDB enforces the isolation per database, as for Aura's memory. | VALIDATED on 26.10.1, one server |
| 2 | **Opaque references cost retrieval under Aura's ranking.** With the spec's `[[PII_…]]` form the dense leg cannot tell people apart: pplx drops from 83 to 63 at rank 1, gemini from 82 to 76, EmbeddingGemma from 79 to 72. Splitting a person into given-name and surname references does not help. | VALIDATED on this set |
| 3 | **Consistent surrogates keep retrieval on gemini-embedding-2**: 83.0 at rank 1 against 82 in clear, MRR 0.878 against 0.877, and the clear value inside the five keys' range. On pplx they cost 2.6 at rank 1 and 2.6 within 3, mostly on the V class (a value in the question: 4 → 2.6) and P (7 → 6.0). On EmbeddingGemma they cost 1.6. | VALIDATED for gemini; PARTIAL for pplx and EmbeddingGemma |
| 4 | **OpenRouter's Sensitive Info redaction is the worst for retrieval and is not reversible.** Its placeholders name only the type, so every person becomes `[PERSON_NAME]`: pplx 63, A class 1 of 7. Companies pass in clear. | VALIDATED, simulated with perfect detection |
| 5 | **Ordering by the fusion instead of the cosine makes the representation almost irrelevant**, because the lexical leg runs locally on the original text: gemini 83 with tokens or redaction against 82 in clear, pplx 80 with tokens. Aura re-ranks by cosine so that its abstention floor means something; ordering by the fusion was measured on 2026-09-09, gave the same ranking on that corpus and was not adopted (`fusedStatement`'s comment). The variant here keeps the cosine floor and orders by the fusion; with names in the corpus the two orders differ. | VALIDATED on this set; a retrieval change, not measured on the live index |
| 6 | **The HTTP `redis` language runs a second command at a line break inside an argument** (`GET x\nSET y`: two replies). Over HTTP a value must never be an argument; vault keys are HMAC hex, values travel only inside the JSON. RESP is length-prefixed and unaffected. | VALIDATED on 26.10.1 |
| 7 | **Transient `SET`/`GET` are unusable for the vault**: RAM only, gone after a restart. Contrary to the docs' "shared hashmap", a key set by one user's connection was not visible to another user's. | VALIDATED on 26.10.1 |

## A real document: the detector, not the vault

The operator then gave one of their own documents: a 7-page Italian utility bill, 27 KB of
`pdftotext` output. Nothing from it is committed: no text, no values, no supplier; only the
counts below. It never left the operator's PC. Rizzo ran there offline, in WSL, at the pinned
weights (`rizzoaiacademy/rizzo-pii-0.3B@a1c3c838`) and source (`30d6f9c6`), with torch 2.14 on CPU
and 2 threads, as on the lab VM. The VM's model cache from 2026-10-12 had been removed.

Hand labels: 18 occurrences of the customer's data. 10 are direct identifiers: the name twice, in
capitals and surname first; the street three times; the town three times; the codice fiscale; and a
partly masked IBAN, `IT` + `X`s + digits, which fails mod-97. 8 are the supply's own codes:
customer code, invoice number, meter point, meter serial, supply code and the customer's bank. 66
further occurrences are the supplier's own data. A span covers an occurrence when it masks every
letter and digit of it.

| detector | direct, of 10 | supply codes, of 8 | supplier's data masked too, of 66 | letters and digits masked |
|---|---|---|---|---|
| Rizzo's patterns alone (checksums) | 1 (the CF) | 0 | 9 | 8.4% |
| Rizzo's model alone | 10 | 7, 1 partial | 65 | 13.7% |
| patterns + model (Rizzo's `analyze`) | **10** | **8** | 65 | 17.9% |
| the same without the AMOUNT, DATE, URL and TIME labels | **10** | **8** | 65 | 8.6% |

Recall stays 18 of 18 when the vault propagates every value found once to all its occurrences, or
is seeded with the identity's own name: both add masking, not recall, on this document.

Time and memory: `analyze` takes 18.7 s median over three runs, after 7.4 s to load the model;
the patterns alone take 10 ms. Peak RSS is 1,366 MiB. At the VM's measured 2026-10-12 rate (4.76 s
for 2.2 KB) this bill would take about a minute; that figure is an extrapolation, not a VM run.

| # | Finding | Verdict |
|---|---|---|
| 8 | **Patterns cannot carry an Italian bill**: a capitalized surname-first name, a street, a masked IBAN and a supplier's codes have no checksum to find. On this document Rizzo's model found all of them. | VALIDATED on one document, 18 occurrences; G10 asks for 200 |
| 9 | **Rizzo's default labels mask the meaning away**: 110 amounts and 62 dates in a bill. Excluding the labels that are not personal data halves the masking and keeps every personal occurrence. Company names, the supplier's VAT number and its addresses stay masked (65 of 66), so a policy that wants them readable needs a list of the identity's counterparties, or a different label set. | VALIDATED on one document |
| 10 | **At 18.7 s for one bill on two cores, the model belongs to ingestion, not to the turn.** Once ingested, a document's values are in the vault, and pseudonymizing the turn by dictionary takes milliseconds (0.09 s for 217 texts above). A file opened directly without ingestion would still pay the model's cost. | PARTIAL: the split is measured, the ingestion-time path is not built |

### A second real document: a commercial offer

A `.docx` offer between two companies, the operator's and a client's, naming one contact and a
third, end customer: 9.3 KB of text, read with the standard library from the body, headers,
footers and notes. Same rules: run on the operator's PC, nothing committed but counts. Labels:
2 personal occurrences (the contact's name, a mobile-style phone); 19 business ones (the three
companies' names in five forms, two addresses, two VAT numbers, an email, a website, the offer
number); 9 mentions of public manufacturers' brands, which should stay readable.

| detector | personal, of 2 | business, of 19 | brands masked, of 9 | masked |
|---|---|---|---|---|
| Rizzo's patterns alone | 1 (the phone) | 3, 1 partial | 0 | 1.0% |
| patterns + model | 2 | 14 | 5 | 3.8% |
| without AMOUNT, DATE, URL, TIME, + vault propagation of exact values | 2 | 13 | 9 | 3.7% |
| the same, propagating each proper word of a company found once | 2 | **18** | 9 | 4.3% |

The model missed five mentions of one company written differently from its letterhead; once the
letterhead's name is in the vault word by word, every mention is replaced. The last miss is that
company's website, a URL the policy leaves readable. `analyze` took 3.7 s on two cores.

Across both documents the model with the patterns found **20 of 20 personal occurrences**.

| # | Finding | Verdict |
|---|---|---|
| 11 | **The vault closes what the model finds once and misses later**, if it propagates a company by its proper words: 13 → 18 of 19 business occurrences. Exact-value propagation adds nothing here, as on the bill. | VALIDATED on one document |
| 12 | **Public names need an allowlist.** Manufacturers' brands are tagged ORG, and propagation masks every mention: 9 of 9. Nothing about them is personal or confidential, and masking them takes the meaning out of a technical offer. | VALIDATED on one document |

## OpenRouter guardrails, read 2026-10-20

From the guardrails page and its Sensitive Info page:

- **Where it runs.** On OpenRouter, "before it is forwarded to the model provider": OpenRouter
  receives the clear text. It shields the downstream provider only.
- **What it covers.** Email, phone, SSN, credit card, IP, secrets (regex); person names and
  addresses "via Presidio", marked beta; the page warns that names without context, uncommon names
  and single-word names may be missed. There is no Italian preset: CF, P.IVA and IBAN need custom
  JavaScript regex, which forbid lookarounds and have no checksum.
- **What it inspects.** "message content, tool call arguments, and prompt strings", input only;
  model responses are not scanned. Embedding requests are not mentioned.
- **The placeholders cannot be restored**: `[PERSON_NAME]`, `[ADDRESS]`, `[REDACTED]`.
- **Failure mode.** The page says a request whose NLP check times out proceeds unblocked: it fails
  open. The spec's protected mode refuses to send on any detector failure.
- **What it does well and the spec does not replace**: Zero Data Retention enforcement per model
  group (`enforce_zdr_*`), provider and model allowlists, budget limits, and EU data region
  (`eu.openrouter.ai`, Business or Enterprise plan). These limit what a provider keeps; they do not
  keep personal data from leaving Aura.

## What this spike does not show

- **Detection is perfect by construction.** The vault is filled from the generator's ground truth.
  Recall on real documents is spike 109's G10 and is not touched here. Dates of birth, amounts,
  cities and postcodes are left in clear on purpose; D12 still carries "nata il 21/06/1990".
- **A small, synthetic, self-written set.** The questions were written with the documents. One to
  three questions of difference is within what another set, or another five keys, could reverse.
- **Simulated ranking.** BM25 stands in for ArcadeDB's Lucene leg; the card leg and the memory
  family are not measured; the floors are EmbeddingGemma's for every model, as in Aura today.
- **One passage per synthetic document.** The real chunker was not run on them.
- **Only the embedding exit.** The chat model's handling of tokens or surrogates, and restoring them
  in tool arguments and streams, is spike 109's Q3/Q5. Surrogates are safe here because an embedding
  never comes back; on a path that must be restored, a surrogate can collide with real text, a
  token cannot.
- **A throwaway single server.** No HA, no backup, no load, no key rotation.
