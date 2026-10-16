# Card description vector implementation plan

**Status (2026-10-16):** PARTIAL.
- **Landed on master:** Tasks 0 to 7, in 6a8f1aaf9, 84ac521f0, bba43ff65, 39a9f7290, 8ec1df3ed,
  a70640dad, b33096b3d, 003fd665d and 97fec747e. The measurements before them are 6334cfb1e,
  81cae63d9 and 0c48e281d, and the spec is bddef47d0 and 88f0a1557.
- **Measured so far:** the ingest suite is 220/220 in an image built from master. The billed
  media calls fall from two to one per re-run of `_extract`, and cocoindex 1.0.25 kept every
  memo (0 re-extractions over 6,035 files).
- **Open:** Task 8 (acceptance 3, the real ingest over `cb-arcade`), Task 9 (review and push)
  and Task 10 (the appliance count, the edge publish and the lab VM run).

> **For agentic workers:** Use superpowers:executing-plans with test-driven-development. The
> operator requires the work in D:/Aura on master, with no worktree.

**Goal:** A document card's vector describes the file and nothing else. Source files can be
uploaded and carry an honest card, and existing libraries get the new vectors without
re-embedding their passages.

**Architecture:** filecard renders one layout two ways: `Render()` is the stored card and
`Description()` is the card without its caveats, cut notice and trailer. The ingest gets both
from one `aura-filecard -json` call in `_extract`, stores the card and embeds the description.
The source-extension list is Go's, and the upload gate, the asset presign and the ingest all
read it. The billed media calls are memoized on the file's bytes, so the one re-run of
`_extract` this release forces bills nothing again afterwards.

**Tech Stack:** Go (filecard, documents, assets), Python 3.12 with cocoindex 1.0.25 (the
ingest), React/TypeScript (cockpit attachments), ArcadeDB, WSL and Docker.

**Spec:** docs/superpowers/specs/2026-10-16-card-description-vector-design.md (approved by the
operator on 2026-10-16; decisions 3 and 5 amended while planning).

## Global Constraints

- No Postgres migration and no ArcadeDB schema change. `IndexedDocument.card` and `.embedding`
  keep their meaning, and only what is embedded changes.
- No new environment variable.
- One list of source extensions, in `internal/documents/filecard/source.go`: 314 extensions,
  the operator's 310 plus `.css`, `.cypher`, `.jsx` and `.mjs`. There is no second copy and no
  parity test.
- No file over 600 lines. Stage explicit paths only and never touch other sessions' files
  (`.claude/settings.json`).
- No paid run without the operator's OK. A corpus reaches no vision route: PDFs are checked for
  a text layer offline first.
- Go tests run from WSL (`scratchpad/cb/wt-go.sh`) on the touched packages, with `-race`. The
  Python tests run in an image built from master (`scratchpad/cb/wt-pytest.sh --build`), against
  `cb-arcade`.

## Review Focus

1. **A card with invalid UTF-8 in it** (the SICK PDF before a70640dad). Go's JSON encoder writes
   U+FFFD, so the card and the description must still agree line for line.
2. **A legacy `.xls` or `.ods`.** Its description must come from the converted file, as its card
   does. A raw `.xls` describes itself as "file, 6 KB".
3. **An image no route can read.** `extract_media` raises `NoVisionRoute` and nothing is
   memoized there. `index_text` returns "", which `_extract` memoizes, and the image stays
   card-only until its bytes change (prd.md §11).
4. **A spreadsheet description at the dense-card size.** `embed.py` cuts the vector's input to
   the token budget, for a description as it did for a card.
5. **A binary named `.go`** (a NUL byte, not UTF-16). The card says Aura did not read it, so the
   description is the name line alone and `_read_text` returns "".

## Decisions taken while writing this plan

- Ruling: no `_describe` memo; `_extract` gets the card and the description from one filecard
  call. A second memo would convert a legacy file twice and save nothing this release, since
  `_extract`'s body changes anyway. Cost if wrong: one local re-extraction per file, already
  paid.
- Ruling: the web client keeps no allowlist copy. It accepts `*`, and the server's presign
  decides; the chip's type comes from `asset.modality`. Cost if wrong: an unsupported file is
  refused after it is picked, with a message naming its type.
- Ruling: a `ModalityUnknown` refusal says `Aura does not read ".exe" files`.
- Ruling: an encrypted PDF skips its Info strings and still reads a plaintext XMP packet.
- Ruling: cocoindex 1.0.25 is taken first, as Task 1, so the upgrade pass runs on the version
  that ships.
