# Runs inside aura-ingest on the lab VM, read-only. Embeds the question set with the local
# EmbeddingGemma sidecar exactly as Aura embeds a query, and re-embeds three stored passages as
# Aura embeds a document, so the stored vectors can be checked against the sidecar's answer.
# Usage: python3 baseline_vm.py questions.json passages.json   (JSON on stdout)
import json, sys, time, urllib.request

SIDECAR = "http://aura-llama-embed:8081/v1/embeddings"
QUERY_PREFIX = "task: search result | query: "
DOC_PREFIX = "title: none | text: "


def embed(texts):
    req = urllib.request.Request(
        SIDECAR, data=json.dumps({"input": texts, "model": "embeddinggemma"}).encode(),
        headers={"Content-Type": "application/json"})
    body = json.loads(urllib.request.urlopen(req, timeout=600).read())
    return [item["embedding"] for item in sorted(body["data"], key=lambda d: d["index"])]


questions = json.load(open(sys.argv[1], encoding="utf-8"))
samples = json.load(open(sys.argv[2], encoding="utf-8"))
t0 = time.time()
queries = embed([QUERY_PREFIX + q["q"] for q in questions])
single = []
for q in questions[:3]:
    t = time.time()
    embed([QUERY_PREFIX + q["q"]])
    single.append(time.time() - t)
docs = embed([DOC_PREFIX + s for s in samples])
print(json.dumps({"queries": queries, "docs": docs, "single_query_s": single,
                  "total_s": time.time() - t0}))
