# Craft web apps in the cockpit: verification

Date: 2026-10-19. Tasks 1–7 of `docs/superpowers/plans/2026-10-19-craft-web-apps.md`, against the spec
(`docs/superpowers/specs/2026-10-19-craft-web-apps-design.md`). Run in a cloud dev container (4 CPUs,
15 GB, no Docker daemon, no aura stack), on branch `claude/photo-video-craft-wasm-ui-t7bkgi`.

**Verdict:** the six apps open, edit and save a real file through the bridge under aura's own page
policy; the cockpit's frame and Edit are covered by unit tests; **the acceptance run is still open**:
neither the image nor a cockpit against a running aura was exercised here.

## What was measured

| app | build | opened | edited | saved, checked outside the app |
|---|---|---|---|---|
| PdfCraft | official 0.9.0, then 0.9.0 + patch | `Appunti.pdf`, a 15-page manual | Organize pages › rotate › Save pages | `Appunti.pdf`, `pdfinfo` `Page rot: 90`; the manual byte-identical after Ctrl+S (both builds; the patched one also in `it-IT`) |
| WordCraft | 0.8.0 + patch | `sample.docx` | text typed in the last paragraph | `sample.docx`, valid zip, `document.xml` ends with the text |
| DeckCraft | 0.8.0 + patch | `sample.pptx` | title edited | `sample.pptx` (not `.deckcraft`), 2 slides, the new title |
| PhotoCraft | 0.10.0 + patch | `photo.png` | Image › Invert | `photo.png`, all 65,536 pixels the exact inverse (again on the language build, `en-US` and `it-IT`) |
| FilmCraft | 0.9.0 + patch | `clip.mp4` (imported, sequence made) | — | `Untitled.fcproj` on Ctrl+S; `edited.mp4` on export, 4.017 s 320×180 H.264 + AAC |

How: `spikes/craft-web/` (README, FINDINGS, `runs/*.json`). Each app was served by `internal/staticapp`
under `/apps/<app>/` with aura's policy; no policy violation was logged. Every save came back as
`<app>:save` with the file's own name, and `<app>:dirty` went true on the edit and false after the save
(FilmCraft: after Ctrl+S; an export leaves the project as it was).

**The browser's language** (Task 7, FINDINGS 13–15), each app framed fresh with `drive.mjs --locale`:

| browser languages | PdfCraft | PhotoCraft |
|---|---|---|
| `it-IT` | Italian | Italian |
| `ja-JP,it-IT` | Italian | Italian |
| `ja-JP` | English | English |
| `zh-CN` | not run | English |
| `zh-TW` | English | not run |
| `ko-KR` | no catalog | English |
| `ar-SA`, `te-IN` | English | no catalog |
| `ru-RU` | Russian (first patched build) | not run |
| `kk-KZ` | Kazakh | no catalog |
| `el-GR` | Greek | Greek |
| `vi-VN` | not run | Vietnamese |

Before the face check, the first patched builds drew `ja-JP`, `zh-CN`, `ar-SA` and `te-IN` as boxes.
With `ja-JP`, FilmCraft stays in English with a notice; WordCraft draws boxes (FINDINGS 15).

`docker/craft-web/build.sh` ran end to end outside Docker for DeckCraft (776 s, sharing the CPUs with
other builds): tag cloned and checked against its commit, patch applied, Trunk offline. Its wasm is
30,561,349 bytes, the same as the manual build and within 0.3% of the official one, and it opened,
edited and saved `sample.pptx` through the bridge like the manual build. The same for PdfCraft with its
patch (548 s from a fresh clone and an empty target): its wasm, 39,088,994 bytes, differs from the tested
build by two bytes (not traced; the two builds ran from different source paths), and its `it-IT`, `ja-JP` and `en-US` screens are
byte-identical to the tested build's, the manual byte-identical after Ctrl+S.

## Gates

- Go: `go vet ./...`, `go build ./...`, `go test -race` on `internal/staticapp`, `internal/config`,
  `cmd/aura` (including the release-context test that found `docker/craft-web` missing from
  `.goreleaser.yaml`); golangci-lint v2.14.0 (the CI pin) on those packages and the spike: 0 issues.
- Web: lint, typecheck, format, knip, and the suite with coverage: 369 files, 3,191 tests, statements
  92.51%, branches 86.91%, lines 94.41%; `internal/webui/dist` rebuilt.
- Patches: `cargo clippy --target wasm32-unknown-unknown` on the five web crates: no warning in the
  patched files (with `-D warnings` DeckCraft, PhotoCraft and FilmCraft stop earlier, on lints of
  Rust 1.97 in upstream crates the patches do not touch). Each patch applies to a fresh checkout of
  its tag; for PdfCraft and PhotoCraft, the patch plus `build.sh`'s `sed` reproduce the tested source
  file for file. PhotoCraft's new unit test (`the_web_build_skips_the_languages_it_has_no_face_for`)
  passes natively. Upstream's own suites were not run against the patches (FINDINGS, "does not show").

## Not shown here

- The aura image was not built (no Docker daemon): the `craft-official` and `craft-web` stages, the
  build time in CI and the image size change are unmeasured.
- No cockpit against a running aura: the file manager's Edit and the save into Garage through
  `saveBeside` were not exercised end to end, and the Playwright suites were not run (they need the
  stack). The file manager's spec expects the PDF preview's `blob:` frame, which the viewer keeps.
- One small fixture per app; WebGPU, Firefox, Safari and phones untried; no memory measurement.
