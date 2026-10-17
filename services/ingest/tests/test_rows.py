"""A workbook's rows become passages the way the 2026-10-16 spike measured them.

Fifteen rows to a group, two of them shared with the next, each group headed by its file and
its sheet and each row written "Riga N: Header: value; ..." under the worksheet's own row number (spec,
decision 2). A group longer than the embedding budget is split, and every piece keeps the
file and sheet line, so no piece of a row reaches the index without saying where it comes from.
"""

import re

from ingest import rows
from ingest.chunk import EMBED_DOC_PREFIX, MODEL_MAX_TOKENS, count_tokens

FILE = "budget_consolidato_2026.xlsx"

def _rows(sheet, numbers, text=lambda number: f"Codice: RIC-{10000 + number}; Giacenza: {number}"):
    return [rows.Row(sheet=sheet, number=number, text=text(number)) for number in numbers]


def _numbers(piece):
    return [int(found) for found in re.findall(r"^Riga (\d+): ", piece.text, re.MULTILINE)]


def test_rows_group_fifteen_at_a_time_sharing_two():
    pieces = rows.group(_rows("Giacenze", range(1, 41)), FILE)

    assert [_numbers(piece) for piece in pieces] == [
        list(range(1, 16)), list(range(14, 29)), list(range(27, 41)),
    ]


def test_a_row_is_written_under_its_worksheet_row_number():
    pieces = rows.group(_rows("Giacenze", [139]), FILE)

    assert pieces[0].text == (
        f"File: {FILE} — Foglio: Giacenze\nRiga 139: Codice: RIC-10139; Giacenza: 139")


def test_every_group_names_its_file_and_sheet():
    pieces = rows.group(_rows("Ricavi", range(2, 42)), FILE)

    assert pieces
    for piece in pieces:
        assert piece.text.splitlines()[0] == "File: budget_consolidato_2026.xlsx — Foglio: Ricavi"


def test_a_group_is_filed_under_its_sheet_and_has_no_span():
    pieces = rows.group(_rows("Giacenze", range(2, 42)), FILE)

    assert pieces
    for piece in pieces:
        assert piece.heading_path == ["Giacenze"]
        assert (piece.start, piece.end) == (None, None), "a row group has no span in extracted text"


def test_a_group_over_budget_splits_and_every_piece_keeps_the_header():
    # A wide sheet with long text cells: fifteen rows of about 2,100 characters each are far
    # past the 2,048-token ceiling under either tokenizer.
    def long_note(number):
        return f"Nota: riga{number} " + "controllo eseguito sul lotto " * 70

    pieces = rows.group(_rows("Note", range(2, 17), long_note), FILE)

    assert len(pieces) > 1
    for piece in pieces:
        assert piece.text.startswith(f"File: {FILE} — Foglio: Note\n")
        assert piece.heading_path == ["Note"]
        assert (piece.start, piece.end) == (None, None)
        assert count_tokens(EMBED_DOC_PREFIX + piece.text, add_special=True) <= MODEL_MAX_TOKENS
    joined = "\n".join(piece.text for piece in pieces)
    assert all(f"riga{number} " in joined for number in range(2, 17)), "a split lost a row"


def test_sheets_never_share_a_group():
    pieces = rows.group(_rows("Giacenze", range(2, 7)) + _rows("Fornitori", range(2, 5)), FILE)

    assert [piece.text.splitlines()[0] for piece in pieces] == [
        f"File: {FILE} — Foglio: Giacenze", f"File: {FILE} — Foglio: Fornitori"]
    assert [_numbers(piece) for piece in pieces] == [[2, 3, 4, 5, 6], [2, 3, 4]]
