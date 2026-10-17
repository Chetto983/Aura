# Paperless-ngx: behaviours for D5 (clean-room brief), 2026-10-17

Source: a shallow clone of `paperless-ngx` `dev` at `0cb2cdb` ("Fix: handle single-class classifier
in threshold prediction (#14471)"), read only. Paths below are relative to the clone's `src/`
unless they start with `docs/`. Paperless is GPL-3.0 and Aura is MIT, so this file describes
behaviour only. It does not reproduce Paperless code, regexes or prompts.

## Verdict

Four Paperless mechanisms map onto D5 directly:
- **the field model**: a small fixed set of first-class fields (correspondent, document type,
  created date, tags) plus typed custom fields, where each type has its own validator and its own
  set of filter operators;
- **the deterministic matchers** (Any, All, Exact, Regex, Fuzzy), because they are the "rules
  first" step and the baseline every learned piece has to beat;
- **the classifier's training and abstention discipline**: it learns only from reviewed (non-inbox)
  documents, it learns a label only when the operator opts that label into Auto, it falls back to
  "no label" when unsure, it weights classes for balance, and it retrains only when the labels or
  documents changed;
- **filtered search**: full-text hits are intersected with a structured filter set, the order is
  kept, and the custom-field query language is bounded.

Paperless offers nothing for the part of D5 that Aura needs most. **It never extracts a custom
field's value from a document's content.** The values come from the operator, from the API, or
from a workflow's constant value (`documents/workflows/mutations.py:92-117`). The only content
extraction is the created date, which is the first plausible date found by a regex sweep, and
the four taxonomy objects. So an invoice's "Cliente | Gialli Import" or "Data emissione | 2026-03-27"
preamble pairs, which filecard drops today, have no counterpart in Paperless. Turning those pairs
into typed fields is Aura's own design, and the STOP BEFORE BESPOKE rule applies to it.

The LLM path is a useful pattern for the last resort, and only as a pattern:
- it is opt-in;
- the model chooses from candidate IDs that retrieval proposes;
- an ID it was not shown is rejected;
- what comes back is a suggestion until the operator or a workflow accepts it.

Its RAG and chat stack duplicates Aura's own and is not worth taking. Versioning stays dropped,
as the spec already decided.

## 1. Data model of document metadata

**What it is.** Each document has these fields:

| Field | Shape | Notes and refs |
|---|---|---|
| correspondent | one optional reference to a Correspondent | `documents/models.py:163-170`. Deleting the correspondent nulls the field |
| document_type | one optional reference to a DocumentType | `models.py:183-190` |
| storage_path | one optional reference to a StoragePath (a filing template) | `models.py:172-179`, `148-156` |
| tags | many-to-many, nested up to depth 5 | `models.py:212-217`. Depth cap `MAX_NESTING_DEPTH = 5` at `:106`. Parent-cycle and depth checks at `:121-139` |
| created | a date, distinct from `added` (a datetime set at ingest) and `modified` | `models.py:249-267` |
| archive_serial_number | optional unique integer, 0 to 2^32-1 | `models.py:299-315` |
| custom fields | one row per (document, field), unique per pair | `models.py:1309-1410`. Unique constraint at `:1404-1409` |

Correspondent, type, tag and storage path all share one base (`MatchingModel`, `models.py:47-94`).
Each has a name, which is unique per owner (`:81-91`), plus a `match` string of up to 256
characters, an algorithm (default Any, `:69-74`) and a case-insensitive flag (default on, `:76`).
So the rule lives on the label itself. One label has exactly one rule.

The **inbox tag** (`models.py:108-115`) marks "not yet reviewed". Every new document gets the inbox
tags (`signals/handlers.py:83-93`), and the classifier ignores any document that carries one
(item 3).

**Custom field types** (`models.py:1257-1267`). The type is fixed when the field is created
(`editable=False`, `:1277-1282`). Each type writes to its own typed column (`:1315-1326`):

| Type | Stored as | Validation on write (`serialisers.py:906-977`) | Filter operators (`filters.py:386-397`) |
|---|---|---|---|
| string | text, at most 128 characters | maximum length 128 | equality, in, isnull, exists, contains, starts with, ends with |
| longtext | unbounded text | none | the same as string |
| url | URL | must parse with a scheme and a host or path (`validators.py:5-48`) | the same as string |
| date | date | parsed as an ISO date | equality, in, isnull, exists, gt, gte, lt, lte, range. Also year, month, quarter, week and other date parts (`filters.py:399-408`) |
| boolean | boolean | none | equality, in, isnull, exists |
| integer | integer | must be an integer inside the signed 32-bit range | basic and arithmetic |
| float | float | none | basic and arithmetic |
| monetary | the string as written, plus a generated decimal amount | either a plain decimal (12 digits, 2 places) or an ISO-4217-style three-letter code, an optional minus, digits and exactly two decimals (`:931-946`). An optional per-field `default_currency` must be three characters (`serialisers.py:823-839`) | basic, string and arithmetic. Arithmetic and equality compare the generated decimal amount (`models.py:1368-1388`, `filters.py:549-553`) |
| select | an option ID (16 random characters) | must be one of the field's option IDs. Options need non-empty labels, and IDs are generated if missing (`serialisers.py:801-822`) | basic. A filter may give the option's label instead of its ID (`filters.py:657-659`) |
| documentlink | a list of document IDs | must be a list of documents that exist (`serialisers.py:857-875`). Links are mirrored onto the target document (`bulk_edit.reflect_doclinks`) | basic plus "contains" |

The full-text index sees a select field's label, not its ID (`models.py:1432-1449`).

Edge cases worth keeping:
- monetary is stored as written and the amount is derived from it, so the currency survives and
  ranges still compare numbers;
- the amount column takes a value that starts with a digit as it is, and otherwise drops the
  first three characters (`models.py:1371-1384`);
- `created` is a date, not a datetime.

**For Aura.**
- **Where it lands.** The document-level fields belong on `IndexedDocument`
  (`services/ingest/app.py:61-97`), with matching DDL in `services/ingest/arcade.py:307-320`, so
  that `candidateWhere` (`internal/arcadedb/document_retrieval.go:309-330`) can filter on them.
  `CandidateFilter` (`:51-57`) gains typed predicates. `RetrievalRequest`
  (`internal/documents/retrieval.go:70-82`) and the `document_search` tool schema
  (`internal/agent/tools/document_search.go:28-31`) gain the matching inputs.
- **What exists.** Only `DocumentIDs`, `SourceKeys` and `SourcePrefixes` are there.
- **What is new.** A small field catalog, per identity, holding a name, a fixed type and select
  options. Then per-document values. Then a per-field `source`, so the operator, a rule, the
  classifier and the LLM stay distinguishable (item 7).
- **What to measure.** Start with three fields at most: counterparty, document type and the
  document's own date. That is what q09 ("tutte le fatture del cliente Gialli Import", FILTER) and
  q18 ("fatture emesse a marzo 2026", TEMPORAL) ask for (`scripts/fixtures/document_retrieval_eval/qrels.json:12,21`).
  Add amount only if a question in the "which documents…" set needs it.
- **Risk.**
  - Fixing a type at creation is right, but it needs a migration story in ArcadeDB.
  - The q09 and q18 gold sets both include `contabilita_2026.xlsx`, a ledger rather than an
    invoice. A hard "type = invoice" or "created in March" filter would cut a gold document that
    ranking finds today. The filter has to apply to what the document says, or be a boost rather
    than a cut, and that choice is measured, not assumed.
  - Tags are not in D5. Nesting is not worth taking without a measured question that needs it.

## 2. Matching (the rules)

**Where it runs.** At the end of ingest, inside the storing transaction, the handlers run in this
order:
1. inbox tags;
2. correspondent;
3. type;
4. tags;
5. storage path;
6. the "document added" workflows.

The order is set in `documents/apps.py:24-29`. The handlers are fired in `consumer.py:685-692`,
and the classifier is loaded once beforehand (`consumer.py:596`). A value given explicitly at
upload, or set by a workflow at consumption start, is applied first (`consumer.py:78-90`,
`914-977`). The single-value handlers then skip a field that is already set unless `replace` is
on (`signals/handlers.py:124-125`, `185-186`, `311-312`). So explicit beats rule, and rule beats
classifier only because both are folded into one candidate list (below).

**What the rules match against.** Only the document's extracted text (`matching.py:168`, the
latest version's content). Title, filename and metadata are not matched. An empty `match` never
matches (`:171-172`). Case-insensitive is the default.

| Algorithm | Behaviour (`matching.py:165-259`) |
|---|---|
| None | never matches |
| Any | the match string is split into terms; matches if any term occurs as a whole word |
| All | matches only if every term occurs as a whole word, in any order |
| Exact ("Literal") | the whole match string, escaped, occurs bounded by word boundaries |
| Regex | the operator's pattern is searched for anywhere. Patterns are checked at save (`serialisers.py:199-211`). At match time an invalid pattern or a timeout counts as no match and is logged (`regex.py:28-52`). The timeout is `MATCH_REGEX_TIMEOUT_SECONDS`, default 0.1 s (`paperless/settings/__init__.py:103-106`) |
| Fuzzy | punctuation is stripped from both sides, and both are lowercased when insensitive. It matches if a partial-ratio similarity of at least 90 is found anywhere in the text (`rapidfuzz.fuzz.partial_ratio`, `matching.py:228-245`). The cost scales with the text length |
| Auto | never matches by rule; see item 3 |

Term splitting (`matching.py:262-278`):
- a double-quoted group is one term;
- runs of whitespace inside a quoted term match any amount of whitespace;
- every term is escaped, so terms are literal and never patterns.

"Whole word" is the regex word boundary, so "alphas" does not satisfy "alpha" and "123456" does
not satisfy "12".

**Conflicts.**
- **Single-value fields.** Every rule match plus the classifier's one prediction (when that label
  is Auto) forms the candidate list. The list is ordered by the label's name, which is the model's
  default ordering (`models.py:80`). The first candidate wins by default (`use_first=True`), and
  the alternative mode assigns nothing when more than one candidate matches
  (`signals/handlers.py:127-144`). A rule and the classifier have equal standing. Alphabetical
  order decides, which is arbitrary.
- **Tags.** All matches are added, each with its ancestors (`models.py:541-548`).
- **Replace mode.** Rule- and Auto-managed tags are removed before the new set is applied, while
  inbox tags and tags with no rule are kept (`signals/handlers.py:248-270`). A single-value field
  in replace mode with no match is cleared to empty (`:146`).
- **Explanations.** Every match logs a one-line reason (`matching.py:33-44`).

**For Aura.**
- **Where it lands.** The ingest, after extraction and the card, in `services/ingest/app.py`
  `index_object` (`:325`). Or in Go beside filecard, if the rules have to read the card's
  key/value pairs.
- **What exists.** Nothing. Aura has no per-label rules.
- **What is new.**
  - A rule per field value. For example, "counterparty = Gialli Import when Exact
    'Gialli Import'".
  - Run the rules against more than the content: the file name and, above all, the preamble
    key/value pairs filecard now discards (`internal/documents/filecard/table.go:110-150`, where
    `rowsSkipped` counts them and throws them away).
  - Resolve a conflict by abstaining, not by alphabetical order. Two rules for different
    counterparties firing on one document is a signal to surface, not a coin toss.
  - Record the reason line.
- **What to measure.** The rule-only assignment's precision and recall per field on a labelled
  sample. This is the dumb baseline for item 3.
- **Risk.**
  - Fuzzy over a whole document is slow and imprecise. Prefer Exact, or Fuzzy only against the
    short key/value values.
  - Regex rules need a timeout. Go's RE2 has no catastrophic backtracking, so the timeout is less
    critical, but the semantics differ from Python's `regex`.

## 3. The "Auto" classifier

**Features** (`classifier.py:463-492`, `586-597`, `124-147`):
- The text is NFC-normalised.
- With a configured language, a Tantivy analyser builds the tokens. It splits on word characters,
  lowercases, drops stopwords (Snowball's English list for English, Tantivy's own list for the
  others) and stems. Accents are kept, because stemmers need them.
- Without a language, the text is lowercased and split into word tokens.
- The language comes from the primary OCR language through a fixed table of 12 languages, Italian
  among them (`paperless/settings/__init__.py:1067-1090`, `1136`).
- The features are then a bag of word unigrams and bigrams (`CountVectorizer`, n-gram range 1–2,
  `min_df=0.01`, `classifier.py:479-483`), fitted over the whole training corpus. The content is
  streamed in chunks of 1000 documents (`:463-477`).

**Model** (`classifier.py:495-571`):
- There is one scikit-learn `MLPClassifier` per target: tags, correspondent, type and storage
  path, all with default hidden layers, `tol=0.01` and `random_state=0`.
- Correspondent, type and storage path are single-label multi-class, with balanced sample weights
  (`compute_sample_weight("balanced")`), because the MLP has no class weights (`:452-457`).
- Tags are multi-label through a multi-label binarizer. With exactly one Auto tag they fall back
  to binary classification (`:499-511`).

**What counts as a label** (`classifier.py:341-407`):
- Only documents with no inbox tag train the model (`:342-344`).
- A document's target is the assigned object only if that object's own algorithm is Auto.
  Otherwise the target is the pseudo-class −1, "none". So documents whose label was assigned by a
  rule or by hand teach the model "no Auto label here", and the model learns only the labels the
  operator chose to make learnable.
- Tag targets are only the Auto tags (`:367-377`, test `test_train_uses_only_auto_tags_as_labels`).

**Retraining trigger.**
- A scheduled task runs hourly at minute 5 by default (`paperless/settings/custom.py:92-101`).
- It does nothing, and deletes any old model file, when no object anywhere is set to Auto
  (`tasks.py:150-167`).
- `train()` skips the work if the latest document `modified` time is not newer than the last
  training, and a hash of all label assignments is unchanged (`classifier.py:363-431`).

**Persistence and versioning.**
- The model is a pickle signed with an HMAC (SHA-256 keyed by the secret key) and carries a
  `FORMAT_VERSION` (now 11, `classifier.py:202-210`).
- On a version mismatch or a corrupt or tampered file, the file is deleted and the classifier is
  absent until the next training (`:160-199`).
- Training-only MLP state is stripped before saving (`:238-250`).
- Vectorised inputs are cached for 5 minutes, keyed by content hash, format version, language and
  vectoriser hash (`:599-622`).

**Too little data.**
- No documents at all raises "No training data available." (`classifier.py:347-349`).
- An inbox-only library raises the same error (test `test_no_non_inbox_tags`).
- A target with zero learnable classes trains no model for that target (`:433-440`, `516-571`).
- There is no minimum per class. The documentation states the limits plainly
  (`docs/advanced_usage.md:55-98`):
  - a rare correspondent will not be learned;
  - negative examples are needed;
  - with only two Auto correspondents, every new document gets one of them;
  - a tag that does not correlate with the text ("TODO") cannot be learned.

**From prediction to assignment.**
- For the single-value targets, the winning class must not be −1 and its probability must be at
  least `CLASSIFIER_MATCH_THRESHOLD`, default 0.3 (`paperless/settings/__init__.py:96-102`,
  `classifier.py:47-69`).
- A value exactly at the threshold is accepted, and 0 disables the check (tests
  `test_predict_with_threshold` cases at `tests/test_classifier.py:1010-1027`).
- A single-class fit uses only probability column 0 (`:57-60`). That was the bug the head commit
  fixed.
- Tags use plain `predict` with no threshold (`classifier.py:646-665`).
- A prediction counts only for an object whose algorithm is Auto (`matching.py:68-71`). It then
  joins the rule matches under the conflict policy of item 2.
- Prediction reads `suggestion_content`. When the text exceeds 1.2 M characters, that is the first
  800 k plus the last 200 k (`models.py:437-462`).

**For Aura.**
- **Where it lands.** A learned component in the ingest sidecar. The ingest's dependencies
  (`docker/aura-ingest/requirements.txt`) carry no scikit-learn today, so this is a new
  dependency and a new artefact to store per identity.
- **What exists.** Nothing.
- **What is new.**
  - Training data is the operator's own confirmed values.
  - Aura has no inbox. Its equivalent is a field value whose `source` is `operator` (accepted or
    corrected) versus `rule`, `classifier` or `llm` (unreviewed). Train only on `operator`.
  - Keep the opt-in per label ("Auto" per value), the "none" pseudo-class, balanced weights, the
    probability floor and abstention.
- **What to measure (BASELINE OR DROP).**
  - Out of sample, on documents the operator has labelled: rules alone, then rules plus the
    classifier, with precision and recall per field.
  - A second dumb baseline: the most frequent value per file-name pattern or per folder
    (`source_key` prefix).
  - The classifier stays only if it beats both. Paperless publishes no such measurement. Its
    threshold of 0.3 and its `min_df` are defaults, not evidence.
- **Risk.**
  - Aura libraries per identity are small, so the documentation's own caveats bite: the classifier
    may never have enough examples.
  - Retraining and versioning add a model artefact per tenant.
  - The cheapest honest outcome may be "rules only".

## 4. Date extraction (the created date)

**Order of precedence** (`consumer.py:557-568`, `862-878`):
1. a date given at upload;
2. a date the format parser supplies:
   - for mail, the message date (`paperless/parsers/mail.py:270`);
   - for Tika office files, the metadata creation date (`paperless/parsers/tika.py:270-272`);
   - for scanned PDFs, none (`paperless/parsers/tesseract.py:120`, `149-150`);
3. the first valid date from the date parser;
4. the file's modification time.

The fourth step means an undated document gets a confident-looking date that is really the
file's own.

**The parser** (`documents/plugins/date_parsing/`):
- **Plugin choice.** A plugin entry point can replace the parser; the first plugin by name wins
  (`__init__.py:20-69`). The default is a regex sweep (`regex_parser.py:12-67`).
- **Shapes found.** The sweep finds candidate substrings shaped like:
  - day, month and year with `.`, `/` or `-` separators and a 2- or 4-digit year;
  - year first;
  - a day and a month name with a year;
  - a month name, a day and a comma with a year;
  - a month name with a year;
  - ordinal-suffixed days;
  - three-letter month abbreviations.

  Month names allow a fixed set of accented Latin letters. The boundaries also accept `_` and
  `-`, so a file name such as `scan_2024-01-15` is read.
- **Interpretation.** Each candidate goes to `dateparser.parse` with the configured `DATE_ORDER`
  (default `DMY`), "prefer first day of month" for dates with no day, timezone-aware output and the
  configured locales (`base.py:68-93`).
- **Locales.** `DATE_PARSER_LANGUAGES`, else derived from the OCR languages
  (`paperless/utils.py:118-160`).
- **Filter.** A candidate is kept only if its year is after 1900, it is not after the reference
  time ("now"), and it is not in `IGNORE_DATES` (`base.py:95-112`). The operator lists the ignored
  dates, typically a birth date that appears on every document (`paperless/settings/__init__.py:1050-1053`,
  `custom.py:358-385`).
- **Filename first.** If `FILENAME_DATE_ORDER` is set, the filename is swept first with that
  order. Otherwise the filename is skipped (`regex_parser.py:58-67`).
- **Choice among several.** The **first** valid one in document order wins (`consumer.py:568`).
  There is no scoring by label ("Data emissione") or by position.
- **Suggestions.** The suggestions API returns up to `NUMBER_OF_SUGGESTED_DATES` (default 3)
  distinct dates, sorted (`views.py:1595-1607`, `paperless/settings/__init__.py:1014`).
- **Timeout.** The sweep runs under the same regex timeout (`documents/regex.py:107-120`).

**For Aura.**
- **Where it lands.** The ingest, after extraction. Or filecard, for a workbook whose preamble
  holds a labelled date.
- **What exists.** filecard types columns and already recognises dates in table values
  (`internal/documents/filecard/table.go` kinds). Nothing picks a document date.
- **What is new.**
  - A labelled date ("Data emissione", "Data fattura", "Invoice date") taken from a key/value
    pair beats the first date in the text. Paperless's first-date rule is the baseline to beat.
  - Keep the future-date and ignore-list filters.
  - Keep a configurable day/month order. Aura's corpora are Italian, so DMY.
  - Do not fall back to the file's modification time silently. Leave the field empty instead.
- **What to measure.** The exact-date accuracy of the first-date baseline against the
  labelled-pair rule on the office corpus's invoices. Then the effect on q18.
- **Risk.**
  - Go has no `dateparser` equivalent in Aura's dependencies.
  - Writing a multilingual month-name parser is bespoke work: inventory first, and ask before
    building it.
  - Python's `dateparser` would be a new ingest dependency.

## 5. Search with filters

**The plain list API.** It filters on the database (`documents/filters.py:788-917`):
- **IDs.** Equality and `in` on the ID of the correspondent, type, storage path, tag and owner.
- **Exclusions and tag sets.** `__none` excludes a set, `tags__id__all` requires every listed
  tag, and `tags__id__in` accepts any listed tag (`filters.py:136-162`, `802-814`).
- **Names.** Contains, starts with, ends with and exact on the names of those objects and on the
  title (`CHAR_KWARGS`, `:61`).
- **Dates.** On `created`: year, month, day, gt, gte, lt and lte (`DATE_KWARGS`, `:64-72`). The
  same for `added` and `modified`, with datetime variants.
- **ASN.** Integer comparisons (`:63`).
- **Inbox and nulls.** An inbox filter, and isnull on each single-value field.
- **Custom fields.** `custom_field_query` takes a JSON expression tree:
  - a leaf is `[field name or ID, operator, value]`;
  - inner nodes are AND, OR and NOT;
  - depth is capped at 10 and leaves at 20 (`filters.py:87-88`, `368-450`, `520-575`, `747-753`);
  - an operator must be legal for the field's type (table in item 1, `:603-635`);
  - the value is validated by the type's own validator (`:637-700`);
  - `in` takes a non-empty list and `range` exactly two bounds;
  - `exists` is distinct from `isnull`, because a field can be attached with an empty value;
  - each leaf is a separate counted annotation, so two leaves can address two different fields
    on one document (`:555-573`).

**Full text** (`documents/search/`):
- The index is Tantivy. The query-visible fields include title, content, correspondent,
  document_type (alias `type`), storage_path, original_filename, tag, asn, page_count, created
  (a date), added, modified, notes and custom fields as name/value JSON (`search/_fields.py:17-47`).
- Query syntax supports field-scoped terms and date keywords. The documented ones are `today`,
  `yesterday`, `previous week`, `this month`, `previous month`, `previous quarter`, `this year`
  and `previous year`, plus ranges with relative bounds, as in `created:[2005 to 2009]` or
  `added:[-1 week to now]` (`docs/usage.md:999-1023`).
- A plain search does not look at custom fields. They need the field-scoped syntax
  (`docs/usage.md:1025-1031`).

**How the two combine** (`documents/views.py:2575-2700`):
1. Tantivy returns the ordered IDs of every hit (permission-filtered, with an optional
   `filter_query` that hits must also match).
2. The view intersects those IDs with the database queryset after all the filters above.
3. Tantivy's relevance order is kept.

Below 5,000 hits the intersection is an IN clause; above that it is a scan plus a set
intersection (`views.py:285-292`, `2620-2648`). The filter is a hard cut applied after ranking,
and pagination happens after the intersection.

**For Aura.**
- **Where it lands.** `candidateWhere` (`internal/arcadedb/document_retrieval.go:309-330`) already
  builds one WHERE clause shared by both the vector and the full-text leg. The filters would go
  there, server-side, before fusion. That is better than Paperless's after-the-fact
  intersection, because the candidate limit is spent only on admissible documents.
- **What exists.**
  - Document, key and prefix filters.
  - Passages carry `search_document_id`, so a document-level filter has to reach the passage rows
    in one of two ways:
    - denormalise the few filter fields onto `Passage`;
    - or select the admissible `search_document_id`s from `IndexedDocument` first and pass them as
      `DocumentIDs`. That path already exists and is capped by `MaxDocumentFilters`.

    Which of the two is right is a measurement on ArcadeDB, not a guess.
- **What is new.**
  - Typed predicates: equality and `in` on counterparty and type, and a range on the date.
  - Bounded size, copying the depth and leaf caps as behaviour.
  - The agent-facing `document_search` arguments, so the agent can say "counterparty Gialli
    Import, date in 2026-03".
  - D4's "which documents…" sweep would read the same fields straight from `IndexedDocument`
    with no text query.
- **What to measure.** q09 and q18, plus a new "which documents…" set of at least 10 questions per
  corpus. Compare r@8 and exact set recall with filters against without.
- **Risk.**
  - A hard filter on a wrongly assigned field silently hides the right document. Paperless
    accepts this because the operator reviews every inbox document. Aura's operator does not.
  - Consider "filter only on operator- or rule-sourced values", or a fallback to an unfiltered
    query when the filtered one abstains.

## 6. Workflows that assign metadata on consumption

- **Triggers.** A workflow has triggers (consumption started, document added, document updated,
  scheduled) and an ordered list of actions (`docs/usage.md:483-490`).
- **Consumption-started trigger.** Filters on source, mail rule, a filename glob (lowercased) and
  a path glob (`matching.py:281-351`).
- **Added and updated triggers.** These can also require a content match, using the same
  algorithms as item 2. They can also require any, all or none of given tags, one of or none of
  given correspondents, types or storage paths, and a custom-field query
  (`matching.py:354-550`).
- **Assignment actions.** These set fixed values: tags (merged, with ancestors), correspondent,
  type, storage path, owner, a templated title, permissions (merged), and custom fields with a
  fixed value or none (`workflows/mutations.py:16-118`).
- **Consumption-started overrides.** These assignments become upload overrides
  (`mutations.py:120-199`), and so beat the rules and the classifier (item 2).
- **Precedence.** For a single-value field, a later action overrides an earlier one. Tags, custom
  fields and permissions are merged (`docs/usage.md:487-489`).
- **Updating a custom field.** An empty value does not overwrite an existing instance
  (`mutations.py:107-117`).
- **Removal actions** strip tags and the like (`mutations.py:201+`).
- **The AI action.** One action type runs the AI suggestions (item 7).

**For Aura.** Glob rules on the source key ("everything under `fatture/` is type invoice") are a
cheap, strong rule for D5. They are a variant of item 2 that Aura's `SourcePrefixes` already
half-implements as a filter. They would land as a rule kind in the same rule table, not as a
workflow engine. Measure them as part of the rule baseline. The risk is low. A full workflow
engine with webhooks, e-mail and scheduled triggers is out of scope.

## 7. The AI/LLM suggestion path

**Switches.**
- Off by default: `PAPERLESS_AI_ENABLED` defaults to `NO` (`paperless/settings/__init__.py:1212`).
  The database setting overrides the environment (`paperless/config.py:251-306`).
- The backend is `ollama` or `openai-like` (`settings/__init__.py:1232-1235`).
- The documentation warns that document content leaves the server with a remote backend
  (`docs/advanced_usage.md:100-115`).
- The built-in matching is never replaced (`:102-104`).

**What is sent** (`paperless_ai/ai_classifier.py:110-150`, `176-258`, `310-352`):
- **Prompt content.** The filename and the first 4,000 characters of content, further truncated
  to the context budget (`LLM_CONTEXT_SIZE`, default 8192). The content is explicitly marked as
  untrusted data whose instructions must not be followed (`prompts/classification.j2`).
- **Taxonomy candidates.**
  - Similar documents are retrieved: top 15 by vector similarity if an embedding backend is
    configured, else by full-text "more like this" with rank-derived weights (`:42-87`, `164-258`).
  - Their current tags, type, correspondent and storage path are weighted by similarity (summed
    per object). Each list is permission-filtered and capped: 10 tags and 5 of each single-value
    field (`paperless_ai/taxonomy.py:24-25`, `115-193`).
  - Inbox tags are excluded from the candidates (`taxonomy.py:145`).
  - The text of up to 5 similar documents, 1,000 characters each, is added as RAG context.
  - If retrieval fails, the prompt degrades to having no candidates (`:250-257`).
- **What the prompt asks for.** The model must:
  - first propose names from the document itself;
  - then, as a separate step, map a name to a candidate ID only when it means the same thing,
    never merely a related thing.

**What comes back.**
- **Structured output.** Ollama uses a JSON schema format with thinking off. Other backends use a
  required tool call (`paperless_ai/client.py:122-175`).
- **Bounds.** The schema bounds every list: 8 new names, 10 existing IDs, 4 single-value names,
  3 dates, and a title of up to 128 characters (`paperless_ai/base_model.py:12-17`). Over-long
  lists are truncated, not rejected (`:20-36`, `170-190`).
- **Validation** (`base_model.py:210-274`). An ID mapping is accepted only if:
  - its name is one of the model's own proposed names;
  - its ID was among the candidates shown for that category;
  - it is not a duplicate.

  An invalid mapping leaves the name as a "new name". An ID the model invents can therefore never
  be assigned.
- **Name matching.** New names are matched to existing objects: an exact match after
  lowercasing and stripping punctuation, else `difflib` close-match at a cutoff of 0.8. Each
  object can be claimed once (`paperless_ai/matching.py:13`, `115-147`).
- **Localisation.** An optional second LLM call localises the names, but never the IDs or the
  correspondents (`ai_classifier.py:316-350`).

**Suggestions versus accepted values.**
- The `ai_suggestions` endpoint returns matched IDs and `suggested_*` names that are not yet
  objects, and writes nothing (`views.py:1628-1810`).
- The suggestions are cached per document, backend, model, language and user, and the cache is
  invalidated on save (`signals/handlers.py:747-752`).
- The operator accepts or rejects suggestions in the interface.
- Only an explicit workflow action writes them (`workflows/ai.py:110-273`):
  - the workflow chooses which fields;
  - it sets a field only when that field is empty, unless "overwrite existing" is on;
  - it creates missing objects only when "create missing" is on, and never storage paths;
  - it always adds the suggested tags;
  - it uses the first valid YYYY-MM-DD date as `created`;
  - it skips a document with no content;
  - a bad configuration is swallowed, while a timeout or rate limit propagates for a retry.
- The AI path never touches custom fields.

**For Aura.**
- **Where it lands.** The last resort of D5, for a field neither a rule nor the classifier fills.
  The appliance's local LLM serves it, so the content stays local.
- **What exists.** Aura's own retrieval and LLM clients, and D6's planned per-file summary
  memoised on the file's bytes.
- **Worth taking as behaviour:**
  - the candidate list built from the operator's own values on similar documents;
  - accept only IDs that were shown;
  - names proposed first, mapping second;
  - bounded output;
  - content marked as untrusted data;
  - store the result as `source = llm` and never let it overwrite an `operator` value;
  - memoise the result on the file's hash and the model.
- **What to measure.** The marginal precision of the LLM step over rules plus the classifier, on
  documents the earlier steps left empty, and its cost per file on the appliance.
- **Risk.**
  - Prompt injection from document content, mitigated by the allow-list.
  - The cost on a large library.
  - The spec already notes that free LLM tags add little.

## 8. Their tests, as a source of Aura test cases

The test names and what each asserts are listed below. Rebuild the cases in Aura's own words and
fixtures. Do not copy the strings.

**Matching** (`documents/tests/test_matchables.py`):
- `test_match_none` (:129): the None algorithm never matches.
- `test_match_all`, three parametrised groups (:141-197):
  - all words in any order;
  - numbers as whole words, where "120" does not satisfy "12";
  - a quoted phrase whose inner whitespace may vary, but punctuation inside it breaks the match.
- `test_match_any` (:199-257): any single whole word, and quoted phrases as units.
- `test_match_literal` (:259-313): the whole string as one bounded unit; a partial word or a
  reordering does not match.
- `test_match_regex` (:314), `test_invalid_regex` (:338: an invalid pattern is no match, not an
  error), `test_match_regex_timeout_returns_false` (:350: a catastrophic pattern times out to no
  match with a warning).
- `test_match_fuzzy` (:365):
  - matches a town name with a dropped letter, a split word or missing punctuation;
  - does not match a different state abbreviation.
- `TestCaseSensitiveMatching` (:383-605): with insensitivity off, a capital letter in either the
  pattern or the text breaks the match, for every algorithm.
- `TestDocumentConsumptionFinishedSignal` (:607-662): the ingest-end event applies a matching tag
  or correspondent, and not a non-matching one.
- `test_root_uses_latest_version_content` (:86): versioning, not relevant.

**Classifier** (`documents/tests/test_classifier.py`):
- training data: no documents raises (`test_no_training_data`, :139); an inbox-only library raises
  the same (`test_no_non_inbox_tags`, :151);
- retraining: none without change (:296), yes with change (:311), and yes when a label is switched
  to Auto with no document change (`test_retrain_if_auto_match_set_changed`, :331);
- model file: load and classify (:388); a corrupt, tampered, truncated, wrong-key or older-library
  file is rejected and deleted (:397-518, :845-913);
- predictions: a single learnable class among "none" documents, for correspondent, type and path
  (:519-632); below-threshold rejection (:633); balanced weights (:683); tag cases with one tag,
  one unassigned, two tags on one document, two tags, and one tag among several (:730-844);
- the threshold table `test_predict_with_threshold` (:1010-1027), five cases: above accepted,
  below rejected, exactly at accepted, "none" winning rejected at any confidence, 0 disables;
  plus single-class handling (:1040-1080);
- preprocessing: a supported language stems and drops stopwords, an unsupported one only
  lowercases, empty content is handled (:1083-1147); every language builds and underscores stay
  inside tokens (:1280-1330);
- labels and alignment: only Auto tags are labels (:1189); features and labels stay aligned
  across chunks and when a document is deleted mid-training (:1216-1278).

**Dates** (`documents/tests/date_parsing/test_date_parsing.py`, `tests/test_consumer.py:1214-1340`):
- parsing: DMY, MDY and YMD with slashes, dots and dashes, and month names or abbreviations, are
  valid; plain text, an impossible day or month, empty input, gibberish and out-of-range dates
  give nothing;
- filter: the reference time is an inclusive bound and a later date is rejected; 1901 passes,
  1899 and 1900 fail; an ignored date fails at any time of day; the dates are timezone-aware;
- parser: the filename and content orders apply separately; an ambiguous date follows the
  configured order; one bad candidate is skipped while good ones are still yielded; no match is
  an empty result; the filename is skipped when its order is unset;
- consumer: the date comes from the content, or from the filename when enabled, and ignored dates
  in the content are skipped.

**Custom-field filters** (`documents/tests/test_api_filter_by_custom_fields.py:246-785`):
- name/value association (one leaf must not combine one field's name with another field's
  value), several fields at once;
- exact, in, isnull versus exists, exists false, select by label;
- contains, starts with and ends with, including on URL;
- gt, gte, lt, lte, range, and date parts;
- monetary greater-than, exact and in, with and without a currency prefix, and an invalid
  monetary value;
- document-link contains (empty set, no reverse link, unrelated link field);
- AND, OR, NOT;
- invalid JSON, expression, field name, operator, value, logical operator, list or operator
  prefix;
- too deep, too many atoms.

**AI** (`paperless_ai/tests/`):
- `test_base_model.py`:
  - valid candidate mappings replace only the matched names;
  - an invalid or unpaired mapping keeps the name;
  - a blank name cannot authorise a mapping;
  - every list in the schema is bounded;
  - single-value lists are capped below the tag list;
  - an over-long response is truncated.
- `test_ai_classifier.py`:
  - retrieval failure degrades to no hints (:1050, :1078);
  - permissions: with no user only unowned documents are used, and private content never reaches
    the context (:616-760, :900-1048);
  - the prompt never includes the document's own already-assigned metadata (:1176);
  - localisation localises only new names (:1205).
- `test_matching.py`: exact, then fuzzy, matching of names to objects; empty names; IDs that do
  not exist or are not visible are dropped.

**For Aura.** The cases that carry over directly:
- the boundary-sensitive matcher table, rebuilt with Italian invoice text;
- the five-case threshold table;
- the ignore-dates, future-date and DMY-ambiguity date cases;
- the operator-per-type legality and bounded-query cases;
- the "ID not shown is rejected" LLM case.

Paperless has no test that measures the classifier against a baseline. Aura's BASELINE OR DROP
gate is an addition, not a port.

## Not worth taking

- **Storage paths.** They are filing templates for Paperless's own file tree. Aura's objects live
  in Garage under their own keys.
- **Nested tags, inbox tags and tag colours.** They are interface organisation. Aura's "unreviewed"
  is a value's `source`, not a tag. Nesting brings complexity no measured question needs.
- **Archive serial number and barcodes.** They are a physical-archive feature.
- **Owner and permission machinery, saved views, share links.** Aura isolates per identity by
  database, not by row permissions.
- **The Tantivy index and its query language, the RAG index, chat and embeddings.** Aura has its
  own engine (ArcadeDB full-text plus vector, fused server-side) and its own retrieval contract.
- **The workflow engine beyond glob rules.** It brings webhooks, e-mail, scheduled triggers and
  password removal.
- **Versions.** Already dropped in the spec, 2026-10-17.
- **The `modified`-time fallback for the created date.** It manufactures a date the document never
  stated.
- **Alphabetical first-wins conflict resolution.** It is arbitrary. Abstain instead.
- **Fuzzy matching over the whole text.** It is slow on long documents and imprecise.
- **The hourly cron retraining.** Aura would retrain on the event "operator corrected a value",
  if it keeps the classifier at all.

## Clean-room note

No Paperless code was copied, translated or paraphrased line by line into this file or into Aura,
and no file in `/home/user/Aura` was changed except this report. Paperless code was not run,
imported or executed; it was read with `grep`, `sed` and `cat`. The date regex, the prompt
templates and the test strings are described, never reproduced.

Constants and values quoted as evidence: the algorithm names (None, Any, All, Exact, Regex, Fuzzy,
Auto); fuzzy cutoff 90; regex timeout 0.1 s; `min_df=0.01`, n-grams 1–2,
`MLPClassifier(tol=0.01, random_state=0)`, balanced sample weights; `CLASSIFIER_MATCH_THRESHOLD`
0.3; `FORMAT_VERSION = 11`; retrain cron "5 */1 * * *"; the 1.2 M / 800 k / 200 k content cut;
12 classifier languages; `DATE_ORDER` `DMY`; the date filter (year after 1900, not after now, not
ignored); `NUMBER_OF_SUGGESTED_DATES` 3; tag depth 5; ASN 0 to 0xFFFFFFFF; string field 128;
integer ±2^31; monetary as a three-letter code plus two decimals; custom-field query depth 10 and
leaves 20; search intersection threshold 5,000; `PAPERLESS_AI_ENABLED` `NO`; AI candidate caps 10
and 5, RAG top-k 15 with 5 × 1,000 characters, prompt content 4,000 characters,
`LLM_CONTEXT_SIZE` 8192, schema caps 8, 10, 4 and 3 with a 128-character title, name-match
cutoff 0.8; the docstring phrase "No training data available.".

## What this read does not prove

- It was read at one commit (`0cb2cdb` on `dev`) of a shallow clone. Behaviour on a release tag,
  or after this commit, may differ. The head commit itself changed classifier prediction.
- Nothing was executed. The behaviour is inferred from the source, the documentation and the
  tests' declared expectations, not observed. In particular, the claimed effect of balanced
  weights, the 0.3 threshold and `min_df` on accuracy is unmeasured here, and Paperless publishes
  no accuracy figure.
- `docs/configuration.md` was not read in full, the frontend (how suggestions are shown and
  accepted) was not read, and `paperless_mail` rules were not read beyond what workflows reference.
- It does not measure anything on Aura. None of the "what to measure" lines above has run. The
  q09/q18 note about `contabilita_2026.xlsx` comes from the gold lists, not from a filtered run.
- It does not establish that ArcadeDB can filter efficiently on the new properties, or which of
  the two filter shapes in item 5 is right. That needs a read of ArcadeDB's documentation first,
  then a probe.
