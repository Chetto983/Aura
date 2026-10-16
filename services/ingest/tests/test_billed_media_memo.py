"""A billed vision or speech-to-text call runs once per content, and only a new content runs it.

_extract re-runs for every file whenever its code changes, and before this it repeated every
image description and every scanned PDF's OCR with it: media.derive had no memo, and
extract_scanned_pdf's was keyed on the random temporary path the bytes were written to. A
scan was also read again whenever the media route changed. The operator's rule (prd.md §11,
2026-10-16): a file is read again only when it changes.

Here a changed content type stands in for a changed _extract: it re-runs _extract with the
bytes and the name unchanged, through the production memos (media_driver.py). The stub bridge
records every invocation, so a pipeline that asked it for anything else -- the route it would
use, say -- fails the test as surely as one that read the file twice.
"""

import os
import subprocess
import sys

import pytest

_STUB = """#!/bin/sh
echo "$*" >> "$STUB_CALLS"
if [ -n "$STUB_EXIT" ]; then echo "no configured route accepts images" >&2; exit "$STUB_EXIT"; fi
echo "una valvola rossa su un tubo grigio"
"""
# cmd/aura-media-index exitNoVisionRoute.
_NO_VISION_ROUTE = "3"


def _blank_pdf() -> bytes:
    """One empty page and no text layer, which is what sends a PDF to vision."""
    objects = [b"<</Type/Catalog/Pages 2 0 R>>", b"<</Type/Pages/Kids[3 0 R]/Count 1>>",
               b"<</Type/Page/Parent 2 0 R/MediaBox[0 0 612 792]>>"]
    out, offsets = bytearray(b"%PDF-1.4\n"), []
    for number, body in enumerate(objects, 1):
        offsets.append(len(out))
        out += b"%d 0 obj%s endobj\n" % (number, body)
    xref = len(out)
    out += b"xref\n0 %d\n0000000000 65535 f \n" % (len(objects) + 1)
    out += b"".join(b"%010d 00000 n \n" % offset for offset in offsets)
    out += b"trailer<</Size %d/Root 1 0 R>>\nstartxref\n%d\n%%%%EOF\n" % (len(objects) + 1, xref)
    return bytes(out)


@pytest.fixture
def bridge(tmp_path):
    """Run one driver pass; return how many files it extracted and the bridge's calls so far."""
    bin_dir = tmp_path / "bin"
    bin_dir.mkdir()
    stub = bin_dir / "aura-media-index"
    stub.write_text(_STUB)
    stub.chmod(0o755)
    calls = tmp_path / "calls"
    calls.touch()

    def run(path, content_type, exit_code=""):
        env = dict(os.environ, PATH=f"{bin_dir}:{os.environ['PATH']}", STUB_CALLS=str(calls),
                   STUB_EXIT=exit_code, COCOINDEX_DB=str(tmp_path / "coco.db"))
        done = subprocess.run(
            [sys.executable, "-m", "ingest.tests.media_driver", str(path), content_type],
            env=env, capture_output=True, text=True, timeout=300,
        )
        assert done.returncode == 0, done.stdout + done.stderr
        return done.stdout.count("[extract] "), calls.read_text().splitlines()

    return run


def test_an_image_is_described_once_when_extract_runs_again(bridge, tmp_path):
    image = tmp_path / "valvola.png"
    image.write_bytes(b"\x89PNG\r\n\x1a\n not a real image")

    extracted, calls = bridge(image, "image/png")
    assert extracted == 1 and len(calls) == 1 and calls[0].startswith("-kind image -name valvola.png ")
    extracted, calls = bridge(image, "application/octet-stream")

    assert extracted == 1, "the changed content type did not re-run _extract"
    assert len(calls) == 1, f"the image was described again: {calls}"


def test_a_scanned_pdf_is_read_once_when_extract_runs_again(bridge, tmp_path):
    scan = tmp_path / "verbale.pdf"
    scan.write_bytes(_blank_pdf())

    extracted, calls = bridge(scan, "application/pdf")
    assert extracted == 1 and len(calls) == 1 and calls[0].startswith("-kind pdf -name verbale.pdf ")
    extracted, calls = bridge(scan, "application/octet-stream")

    assert extracted == 1, "the changed content type did not re-run _extract"
    assert len(calls) == 1, f"the scan was read again: {calls}"


def test_an_image_no_route_reads_stays_card_only_without_a_retry_each_cycle(bridge, tmp_path):
    # The component succeeds (bridge() asserts the driver's exit code) and _extract keeps the
    # blank, so an unchanged pass asks nothing. The refusal itself is not memoized, so the next
    # re-run of _extract asks the bridge again.
    image = tmp_path / "valvola.png"
    image.write_bytes(b"\x89PNG\r\n\x1a\n not a real image")

    extracted, calls = bridge(image, "image/png", _NO_VISION_ROUTE)
    assert (extracted, len(calls)) == (1, 1)
    extracted, calls = bridge(image, "image/png", _NO_VISION_ROUTE)
    assert (extracted, len(calls)) == (0, 1), "an unchanged pass asked the bridge again"
    extracted, calls = bridge(image, "application/octet-stream", _NO_VISION_ROUTE)

    assert (extracted, len(calls)) == (1, 2), "the refusal was memoized"
