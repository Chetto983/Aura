"""A stand-in embedding server for the re-run test: deterministic, countable, and able to
refuse one input in one space. It can also answer /props and hold each request open, so a test
can see how many it served at once.

A vector depends on the space as well as on the input, so a row whose stamp and vector
disagree fails the test's comparison.
"""

import contextlib
import hashlib
import io
import json
import math
import os
import threading
import time
import urllib.error
import urllib.request

REFUSED = "REFUSED-IN-THIS-SPACE"

_requests = 0
_props_requests = 0
_serving = 0
_peak = 0
_serving_lock = threading.Lock()
_real_urlopen = urllib.request.urlopen


def vector(space: str, sent: str, dimensions: int) -> list[float]:
    digest = hashlib.sha256(f"{space}\x00{sent}".encode()).digest()
    raw = [float(digest[i % len(digest)]) + 1.0 for i in range(dimensions)]
    norm = math.sqrt(sum(value * value for value in raw))
    return [value / norm for value in raw]


def requests() -> int:
    return _requests


def props_requests() -> int:
    return _props_requests


def peak() -> int:
    """The most /v1/embeddings requests served at once; this and both counts start at install()."""
    return _peak


def install(props: tuple[int, bytes] | None = None, hold_s: float = 0.0) -> None:
    """Answer /v1/embeddings in this process, holding each one open for hold_s, and /props with
    props as (status, body) when given; the tokenizer, and /props otherwise, go to the real
    sidecar."""
    from ingest import embed

    global _requests, _props_requests, _peak
    _requests, _props_requests, _peak = 0, 0, 0

    def urlopen(req, *args, **kwargs):
        global _props_requests
        if req.full_url.endswith("/props"):
            _props_requests += 1
            if props is not None:
                status, body = props
                if status != 200:
                    raise urllib.error.HTTPError(req.full_url, status, "refused", None, io.BytesIO(body))
                return io.BytesIO(body)
        if not req.full_url.endswith("/v1/embeddings"):
            return _real_urlopen(req, *args, **kwargs)
        with _served():
            time.sleep(hold_s)
        inputs = json.loads(req.data)["input"]
        if os.environ.get("SPACE_THAT_REFUSES") == embed.SPACE and any(REFUSED in text for text in inputs):
            raise urllib.error.HTTPError(req.full_url, 400, "Bad Request", None, io.BytesIO(b"refused"))
        data = [{"index": i, "embedding": vector(embed.SPACE, text, embed.DIMENSIONS)}
                for i, text in enumerate(inputs)]
        return io.BytesIO(json.dumps({"data": data}).encode())

    urllib.request.urlopen = urlopen


@contextlib.contextmanager
def _served():
    global _requests, _serving, _peak
    with _serving_lock:
        _requests += 1
        _serving += 1
        _peak = max(_peak, _serving)
    try:
        yield
    finally:
        with _serving_lock:
            _serving -= 1
