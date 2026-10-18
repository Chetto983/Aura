# The xlsx preview is GridCraft

Date: 2026-10-18. The operator's call, after the Documenti detail showed an invoice as a bare HTML
table ("questa vista fa cacare"). It replaces the renderer every surface shares
(`web/src/chat/artifacts/renderers/XlsxPreview.tsx`): the Documenti detail, the file manager's
preview and the chat's artifacts.

## Inventory (measured 2026-10-18)

| Candidate | License | What it is | Verdict |
|---|---|---|---|
| `read-excel-file` (today) | MIT | cell values only, rendered by us as `<table>` | no styles, no merges, no sheet tabs, no formulas |
| `storytold/gridcraft` v0.8.0 | MIT or Apache-2.0 | a clean-room Excel-style app in Rust; the web build is a 7.1 MB static site (19 MB wasm), with an iframe host mode | **chosen** |
| `@extend-ai/react-xlsx` 0.17.2 | MIT | React viewer over a WASM parser, 19.5 MB unpacked | young (24 stars), read-only |
| FortuneSheet + FortuneExcel | MIT | canvas spreadsheet plus an xlsx import plugin | an editor to wire, its import is a separate project |
| Univer | Apache-2.0 | full spreadsheet | xlsx import is Univer Pro, paid |
| `AnirudhaPatil-1/Excel-Clone` | none | a jQuery portfolio app | the look the operator wants, no xlsx reading |

## Decisions

1. **The image ships GridCraft's pinned web release.** `docker/aura/Dockerfile` downloads
   `gridcraft-web-0.8.0.zip` from the GitHub release, checks its SHA-256
   (`532966ca…f1dd4`, the release's own digest), and gzips the wasm and the glue at build time.
   Nothing is fetched at run time: the appliance answers offline.
2. **aura serves it at `/apps/gridcraft/`, from `AURA_GRIDCRAFT_DIR`.**
   - The route is public. It carries no data, and the share page's preview needs it without a
     session.
   - Headers follow GridCraft's `HOSTING.md`: `.wasm` as `application/wasm`, the hashed files
     cached as immutable, `index.html` as `no-cache`, and the precompressed file when the browser
     accepts gzip. Caddy compresses nothing.
   - A directory that is missing answers 404.
3. **The page carries a CSP of its own.**
   - Scripts come only from the same origin, plus the inline loader by its hash, computed from the
     served `index.html`. `'wasm-unsafe-eval'` allows the module to compile.
   - `connect-src 'self'` and `frame-ancestors 'self'`.
   - This is defense in depth. GridCraft's `WEBSERVICE` and `FILTERXML` return `#VALUE!` and make
     no request, and a hyperlink opens only on a click. The frame has to be same-origin, because
     the bridge answers only its own origin's parent.
4. **The preview frames `/apps/gridcraft/?host=parent`.**
   - On `gridcraft:ready` it sends `gridcraft:open { name, bytes }`, with the bytes read through
     `useAssetContent`, so the share page's token-scoped route works unchanged.
   - `gridcraft:failed` shows the preview's error.
   - `read-excel-file` leaves the dependencies.
5. **A save writes back where the source can write** (the operator's call, the same day).
   - `gridcraft:save` goes through the asset source's optional `save`, which only the file
     manager's tier has. That covers the file manager and the Documenti detail.
   - A save under the opened name replaces the file at its own key. aura keeps the object's
     metadata, so a chat document stays known by its name and not its key; the ingest reads the
     new bytes, and the document's fields stay (acceptance 4 of the document-fields spec).
   - Any other name (Save As, a PDF export) is a new file in the same folder.
   - The chat's artifacts and the share page have no `save`. There an edit or a save shows that
     nothing made here is stored.

## What this does not do

- **No write-back for the chat's artifacts or the share page.** Their sources have no `save`.
- **GridCraft draws its own light chrome** inside the dark cockpit.
- **The first open downloads about 6 MB compressed**; the hashed files are cached after that.
