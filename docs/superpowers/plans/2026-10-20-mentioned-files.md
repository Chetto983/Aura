# A file named with @ reaches the model by its id: implementation plan

**Status (2026-10-20):** DONE. Task 2's first live run found the gap Task 3 closes; the rerun on
`1504dc5ed` passed. The operator asked for the conversation
"Analisi approfondita file Financial Sample" to be read for its frictions ("TROVA LE FRIZIONI"),
then approved fixing the first one ("VAI").

> **For agentic workers:** REQUIRED SUB-SKILL: superpowers:executing-plans, task by task, with
> test-driven-development. Commits on master (the operator's standing rule).

**Goal:** when the user names a file with `@`, the model gets the id `document_open` takes, so it
works on the file's current version instead of passing the file manager's path to a tool.

**Architecture:**
- `internal/agui/server_context.go` hands the turn's file scopes (object keys) to
  `BuildTurnContext`; folders stay a retrieval filter only.
- `internal/assets/context_mentions.go` names each key the way the index does
  (`SearchDocumentID(identity, "s3", key)`), asks the index whether it holds it, looks up the
  identity's upload row on the key (`Store.ByObjectKey`), and renders a `<mentioned_files>` block
  beside the attachments.
- Telegram has no `@` mention and passes none.

**Tech Stack:** Go 1.27.

**Spec:** none; the measured friction below is the reason, and the change is one block of the turn
context.

## The friction, measured 2026-10-20 on the lab VM

Both conversations on `@file:"/prove-craft/Financial_Sample.xlsx"`:
- The composer sends the mention as `document_scope`, and the server made it a retrieval filter
  only (`documents.WithSourceScopes`). The model saw just the text path.
- `gridcraft__open_workbook` got `/prove-craft/Financial_Sample.xlsx` and failed with `ENOENT`.
- `gridcraft__inspect_workbook` then answered with the workbook another conversation had left open:
  `/workspace/documents/document-0125204f…/Financial_Sample.xlsx`, copied at 09:03:06, while the
  library file changed at 09:03:57. The analysis ran on that older copy.

## Global Constraints

- Test-first; `go vet`, build, tests, `-race` and golangci-lint (WSL) per touched package.
- The lookup is the identity's own; another identity's row on the same key stays unknown.
- No copy into the box before the turn: `document_open` writes fresh bytes on every call.

## Review Focus

1. A mention resolves only under the turn's identity.
2. A folder mention adds nothing to the model's message.
3. A mentioned file leaves the knowledge catalog, as an attachment does.

## Decisions taken while writing this plan

- **The model opens the file, the server does not.** Opening it before the turn would copy every
  mentioned file into the box whether the turn needs it or not; the block names the id and the
  model calls `document_open` when it works on the file.
- **The block warns about a box tool's open file.** GridCraft keeps one workbook per identity
  across conversations, so a workbook already open may be an older copy.
- **A key nothing holds is still listed**, with the note that it cannot be reached, so the model
  does not retry the path. (Task 1 said that of any key without an asset row, which is every file
  the file manager writes; Task 3 corrects it.)

## File structure

- `internal/assets/context_mentions.go` (new): `MentionedFile`, `BuildMentionBlock`,
  `resolveMentions`; `internal/assets/context.go`: `BuildTurnContext` takes the keys.
- `internal/agui/server_context.go`: `mentionedFiles`; `internal/agui/asset_service.go`;
  `internal/channels/telegram/bot.go` and its two callers.

## Tasks

### Task 1: the block

- [x] Failing tests:
  - `TestBuildTurnContextGivesAMentionedFileTheIDDocumentOpenTakes`: path, asset id, document id,
    the `document_open` line, the warning, an unknown key's note; no catalog entry.
  - `TestBuildTurnContextResolvesAMentionOnlyForItsOwnIdentity`.
  - `TestServerRunHandsTheMentionedFilesToTheTurnContext`: the file's key reaches the turn context,
    the folder's does not.
- [x] `context_mentions.go`, `BuildTurnContext`, `mentionedFiles`, the interfaces and fakes.
- [x] `go vet`, build, `go test` and `-race` on `internal/assets`, `internal/agui`,
  `internal/channels/telegram`; golangci-lint: 0 issues.

### Task 2: on the lab VM, with the real agent

- [x] Push, publish edge, wait for the VM's updater (`85cfe66ba`, applied 2026-10-20).
- [x] First run (operator, conversation `01a158d1-…`, 10:18 UTC):
  - `@file:"/prove-craft/Financial_Sample.xlsx" apri il file e dimmi cosa vedi`;
  - no `ENOENT`, and GridCraft opened the current copy (sha `e9c1c41d…`, Q2 included).
  - The block told the model "no asset holds this path, so document_open cannot reach it", which
    was false: a file the file manager writes has no asset row, only the index's document
    (`SearchDocumentID(identity, "s3", key)` = `doc_3c6de60c…`, the id the model then found
    through `document_search`). The design assumed an asset row without checking one existed.
- [x] After Task 3 (`1504dc5ed`, conversation `01a158ea-…`, 10:45 UTC): the first call is
  `document_open` on the block's `doc_3c6de60c…`, with no search, then GridCraft on its path. No
  error; the operator: "funziona".
- [x] Recorded in `docs/superpowers/verification/2026-10-20-mentioned-files.md`.

### Task 3: name the file the way the index does (2026-10-20)

- [x] Failing tests:
  - a file-manager file with no asset row but indexed gets its `doc_…` id and the
    `document_open` line, and no asset id;
  - an upload the index holds opens by the document id, one it does not hold yet by the asset
    id;
  - a key nothing holds says it cannot be reached;
  - the isolation test now covers the index too.
- [x] `resolveMentions` derives `SearchDocumentID(identity, "s3", key)`, asks `DocumentScope`
  which ids the index holds, and looks up the upload row. `bucketSourceKind` names `"s3"` for it
  and for `DocumentProcessor`.
- [x] `go vet`, build, `go test` and `-race` on `internal/assets`, `internal/agui` and
  `internal/channels/telegram`; golangci-lint: 0 issues.
