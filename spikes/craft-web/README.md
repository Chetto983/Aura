# Spike: the Craft apps' web builds in Aura's cockpit

Date: 2026-10-19. Asked by the operator: "La parte di photo e video craft ha la Wasm da mettere sulla UI
di Aura?", then "Guarda tutte le App che usiamo come MCP oggi", then "procedi con lo spike su PdfCraft e
tutte le modifiche necessarie per le altre non supportate a oggi con la modifica ed il salvataggio".

Projects 2 and 4 of `docs/superpowers/specs/2026-10-18-craft-mcp-design.md` (FilmCraft as the cockpit's
video editor; PhotoCraft, WordCraft and DeckCraft in the cockpit) and project 3 (one pipeline for patched
Craft web builds). GridCraft is already framed as the xlsx preview
(`docs/superpowers/specs/2026-10-18-xlsx-viewer-gridcraft-design.md`).

## Questions

1. **The builds.** Does every Craft app the agent drives as an MCP server publish a web build, and at
   which size?
2. **The bridge.** Which of them can a page open a file in and take the save back from, the way
   `XlsxPreview` drives GridCraft (`?host=parent`, `postMessage`)?
3. **PdfCraft.** Does its official build open, edit and save through that bridge inside Aura's own
   policy (`internal/staticapp`)?
4. **The others.** What does it take to give WordCraft, DeckCraft, PhotoCraft and FilmCraft the same
   bridge, and does a patched build open, edit and save a real file?

## Method

- `serve/` serves each build through `internal/staticapp` under `/apps/<app>/`, so the page carries the
  same Content-Security-Policy and headers aura sends, beside a host page (`host/`) that speaks the
  bridge as `XlsxPreview` does and posts every save back to the server, which writes it to disk.
- `drive.mjs` drives one app in headless Chromium (the image's Chromium, SwiftShader WebGL2): the file
  sent on `<app>:ready`, then clicks and keys from a step list, and records every message, request and
  console error. Each saved file is then checked with a tool outside the app (`pdfinfo`, `zipfile`,
  Pillow, an MP4 box parser).
- Builds: official release zips for GridCraft 0.8.0 and PdfCraft 0.9.0; for the other four, the
  release tag's source plus `docker/craft-web/<app>.patch`, built with Trunk 0.21.14 (FilmCraft:
  `cargo xtask web`), wasm-bindgen 0.2.129 and binaryen 123.

```sh
go build -o /tmp/craftserve ./spikes/craft-web/serve
/tmp/craftserve -apps <dir with one folder per app> -out <dir for saves> -fixtures <dir>
node spikes/craft-web/drive.mjs --app pdfcraft --file /fixtures/Appunti.pdf \
  --steps 'wait:4000,click:0.5:0.5,key:Control+s,save' --out <dir>
```

## Findings

See `FINDINGS.md`. The run reports are in `runs/`.
