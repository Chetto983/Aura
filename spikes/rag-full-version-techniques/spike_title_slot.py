"""Spike: re-embed every passage with its file name in EmbeddingGemma's title slot.

Usage: spike_title_slot.py DATABASE
The model card's document prompt is `title: {title | "none"} | text: {content}` and says a real
title improves document embeddings; Aura sends `title: none`. Text and full-text are untouched,
so only the dense leg can move. Writes into a COPY of the tenant.
"""
import base64
import json
import pathlib
import sys
import urllib.request

database = sys.argv[1]
ARCADE, EMBED = "http://127.0.0.1:2480", "http://127.0.0.1:8081"
AUTH = "Basic " + base64.b64encode(b"root:benchpass123").decode()


def post(url, body, auth=True):
    headers = {"Content-Type": "application/json"}
    if auth:
        headers["Authorization"] = AUTH
    request = urllib.request.Request(url, data=json.dumps(body).encode(), headers=headers)
    with urllib.request.urlopen(request, timeout=600) as response:
        return json.loads(response.read())


def sql(command, params=None):
    return post(f"{ARCADE}/api/v1/command/{database}",
                {"language": "sql", "command": command, "params": params or {}})["result"]


rows = sql("SELECT @rid AS rid, source_key, text FROM Passage")
for start in range(0, len(rows), 16):
    batch = rows[start:start + 16]
    inputs = [f"title: {pathlib.PurePosixPath(r['source_key']).name} | text: {r['text']}" for r in batch]
    vectors = post(f"{EMBED}/v1/embeddings", {"input": inputs}, auth=False)["data"]
    for row, item in zip(batch, vectors, strict=True):
        sql(f"UPDATE {row['rid']} SET embedding = :e", {"e": item["embedding"]})
print(f"re-embedded {len(rows)} passages with their file name as title")
