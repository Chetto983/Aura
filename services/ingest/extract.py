"""Text extraction, and the door for the formats Aura could not open at all.

iscc-tika (Apache Tika via a GraalVM native image) is the extractor: measured on a
real corpus it handled 15 formats of 16 with zero failures, 18x faster than Docling
on the same PDF. It is the maintained fork of extractous, which has not been pushed
since 2024-12-21. NEVER import both in one process -- two native images collide with
NoSuchMethodError ... TesseractOCRConfig.setSkipOcr (scripts/ingest_image_contract_test.sh
asserts extractous stays out of sys.modules for this reason).

LibreOffice is here for a narrower reason: internal/documents/filecard/build.go routes
only .xlsx and .xlsm, so an ordinary 2010 .xls has no way into Aura's ETL path at all.
`soffice --convert-to` preserves what filecard needs -- banner rows, the real header
below them, and numeric cells that stay numeric.

Source files and .txt bypass both (prd.md §11, 2026-10-16): Tika routes on sniffed content,
so a .tsx holding markup lost its code to the HTML parser and a Go test opening with a %PDF
literal failed in the PDF parser. They are plain text, and _read_text reads them as such.
"""

import codecs
import contextlib
import pathlib
import subprocess
import tempfile

from iscc_tika import Extractor

from ingest.source_extensions import source_extensions

# Everything LibreOffice can turn into the OOXML that BOTH consumers understand.
#
# It started as the three legacy Microsoft formats, for extraction only. The ODF family
# and .rtf are here now because of what a card looked like without them: MEASURED
# 2026-08-08 across the fixture set, .doc/.xls/.ppt/.odt/.ods/.odp/.rtf each produced
# "sample.ods -- file, 12 KB" and nothing else, because filecard routes .xlsx/.docx/.pptx
# and falls through to a bare size for anything else. Converting first means one
# spreadsheet gets the same card whether it arrived as .xlsx, .xls or .ods.
_LEGACY = {
    ".doc": "docx", ".xls": "xlsx", ".ppt": "pptx",
    ".odt": "docx", ".ods": "xlsx", ".odp": "pptx",
    ".rtf": "docx",
}

# This is a builder returning a NEW extractor, not a mutating setter:
# https://github.com/iscc/iscc-tika#python. Discarding it truncated the
# 1,133-page ArcadeDB manual to 500,000 of its 1,944,898 characters.
_extractor = Extractor().set_extract_string_max_length(50_000_000)


# The formats the extractor is PROVEN on, from the measured matrix in this module's
# docstring plus everything LibreOffice normalises into them.
#
# Routing on this set is the whole fix for a live failure: Tika raises on a format it
# cannot parse -- `.png` gives "image/png parse error" -- and the raise killed the whole
# CocoIndex component, so ONE holiday photo in a bucket stopped that file being indexed at
# all. Deciding before the call, rather than catching after it, is the same shape the
# library's own image example uses (a path matcher choosing the processor per file type),
# and it keeps a genuine I/O failure loud instead of swallowing it in a bare except.
#
# A file outside this set is NOT skipped: it still gets a row, so it is searchable by name
# and openable. That is Aura's own rule, already stated where the card is decoded -- "a
# document that cannot be described still exists, is still searchable by name, and can
# still be opened" -- and it is why a photo belongs in the index with no text rather than
# outside it.
_TEXTUAL = {
    ".pdf", ".docx", ".xlsx", ".xlsm", ".pptx", ".txt", ".md", ".markdown",
    ".csv", ".tsv", ".json", ".xml", ".html", ".htm", ".epub", ".rst",
} | set(_LEGACY)

def _plain_text(suffix: str) -> bool:
    return suffix == ".txt" or suffix in source_extensions()


def needs_normalisation(path: str) -> bool:
    return pathlib.Path(path).suffix.lower() in _LEGACY


def extractable(path: str) -> bool:
    """Whether this file has text to read at all. See _TEXTUAL and _plain_text."""
    suffix = pathlib.Path(path).suffix.lower()
    return suffix in _TEXTUAL or _plain_text(suffix)


def _read_text(path: str) -> str:
    """A plain-text file's text, or "" when its bytes are not text whatever its name says.

    A NUL byte outside UTF-16 marks a binary -- a .db is a Siemens data block source and far
    more often SQLite -- and "" keeps such a file's row by name, like any file without text.
    cp1252 follows UTF-8 because STEP 7 exports and older Windows sources often carry it.
    """
    data = pathlib.Path(path).read_bytes()
    if data.startswith((codecs.BOM_UTF16_LE, codecs.BOM_UTF16_BE)):
        encodings = ("utf-16",)
    elif b"\x00" in data:
        return ""
    else:
        encodings = ("utf-8-sig", "cp1252")
    for encoding in encodings:
        try:
            return data.decode(encoding)
        except UnicodeDecodeError:
            continue
    return ""


def normalise(path: str, outdir: str) -> str:
    """Convert a legacy Office file to its OOXML equivalent. Returns the new path."""
    target = _LEGACY[pathlib.Path(path).suffix.lower()]
    subprocess.run(
        ["soffice", "--headless", "--convert-to", target, "--outdir", outdir, path],
        check=True, capture_output=True, timeout=300,
    )
    out = pathlib.Path(outdir) / (pathlib.Path(path).stem + "." + target)
    if not out.exists():
        raise RuntimeError(f"soffice produced no {target} for {path}")
    return str(out)


@contextlib.contextmanager
def prepared(path: str):
    """Yield the path every consumer should read: OOXML, converting once if needed.

    Both consumers want the same converted file -- the extractor for its text and
    aura-filecard for its card -- and converting twice would mean running LibreOffice
    twice on the same bytes for one document. Yielding the path instead of hiding the
    conversion inside extract_text is what lets the card share it.
    """
    if not needs_normalisation(path):
        yield path
        return
    with tempfile.TemporaryDirectory() as tmp:
        yield normalise(path, tmp)


def extract_text(path: str) -> str:
    """Extract plain text: plain-text files as they are, documents through Tika."""
    # An empty file has no text, and the extractor refuses it ("InputStream must have > 0
    # bytes"): raising failed the file on every cycle and left it without a row (lab VM,
    # 2026-09-24, an empty prompt.txt, 59 failures an hour).
    if pathlib.Path(path).stat().st_size == 0:
        return ""
    if _plain_text(pathlib.Path(path).suffix.lower()):
        return _read_text(path)
    with prepared(path) as ready:
        text, metadata = _extractor.extract_file_to_string(ready)
        if "true" in metadata.get("X-TIKA:EXCEPTION:write_limit_reached", []):
            raise RuntimeError(f"document text extraction truncated at its write limit: {path}")
        return text
