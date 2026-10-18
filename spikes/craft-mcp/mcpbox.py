"""A stdio MCP client for a server running in an identity's box, through `docker exec -i`.

The server is launched the way Aura's box runtime launches it (cmd/aura/sandbox_mcp_box.go):
inside the identity's box, working directory /workspace. Each call is timed; the server's peak
resident memory is read from /proc before the session closes.
"""
import json
import os
import subprocess
import time

BOX = "aura-box-15d43203-e273-4366-a5af-897083676ced"
ROOTS = ["--automation-read-root", "/workspace", "--automation-write-root", "/workspace"]
SERVERS = {
    "gridcraft": ["gridcraft-cli", "mcp"],
    "pdfcraft": ["pdfcraft-cli", "mcp", "--compact", *ROOTS],
    "wordcraft": ["wordcraft-cli", "mcp", *ROOTS],
    "deckcraft": ["deckcraft-cli", "mcp", *ROOTS],
    "filmcraft": ["filmcraft-cli", "mcp"],
    "photocraft": ["photocraft-cli", "mcp", *ROOTS],
}
# CRAFT_WRAP prefixes every server, e.g. the recipes' landrun fence (internal/mcp/manager/catalog_craft.go).
WRAP = os.environ.get("CRAFT_WRAP", "").split()
SERVERS = {name: WRAP + argv for name, argv in SERVERS.items()}


def box(script, timeout=900):
    """Run a shell script in the box and return (seconds, output).

    The script goes as bytes: in text mode Windows would send CRLF line ends to sh."""
    start = time.monotonic()
    out = subprocess.run(["docker", "exec", "-i", BOX, "sh", "-s"], input=script.encode("utf-8"),
                         capture_output=True, timeout=timeout)
    text = (out.stdout + out.stderr).decode("utf-8", "replace").strip()
    return round(time.monotonic() - start, 2), text


class Session:
    def __init__(self, server):
        self.server = server
        self.argv = SERVERS[server]
        self.proc = subprocess.Popen(["docker", "exec", "-i", "-w", "/workspace", BOX, *self.argv],
                                     stdin=subprocess.PIPE, stdout=subprocess.PIPE, stderr=subprocess.PIPE)
        self.next_id = 1
        self.log = []
        self.request("initialize", {"protocolVersion": "2025-06-18", "capabilities": {},
                                    "clientInfo": {"name": "craft-spike", "version": "0"}})
        self.send({"jsonrpc": "2.0", "method": "notifications/initialized"})

    def send(self, msg):
        self.proc.stdin.write((json.dumps(msg) + "\n").encode("utf-8"))
        self.proc.stdin.flush()

    def request(self, method, params):
        msg = {"jsonrpc": "2.0", "id": self.next_id, "method": method, "params": params}
        self.next_id += 1
        start = time.monotonic()
        self.send(msg)
        while True:
            line = self.proc.stdout.readline()
            if not line:
                raise RuntimeError(f"{self.server} closed stdout during {method}")
            try:
                reply = json.loads(line)
            except json.JSONDecodeError:
                continue
            if reply.get("id") == msg["id"]:
                return reply, round((time.monotonic() - start) * 1000)

    def call(self, tool, **args):
        """Call a tool; return (text, images, is_error). The log keeps every call, failures included."""
        reply, ms = self.request("tools/call", {"name": tool, "arguments": args})
        result = reply.get("result") or {}
        text = "".join(c.get("text", "") for c in result.get("content", []) if c.get("type") == "text")
        images = [len(c.get("data", "")) for c in result.get("content", []) if c.get("type") == "image"]
        error = bool(result.get("isError")) or "error" in reply
        if "error" in reply:
            text = json.dumps(reply["error"])
        self.log.append({"tool": tool, "args": args, "ms": ms, "error": error, "text": text[:600],
                         "image_b64_chars": images})
        return text, images, error

    def json(self, tool, **args):
        text, _, error = self.call(tool, **args)
        if error:
            return None
        try:
            return json.loads(text)
        except json.JSONDecodeError:
            return text

    def peak_kb(self):
        # The box has neither pgrep nor ps.
        _, out = box(f"for d in /proc/[0-9]*; do tr '\\0' ' ' < $d/cmdline 2>/dev/null | grep -q '^{self.argv[0]} mcp' "
                     f"&& grep VmHWM $d/status; done", 30)
        values = [int(v) for v in out.split() if v.isdigit()]
        return max(values) if values else None

    def close(self):
        peak = self.peak_kb()
        self.proc.stdin.close()
        try:
            self.proc.wait(timeout=30)
        except subprocess.TimeoutExpired:
            self.proc.kill()
        return peak
