# Document cards: embed what describes the file

Proposed on 2026-10-16 at `0c48e281d`. The operator approved the five-point direction in
conversation that day ("vai"). This written spec is still for review.

It rests on the measurements in
`docs/superpowers/verification/2026-10-16-coding-agent-questions-over-aura-sources.md`, sections
"What a card's vector is made of" and "Every caveat, and PDFs". Every number below comes from
there. Nothing here has been built.

Risk tier: **medium**. It changes the vector of every document card in every library, the
ingest's memo layout, and the upload allowlist. No Postgres migration, and no ArcadeDB schema
change: the `card` and `embedding` properties keep their meaning.

## The problem, measured

**1. The card's vector carries sentences about Aura.** The ingest embeds the whole rendered card
(`services/ingest/app.py`, `index_object`).
- Every card ends with filecard's trailer, "Written by Aura at ingest from the file itself … open
  it with document_open …".
- Most cards also carry caveats about what Aura read. Examples: "Its pages were all read, and
  only their opening …", "Only the opening of the file is described here.", "Sheet … was sampled".

Over 6,035 cards the trailer is on all of them. Embedding the card without any caveat, the cut
notice or the trailer was measured on five question sets:

| Set | Questions | r@8 | MRR | Golds better / worse |
|---|---|---|---|---|
| English code | 24 | 0.792 → 0.833 | 0.552 → 0.557 | 1 / 1 |
| Italian code | 24 | 0.625 → 0.667 | 0.453 → 0.462 | 3 / 0 |
| find-by-name | 6 | 1.000 | 0.750 → 1.000 | 3 / 0 |
| office | 20 | 0.800 → 0.900 | 0.650 → 0.667 | 2 / 1 |
| PDF corpus | 38 | 0.895 → 0.947 | 0.757 → 0.768 | 3 / 2 |

- **What moves.** The gains are spreadsheets and code. The PDF golds neither gain nor lose: 21 of
  22 already rank first.
- **The losses are near-ties**, or a spreadsheet newly taking the reserved place.
- **Coding questions get less noise.** Cards answering alone in the top 8 fall from 15 to 5 in
  English and from 18 to 5 in Italian.
- **Run-to-run noise** is one question moving one place.

**2. Source files carry a false caveat.** 5,046 of the 5,983 cards over Aura's own sources say
"Aura did not read this file at ingest: nothing here describes its contents."
- filecard has no handler for `.go`, `.ts` and the other source extensions.
- The ingest has read those files directly as passages since ed4126f86.

**3. Source files cannot be uploaded.** `supportedDocumentExt` (`internal/documents/extensions.go`)
is the gate the CLI, Telegram and cockpit uploads share. It holds no source extension, so the
reader of ed4126f86 only sees files dropped into the bucket directly.

**4. An encrypted PDF's metadata reaches the card as ciphertext.**
- `scheda_sick.pdf` is RC4-encrypted with an owner password only, so it opens without one.
  `pdfinfo` reads "W100 WT100-P1432, Data sheet".
- filecard's raw-byte metadata reader writes the encrypted strings as the card's Title, Subject
  and Author.

## Goal

A card's vector describes the file and nothing else. The card the agent reads stays as it is,
caveats included, because they are true and useful to a reader. Source files are uploadable and
carry an honest card. One list of source extensions serves Go and Python. Existing libraries get
the new vectors without re-embedding their passages.

## Decisions (proposed)

### 1. filecard renders the description apart from the card

`Card` gains `Description() string`. It is the card as `Render()` lays it out, minus three kinds of
line:
- every caveat;
- the cut notice;
- the trailer.

`Render()` keeps producing the stored card, and both come from one layout, so the description is
always a subset of the card's lines. The measurement rebuilt with the caveats removed *before*
layout. That differs from this only for a truncated card, and 2 of the 6,005 cards counted are
truncated.

`aura-filecard -json` emits `{"card": …, "description": …}`. The plain-text mode is unchanged.

### 2. The ingest embeds the description and stores the card

`IndexedDocument.card` stays the rendered card. Its full-text index and every reader are
unchanged. `IndexedDocument.embedding` becomes the description's vector. An empty description
embeds as the zero vector, as an empty card does today.

### 3. The card moves out of `_extract` into its own memo

The constraint, read from the installed CocoIndex (`cocoindex/_internal/function.py`,
`_compute_logic_fingerprint`):
- **What a memo keys on.** It is keyed on the decorated function's own AST, docstrings and
  comments aside, plus its arguments. A plain function it calls is not part of the key.
