# Spike: the Craft MCP servers in Aura's box — findings

Date: 2026-10-18. Run on the operator's dev stack (`aura-sandbox:local` = the box image plus six Craft CLIs,
`spikes/craft-mcp/Dockerfile`), as the operator identity, against nine public real-world files
(`D:/tmp/craft-real/SHA256SUMS`): Damodaran's FCFF valuation workbook (16 sheets, 906 formulas, .xls), ANAC's
bando-tipo n. 2 (.docx, 821 paragraphs, 12 tables) and its 42-page note (.pdf), a Horizon Europe Part B
template (.docx), EU-OSHA's Italian deck (.pptx, 15 slides, 23 pictures), an Agenzia delle Entrate form
(.pdf), a 352-page archive.org scan (.pdf), Blender's Sintel trailer (1080p .mp4, 52 s) and a 19-megapixel
Wikimedia photo of Ponte Vecchio (.jpg).

Driver: `mcpbox.py` (a stdio client through `docker exec -i` into the identity's box, `-w /workspace`, as
`cmd/aura/sandbox_mcp_box.go` launches a box server), `tasks.py` and `tasks_more.py` (one real task per server,
an independent check of what it wrote, and the same task on today's box toolbelt). Raw results: `runs/`.

## Verdicts

| # | Finding | Verdict |
|---|---|---|
| 1 | The six servers mount through `aura mcp add <name> --box -- <cli> mcp …` in about 1.2 s each, the box recreated on the new image with its volumes kept; the cockpit's probe lists them as the operator (21, 17, 16, 25, 17, 25 tools). | VALIDATED |
| 2 | **GridCraft** opened the .xls model, recalculated the value per share from 377.14 to 333.62 after the riskfree rate moved from 4.5% to 5%, equal to LibreOffice's recalculation to the last digit, and saved an .xlsx with cached values, in 0.03 s. Today: openpyxl drops every cached value on save, and the LibreOffice round trip takes 4.0 s. No formula lost (12 cells LibreOffice writes as an empty `=` are `#DIV/0!` constants, which GridCraft keeps as values). | VALIDATED |
| 3 | **PdfCraft** redacted every "ANAC" in the 42-page note (10 marks, 10 applied, audit 0 findings, pdftotext 8 → 0, word count unchanged) and exported it to Word with headings and 120 tables in 0.13 s. Today: no tool in the box removes text under a mark, and LibreOffice's PDF import wrote an empty .docx in 8.6 s. | VALIDATED |
| 4 | **PdfCraft OCR**: the CLI package has no recognition models, and the engine reads English only. Not needed: Aura's model reads a rendered page. | INVALIDATED (out of scope) |
| 5 | **WordCraft** filled a fill-in spread over 19 runs in the bando, kept every bold and italic character (11,574 / 32,732), and saved .docx and a 51-page .pdf (Word declares 50) in 0.41 s. Today: python-docx loses 108 italic characters in the one paragraph it touches, and LibreOffice's PDF has 66 pages. | VALIDATED |
| 6 | **DeckCraft** retitled the deck's first slide, kept 15 slides and 23 pictures, rendered the slide and exported a 15-page PDF, 1.28 s against 5.0 s for python-pptx plus LibreOffice. It reports Arial and Wingdings as missing; the report is informational (`crates/engine/src/fonts.rs`): text draws in a stand-in, and DeckCraft bundles the metric-compatible Liberation and Carlito for Arial, Times New Roman, Courier New and Calibri. | VALIDATED |
| 7 | **FilmCraft** imported the trailer, built a sequence, rendered a frame and exported all 1,253 frames as H.264 1280x544, headless, in 89 s at the box's 2 CPUs (14 fps), peak 1.0 GB. Its default bitrate wrote 97 MB; `bitrateMode: "crf", crf: 23` wrote 7.5 MB. The box has no ffmpeg: there is no server-side video path today. | VALIDATED, with explicit CRF |
| 8 | **PhotoCraft** resized the photo to 2400x1836, applied brightness/contrast and exported a JPEG keeping its EXIF, in 1.16 s, peak 330 MB. Pillow does the same in 0.53 s and drops the EXIF. | VALIDATED |
| 9 | **Annotations are not usable as policy.** FilmCraft (`command_run`, `command_batch`), PdfCraft (`doc_save`, `command_run`, `tool_call`) and PhotoCraft (`doc_save`, `doc_export`, even `jobs_list`) declare `destructiveHint: true` on tools that only write files inside the box, and Aura's bridge lets a declared destructive hint override a recipe's table. | VALIDATED (a gap in Aura) |
| 10 | **21 tools need the desktop app** and fail headless: GridCraft `screenshot`; WordCraft `screenshot`, `click`, `key`, `ui_inspect`; DeckCraft `screenshot`, `call_app`; FilmCraft `ui_click`, `ui_control`, `ui_drag`, `ui_elements`, `ui_inspect`, `ui_key`, `ui_screenshot`, `ui_type`; PhotoCraft `control_call`, `ui_inspect`, `ui_menu_invoke`, `ui_pointer`, `ui_screenshot`, `ui_set`. No server has a flag to leave them out, and Aura has no way to hide a tool of a mounted server. The agent keeps its eyes without them: WordCraft `render_page`, DeckCraft `render_slide`, PdfCraft `render_preview`/`page_render`, PhotoCraft `render_preview` and FilmCraft `render_frame` render headless; GridCraft has none, but `execute_command file.exportPdf {range, fitToPage}` then PdfCraft's `render_preview` gives the sheet as an image (measured: the right sheet and range, one page, under a second). | VALIDATED (a gap in Aura) |
| 11 | **Confinement**: PdfCraft, WordCraft, DeckCraft and PhotoCraft take `--automation-read-root/--automation-write-root /workspace`; GridCraft and FilmCraft have no such flags (the survey measured GridCraft writing outside its folder). The box was their only fence until landrun (13). | VALIDATED |
| 12 | **The real agent** (one turn, local model, GridCraft task): it loaded the skill, found the tools through `tool_search`, opened, edited, saved and sent the file with no approval in the way, but read whole sheets 15 times, counted `read_range`'s unnumbered rows from 0 and wrote 0.05 into B38 instead of B39, and spent 1.6 M input tokens. The skill must say: read small windows at explicit addresses, check the cell with `get_cell` before writing, and read the output cell after. | PARTIAL: one turn, skill to correct |
| 13 | **The write fence.** Landlock works in the box: ABI 7 on the WSL2 kernel 6.18, under Docker's default seccomp, as root. Through their own save tools GridCraft (`save_workbook`) and FilmCraft (`file.saveAs`) wrote to `/tmp` and `/root`; under landrun 0.1.17 (`--best-effort --rox / --rw /workspace --rw /dev`) both got EACCES outside `/workspace`, and the six tasks above gave the same calls and outputs as unfenced (`escape.py`, `runs-landrun/`). The release binary needs glibc 2.38 and bookworm has 2.36, so the image builds it statically from its module. On a kernel without Landlock, `--best-effort` runs unfenced: the image contract proves the fence on the kernel it runs on. | VALIDATED |

## What this spike does not show

- Only one agent turn was run (GridCraft), on the local model; the other five servers were driven by the
  scripted client, not by the agent. Step 7-style acceptance on the lab VM is still the gate.
- Files came from `/workspace`, not from the library through `document_open`.
- The cockpit side (PhotoCraft and FilmCraft as editors in the browser) was not tried.
- Legacy formats LibreOffice converts today (.doc, .ppt, .xls written by old Office) were tried only for
  .xls; DeckCraft opens .pptx and .deckcraft only.
- One machine, one run per task.
