"""An invoice's header pairs reach its IndexedDocument, through the production indexing path.

filecard keeps the label-value rows above a workbook's table (internal/documents/filecard,
HeaderPair), and the ingest stores them as "<Key>: <Value>" for Aura's document fields to suggest
a counterparty and a date from. space_driver runs _extract and index_object on the real file
against a disposable database, with fake_embedder answering.
"""

import pytest

from ingest import app
from ingest.tests import disposable

INVOICE = "/eval/corpus/fattura_2026_001.xlsx"
NAME = "fattura_2026_001.xlsx"
NOTE = "nota.txt"
DATABASE = "aura_t_header_pairs"


@pytest.fixture(scope="module")
def indexed(tmp_path_factory):
    disposable.drop(DATABASE)
    try:
        disposable.space_driver_pass(DATABASE, [f"{NAME}=@{INVOICE}", f"{NOTE}=Una nota senza tabelle."],
                                     tmp_path_factory.mktemp("header") / "coco.db")
        yield DATABASE
    finally:
        disposable.drop(DATABASE)


def test_header_pairs_reach_indexed_document(indexed):
    (invoice,) = disposable.query(indexed, "SELECT header_pairs FROM IndexedDocument WHERE source_key = :key",
                                  {"key": NAME})
    assert invoice["header_pairs"] == [
        "Fattura n.: 2026_001",
        "Cliente: ACME SpA",
        "Partita IVA: IT01234567890",
        "Data emissione: 2026-01-12",
        "Stato: pagata",
    ]
    (note,) = disposable.query(indexed, "SELECT header_pairs FROM IndexedDocument WHERE source_key = :key",
                               {"key": NOTE})
    assert note["header_pairs"] == []


def test_the_pairs_come_from_filecard_not_from_the_rows():
    card = app._card(INVOICE, NAME)

    assert card.header[1] == "Cliente: ACME SpA"
    assert not any("ACME" in row.text for row in card.rows)
