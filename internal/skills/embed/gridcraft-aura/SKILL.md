---
name: gridcraft-aura
description: "Open, recalculate, edit and save Excel workbooks (.xlsx, .xls, .xlsm, .xlsb, .ods, .csv) with the gridcraft MCP server in your box — a real formula engine, 513 Excel functions, cached values written on save. Use it for any spreadsheet task: change an input and read the new result, fill or fix cells, format, sort, filter, add a table or a chart, convert .xls to .xlsx. Italian triggers: «foglio Excel», «cambia la cella», «ricalcola», «aggiorna il budget», «salva in xlsx», «converti l'xls». Load it BEFORE the first gridcraft call."
---

# GridCraft in Aura

Your tools are `gridcraft__<tool>`, deferred: `tool_search` with "gridcraft" lists them.

## The round trip

1. A library document: `document_open` copies it to `/workspace/documents/<id>/<file>`. A file you made is
   already in `/workspace`.
2. `gridcraft__open_workbook {path}` — absolute path. It opens .xls too (measured: a 16-sheet, 906-formula
   valuation model).
3. Read before you write: `gridcraft__inspect_workbook` (sheets, used ranges, names), then
   `gridcraft__read_range {sheet, range}` — add `formulas: true` to see the formulas instead of values.
   - Read small windows at explicit addresses (`A30:B45`, never a whole sheet): the answer is rows WITHOUT
     row numbers, so the first row is the window's first row. Count from it, not from 0.
   - Before you write a cell you located that way, confirm it: `gridcraft__get_cell {cell}` must show the
     label or value you expect. Writing one row off changes a different input and nothing warns you.
4. Change: `gridcraft__set_cell {cell, input}` types exactly as a user would (a number, text, or `=FORMULA`).
   A cell on another sheet is quoted: `'Master Inputs Start here'!B39`. Several cells:
   `gridcraft__write_range {range, values}`.
5. Every dependent formula recalculates at once: read the output cell with `get_cell` and report the number,
   old and new. Measured: the result matched LibreOffice's recalculation to the last digit.
6. `gridcraft__save_workbook {path}` — always a NEW path under `/workspace` (`<name>-modificato.xlsx`),
   never over the original. The saved file carries the cached values, so any reader sees the new numbers.
7. `send_file` delivers it; that copy also lands in the library.

`gridcraft__evaluate_formula {formula}` answers "what would X be" without touching the sheet.

To LOOK at a sheet (its layout, a chart): select the sheet (`gridcraft__select {range: "'Sheet'!A1"}`), then
`gridcraft__execute_command {command: "file.exportPdf", params: {path: "/workspace/.../view.pdf",
range: "A1:H40", fitToPage: true}}`, then `pdfcraft__doc_open` and `pdfcraft__render_preview` give you the
image. `##########` there is a number wider than its column, as in Excel: read the value with `read_range`.

## Limits (measured 2026-10-18, GridCraft 0.8.0)

- No confinement flags: the server can write anywhere in the box. Write only under `/workspace`.
- `screenshot` needs a desktop app (it is not offered); look through `file.exportPdf` as above.
- Error constants (`#DIV/0!`) are kept as values, not as empty formulas.
- Alpha software: after a save, re-open the file and read the cells you changed before you report.
