# Spike: the Craft apps' web builds in Aura's cockpit — findings

Date: 2026-10-19. Run in the cloud dev container (4 CPUs, 15 GB), headless Chromium 1194 on
SwiftShader WebGL2, each build served through `internal/staticapp` with aura's page policy. Fixtures
from the repository: `web/e2e/fixtures/documents/Appunti.pdf` (1 page),
`scripts/fixtures/document_retrieval_eval/complex_corpus/manuale_confezionatrice_FX300.pdf` (15 pages),
`scripts/fixtures/document_pipeline_e2e/sample.docx` and `sample.pptx` (2 slides),
`web/e2e/fixtures/media/photo.png` (256×256) and `clip.mp4` (4 s, 320×180, H.264 + AAC).

## Verdicts

| # | Finding | Verdict |
|---|---|---|
| 1 | **Every Craft app publishes a web build** as `<app>-web-<version>.zip` on its GitHub release, and the versions the box pins are the latest tags (GridCraft 0.8.0, PdfCraft 0.9.0, WordCraft 0.8.0, DeckCraft 0.8.0, PhotoCraft 0.10.0, FilmCraft 0.9.0). WebAssembly raw / gzip -9: GridCraft 18.9 / 7.1 MB, PdfCraft 39.0 / 14.4, WordCraft 42.2 / 15.7, DeckCraft 30.6 / 10.9, PhotoCraft 24.7 / 11.3, FilmCraft 32.5 / 12.4 (the last four as built here). | VALIDATED |
| 2 | **Only GridCraft and PdfCraft have the bridge** in their official builds: `?host=parent`, `<app>:ready/open/save/dirty/failed`, the parent heard only on its own origin. Their glue imports `window.parent` and `postMessage`; the protocol strings are in their wasm and, for PdfCraft, in `apps/pdfcraft-web/src/main.rs` at the v0.9.0 tag (written upstream for a Nextcloud app). WordCraft, DeckCraft and PhotoCraft have neither, at the tag or on main of 2026-10-18/19: they open from the browser's picker or a drop and save as a download. FilmCraft has no bridge either, but exposes `window.filmcraft` (import, execute, readFile) to a same-origin parent. | VALIDATED |
| 3 | **PdfCraft works unmodified under aura's policy.** It says ready in 0.4–1.5 s; the PDF sent opens; Ctrl+S hands back the same bytes (582 B and a 15-page 29,805 B manual, both identical to the original). Organize pages › rotate › Save pages hands back a PDF `pdfinfo` reads as `Page rot: 90`, `dirty` going true then false; Ctrl+S after it saves again. No policy violation in the console. | VALIDATED |
| 4 | **The four others need one small patch each**, because each funnels every download through one place: WordCraft `Services::download`, DeckCraft `Services::download`, PhotoCraft `Services::write`, FilmCraft `fs::set_on_write`. The patch adds a `host.rs` (GridCraft's protocol with the app's prefix), wires it in the web crate (~30 lines), and adds `MessageEvent`/`UrlSearchParams` to web-sys. 176–244 changed lines per app (`docker/craft-web/<app>.patch`). | VALIDATED |
| 5 | **WordCraft patched**: `sample.docx` opened, text typed at the end of the last paragraph, `dirty` true, Ctrl+S handed back `sample.docx` (8,178 B), a valid zip whose `word/document.xml` ends with the typed text; `dirty` false after it. | VALIDATED |
| 6 | **DeckCraft 0.8.0's official web build does not start**: it panics at the first document with `no pids on this platform` (`crates/engine/src/recovery.rs`, `instance_id` calls `std::process::id()` on wasm). The patch gates that call off wasm. Patched: `sample.pptx` opened, the title edited, Ctrl+S handed back `sample.pptx` (13,148 B), 2 slides, title `Corpus di prova Aura modificato`. DeckCraft saves its own `.deckcraft` format; the bridge turns a deck the parent sent as `.pptx` back into `.pptx` (`open_checked` then `save_bytes(.., "pptx")`). | VALIDATED (upstream bug fixed in the patch) |
| 7 | **PhotoCraft patched**: `photo.png` opened under its own name (`open_file`, so Save writes back to it rather than through Save As to `.psd`), Image › Invert (Ctrl+I), Ctrl+S handed back `photo.png` (9,749 B) whose sampled pixels are each the exact inverse of the original's. | VALIDATED |
| 8 | **FilmCraft patched**: `clip.mp4` sent is imported and a sequence made from it, so the editor starts on its timeline; Ctrl+S handed back `Untitled.fcproj` (the project, 5,616 B, `filmcraft.project` schema 15); an H.264 export (through `window.filmcraft`, the Export button's command) handed back `edited.mp4`: 4.017 s, 320×180, avc1 + mp4a. | VALIDATED, export driven by the page API |
| 9 | **The ArtCraft brand terms bind a modified build**: "Forks and modified versions must remove [the ArtCraft marks]" (`docs/brand/LICENSE-brand.txt` in each repo); a plain-text "based on <App> by the ArtCraft team" is allowed. The patches take out what the web builds render with the name: FilmCraft's compiled wordmark PNG, About texts, Discord/website labels, the translated rows naming ArtCraft (dropped by the build script); the copyright notices stay, as the MIT license requires. | VALIDATED (reading of the terms, not legal advice) |
| 10 | **The build is reproducible offline after the fetch**: Trunk 0.21.14 `--offline` uses wasm-bindgen 0.2.129 and wasm-opt 123 from PATH and downloads nothing (its cache stayed empty). Cold builds here: WordCraft 11.5 min (2.5 of it Trunk fetching its tools), DeckCraft 7.5 min, PhotoCraft about 10.5 min (`wasm-release`: fat LTO and wasm-opt `--converge`), FilmCraft 9.3 min. | VALIDATED |
| 11 | **Static caching must follow the URL, not the folder.** aura cached every file but the page as immutable; FilmCraft loads `audio-worklet.js` and `favicon.png` by bare names, which an upgrade replaces in place. A file is now immutable only when its name carries a content hash or its URL a `?v=` version. | VALIDATED (fixed in `internal/staticapp`) |
| 12 | **The official web builds embed no craft-fonts.** Each app's `release.yml` names a craft-fonts commit, but DeckCraft's official wasm is 30.5 MB, as a build without them (30.6), while one with them is 36.0 MB: the string `BIZ UDPGothic` appears once in the first two (the code's fallback name) and twice in the third (the face). The fonts are for the desktop builds. | VALIDATED |

## What this spike does not show

- **Not in the running cockpit.** No aura stack ran here (no Postgres, no Garage): the preview components
  (`CraftFrame`, `EditInCraft`) are covered by unit tests, not by a browser against aura. The live run on
  the lab VM is still the gate.
- **The image stage was not built**: there is no Docker daemon here. `docker/craft-web/build.sh`, which
  the stage runs, was run end to end on DeckCraft outside Docker.
- **One small file per app, one run each.** Round-trip fidelity of real-world documents through WordCraft
  and DeckCraft (styles, embedded media) was not measured; a 51,758 B pptx came back as 13,148 B.
- **WebGPU was not exercised** (headless SwiftShader is WebGL2), nor Firefox or Safari, nor a phone.
- **Memory** of each app in the browser was not measured.
- **FilmCraft's export** was started through its page API, not by clicking Export.
