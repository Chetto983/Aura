"""A workbook's rows as passages, grouped the way the 2026-10-16 spike measured them.

filecard reads the workbook (aura-filecard -json): the headers, the cached values and the caps
are its own, so the card and the passages agree on what a column is called. This module only
groups what it hands over, 15 rows to a group with 2 of them shared with the next, and heads
each group with its sheet, which a row's own text does not name (spec, decision 2; prd.md §11).
"""

import dataclasses
import itertools
from collections.abc import Iterator, Sequence

from ingest import chunk

_GROUP = 15
_SHARED = 2


@dataclasses.dataclass(frozen=True, slots=True)
class Row:
    """One worksheet row: `number` is the row Excel shows, `text` is "Header: value; ..."."""

    sheet: str
    number: int
    text: str


def group(rows: Sequence[Row]) -> list[chunk.Chunk]:
    """The passages of a workbook's rows, sheet by sheet, each opening with "Foglio: <sheet>".

    filecard emits a workbook's rows in sheet order, so each sheet's rows are one run. A group
    past the embedding budget is split by chunk.chunk and the sheet line heads every piece.
    """
    budget = chunk.document_budget()
    pieces: list[chunk.Chunk] = []
    for sheet, sheet_rows in itertools.groupby(rows, key=lambda row: row.sheet):
        header = f"Foglio: {sheet}\n"
        for window in _windows(list(sheet_rows)):
            body = "\n".join(f"Riga {row.number}: {row.text}" for row in window)
            if chunk.count_tokens(header + body) <= budget:
                parts = [body]
            else:
                parts = [part.text for part in
                         chunk.chunk(body, max_tokens=budget - chunk.count_tokens(header))]
            pieces.extend(chunk.Chunk(text=header + part, heading_path=[sheet]) for part in parts)
    return pieces


def _windows(sheet_rows: list[Row]) -> Iterator[list[Row]]:
    start = 0
    while True:
        yield sheet_rows[start:start + _GROUP]
        if start + _GROUP >= len(sheet_rows):
            return
        start += _GROUP - _SHARED
