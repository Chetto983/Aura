"""Spike: give every workbook row-group passages, Rag_Full_Version's key-value serialization.

Usage: spike_kv_rows.py DATABASE SPACE CORPUS_DIR [CORPUS_DIR ...]
Rows are grouped 15 at a time with 2 rows of overlap, each group headed by its sheet and
written as "Riga N: Header: value; Header: value". Writes into a COPY of the tenant.
"""
import base64
import hashlib
import json
import pathlib
import sys
import urllib.request

import openpyxl

database, space, dirs = sys.argv[1], sys.argv[2], sys.argv[3:]
ARCADE, EMBED = "http://127.0.0.1:2480", "http://127.0.0.1:8081"
AUTH = "Basic " + base64.b64encode(b"root:benchpass123").decode()
GROUP, OVERLAP = 15, 2


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


def text(value):
    if value is None:
        return ""
    if isinstance(value, float) and value.is_integer():
        return str(int(value))
    return str(value).strip()


def header_row(rows):
    """The first row with at least two non-empty cells, all of them text."""
    for index, row in enumerate(rows):
        cells = [c for c in row if c not in (None, "")]
        if len(cells) >= 2 and all(isinstance(c, str) for c in cells):
            return index
    return None


def groups(path):
    workbook = openpyxl.load_workbook(path, data_only=True, read_only=True)
    for sheet in workbook.worksheets:
        rows = [list(r) for r in sheet.iter_rows(values_only=True)]
        head = header_row(rows)
        if head is None:
            continue
        headers = [text(h) or f"Colonna {i + 1}" for i, h in enumerate(rows[head])]
        body = [r for r in rows[head + 1:] if any(c not in (None, "") for c in r)]
        start = 0
        while start < len(body):
            lines = [f"Foglio: {sheet.title}"]
            for number, row in enumerate(body[start:start + GROUP], start=start + 1):
                parts = [f"{headers[i]}: {text(v)}" for i, v in enumerate(row[:len(headers)]) if text(v)]
                if parts:
                    lines.append(f"Riga {number}: " + "; ".join(parts))
            yield sheet.title, "\n".join(lines)
            if start + GROUP >= len(body):
                break
            start += GROUP - OVERLAP


files = {p.name: p for d in dirs for p in pathlib.Path(d).glob("*.xlsx")}
documents = sql("SELECT search_document_id, source_key, raw_sha256, source_kind FROM IndexedDocument "
                "WHERE source_key LIKE '%.xlsx'")
insert = ("INSERT INTO Passage SET passage_key = :k, search_document_id = :d, source_kind = :sk, "
          "source_key = :key, raw_sha256 = :raw, schema_version = :sv, ordinal = :o, text = :t, "
          "normalized_text_sha256 = :nh, heading_path = :hp, char_start = :cs, char_end = :ce, "
          "embedding = :e, embed_space = :s")
version = "document-v1:standard-analyzer:cosine:none:768"
total = 0
for doc in documents:
    pieces = list(groups(files[doc["source_key"]]))
    offset = 0
    for start in range(0, len(pieces), 16):
        batch = pieces[start:start + 16]
        vectors = post(f"{EMBED}/v1/embeddings", {"input": ["title: none | text: " + t for _, t in batch]},
                       auth=False)["data"]
        for ordinal, ((sheet, body), item) in enumerate(zip(batch, vectors, strict=True), start=start):
            sql(insert, {"k": f"{doc['search_document_id']}:kv{ordinal}", "d": doc["search_document_id"],
                         "sk": doc["source_kind"], "key": doc["source_key"], "raw": doc["raw_sha256"],
                         "sv": version, "o": 1000 + ordinal, "t": body,
                         "nh": hashlib.sha256(body.encode()).hexdigest(), "hp": [sheet],
                         "cs": offset, "ce": offset + len(body), "e": item["embedding"], "s": space})
            offset += len(body)
    sql("UPDATE IndexedDocument SET passage_count = :n WHERE search_document_id = :d",
        {"n": len(pieces), "d": doc["search_document_id"]})
    total += len(pieces)
    print(f"{doc['source_key']:<36} {len(pieces):>4} row-group passages", flush=True)
print(f"{total} passages written for {len(documents)} workbooks")
