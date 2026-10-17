"""One filecard call gives the ingest both the card it stores and the description it embeds.

The card ends with a trailer about Aura and often carries caveats about what Aura read. Those
are true for a reader and noise in a vector: embedding the card without them raised r@8 on all
five question sets measured on 2026-10-16 (prd.md §11, "The card's vector").
"""

import os

from ingest import app

_TRAILER = "document_open"


def test_the_card_keeps_its_trailer_and_the_description_drops_it(tmp_path):
    path = tmp_path / "fornitori.txt"
    path.write_text("Il fornitore consegna i ricambi ogni martedì mattina.\n", encoding="utf-8")

    card = app._card(str(path), "fornitori.txt")

    assert _TRAILER in card.text
    assert card.description.startswith("fornitori.txt")
    assert "martedì" in card.description
    assert _TRAILER not in card.description
    card_lines = card.text.splitlines()
    assert all(line in card_lines for line in card.description.splitlines())


def test_a_file_filecard_cannot_read_has_neither(tmp_path):
    card = app._card(str(tmp_path / "sparito.txt"), "sparito.txt")

    assert (card.text, card.description) == ("", "")


def test_a_silent_filecard_failure_still_says_why_the_card_is_missing(monkeypatch, tmp_path, capsys):
    # Killed by the OOM killer, say: a non-zero exit with nothing on stderr.
    bin_dir = tmp_path / "bin"
    bin_dir.mkdir()
    stub = bin_dir / "aura-filecard"
    stub.write_text("#!/bin/sh\nexit 137\n")
    stub.chmod(0o755)
    monkeypatch.setenv("PATH", f"{bin_dir}:{os.environ['PATH']}")
    path = tmp_path / "fornitori.txt"
    path.write_text("Il fornitore consegna il martedì.\n", encoding="utf-8")

    card = app._card(str(path), "fornitori.txt")

    assert (card.text, card.description) == ("", "")
    assert "[card] fornitori.txt: aura-filecard exited 137" in capsys.readouterr().out