- **So editing `_card` alone** never reaches an existing file, because `_extract`'s memo still
  hits.
- **And editing `_extract`** re-runs it for every file of every library. Inside it,
  `media.derive` (images and audio) is not memoized. `extract_scanned_pdf` is memoized on the path
  of a temporary file, which is random per run. So a re-run of `_extract` repeats every vision
  and speech-to-text call, the billed ones included. This is read from the code, not measured.
  Acceptance 2 measures it.

`embed_text` is memoized on the text (`embed.py`, `deps=SPACE`), so passages whose text is
unchanged are not re-embedded when `process_file` re-runs.

The decision:
- **`_describe(content, suffix, file_name, content_type)`** is a new `@coco.fn(memo=True)` keyed
  like `_extract`. It runs `extract.prepared` and `aura-filecard -json` and returns the card and
  the description.
- **`process_file`** calls both. `index_object` stores the card from `_describe` and embeds the
  description.
- **`_extract` loses its card**: it returns the text and the anchors only.

The upgrade's cost per library is then:
- **`_describe` once per file.** That is local work: filecard, plus LibreOffice for a legacy
  format.
- **One embedding call per card.**
- **No passage re-embedding.**
- **One re-run of `_extract`**, which repeats the vision and speech-to-text calls once.

That last cost is the operator's decision (open question 1). It is measured before shipping: a
read-only count, on the lab VM and on the appliance, of the images, audio files and text-less
PDFs each library holds.

In the same change the billed calls get memo keys that do not move. `extract_scanned_pdf` and a
memoized wrapper of `media.derive` are keyed on the content's SHA-256, the file name and the media
fingerprint, not on the temporary path. After this release a change to `_extract` re-runs no
billed call. The upgrade itself still pays for them once, because every key is new.

### 4. One list of source extensions, owned by Go

The list moves from `services/ingest/source_extensions.py` to Go, in
`internal/documents/filecard`. filecard is where "what kind is this file" already lives
(`handlerFor`). It is the same 310 extensions plus `.mjs`, `.jsx`, `.css` and `.cypher`, which
indexing Aura's repo showed missing. It serves three readers:
- **filecard's own routing.** A source extension gets `KindSource` and a card whose head line
  names it a source file and its size, with no "did not read" caveat. Describing a source file's
  contents (an outline, its first lines) is not part of this spec: it was not measured.
- **`supportedDocumentExt`**, so a source file can be uploaded from the cockpit, the CLI and
  Telegram.
  - The cockpit's presign checks a second list, `internal/assets` `documentExts`. It is a
    hand-kept copy of the non-image half of `supportedDocumentExt`, and `limits_sync_test.go`
    fails when the two drift.
  - With 314 more extensions, the copy becomes derived: `documents.SupportedDocumentExts()` minus
    the image formats, which ride the image modality. The sync test is deleted with the copy.
- **The ingest.** `aura-filecard -source-extensions` prints the list as JSON, and
  `source_extensions.py` reads it once, on first use. The binary is in the ingest image, and
  `make ingest-test` runs the Python tests inside that image, built from the same tree.

The Python literal is deleted. There is no second copy and no parity test, because nothing is
duplicated.

### 5. An encrypted PDF's metadata is not read raw

When a PDF's trailer declares `/Encrypt`, filecard skips the Info and XMP strings. The title then
comes from the first text block, as it does for a PDF with no metadata. This is a correctness fix:
ciphertext in a card is wrong whatever it does to ranking. Reading the decrypted metadata through
poppler's `pdfinfo` is the alternative. It is left out because it adds a second process per PDF
for a field the first text block usually carries.

### 6. The PRD records the measurement first

A §11 amendment records the card-vector measurement and its perimeter. It lands before the code,
per the PRD-first rule.

## Shape

| File | Change |
|---|---|
| `internal/documents/filecard/card.go` | `Description()`, one shared layout with `Render()` |
| `internal/documents/filecard/build.go` | `KindSource` routing from the extension list |
| `internal/documents/filecard/source_extensions.go` (new) | the list and `SourceExtensions()` |
| `internal/documents/filecard/pdf.go` | skip raw metadata when `/Encrypt` is present |
| `cmd/aura-filecard/main.go` | `-json` gains `description`; `-source-extensions` |
| `internal/documents/extensions.go` | the gate includes `filecard.SourceExtensions()` |
| `internal/assets/limits.go`, `limits_sync_test.go` | `documentExts` derived from the gate; the sync test removed |
| `services/ingest/source_extensions.py` | reads the list from `aura-filecard` |
| `services/ingest/app.py` | `_describe`; `_extract` without the card; `index_object` embeds the description |
| `services/ingest/media.py` | billed calls keyed on content, not on the temporary path |
| `prd.md` §11 | the amendment |

