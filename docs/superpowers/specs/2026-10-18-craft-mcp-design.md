# The Craft apps replace Aura's own document, photo and video tooling

Date: 2026-10-18. The operator's direction: *"con la suite craft cancelliamo tantissimo bespoke e montiamo
MCP nativi"*, then *"prima monta tu gli MCP nativi e valutali"*, then, with the spike's numbers in hand,
*"procedi pure e cancella il bespoke"*, *"anche la parte UI"* (Aura's own photo and video editors) and
*"tutte le librerie pesanti, soffice ecc."*. Project 1 of 4 agreed the same day (the others: FilmCraft as the
cockpit's video editor, one pipeline for patched Craft web builds, PhotoCraft/WordCraft/DeckCraft in the
cockpit).

Evidence: `spikes/craft-mcp/FINDINGS.md` (verdicts 1–12, measured on nine public real-world files) and the
survey `D:/tmp/storytold-survey.md` (licenses, maturity, web builds, every server's tool list).

## What changes

1. **The agent works on documents, photos and video through six Craft MCP servers in its box**: GridCraft
   (spreadsheets), PdfCraft (PDF), WordCraft (Word), DeckCraft (PowerPoint), PhotoCraft (images), FilmCraft
   (video). They replace the LibreOffice, pandoc, pdftk, tesseract and Python/Node Office libraries the
   agent drives through `shell_exec` today, which leave both images.
2. **Aura's own media editors go**: the Video Studio (`web/src/videoStudio`, VideoFlow and mediabunny) and the
   photo editor (Filerobot). Until projects 2 and 4 bring FilmCraft and PhotoCraft into the cockpit, there
   is no in-browser editor; editing is the agent's, through the servers above.
3. **The `aura-video-mcp` plan is dropped**: its spec, plans A and B (223 tasks, none started) and its spike.
   FilmCraft exports an MP4 headless in the box (FINDINGS verdict 7).

## Decisions

1. **Six trusted box recipes, mounted by default**, on the agent-browser pattern
   (`internal/mcp/manager/catalog.go`): `Runtime: box`, `Trust: trusted_recipe`, one `Source` each
   (`recipe:gridcraft` …), listed in `containerDefaultOnRecipes` because every box runs the same image.
   - Commands: `gridcraft-cli mcp`; `pdfcraft-cli mcp --compact` (17 tools; the other ~150 through its own
     `tool_search`/`tool_call`); `wordcraft-cli mcp`, `deckcraft-cli mcp`, `photocraft-cli mcp`, each with
     `--automation-read-root /workspace --automation-write-root /workspace`; `filmcraft-cli mcp`.
   - Every server runs inside landrun's Landlock fence: it writes only in `/workspace` and `/dev`, reads and
     the network stay as they were (FINDINGS 13). GridCraft and FilmCraft have no flags of their own
     (FINDINGS 11), and saved to `/tmp` and `/root` without it. Garage stays out of reach: the box holds no
     store credentials, and only Aura's `send_file` writes there.
2. **The binaries ship in `docker/aura-sandbox/Dockerfile`**, pinned like agent-browser: the release tarball's
   SHA-256 and the extracted binary's SHA-256, per architecture, with the licenses and notices copied to
   `/usr/local/share/licenses/<app>/`. Versions: GridCraft 0.8.0, PdfCraft 0.9.0 (CLI package, no OCR models:
   FINDINGS 4, not needed), WordCraft 0.8.0, DeckCraft 0.8.0, FilmCraft 0.9.0, PhotoCraft 0.10.0. Unmodified
   official builds, so the ArtCraft brand terms are met.
3. **Aura hides the 21 tools that need the desktop app** (FINDINGS 10). A per-recipe table in
   `internal/agent/mcptools` drops them before the tools are bridged, so neither the manifest nor
   `tool_search` ever offers them. A test pins each hidden name against the server's tool list at the pinned
   version, so an upgrade that renames one fails the build instead of leaking it.
   - The agent keeps its eyes: every server but GridCraft renders headless (`render_page`, `render_slide`,
     `render_preview`, `render_frame`), and a sheet is seen through GridCraft's `file.exportPdf` of a range,
     rendered by PdfCraft. The hidden `screenshot` tools photograph a desktop window the box does not have.
4. **Aura's per-tool action table is the policy for these recipes, not the servers' annotations**
   (FINDINGS 9). Reads are reads; everything that writes is a reversible mutation: it writes files in the
   identity's box and nothing outside it. For these six sources, a server's `destructiveHint` does not
   override the table. A tool missing from the table stays unknown and fails closed, as today.
5. **Six skills ship embedded** (`internal/skills/embed/<app>-aura/SKILL.md`), written from the spike's
   measured recipes and limits, plus what the real agent got wrong (FINDINGS 12): read small windows at
   explicit addresses, check a cell with `get_cell` before writing it, read the output after; always set
   `bitrateMode`/`crf` on a FilmCraft export.
6. **The heavy toolchain leaves both images.** From `docker/aura-sandbox/Dockerfile` and
   `docker/aura/Dockerfile`: `libreoffice-writer/-calc/-impress`, `pandoc`, `pdftk-java`, `tesseract-ocr`,
   `qpdf`, the npm `docx` and `pptxgenjs`, and the Python `python-docx`, `python-pptx`, `openpyxl`,
   `xlsxwriter`, `pypdf`, `pdfplumber`, `reportlab`, `pdf2image`, `pytesseract`, `markitdown`.
   - Stays: `pandas`, `Pillow`, `lxml`, `defusedxml`, `file`, `xxd`, `fonts-liberation`;
     `poppler-utils` stays in the aura image only, where Go runs `pdftotext` (`internal/pdftext`) and
     `pdftoppm` (`cmd/aura-media-index`).
   - The aura image's copy was already unused: shell tools have no host execution path
     (`internal/sandbox/usersandbox/router.go`).
   - What the agent is told changes with it: the system prompt (`internal/agent/prompt.go`), the
     `document_open` description, `find-skills-aura`, and the toolchain smoke
     (`scripts/workspace_toolchain_smoke.sh`) with the CI steps that run it.
   - The ingest image keeps its own LibreOffice: it extracts text for indexing, a separate pipeline and a
     separate decision.
7. **The editors leave the cockpit** with their e2e specs, fixtures, i18n, dependencies and the server code
   that only they use; the inventory is the plan's Task list. Studio *generation* (image and video
   generation, its history and model picker) stays.

## What this does not do

- **No `.ppt` and no other format only LibreOffice read.** DeckCraft opens `.pptx` and `.deckcraft`; WordCraft
  opens `.doc`, `.odt`, `.rtf`, `.md`; GridCraft opens `.xls`, `.xlsb`, `.ods`, `.csv`. A `.ppt` the operator
  sends cannot be edited until a Craft server reads it.
- **No OCR in the box.** The model reads a rendered page.
- **No in-browser editing of photos and videos** until projects 2 and 4.
- **Six server processes per identity's box once mounted**, about 9–66 MB each idle; FilmCraft takes about
  1 GB while exporting, inside the box's 2 GB.
- **The ingest pipeline is untouched.**

## Acceptance

1. The box image builds with the six pinned binaries and without the removed toolchain; the image size
   change is measured and recorded.
2. A fresh identity's agent sees the six servers mounted, none of the 21 hidden tools, and no approval
   prompt on a save.
3. The real agent completes, in one turn each, the six spike tasks on the same real files through the
   library (`document_open` → server → `send_file`), each output checked by a tool outside the server.
   Recorded in `docs/superpowers/verification/2026-10-18-craft-mcp.md`.
4. The cockpit builds and its suites pass with the editors gone; nothing imports the removed packages.
5. Gates: `make quality`, the web gates, the coverage policy.
