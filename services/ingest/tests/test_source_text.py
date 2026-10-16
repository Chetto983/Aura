"""Source files and .txt are read as text directly, without Tika (prd.md §11, 2026-10-16).

Tika routes on sniffed content, not on the file name. Measured through the production
extractor: a .tsx holding markup went to its HTML parser and lost the code, and a Go test whose
string literal starts a PDF went to the PDF parser and failed the file, the one of Aura's 5,504
source files that indexing left out. These pin the direct read and the rules that keep a binary
file with a source-looking name out of the text index.
"""

import sqlite3

import pytest

from ingest.extract import _LEGACY, _TEXTUAL, extractable, extract_text
from ingest.media import _AUDIO_EXTENSIONS, _IMAGE_EXTENSIONS, _VIDEO_EXTENSIONS
from ingest.source_extensions import source_extensions

GO = 'package nastro\n\n// La temperatura è già oltre la soglia.\nfunc Fermo() string { return "fermo" }\n'
TSX = (
    "export function Page() {\n"
    '  return <html><body><Panel title="Nastro" /></body></html>;\n'
    "}\n"
)
PY = "def fermo():\n    return 'nastro fermo — attenzione'\n"
ST = (
    "FUNCTION_BLOCK FB_Conveyor\nVAR_INPUT\n    xStart : BOOL;\nEND_VAR\n"
    "(* avvio ritardato *)\nIF xStart THEN xRunning := TRUE; END_IF;\nEND_FUNCTION_BLOCK\n"
)
SCL = (
    'FUNCTION_BLOCK "FB_Motor"\n{ S7_Optimized_Access := \'TRUE\' }\nVERSION : 0.1\n'
    "BEGIN\n\t#Running := #Start AND NOT #Stop;\nEND_FUNCTION_BLOCK\n"
)
DB = (
    'DATA_BLOCK "DB_Recipe"\nVERSION : 0.1\n   STRUCT\n      Temperature : Real := 72.5;\n'
    "   END_STRUCT;\nBEGIN\nEND_DATA_BLOCK\n"
)
UDT = 'TYPE "UDT_Valve"\nVERSION : 0.1\n   STRUCT\n      Open : Bool;\n   END_STRUCT;\nEND_TYPE\n'


@pytest.mark.parametrize("name, text", [
    ("nastro.go", GO), ("page.tsx", TSX), ("fermo.py", PY), ("conveyor.st", ST),
    ("conveyor.iecst", ST), ("Motor.scl", SCL), ("Recipe.db", DB), ("Valve.udt", UDT),
])
def test_a_source_file_yields_its_exact_text(tmp_path, name, text):
    path = tmp_path / name
    path.write_bytes(text.encode("utf-8"))
    assert extractable(str(path)), name
    assert extract_text(str(path)) == text


def test_a_text_file_that_starts_like_a_pdf_is_still_text(tmp_path):
    """internal/pdftext/pdftext_test.go, as Aura's own sources were indexed on 2026-10-16: Tika
    typed it as a PDF from the literal in its first bytes and raised TIKA-198 from PDFParser."""
    text = (
        "package pdftext\n\nimport (\n\t\"strings\"\n\t\"testing\"\n)\n\n"
        "func pdfWithPages(texts ...string) string {\n\tvar b strings.Builder\n"
        '\tb.WriteString("%PDF-1.4\\n1 0 obj<</Type/Catalog/Pages 2 0 R>>endobj\\n")\n'
        "\treturn b.String()\n}\n"
    )
    path = tmp_path / "pdftext_test.go.txt"
    path.write_bytes(text.encode("utf-8"))
    assert extract_text(str(path)) == text


def test_a_binary_database_keeps_its_name_only_row(tmp_path):
    """.db is a Siemens data block source, and far more often a SQLite file: its NUL bytes keep it
    out of the text index, and it still gets its row by name."""
    path = tmp_path / "app.db"
    with sqlite3.connect(path) as db:
        db.execute("CREATE TABLE t (x TEXT)")
        db.execute("INSERT INTO t VALUES ('hello')")
    assert extractable(str(path))
    assert extract_text(str(path)) == ""


def test_an_ansi_step7_export_decodes_as_cp1252(tmp_path):
    text = (
        "FUNCTION FC 10 : VOID\nTITLE = Pumpe Störung\nBEGIN\n"
        '      U     "Saugventil_Offen";\n      =     "Pumpe_Ein";\nEND_FUNCTION\n'
    )
    path = tmp_path / "Legacy.awl"
    path.write_bytes(text.encode("cp1252"))
    assert extract_text(str(path)) == text


def test_a_utf16_text_file_with_its_bom_is_text(tmp_path):
    """Its NUL bytes are the encoding's, not a binary's: the BOM decides before the NUL rule."""
    text = "Note di manutenzione: è necessario fermare il nastro.\n"
    path = tmp_path / "note.txt"
    path.write_bytes(text.encode("utf-16"))
    assert extract_text(str(path)) == text


def test_a_utf8_bom_is_not_part_of_the_text(tmp_path):
    """Windows editors save C# and PowerShell with one; kept, it glues itself to the first word."""
    text = "using System;\n// Nastro trasportatore\n"
    path = tmp_path / "Program.cs"
    path.write_bytes(text.encode("utf-8-sig"))
    assert extract_text(str(path)) == text


def test_a_file_neither_encoding_reads_keeps_its_name_only_row(tmp_path):
    """0x81 is a stray UTF-8 continuation byte and has no character in cp1252."""
    path = tmp_path / "blob.bin.c"
    path.write_bytes(b"int x;\x81\x8d\n")
    assert extractable(str(path))
    assert extract_text(str(path)) == ""


def test_source_extensions_route_and_unnamed_files_do_not():
    for name in ["a.c", "b.cpp", "c.cs", "d.java", "e.kt", "f.rs", "g.js", "h.ts", "i.rb",
                 "j.php", "k.swift", "l.sql", "m.ps1", "n.yaml", "o.toml", "p.f90", "q.svelte",
                 "r.zig", "s.sol", "T.SCL", "u.awl", "v.udt", "w.iecst"]:
        assert extractable(name), name
    for name in ["Makefile", "Dockerfile", "notebook.ipynb", "photo.png"]:
        assert not extractable(name), name


def test_the_list_is_the_one_aura_filecard_owns():
    """.css, .cypher, .jsx and .mjs exist only in Go's list (internal/documents/filecard/source.go)."""
    for name in ["main.css", "seed.cypher", "Panel.jsx", "run-speed.mjs"]:
        assert extractable(name), name


def test_source_extensions_take_nothing_another_route_owns():
    owned = _TEXTUAL | set(_LEGACY) | _IMAGE_EXTENSIONS | _AUDIO_EXTENSIONS | _VIDEO_EXTENSIONS
    sources = source_extensions()
    assert sources.isdisjoint(owned), sorted(sources & owned)
    assert all(ext == ext.lower() and ext.count(".") == 1 for ext in sources)
