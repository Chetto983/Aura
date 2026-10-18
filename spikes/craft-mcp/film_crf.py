"""FilmCraft export with an explicit quality: the default wrote 97 MB for a 52 s 720p trailer."""
import json
import time

from mcpbox import Session, box

s = Session("filmcraft")
item = s.json("media_import", paths=["/workspace/craft-spike/real/sintel-trailer-1080p.mp4"])["items"][0]
s.json("command_run", id="file.newSequence", params={"name": "Trailer", "fromItem": item})
t = time.monotonic()
out = s.json("command_run", id="file.exportMedia", params={
    "path": "/workspace/craft-spike/out/sintel-720-crf23.mp4", "format": "h264", "width": 1280, "height": 544,
    "bitrateMode": "crf", "crf": 23, "wait": True})
print("export", round(time.monotonic() - t, 1), "s", json.dumps(out)[:400])
print("peak_kb", s.close())
print(box("ls -l /workspace/craft-spike/out/sintel-720-crf23.mp4")[1])
