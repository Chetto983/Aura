"""Drive a stdio MCP server through a scripted session and time each reply.

python3 -I driver.py <script.json> <out.json> -- <argv...>

script.json is a list of {"method": ..., "params": {...}} requests (tools/call etc.).
Each request waits for its reply (timeout 120 s). The transcript (request, reply
truncated, elapsed ms) goes to out.json; images are reduced to their byte length.
"""
import json
import subprocess
import sys
import time

script_path, out_path = sys.argv[1], sys.argv[2]
argv = sys.argv[sys.argv.index("--") + 1:]
steps = json.load(open(script_path, encoding="utf-8"))

proc = subprocess.Popen(argv, stdin=subprocess.PIPE, stdout=subprocess.PIPE, stderr=subprocess.PIPE,
                        text=True, encoding="utf-8", bufsize=1)
transcript = []
next_id = 1


def shrink(value):
    if isinstance(value, dict):
        out = {}
        for k, v in value.items():
            if k == "data" and isinstance(v, str) and len(v) > 200:
                out[k] = f"<{len(v)} base64 chars>"
            else:
                out[k] = shrink(v)
        return out
    if isinstance(value, list):
        return [shrink(v) for v in value]
    if isinstance(value, str) and len(value) > 1500:
        return value[:1500] + f"...<{len(value)} chars>"
    return value


def call(method, params, notify=False):
    global next_id
    msg = {"jsonrpc": "2.0", "method": method}
    if params is not None:
        msg["params"] = params
    if not notify:
        msg["id"] = next_id
        next_id += 1
    start = time.monotonic()
    proc.stdin.write(json.dumps(msg) + "\n")
    proc.stdin.flush()
    if notify:
        return None
    while True:
        line = proc.stdout.readline()
        if not line:
            transcript.append({"method": method, "params": params, "error": "server closed stdout",
                               "ms": round((time.monotonic() - start) * 1000)})
            return None
        try:
            reply = json.loads(line)
        except json.JSONDecodeError:
            transcript.append({"non_json_stdout": line[:300]})
            continue
        if reply.get("id") == msg["id"]:
            transcript.append({"method": method, "params": params, "ms": round((time.monotonic() - start) * 1000),
                               "reply": shrink(reply)})
            return reply


call("initialize", {"protocolVersion": "2025-06-18", "capabilities": {},
                    "clientInfo": {"name": "aura-survey", "version": "0"}})
call("notifications/initialized", None, notify=True)
for step in steps:
    call(step["method"], step.get("params", {}))
proc.stdin.close()
try:
    proc.wait(timeout=30)
except subprocess.TimeoutExpired:
    proc.kill()
stderr = proc.stderr.read()
json.dump({"argv": argv, "exit": proc.returncode, "stderr": stderr[-3000:], "transcript": transcript},
          open(out_path, "w", encoding="utf-8"), indent=1)
for t in transcript:
    if "method" in t:
        r = t.get("reply", {})
        res = r.get("result", {})
        flag = "ERR" if ("error" in r or res.get("isError") or "error" in t) else "ok"
        name = (t.get("params") or {}).get("name", "") if isinstance(t.get("params"), dict) else ""
        print(f"{t['method']:<16} {name:<26} {flag:<4} {t['ms']:>6} ms")
print(f"exit={proc.returncode}")
