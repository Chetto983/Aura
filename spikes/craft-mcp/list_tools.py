"""Write each server's full tools/list, as the box serves it, to tools/<server>.json.

python -I list_tools.py <box container>
"""
import json
import subprocess
import sys

BOX = sys.argv[1]
ROOTS = ["--automation-read-root", "/workspace", "--automation-write-root", "/workspace"]
SERVERS = {
    "gridcraft": ["gridcraft-cli", "mcp"],
    "pdfcraft": ["pdfcraft-cli", "mcp", "--compact", *ROOTS],
    "wordcraft": ["wordcraft-cli", "mcp", *ROOTS],
    "deckcraft": ["deckcraft-cli", "mcp", *ROOTS],
    "filmcraft": ["filmcraft-cli", "mcp"],
    "photocraft": ["photocraft-cli", "mcp", *ROOTS],
}
for name, argv in SERVERS.items():
    proc = subprocess.Popen(["docker", "exec", "-i", "-w", "/workspace", BOX, *argv], stdin=subprocess.PIPE,
                            stdout=subprocess.PIPE, stderr=subprocess.DEVNULL, text=True, encoding="utf-8")
    msgs = [{"jsonrpc": "2.0", "id": 1, "method": "initialize", "params": {"protocolVersion": "2025-06-18",
             "capabilities": {}, "clientInfo": {"name": "craft-spike", "version": "0"}}},
            {"jsonrpc": "2.0", "method": "notifications/initialized"},
            {"jsonrpc": "2.0", "id": 2, "method": "tools/list", "params": {}}]
    out, _ = proc.communicate("\n".join(json.dumps(m) for m in msgs) + "\n", timeout=60)
    tools = next(json.loads(line) for line in out.splitlines() if '"id":2' in line.replace(" ", ""))["result"]["tools"]
    json.dump(tools, open(f"tools/{name}.json", "w", encoding="utf-8"), indent=1)
    print(name, len(tools), "tools,", len(json.dumps(tools)), "bytes of schema")
