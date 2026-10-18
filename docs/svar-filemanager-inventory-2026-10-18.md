# SVAR React File Manager 2.6.0: what the pinned code supports (2026-10-18)

The question: can Aura's document browser (`web/src/files/`) show, edit and filter per-document
fields in the style of Paperless-ngx's document list, without forking SVAR?

**Method.**
- The exact versions of `web/package-lock.json` were fetched with `npm pack --ignore-scripts` and
  read, never installed or run:
  - react-filemanager, filemanager-store, filemanager-data-provider, react-menu: 2.6.0;
  - react-grid and grid-store: 2.7.0;
  - react-core: 2.6.1, forced through `overrides` in `web/package.json:123-125`.
- Every package is MIT.
- Line numbers below refer to the published files: `react-filemanager/dist/index.es.js` (FM),
  `types/index.d.ts` (FMT), and filemanager-store's `dist/types/types.d.ts` (STT).

## Verdict

**SVAR's own table view cannot become a Paperless-style list without a fork.**
- Its columns are a closed constant: name, size, date (FM:479-503).
- The grid's api is never exposed (FM:554-556).
- The toolbar is fixed (FM:767-792), and the cards show only a name and an image (FM:123-161).

**Everything else is reachable through public hooks:**
- custom menu actions with handlers (`menuOptions`, modes `folder/file/body/add/multiselect`);
- reading the multi-selection;
- extra per-item properties, kept on the parsed item;
- key/value rows in the info panel (`extraInfo`, single selection only);
- sorting by any property from code;
- feeding a list Aura filtered on the server (`data` or `exec('provide-data')`).

**`@svar-ui/react-grid` 2.7.0 is the way without a fork.** It is already in the tree as the file
manager's own dependency, and its public API has:
- column `cell` components and an `editor`;
- header `filter` and `sort`;
- `multiselect` and `reorder`.

**A trap.** The file manager binds its hotkeys to a native `keydown` on its whole root, with no
check of the event's target. A text input placed inside the widget would therefore drive file
actions: the arrows, Delete, and Ctrl+C/X/V/R/D. This is read from the code, not run. Field editing
belongs outside the widget.

## Capability table

| Capability | At 2.6.0 | How |
|---|---|---|
| Extra or custom columns in the table view | no | fixed columns, private grid |
| Sort by any item property | from code only | `exec('sort-files', {key, order})`; headers only for the 3 built-in columns |
| Custom cards | no | `previews` and `icons` return image URLs only |
| Info panel with Aura's content | partly | `extraInfo(file)` key/value rows below the fixed Type, Size and Date rows; single selection only |
| Hover tooltip with React content | yes | exported `Tooltip` |
| Custom context-menu and "Add New" actions, hotkeys | yes | `menuOptions(mode, item)`, an option's `handler` and `hotkey` |
| Custom toolbar controls | no | fixed toolbar |
| Multi-selection visible to Aura | yes | `getState().panels[…].selected`, `intercept/on('select-file')` |
| Built-in filtering beyond names | no | `filter-files` is a name substring search |
| A list Aura filtered itself | yes | `data` or `provide-data`. SVAR's search mode re-filters it by name |
| An edit-fields action | no | no update action in the store or the provider |
| Drag and drop | OS upload only | — |
| Large lists | table virtualized, cards all rendered | — |
| i18n, theming, read-only | yes | `Locale`, `Willow`/`WillowDark`/`Material`, `readonly` |

## What Aura already does with it

- **`FilesWorkspace.tsx` mounts the manager.** It uses `Willow`/`WillowDark` with an in-house Italian
  locale (`filesLocale.ts`), `icons="simple"` and same-origin styles.
- **Writes go through the REST provider** (`api.setNext(provider)`, `filesApi.ts`). Folders
  lazy-load through `onRequestData` and `provide-data`.
- **Selection** (`fileSelection.ts`): "Download" for a multi-selection, "Select", and a
  tap-to-toggle intercept.
- **Labels.** The server sends `parent: 0`, `name` and `ext`, so labels are not rebuilt from the
  uuid id (`filesApi.store.test.ts:35-56`).
- **Unused so far:** `extraInfo`, `previews`, `mode` and `readonly`.

## Not determined

- **Runtime behaviour.** Nothing was executed: not React elements as `extraInfo` values, not focus
  inside them, and not replacing `data` while folders are expanded.
- **react-grid outside its types and its virtualization.**
