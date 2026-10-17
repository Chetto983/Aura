"""A workbook's rows reach ArcadeDB as passages, through the production indexing path.

space_driver runs _extract and index_object on the real file against a disposable database,
with fake_embedder answering, and every read is the SQL Aura's reader uses
(internal/arcadedb/document_retrieval.go, document_lexical.go).

A row passage has no character span: the extractor produced no text for a workbook, so a span
would anchor in nothing (spec, decision 4). Aura's reader takes an absent or null span as none
and cites the passage by ordinal; what it must never meet is a span left behind by an earlier
version of the same passage (Review Focus 5).
"""

import collections

import pytest

from ingest import app
from ingest.tests import disposable

WORKBOOK = "/eval/complex_corpus/magazzino_ricambi_2026.xlsx"
NAME = "magazzino_ricambi_2026.xlsx"
BUDGET = "/eval/complex_corpus/budget_consolidato_2026.xlsx"
BUDGET_NAME = "budget_consolidato_2026.xlsx"
INVOICE = "/eval/corpus/fattura_2026_001.xlsx"
INVOICE_NAME = "fattura_2026_001.xlsx"
DATABASE = "aura_t_row_passages"
SHOWN_WHOLE_DATABASE = "aura_t_row_whole"
REWRITE_DATABASE = "aura_t_row_rewrite"
_PASSAGES = ("SELECT passage_key, ordinal, text, heading_path, char_start, char_end FROM Passage "
             "WHERE source_key = :key ORDER BY ordinal")


def _groups(row_count: int) -> int:
    """Groups of 15 rows that share 2 with the next, counted without rows.py."""
    return 1 + max(0, -(-(row_count - 15) // 13))


@pytest.fixture(scope="module")
def workbook(tmp_path_factory):
    disposable.drop(DATABASE)
    try:
        disposable.space_driver_pass(DATABASE, [f"{NAME}=@{WORKBOOK}"],
                                     tmp_path_factory.mktemp("rows") / "coco.db")
        yield DATABASE
    finally:
        disposable.drop(DATABASE)


@pytest.fixture(scope="module")
def shown_whole(tmp_path_factory):
    disposable.drop(SHOWN_WHOLE_DATABASE)
    try:
        disposable.space_driver_pass(
            SHOWN_WHOLE_DATABASE, [f"{BUDGET_NAME}=@{BUDGET}", f"{INVOICE_NAME}=@{INVOICE}"],
            tmp_path_factory.mktemp("whole") / "coco.db")
        yield SHOWN_WHOLE_DATABASE
    finally:
        disposable.drop(SHOWN_WHOLE_DATABASE)


@pytest.fixture
def rewrite_database():
    disposable.drop(REWRITE_DATABASE)
    try:
        yield REWRITE_DATABASE
    finally:
        disposable.drop(REWRITE_DATABASE)


def test_a_workbook_gets_row_passages_without_a_span(workbook):
    rows_per_sheet = collections.Counter(row.sheet for row in app._card(WORKBOOK, NAME).rows)
    (document,) = disposable.query(workbook, "SELECT passage_count, normalized_text_sha256 "
                                   "FROM IndexedDocument WHERE source_key = :key", {"key": NAME})
    passages = disposable.query(workbook, _PASSAGES, {"key": NAME})

    # No group of this workbook is past the embedding budget: its longest row is under 200
    # characters, so every group is one passage.
    assert document["passage_count"] == sum(map(_groups, rows_per_sheet.values())) == len(passages)
    assert document["normalized_text_sha256"] == ""
    assert [passage["ordinal"] for passage in passages] == list(range(len(passages)))
    for passage in passages:
        file_line, sheet = passage["text"].splitlines()[0].split(" — Foglio: ")
        assert file_line == f"File: {NAME}"
        assert sheet in rows_per_sheet
        assert passage["heading_path"] == [sheet]
        assert passage.get("char_start") is None and passage.get("char_end") is None


def test_a_sheet_the_card_shows_whole_gets_no_row_passages(shown_whole):
    card = app._card(BUDGET, BUDGET_NAME)
    assert set(card.shown_whole) == {"Ipotesi", "Ricavi", "Consolidato"}
    kept = collections.Counter(row.sheet for row in card.rows if row.sheet not in card.shown_whole)
    assert set(kept) == {"Costi"}

    passages = disposable.query(shown_whole, _PASSAGES, {"key": BUDGET_NAME})
    (document,) = disposable.query(shown_whole, "SELECT passage_count FROM IndexedDocument "
                                   "WHERE source_key = :key", {"key": BUDGET_NAME})

    assert document["passage_count"] == sum(map(_groups, kept.values())) == len(passages)
    assert all(passage["heading_path"] == ["Costi"] for passage in passages)


def test_a_workbook_shown_whole_keeps_passage_count_zero(shown_whole):
    card = app._card(INVOICE, INVOICE_NAME)
    assert card.rows and {row.sheet for row in card.rows} == set(card.shown_whole)

    passages = disposable.query(shown_whole, _PASSAGES, {"key": INVOICE_NAME})
    (document,) = disposable.query(shown_whole, "SELECT passage_count, card FROM IndexedDocument "
                                   "WHERE source_key = :key", {"key": INVOICE_NAME})

    assert passages == []
    assert document["passage_count"] == 0
    assert document["card"]


def test_a_span_does_not_survive_a_rewrite_without_one(rewrite_database, tmp_path):
    state = tmp_path / "coco.db"
    text = "Giacenze di magazzino al 31 marzo."
    disposable.space_driver_pass(rewrite_database, [f"{NAME}={text}"], state)
    (before,) = disposable.query(rewrite_database, _PASSAGES, {"key": NAME})
    assert (before["char_start"], before["char_end"]) == (0, len(text))

    disposable.space_driver_pass(rewrite_database, [f"{NAME}=@{WORKBOOK}"], state)
    after = disposable.query(rewrite_database, _PASSAGES, {"key": NAME})[0]

    assert after["passage_key"] == before["passage_key"]
    assert after["text"].startswith(f"File: {NAME} — Foglio: Giacenze\n")
    assert after.get("char_start") is None and after.get("char_end") is None, (
        f"a stale span survived the rewrite: {after}")


def test_a_part_number_in_a_row_is_found_by_full_text(workbook):
    hits = disposable.query(
        workbook,
        "SELECT text, heading_path, $score AS lexical_score FROM Passage "
        "WHERE SEARCH_INDEX('Passage[text]', :query) = true ORDER BY lexical_score DESC LIMIT 3",
        # Escaped as Aura's escapeLucene escapes it, so the hyphen is not an operator.
        {"query": r"RIC\-10959"},
    )

    assert hits, "the part number is in no passage"
    assert "\nRiga 139: Codice: RIC-10959; Descrizione: Encoder rinforzato 63 mm" in hits[0]["text"]
    assert hits[0]["heading_path"] == ["Giacenze"]
