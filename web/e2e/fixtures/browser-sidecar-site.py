"""Synthetic-only download fixture; run --self-test before publishing temporarily."""

import argparse
import hashlib
import http.server
import threading
import time
import urllib.request

CHUNK = bytes(128 * 1024)
CHUNKS = 64
SIZE = len(CHUNK) * CHUNKS
SHA256 = "2daeb1f36095b44b318410b3f4e8b5d989dcc7bb023d1426c492dab0a3053e74"
PAGE = b'<!doctype html><title>Browser sidecar fixture</title><h1>Synthetic download</h1><a id="download" href="/browser-sidecar.bin" download>Download 8 MiB</a>'


class Handler(http.server.BaseHTTPRequestHandler):
    def log_message(self, *_args):
        pass

    def do_GET(self):
        if self.path == "/":
            body, content_type = PAGE, "text/html; charset=utf-8"
        elif self.path == "/browser-sidecar.bin":
            body, content_type = None, "application/octet-stream"
        else:
            self.send_error(404)
            return
        self.send_response(200)
        self.send_header("Content-Type", content_type)
        self.send_header("Content-Length", str(len(body) if body is not None else SIZE))
        self.send_header("Cache-Control", "no-store")
        self.send_header("X-Content-Type-Options", "nosniff")
        if body is None:
            self.send_header("Content-Disposition", 'attachment; filename="browser-sidecar.bin"')
        self.end_headers()
        try:
            if body is not None:
                self.wfile.write(body)
                return
            for _ in range(CHUNKS):
                time.sleep(0.125)
                self.wfile.write(CHUNK)
                self.wfile.flush()
        except (BrokenPipeError, ConnectionResetError):
            pass


def self_test():
    with http.server.ThreadingHTTPServer(("127.0.0.1", 0), Handler) as server:
        worker = threading.Thread(target=server.serve_forever, daemon=True)
        worker.start()
        try:
            origin = f"http://127.0.0.1:{server.server_port}"
            with urllib.request.urlopen(origin, timeout=5) as response:
                assert b'id="download"' in response.read()
            started = time.monotonic()
            with urllib.request.urlopen(origin + "/browser-sidecar.bin", timeout=15) as response:
                assert response.headers["Content-Disposition"].startswith("attachment;")
                assert int(response.headers["Content-Length"]) == SIZE
                payload = response.read()
            elapsed = time.monotonic() - started
            assert len(payload) == SIZE
            assert hashlib.sha256(payload).hexdigest() == SHA256
            assert 8 <= elapsed < 13, elapsed
            for path, method, expected in [("/not-found", "GET", 404), ("/", "POST", 501)]:
                try:
                    urllib.request.urlopen(urllib.request.Request(origin + path, method=method))
                    raise AssertionError("unexpected accepted request")
                except urllib.error.HTTPError as error:
                    assert error.code == expected
            print(f"PASS bytes={SIZE} sha256={SHA256} duration_sec={elapsed:.3f}")
        finally:
            server.shutdown()
            worker.join()


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--bind", default="127.0.0.1")
    parser.add_argument("--port", type=int, default=8765)
    parser.add_argument("--self-test", action="store_true")
    args = parser.parse_args()
    if args.self_test:
        self_test()
    else:
        with http.server.ThreadingHTTPServer((args.bind, args.port), Handler) as server:
            server.serve_forever()
