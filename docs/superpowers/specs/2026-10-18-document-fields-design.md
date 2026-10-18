# Document fields: type, counterparty, date, tags, reviewed by the operator

Proposed on 2026-10-18 at `ba4b03b`. It is item D5 of `2026-10-09-consolidation-best-of-design.md`.
The operator asked for it ("Andiamo avanti con le funzioni di classificazione dei documenti"),
proposed taking Paperless-ngx's interface as the example, chose a new Documenti view on SVAR's
grid over a fork of the file manager ("Va bene"), and approved the three mockups ("Vanno bene,
scrivi la spec").
- **The mockups:** https://claude.ai/artifact/XsQ73gmMzmHC6STSqekmZN. They are private to the
  operator: list and filters, bulk edit, detail and review.
- **The readings:**
  - `docs/paperless-ngx-behaviours-2026-10-17.md` (backend);
  - `docs/paperless-ngx-ui-behaviours-2026-10-18.md` (interface);
  - `docs/svar-filemanager-inventory-2026-10-18.md` (SVAR).
- **Paperless is GPL-3.0.** Behaviours only, no code.

Risk tier: **medium.** It adds data the operator owns and that nothing can rebuild, and it adds
three arguments to an agent tool.

## The problem, measured and read

1. **A question over many documents has no handle.**
   - Office q09 ("tutte le fatture del cliente Gialli Import") and q18 ("fatture emesse a marzo
     2026") ask for a set of documents.
   - `document_search` can filter only by document id, by file and by folder
     (`CandidateFilter`, `internal/arcadedb/document_retrieval.go:51-57`).
   - On 2026-10-17 q18 ranked fourth, "a question over every document, which no ranking
     answers" (`2026-10-17-workbook-row-passages.md:67-68`).
2. **The fields are in the files, and Aura drops them.**
   - filecard reads an invoice's top rows ("Fattura n.", "Cliente", "Partita IVA",
     "Data emissione", "Stato") as a banner and skips them (`internal/documents/filecard/table.go:129-132`).
   - So "Cliente | Gialli Import" is in neither the card nor the row passages. This is read from
     the code; no test pins it.
3. **Nothing about a document survives its key.**
   - `IndexedDocument` belongs to CocoIndex, keyed by `search_document_id`, which hashes the
     object key (`services/ingest/identity.py:38-57`).
   - A rename or a move in the file manager is a copy then a delete
     (`internal/assets/filemanager_ops.go:120-180`). CocoIndex then runs `DETACH DELETE` on the old
     vertex (cocoindex 1.0.25 `connectors/neo4j/_cypher.py:117`).
   - A property written on it by anyone else is gone too. Only `raw_sha256` survives a move, and
     only the key survives an edit.
4. **0098 retired the old catalog on purpose.**
   - `0098_retire_document_catalog.up.sql:30-31`: "document titles and tags stop existing. A
     document's name is its object key, and anything richer has to live in S3 object metadata."
   - A new table of document fields reverses that, unless it is measured to be needed.

## Goal

The operator sees, filters, corrects and reviews a few fields on every document, in a Documenti
view modelled on Paperless's list. The agent filters `document_search` by the same fields.
- Every value records who set it: the operator, a rule, the extractor, a classifier or an LLM.
- A value nobody reviewed is never treated like one the operator set.
- The fields survive a rename, a move and a re-ingest.

## Non-goals

- **A classifier, and LLM suggestions.** They are phases 2 and 3 below, each behind its own
  measurement. This spec ships the fields, the rules, the extractor, the review loop, the view
  and the filters.
- **Saved views, nested tags, permissions per document, a full-text query language.**
- **Versioning**, which the operator dropped on 2026-10-17.
- **Questions over every document** (D4). Filters narrow a search; they do not sweep a library.

## Anti-goals

The slice has failed, even if it works, if any of these happens:
- a filter hides a gold document the unfiltered search found, without the agent being able to
  search again unfiltered;
- a rename or a move loses a value the operator set;
- an unreviewed value is shown or used with the same weight as an operator's;
- one identity sees or changes another's fields.

## Decisions

### 1. The fields

- **Fixed fields:**
  - `type`, one value from the operator's list (fattura, contratto, registro contabile…);
  - `counterparty`, one value from the operator's list;
  - `document_date`, a date;
  - `tags`, a set from the operator's list.
- **Typed custom fields:** date, monetary (an ISO 4217 currency plus an amount), integer, text,
  and select. A field's type is fixed once it is created.
- **The lists** (types, counterparties, tags, custom fields) belong to the identity, with names
  unique per kind.
- **Every value carries:**
  - `set_by`: `operator`, `rule:<name>`, `extractor`, `classifier`, `llm`;
  - `set_at`;
  - for anything but `operator`, the evidence: the rule's pattern, or the header line read.
- **Every document carries `reviewed_at` and `reviewed_by`,** null until the operator marks it
  reviewed.

### 2. Where the fields live: the operator's call, decided by Task 0

**Proposed: S3 object metadata as the source of truth, as 0098 says, mirrored into ArcadeDB by
the ingest.**
- **Writing.** The cockpit writes a document's current values and their `set_by` into the
  object's user metadata, a copy onto itself with the metadata replaced.
- **Rename and move.** Both are a copy, so the metadata travels with the object. This is
  inferred from S3's default copy semantics, not measured (`internal/objectstore/s3.go:286-292`).
- **Mirroring.** The ingest reads the metadata with the object and writes it onto
  `IndexedDocument` and every `Passage` as CocoIndex-owned properties. The search filters them in
  the same statement, with no second store to join.
- **History** goes in `aura.document_field_events`: one row per change, with RLS like
  `aura.assets`. It is an audit and the training data of phase 2, never the current value.
- **The open risk.** A metadata change changes the ETag, so CocoIndex re-processes the file.
  `_extract` and the embeddings are memoized on the bytes (`services/ingest/app.py:253, 294`;
  `embed.py:188`). A re-process should then cost a re-read, not a re-embed, but nobody has
  measured that.

**The alternative: a Postgres table** `aura.document_fields` keyed by `aura.assets.id`.
- **What it buys.** The asset row survives a rename or a move, because `RelocateAsset` rewrites
  `object_key`. A write is a row update, with no re-ingest.
- **What it costs.**
  - It reverses 0098.
  - An object without an asset row has no fields.
  - The search must turn a filter into document ids before querying, and `CandidateFilter`
    caps those at 100 (`defaultDocumentFilterCap`). A filter like "type = fattura" can match more.

**Task 0 measures the proposed path** on a disposable stack: write metadata, rename, move,
re-ingest, edit the bytes, then the time and the embedding calls a metadata-only change costs.
- **If it holds:** the proposed path.
- **If Garage cannot replace metadata in place, the metadata does not survive the copy, or a
  metadata change re-embeds:** the alternative, with the 100-id cap raised or the filter moved
  into the statement, and 0098 amended with the measurement.

### 3. Assignment, in order, never overwriting the operator

1. **The operator.** A value the operator set is never changed by anything below. Their removal
   of a value is recorded, and that value is never re-suggested.
2. **Rules on the label**, as in Paperless: Any, All, Exact, Regex and Fuzzy over the document's
   text and file name.
   - A rule lives on its type, counterparty or tag, case-insensitive by default.
   - It runs at ingest and when the operator saves a rule.
3. **The extractor.** filecard keeps an invoice's header pairs and offers the date, the
   counterparty and the number as candidates.
   - **This is bespoke: STOP BEFORE BESPOKE applies, and the inventory is below.**
   - Paperless never extracts a value from content, except a date found by a regex sweep
     (`paperless-ngx-behaviours-2026-10-17.md`).
   - filecard already reads the rows and only discards them (`table.go:108-145`). Keeping them is
     a change of what it returns, not a new parser.
4. **Nothing else in this spec.** No classifier and no LLM.

A rule's or the extractor's value is a **suggestion** until the operator accepts it, edits it, or
marks the document reviewed.
- Until then the document counts as "da rivedere".
- **A filter matches it only when the agent asks for unreviewed values too** (decision 5).

### 4. The Documenti view

**It lives where the operator already looks.** The cockpit's `documents` surface renders
`FilesWorkspace` today (`web/src/AppShell.tsx:377-378`). It gains a toggle between **Cartelle**,
the existing file manager unchanged, and **Documenti**, the new view. The choice is remembered
like the surface itself.

**The new view is built on `@svar-ui/react-grid` 2.7.0,** added as a direct dependency (MIT). It
is already resolved through react-filemanager (`web/package-lock.json:6089`). It follows the
three mockups and the five behaviours of the UI brief:
1. **List and filters.**
   - Columns: name, type, counterparty, date, the custom fields the operator switches on, tags,
     and review state. Next to a value its operator did not set, a small mark names the source.
   - Above the list, a bar with a search box, then dropdowns for type, counterparty and tags.
     Each item shows its count in the current result, plus Include, Exclude and "Not assigned";
     tags add Any or All.
   - Dates take ranges and presets. A "da rivedere" counter sits at the top, and Reset clears
     the filters.
   - Clicking a value in a row filters by it.
2. **Bulk edit.**
   - While documents are selected, the filter bar becomes the edit bar.
   - Each value shows three states: on all, on some, on none.
   - Apply sends only the difference, after a confirmation naming the change and the number of
     documents.
   - "All" means the whole filter minus exclusions, not a list of ids.
3. **Detail and review.**
   - A preview, the field form, and every suggestion under its field with its source and
     evidence. A click accepts it, and × rejects it; a rejection is recorded.
   - The document's history.
   - Save, plus "Rivisto, salva e avanti" over the "da rivedere" list.
4. **The lists.** One page each for types, counterparties, tags and custom fields: name,
   matching rule shown in words, document count, and "filter documents". The pattern box appears
   only for the algorithms that need one.

**No field editor goes inside the file manager.** Its root keydown handler would turn typing
into file actions (`docs/svar-filemanager-inventory-2026-10-18.md`).

### 5. `document_search` gains filters

The new arguments, all optional, strictly decoded like the existing ones
(`internal/agent/tools/document_search.go:130-146`):
- `type`, `counterparty`: a list of names;
- `tags`: a list, with `tags_mode` `any` or `all`;
- `date_from`, `date_to`;
- `include_unreviewed`: a boolean, default `true`, so a library nobody reviewed still answers.

Each filter becomes a WHERE on both legs of the fused statement and on the card leg, next to
`candidateWhere`. With 26.11.1 it can join the `where` predicate of
`2026-10-17-arcadedb-26-11-native-design.md`.

**The tool's result says which filters were applied and how many documents they excluded.** The
agent can then retry without them, which answers the first anti-goal.

## Shape

| Part | Files |
|---|---|
| Task 0, the measurement | a dated file in `docs/superpowers/verification/`; no product code |
| PRD | `prd.md` §11: the fields, where they live (per Task 0), the provenance, the filters, before the code |
| Storage | per Task 0: the object-metadata codec, or migration `<next>` from `ls internal/db/migrations/ \| tail -1` (today 0141). `aura.document_field_events` either way |
| Ingest | `services/ingest/app.py` and `arcade.py`: read the metadata, write the properties, the rules at ingest |
| filecard | `internal/documents/filecard/table.go` and neighbours: keep the header pairs as candidates, test-first |
| Go API | `internal/agui/`: list documents with fields, facet counts, bulk edit as a diff, the lists and rules; identity from the principal, never a parameter |
| Search | `internal/arcadedb/document_retrieval.go`, `document_cards.go`, `internal/documents/retrieval*.go`, `internal/agent/tools/document_search.go` |
| Cockpit | `web/src/documents/` (new): the view, the bar, the detail and the lists, beside `web/src/files/`; `web/package.json` gains `@svar-ui/react-grid` |

Every file stays under 600 lines.

### New env vars

None.

## Phases 2 and 3, gated

- **Phase 2: a classifier**, only for a label the operator opts into ("Auto"), trained only on
  reviewed documents.
  - It ships only if, out of sample on the operator's own reviewed documents, it beats the rules
    alone (BASELINE OR DROP).
  - It abstains when unsure, and it never sets a value: it suggests one.
- **Phase 3: an LLM suggestion,** last, opt-in, on the appliance's local model.
  - It picks only among the names the operator's lists already hold, or proposes a new name the
    operator must create.
  - The document's content goes in marked untrusted.
  - It must beat phase 2, or the rules where phase 2 was dropped, on the same set.

Each phase gets its own spec after its measurement.

## Acceptance

1. **Task 0** is recorded and decides decision 2.
2. **A "which documents" set.** At least 20 questions over the office corpus, extending q03, q08,
   q09 and q18, with the gold *answer documents* (the invoices themselves, not the ledger that
   lists them) written down before the code.
   - Measured with today's search, then with the filters the questions imply, set by the
     operator on a reviewed library.
   - **The gate:** set recall up, and no gold lost that the agent could not recover by an
     unfiltered retry.
3. **The extractor.** Every office invoice yields its counterparty, date and number as
   candidates, matching the generator's values (`spikes/document-routing-benchmark/generate_corpus.py:123-139`).
4. **Survival.** An operator value survives a rename, a move, a re-ingest and an edit of the
   bytes, on the live stack.
5. **Isolation.** A second identity sees none of the first identity's fields, lists or events,
   through the API, the search and the database.
6. **The cockpit.**
   - Component tests per part.
   - A Playwright run of list, filter, bulk edit with its confirmation, and review with "salva e
     avanti", with its evidence attached to the result.
7. **Go and Python units, race, coverage policy;** `make quality` green.
8. **E2E** on the lab VM: the operator reviews a library in the view, and the real agent answers
   the set through `document_search` with filters. Score above 9.8, with a dated file in
   `docs/superpowers/verification/`.

## Open questions

1. **Decision 2.** Object metadata or a Postgres table: the operator's call, with Task 0's
   number.
2. **Which types, counterparties and tags exist at the start?** An empty list, or one seeded
   from the office corpus.
3. **Does "reviewed" need a reviewer other than the operator?** One operator per identity today.

## What this spec does not prove

- **That filters help on libraries other than office.** The set is office only.
- **That the agent picks the right filter from a question.** Acceptance 8 shows it on one set,
  not in general.
- **The classifier and the LLM,** until their phases run.
