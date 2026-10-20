# Craft web apps in the cockpit: verification

Date: 2026-10-19. Tasks 1–7 of `docs/superpowers/plans/2026-10-19-craft-web-apps.md`, against the spec
(`docs/superpowers/specs/2026-10-19-craft-web-apps-design.md`). Run in a cloud dev container (4 CPUs,
15 GB, no Docker daemon, no aura stack), on branch `claude/photo-video-craft-wasm-ui-t7bkgi`.

**Verdict:** the six apps open, edit and save a real file through the bridge under aura's own page
policy, and so do the bytes the aura image ships; the image builds cold in 51 minutes and grows by
235 MB (130 MB compressed); the cockpit's frame and Edit are covered by unit tests. The live
acceptance run closed on 2026-10-20 with the operator's pass in the cockpit on the lab VM (below).

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

## The image

`docker build` of `docker/aura/Dockerfile` (Task 8), in the same container with its Docker daemon
started by hand (`dockerd`; it is installed but was not running, which an earlier version of this file
mistook for no daemon). Two accommodations of this sandbox, none in the repository: pulls go through the
`mirror.gcr.io` Docker Hub mirror (Docker Hub answered `429 Too Many Requests`), and the build ran
with `--network host` from a copy of the Dockerfile that adds, after each `FROM`, the egress gateway's
CA bundle and the standard variables pointing at it (`SSL_CERT_FILE`, `CARGO_HTTP_CAINFO`,
`GIT_SSL_CAINFO`, `NODE_EXTRA_CA_CERTS`, apt's `CaInfo`): the gateway re-signs TLS, and a build
container does not trust it otherwise. Every product step ran unchanged.

- **Cold build: 3,041 s (50.7 min), exit 0.** `craft-web` is 2,984 s of it, one app per step:
  PdfCraft 700 s, WordCraft 512, DeckCraft 594, PhotoCraft 772, FilmCraft 406. The tools' SHA-256, the
  tags' commits and the five patches all checked or applied inside the stage.
- **Size**, against `master` (8755b92, the branch's base) built the same way: 1,041,068,248 →
  1,406,357,445 bytes. The layers are identical but the apps': GridCraft's 26.2 MB on `master`;
  GridCraft's 26.3 MB plus the five patched apps' 235 MB here, so **+235 MB** on disk (raw and `.gz`
  side by side) and **+130 MB** compressed (`docker save | gzip -1`: 258,087,450 → 388,252,602).
  The sandbox's CA and apt setting add about 220 kB to both.
- **The image's own bytes**, copied out of `/usr/share/aura/apps` and served through
  `internal/staticapp` by the spike server, each app framed with `drive.mjs` and every save checked
  outside it:

  | app | saved | checked |
  |---|---|---|
  | GridCraft | `contabilita_2026.xlsx` (Ctrl+S, `it-IT`) | sheets `registro`, `riepilogo`, as the original |
  | PdfCraft | `Appunti.pdf` after a rotate; the manual (Ctrl+S, `it-IT`) | `Page rot: 90`; byte-identical |
  | WordCraft | `sample.docx` | `document.xml` holds the typed text |
  | DeckCraft | `sample.pptx` | 2 slides, the edited title |
  | PhotoCraft | `photo.png` after Invert (`it-IT`) | 0 of 65,536 pixels wrong |
  | FilmCraft | `Untitled.fcproj`, `edited.mp4` (`it-IT`) | project JSON; 4.017 s 320×180 avc1 + mp4a |

  No policy violation, `dirty` true then false wherever the app reports it. Each app folder carries
  `LICENSE-MIT` and `LICENSE-APACHE`; in the patched wasm, "ArtCraft" survives only in copyright
  notices, a command id, link targets and the allowed "based on" sentence.

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

## Live run on the lab VM

The lab VM `192.168.101.158` ran these builds through its updater on 2026-10-19:
- `fb69fe5bc`: a file opened in the file manager shows the cockpit preview, never a new tab.
- `dcd3a8a38`:
  - an image the file manager lists without a type previews by its extension (`9147f22fc`);
  - a video streams from the file manager's own route (`739cb88d8`);
  - Edit and Full screen sit in the preview's header, beside "Scarica il file", in its style
    (`c0b7fbda5`). The Craft apps' own full-screen command is unhandled by eframe 0.36's web
    backend, so the cockpit's button replaces it.

The operator saved a docx from WordCraft and an xlsx from GridCraft in the cockpit. Both landed in
the bucket and were valid when checked outside the apps: the docx with 334 paragraphs, the xlsx
with 11,216 cells and their number formats. Both contents were equal to the originals', so this
does not show that an edit reached the file. Which edit was made is an open question to the
operator. After the GridCraft save, the xlsx re-indexed with 55 passages instead of 71
(`2026-10-19-ingest-index-orphans.md`).

**2026-10-20, build `5d12271d7`.** The build adds the unsaved-changes notice (plan Task 9). The VM's
updater applied it at 08:53 UTC, and every container was healthy (0 unhealthy). The operator then ran
the pass in the cockpit (Acceptance 2–4) and confirmed it, with the notice and the held first close:
"funziona", "ok tutto a posto".

**Measured afterwards.** All six files of `prove-craft/` were downloaded through the file manager's
own route and compared with the samples uploaded on 2026-10-19 by content, not by bytes:

| file | last saved (UTC) | checked outside the app |
|---|---|---|
| `calibre_demo.docx` (WordCraft) | 2026-10-20 08:53:06 | 334 → 337 paragraphs: three empty paragraphs added, every other paragraph's text identical |
| `Colosseo.jpg` (PhotoCraft) | 2026-10-20 08:54:27 | 1920×1345 kept. 2.53% of pixels differ by more than 60/255, all inside x 274–796, y 138–506; elsewhere the mean difference is at most 0.4/255, which is the re-encode. Not an Invert: the mean difference from the inverted original is 153.6/255 |
| `Financial_Sample.xlsx` (GridCraft) | 2026-10-19 20:50:41 | yesterday's save; 11,216 cells, none different by value |
| `Costituzione_Italiana.pdf` (PdfCraft) | not saved | identical to the original |
| `SampleShow.pptx` (DeckCraft) | not saved | identical to the restored original |
| `Big_Buck_Bunny_10s.mp4` (FilmCraft) | not saved | identical, no project or export beside it |

So an edit made in the cockpit reached its file, checked outside the app, for **WordCraft and
PhotoCraft**. GridCraft's xlsx and the other three files carry no edit from this pass.

**Then GridCraft and DeckCraft, driven.** The same day, Playwright drove the VM's cockpit with
the operator's session (`cockpit-drive.mjs`, Chromium, the frame at 1440×900): it opened each file
from the file manager, clicked Edit, typed, and pressed Ctrl+S. The bridge reported `dirty` true,
then `save` with the file's name, then `dirty` false. Both files were downloaded again through the
file manager's route and compared by content:

| file | saved (UTC) | checked outside the app |
|---|---|---|
| `Financial_Sample.xlsx` (GridCraft) | 2026-10-20 09:03:58 | 11,216 → 11,217 cells; the only difference is Q2, empty before, now `Collaudo Aura 2026-10-20` |
| `SampleShow.pptx` (DeckCraft) | 2026-10-20 09:04:46 | valid zip; slide 1's title went from `Title of the first slide` to `Title of the first slide (collaudo Aura)`, and no other slide text changed |

So an edit reached its file for four apps: WordCraft and PhotoCraft by the operator's hand,
GridCraft and DeckCraft by the driven browser. During the DeckCraft run the console logged two
`503` answers, not traced here. The PdfCraft and FilmCraft edits are not run.

## Not shown here

- The image was built here, not in CI: the release workflow's time, with its buildx cache, and a
  multi-arch build are unmeasured. It ran with the two sandbox accommodations above.
- In the cloud container, no cockpit ran against a running aura; the lab VM pass above is the operator's,
  in the cockpit. The Playwright suites were not run (they need the stack). The file manager's spec expects the PDF preview's `blob:` frame, which the viewer keeps.
- One small fixture per app; WebGPU, Firefox, Safari and phones untried; no memory measurement.