- Ruling: `extract_media` does not read `CONFIG_FINGERPRINT`, so the route dependency is as it
  was. Depending on it would bill every image and recording again on each change of a primary
  model that reads images. Cost if wrong: images keep the previous route's description. Left
  to the operator (prd.md §11).
- Ruling: in the billed-memo test a changed content type stands in for changed `_extract`
  code. Both re-run `_extract` with the bytes and the name unchanged.
- Ruling: the billed-memo PRD paragraph (003fd665d) landed after its code (b33096b3d), not
  before. The measurement came from that commit's own RED test.
- A cross-encoder rerank is not a lever here: the operator had already rejected it (spike 070,
  23 s per 15 documents on CPU).

## File structure

| Files | Responsibility |
|---|---|
| `internal/documents/filecard/card.go`, `description_test.go` | `layout()` shared by `Render()` and `Description()`; `KindSource` |
| `internal/documents/filecard/source.go`, `source_test.go`, `build.go` | the extension list, `SourceExtensions()`, the source card and its NUL rule |
| `internal/documents/filecard/pdf.go`, `pdf_encrypted_test.go` | `/Encrypt` skips the Info strings |
| `cmd/aura-filecard/main.go`, `main_test.go` | `-json`, `-source-extensions`, a testable `run()` |
| `internal/documents/extensions.go`, `internal/assets/limits.go` (+ tests) | the upload gate and the presign derived from the one list |
| `web/src/chat/attachments/upload.ts`, `auraAttachmentAdapter.ts` (+ tests) | accept `*`; the server classifies |
| `services/ingest/source_extensions.py`, `extract.py` | the list read from the binary |
| `services/ingest/media.py`, `tests/media_driver.py`, `tests/test_billed_media_memo.py` | content-keyed billed calls |
| `services/ingest/app.py`, `tests/test_card_description.py`, `tests/test_embed_space_reruns.py` | `Card`, `Extracted.description`, the embedded description |

## Tasks

- [x] **Task 0: PRD amendment.** 6a8f1aaf9 records the five-set measurement and its perimeter.
- [x] **Task 1: cocoindex 1.0.25.** 84ac521f0. A catch-up over three libraries printed
  `[extract]` 0 times on 6,035 files.
- [x] **Task 2: `Description()` and `aura-filecard -json`.** bba43ff65.
- [x] **Task 3: source extensions owned by Go, and an honest source card.** 39a9f7290.
- [x] **Task 4: uploads accept source files.** 8ec1df3ed: the gate, the derived presign list,
  the web client, and the named refusal.
- [x] **Task 5: encrypted PDF metadata.** a70640dad. `scheda_sick.pdf` cards as
  "Titled: DATA SHEET".
- [x] **Task 6: billed media calls keyed on content.** b33096b3d, with the PRD in 003fd665d.
  The tests in `test_billed_media_memo.py` went RED at two calls each, then GREEN.
- [x] **Task 7: the ingest embeds the description.** 97fec747e. RED twice: on the missing
  `Card`, then on a document vector still made from the card. Ingest suite 220/220.
- [ ] **Task 8: acceptance 3 through the real ingest.**
  - [ ] Build `aura-ingest:cc126` from master. Run one catch-up pass over each `cb-arcade`
    library (code, office, pdf) with `scratchpad/cb/cc-pass.sh aura-ingest:cc126 <name>`.
    Expected: `[extract]` once per file, and embedding requests near one per card, since
    passages with unchanged text hit `embed_text`'s memo.
  - [ ] Re-run the five sets with `scratchpad/cb/rules-one.sh <set> <name> desc` and compare
    against `answers-base4-*.json` and the nocav measurement. Expected: r@8 and MRR within one
    question of the PRD table.
  - [ ] Record the pass and the sets in the verification file.
- [ ] **Task 9: review and push.**
  - [ ] One fresh reviewer on the whole range `6334cfb1e^..HEAD`, given this plan, the spec and
    the Review Focus above.
  - [ ] Fix Critical and Important findings test-first, then push with the WSL recipe and read
    CI for this SHA.
- [ ] **Task 10: publish and the lab VM.**
  - [ ] Count the appliance's images, recordings and text-less PDFs read-only. That count is the
    gate before the edge image is published.
  - [ ] Publish edge and update the lab VM through the updater.
  - [ ] Upload a `.go` and a `.tsx` from the cockpit. Their cards must name them source files,
    `document_search` must find them, and the agent must answer office q07 or q14.
  - [ ] Write `docs/superpowers/verification/2026-10-16-card-description-vm.md`.
