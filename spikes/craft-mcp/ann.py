import json, pathlib
for f in sorted(pathlib.Path("tools").glob("*.json")):
    tools = json.loads(f.read_text(encoding="utf-8"))
    hints = [(x["name"], {k: v for k, v in (x.get("annotations") or {}).items() if k.endswith("Hint")}) for x in tools]
    print(f.stem, len(tools),
          "destructive:", [a for a, h in hints if h.get("destructiveHint")],
          "| readOnly:", [a for a, h in hints if h.get("readOnlyHint")],
          "| no hints:", [a for a, h in hints if not h])