No file crosses 600 lines. `app.py` is the one to watch, and the split, if needed, moves `_card`
and `_describe` into `services/ingest/card.py`.

### New env vars

None.

## Acceptance

1. **Unit, Go.**
   - `Description()` excludes every caveat, the cut notice and the trailer, and every one of its
     lines is a line of `Render()`.
   - A source extension yields `KindSource` and no "did not read" caveat.
   - An RC4-encrypted fixture PDF yields no metadata line.
   - `supportedDocumentExt` and the asset presign both accept `.go`, `.tsx`, `.scl` and `.mjs`,
     and both still refuse `.exe`.
2. **Unit, Python.**
   - `index_object` embeds the description and stores the card.
   - `SOURCE_EXTENSIONS` equals the binary's list.
   - A stubbed media bridge called through `_extract` twice, with `_extract`'s code changed in
     between, is invoked once per file. That proves the billed calls no longer re-run.
3. **The five sets through the real ingest.** A catch-up pass of the new code runs over the
   existing `cb-arcade` libraries. Read from the pass:
   - `[extract]` prints once per file;
   - one embedding request per card;
   - no passage re-embedded.

   Then the five sets reproduce the measured r@8 and MRR, within one question.
4. **Lab VM, through the updater.** A `.go` and a `.tsx` uploaded from the cockpit are accepted.
   Their cards name them source files, and `document_search` finds them. The agent answers one
   spreadsheet question from the office fixture whose gold was missing before (q07 or q14). The
   run's file lands in `docs/superpowers/verification/`.

## Open questions

1. **The one-time re-run of the billed calls (decision 3).** Counted read-only on 2026-10-16:
   - **Lab VM.** One identity, 9 documents. One is an audio file, transcribed by the local
     `aura-stt` (whisper) sidecar. There are no images and no PDFs. The re-run costs one local
     transcription and no billed call.
   - **The appliance at 192.168.1.225** was not reachable from the workstation. Its count is a
     gate before the edge image this change ships in is published.

   The operator approved the spec on 2026-10-16, with decision 3 as written. The fallback, if the
   appliance's count is large, keeps `_extract` frozen for one more release: `_describe` still
   provides the card, and `_extract`'s own card is computed and ignored until the release that
   makes the billed calls content-keyed.
2. **Whether `KindSource`'s head line ("source file") moves code ranking.** The measured variant
   gave unread source files a head line of "file, N KB" and nothing else. Acceptance 3 measures
   the new wording before the VM run.

## Later levers, not in this spec

- **Code-aware chunking.** `RecursiveSplitter` with `language=detect_code_language(...)` and
  smaller code chunks, as cocoindex-code does. Unmeasured here.
- **The card's kind stored on `IndexedDocument`.** It would allow a card floor of its own and
  reserve the place only for descriptive cards. The 0.25 floor fixed office and cost code because
  of name-only cards.
- **Table structure for PDFs**, read from `Souheil007/Rag_Full_Version` on 2026-10-16. That repo:
  - parses a table grid at ingest and makes the table the chunk unit;
  - repeats the headers in every row group;
  - embeds rows as `Header: value`, with the caption and heading path in front.

  Its evidence is thin: 25 and 10 author-written questions, substring scoring and no ablation.
  On Aura's PDF corpus the documents already rank first. The open question is whether the right
  passage carries the cell, which this spec's benchmark does not measure.

  The repo also reranks with a cross-encoder. That is not a lever here: the operator had already
  ruled it out. Spike 070 measured 23 s per 15 documents on CPU, and the appliance has no GPU.

## What this spec does not prove

- **The measurements rewrote vectors in place**, from texts embedded by the same embedder with the
  ingest's prefix. Acceptance 3 is the first run through the ingest itself.
- **The question sets are small and written by one agent:** 24, 24, 6, 20 and 38 questions. One
  question moves recall by 0.026 to 0.167.
- **No media file was measured.** The media caveat's removal from the vector is untested.
- **Large PDF libraries.** The PDF corpus is 30 short, distinct files, not hundreds of manuals on
  one topic.
- **The count behind open question 1 is not yet taken.**
