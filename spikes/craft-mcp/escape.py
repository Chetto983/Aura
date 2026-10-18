"""Ask the two servers without roots to write outside /workspace, through their own MCP tools.

CRAFT_WRAP="landlock -rw /workspace -rw /dev --" python escape.py   (and once without, to compare)
"""
import json

from mcpbox import Session, box

g = Session("gridcraft")
print("gridcraft new:", g.call("new_workbook")[0][:80])
for path in ["/tmp/escape.xlsx", "/root/escape.xlsx", "/workspace/inside.xlsx"]:
    text, _, err = g.call("save_workbook", path=path)
    print(f"gridcraft save {path}: {'ERR' if err else 'ok'} {text[:160]!r}")
g.close()

f = Session("filmcraft")
for path in ["/tmp/escape.fcproj", "/workspace/inside.fcproj"]:
    text, _, err = f.call("command_run", id="file.saveAs", params={"path": path})
    print(f"filmcraft save {path}: {'ERR' if err else 'ok'} {text[:160]!r}")
f.close()
print(box("ls -la /tmp/escape.xlsx /root/escape.xlsx /tmp/escape.fcproj /workspace/inside.xlsx /workspace/inside.fcproj 2>&1; rm -f /tmp/escape.* /root/escape.xlsx /workspace/inside.*")[1])
