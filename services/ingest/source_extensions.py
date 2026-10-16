"""Source-file extensions the ingest reads as text (prd.md §11, 2026-10-16).

The list is Go's, in internal/documents/filecard/source.go, where the upload gate and the card
read it too. This module asks the aura-filecard binary beside it in this image for it, once, so
there is one copy and no parity to keep.
"""

import functools
import json
import subprocess


@functools.cache
def source_extensions() -> frozenset[str]:
    done = subprocess.run(
        ["aura-filecard", "-source-extensions"], check=True, capture_output=True, timeout=30,
    )
    return frozenset(json.loads(done.stdout))
