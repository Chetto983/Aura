"""A workbook's rows become passages the way the 2026-10-16 spike measured them.

Fifteen rows to a group, two of them shared with the next, each group headed by its sheet and
each row written "Riga N: Header: value; ..." under the worksheet's own row number (spec,
decision 2). A group longer than the embedding budget is split, and every piece keeps the
sheet line, so no piece of a row reaches the index without saying where it comes from.
"""

import re

from ingest import rows
from ingest.chunk import EMBED_DOC_PREFIX, MODEL_MAX_TOKENS, count_tokens


def _rows(sheet, numbers, text=lambda number: f"Codice: RIC-{10000 + number}; Giacenza: {number}"):
    return [rows.Row(sheet=sheet, number=number, text=text(number)) for number in numbers]


def _numbers(piece):
    return [int(found) for found in re.findall(r"^Riga (\d+): ", piece.text, re.MULTILINE)]


def test_rows_group_fifteen_at_a_time_sharing_two():
    pieces = rows.group(_rows("Giacenze", range(1, 41)))

    assert [_numbers(piece) for piece in pieces] == [
        list(range(1, 16)), list(range(14, 29)), list(range(27, 41)),
    ]


def test_a_row_is_written_under_its_worksheet_row_number():
    pieces = rows.group(_rows("Giacenze", [139]))

    assert pieces[0].text == "Foglio: Giacenze\nRiga 139: Codice: RIC-10139; Giacenza: 139"


def test_every_group_opens_with_its_sheet():
    pieces = rows.group(_rows("Giacenze", range(2, 42)))

    assert pieces
    for piece in pieces:
        assert piece.text.splitlines()[0] == "Foglio: Giacenze"
        assert piece.heading_path == ["Giacenze"]
        assert (piece.start, piece.end) == (None, None), "a row group has no span in extracted text"


def test_a_group_over_budget_splits_and_every_piece_keeps_the_header():
    # A wide sheet with long text cells: fifteen rows of about 2,100 characters each are far
    # past the 2,048-token ceiling under either tokenizer.
    def long_note(number):
        return f"Nota: riga{number} " + "controllo eseguito sul lotto " * 70

    pieces = rows.group(_rows("Note", range(2, 17), long_note))

    assert len(pieces) > 1
    for piece in pieces:
        assert piece.text.startswith("Foglio: Note\n")
        assert piece.heading_path == ["Note"]
        assert (piece.start, piece.end) == (None, None)
        assert count_tokens(EMBED_DOC_PREFIX + piece.text, add_special=True) <= MODEL_MAX_TOKENS
    joined = "\n".join(piece.text for piece in pieces)
    assert all(f"riga{number} " in joined for number in range(2, 17)), "a split lost a row"


def test_sheets_never_share_a_group():
    pieces = rows.group(_rows("Giacenze", range(2, 7)) + _rows("Fornitori", range(2, 5)))

    assert [piece.text.splitlines()[0] for piece in pieces] == ["Foglio: Giacenze", "Foglio: Fornitori"]
    assert [_numbers(piece) for piece in pieces] == [[2, 3, 4, 5, 6], [2, 3, 4]]
