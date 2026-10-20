# A file named with @: verification on the lab VM

Date: 2026-10-20. Plan `docs/superpowers/plans/2026-10-20-mentioned-files.md`, Tasks 2 and 3.
The lab VM `192.168.101.158` ran each build through its updater; the operator asked in the
cockpit and exported both conversations.

**Verdict:** with build `1504dc5ed`, a file named with `@` reaches the model by the index's own id,
and the model's first call opens that id. GridCraft then reads the file's current version, and no
tool receives the file manager's path.

## The friction (before)

Two conversations of 2026-10-20 named `@file:"/prove-craft/Financial_Sample.xlsx"`. In both,
`gridcraft__open_workbook` got `/prove-craft/Financial_Sample.xlsx` and failed with `ENOENT`, at
09:02:56 and 09:04:24 UTC. In the second, `gridcraft__inspect_workbook` then answered with the
workbook another conversation had left open. That copy was taken at 09:03:06, and the library file
changed at 09:03:57.

## Run 1 — build `85cfe66ba` (Task 1)

Conversation `01a158d1-…`, 10:18 UTC, `deepseek/deepseek-v4.1-flash-1017`:
`@file:"/prove-craft/Financial_Sample.xlsx" apri il file e dimmi cosa vedi`.

- No `ENOENT`. The model called `document_search`, then `document_open` on
  `doc_3c6de60c96c09f3a65a8f3cdb53e1153`, then GridCraft on the path it returned
  (`sha256 e9c1c41d…`, 65,761 bytes: the current file, Q2 included).
- The block was wrong. Its reasoning quotes it: "no asset holds this path, so document_open
  cannot reach it". A file the file manager writes has no asset row. The index files it under
  `SearchDocumentID(identity, "s3", key)`, which is exactly the id the model found by searching.
  Task 1 had assumed an asset row without checking one existed.

## Run 2 — build `1504dc5ed` (Task 3)

Conversation `01a158ea-a269-7c9a-8fd5-083e10c7f3b1`, 10:45 UTC, `gemma4:31b-cloud`:
`@file:"/prove-craft/Financial_Sample.xlsx" aprilo e dimmi cosa vedi`.

- The first call is `document_open` with `doc_3c6de60c96c09f3a65a8f3cdb53e1153`, the block's id.
  There was no search.
- `gridcraft__open_workbook` got the returned path
  `/workspace/documents/document-0125204f7dea965014991c1f/Financial_Sample.xlsx`
  (`sha256 e9c1c41d…`). `inspect_workbook` and `read_range` followed, with no error.
- The operator: "funziona".

## What this does not show

- **One file, one identity, two models.** A folder mention, an upload in the library (which has
  both an asset id and a document id), and a file the index has not reached were not run live;
  unit tests cover them.
- **The answer's quality is outside this fix.** Run 2 read ten rows and said the data is from
  2014, while the sheet runs from September 2013 to December 2014. Its `tool_search` for
  `select:gridcraft` named no tool.
- **GridCraft's state across conversations is unchanged.** The block tells the model to reopen a
  file from the new copy; the sidecar still keeps one workbook per identity.
