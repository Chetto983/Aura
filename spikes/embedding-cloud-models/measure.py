"""Dense passage recall of hosted embedding models against the lab VM's EmbeddingGemma.

The corpus is the 86 passages the VM's ingest wrote for one tenant, with their stored vectors;
the questions are questions.json, each with a string that a correct passage contains. Every
model gets the same input Aura sends on a hosted route: the EmbeddingGemma prefixes, and
`dimensions: 768`. A model that refuses `dimensions` is measured at its native width instead,
which is what a space that follows the model's width would store; a wider answer to 768 is
cut and renormalized by Aura's TruncateMRL rule.

Usage: OPENROUTER_API_KEY=... python3 -I measure.py [model ...]
Inputs, none of them committed: the dump dump_passages.py writes and baseline.json from
baseline_vm.py (see README.md). Embeddings are cached per model under cache/, so a rerun costs
nothing.
"""
import json, math, os, re, statistics, sys, time, urllib.error, urllib.request

HERE = os.path.dirname(os.path.abspath(__file__))
DUMP = os.path.join(HERE, "dump-mem_448ddbe1_96ea_405d_8219_4a3d52a425c0.json")
API = "https://openrouter.ai/api/v1"
QUERY_PREFIX = "task: search result | query: "
DOC_PREFIX = "title: none | text: "
WIDTH = 768
BATCH = 8
MODELS = [
    "qwen/qwen3-embedding-8b", "qwen/qwen3-embedding-4b",
    "perplexity/pplx-embed-v1-0.6b", "perplexity/pplx-embed-v1-4b",
    "google/gemini-embedding-001", "google/gemini-embedding-2",
    "openai/text-embedding-3-small", "openai/text-embedding-3-large",
    "mistralai/mistral-embed-2312", "baai/bge-m3",
    "voyageai/voyage-4-lite", "voyageai/voyage-4", "voyageai/voyage-4-large",
    "nvidia/nemotron-3-embed-1b:free", "nvidia/llama-nemotron-embed-vl-1b-v2:free",
]


def norm_text(s):
    return re.sub(r"\s+", " ", s).lower()


def cosine(a, b):
    dot = sum(x * y for x, y in zip(a, b))
    na = math.sqrt(sum(x * x for x in a))
    nb = math.sqrt(sum(y * y for y in b))
    return dot / (na * nb) if na and nb else 0.0


def truncate_mrl(vector, width):
    if len(vector) <= width:
        return vector
    head = vector[:width]
    n = math.sqrt(sum(x * x for x in head))
    return [x / n for x in head]


def post(model, inputs, dimensions):
    body = {"model": model, "input": inputs}
    if dimensions:
        body["dimensions"] = dimensions
    req = urllib.request.Request(
        API + "/embeddings", data=json.dumps(body).encode(),
        headers={"Authorization": "Bearer " + os.environ["OPENROUTER_API_KEY"],
                 "Content-Type": "application/json"})
    for attempt in range(5):
        try:
            t0 = time.time()
            raw = urllib.request.urlopen(req, timeout=180).read()
            elapsed = time.time() - t0
            data = json.loads(raw)
            if "data" not in data:
                raise RuntimeError(f"no data: {str(data)[:300]}")
            vectors = [d["embedding"] for d in sorted(data["data"], key=lambda d: d["index"])]
            return vectors, data.get("usage") or {}, elapsed
        except urllib.error.HTTPError as err:
            detail = err.read().decode("utf-8", "replace")[:400]
            if err.code == 429 or err.code >= 500:
                time.sleep(5 * (attempt + 1))
                continue
            raise RuntimeError(f"HTTP {err.code}: {detail}") from None
    raise RuntimeError("gave up after 5 attempts (429/5xx)")


def embed_model(model, passages, questions):
    """Vectors for every passage and question, the mode used, usage and single-query latency."""
    path = os.path.join(HERE, "cache", model.replace("/", "__").replace(":", "_") + ".json")
    if os.path.exists(path):
        return json.load(open(path, encoding="utf-8"))
    mode, dims = "dimensions=768", WIDTH
    usage = {"prompt_tokens": 0, "cost": 0.0}
    try:
        first, u, _ = post(model, [DOC_PREFIX + passages[0]], dims)
    except RuntimeError as err:
        if "dimension" not in str(err).lower():
            raise
        mode, dims = "native (refuses dimensions)", None
        first, u, _ = post(model, [DOC_PREFIX + passages[0]], dims)
    native_width = len(first[0])

    def add(u):
        usage["prompt_tokens"] += u.get("prompt_tokens") or u.get("total_tokens") or 0
        usage["cost"] += float(u.get("cost") or 0)

    add(u)
    docs = list(first)
    for start in range(1, len(passages), BATCH):
        vectors, u, _ = post(model, [DOC_PREFIX + p for p in passages[start:start + BATCH]], dims)
        docs.extend(vectors)
        add(u)
    queries, u, _ = post(model, [QUERY_PREFIX + q["q"] for q in questions], dims)
    add(u)
    single = []
    for q in questions[:3]:
        _, u, elapsed = post(model, [QUERY_PREFIX + q["q"]], dims)
        single.append(elapsed)
        add(u)
    result = {"model": model, "mode": mode, "returned_width": native_width, "docs": docs,
              "queries": queries, "usage": usage, "single_query_s": single}
    os.makedirs(os.path.dirname(path), exist_ok=True)
    json.dump(result, open(path, "w", encoding="utf-8"))
    return result


def score(doc_vectors, query_vectors, texts, questions):
    ranks = []
    for q, qv in zip(questions, query_vectors):
        truth = norm_text(q["truth"])
        order = sorted(range(len(doc_vectors)), key=lambda i: -cosine(qv, doc_vectors[i]))
        ranks.append(next((r for r, i in enumerate(order) if truth in texts[i]), None))
    n = len(ranks)
    at = lambda k: sum(1 for r in ranks if r is not None and r < k)
    mrr = sum(1 / (r + 1) for r in ranks if r is not None) / n
    return {"r1": at(1), "r3": at(3), "r5": at(5), "n": n, "mrr": mrr, "ranks": ranks}


def main():
    dump = json.load(open(DUMP, encoding="utf-8"))
    questions = json.load(open(os.path.join(HERE, "questions.json"), encoding="utf-8"))
    baseline = json.load(open(os.path.join(HERE, "baseline.json"), encoding="utf-8"))
    passages = [p["text"] for p in dump["passages"]]
    texts = [norm_text(t) for t in passages]
    stored = [p["embedding"] for p in dump["passages"]]

    sample_idx = json.load(open(os.path.join(HERE, "samples-idx.json")))
    drift = [cosine(stored[i], v) for i, v in zip(sample_idx, baseline["docs"])]
    print(f"stored vs fresh sidecar vectors, cosine: {[round(x, 5) for x in drift]}")

    rows = []
    local = score(stored, baseline["queries"], texts, questions)
    local.update(model="local embeddinggemma-300M-Q8_0", mode="local", width=WIDTH, cost=0.0,
                 tokens=0, latency=statistics.median(baseline["single_query_s"]))
    rows.append(local)

    wanted = sys.argv[1:] or MODELS
    if "OPENROUTER_API_KEY" not in os.environ:
        wanted = []
        print("OPENROUTER_API_KEY not set: only the local baseline runs")
    for model in wanted:
        try:
            r = embed_model(model, passages, questions)
        except RuntimeError as err:
            rows.append({"model": model, "error": str(err)})
            print(f"{model}: {err}")
            continue
        docs = [truncate_mrl(v, WIDTH) if r["mode"].startswith("dimensions") else v for v in r["docs"]]
        qs = [truncate_mrl(v, WIDTH) if r["mode"].startswith("dimensions") else v for v in r["queries"]]
        s = score(docs, qs, texts, questions)
        s.update(model=model, mode=r["mode"], width=len(docs[0]), returned=r["returned_width"],
                 cost=r["usage"]["cost"], tokens=r["usage"]["prompt_tokens"],
                 latency=statistics.median(r["single_query_s"]))
        rows.append(s)
        print(f"{model}: r@1 {s['r1']}/{s['n']} r@3 {s['r3']} r@5 {s['r5']} mrr {s['mrr']:.3f}")

    json.dump(rows, open(os.path.join(HERE, "results.json"), "w", encoding="utf-8"), indent=1)
    groups = sorted({q["doc"] for q in questions})
    with open(os.path.join(HERE, "results.txt"), "w", encoding="utf-8") as out:
        out.write("model | mode | width | r@1 | r@3 | r@5 | MRR | Costituzione r@1 | others r@1 | query s | tokens | cost $\n")
        for r in rows:
            if "error" in r:
                out.write(f"{r['model']} | ERROR {r['error'][:160]}\n")
                continue
            const_hits = sum(1 for q, k in zip(questions, r["ranks"]) if q["doc"] == "costituzione" and k == 0)
            ncost = sum(1 for q in questions if q["doc"] == "costituzione")
            other = sum(1 for q, k in zip(questions, r["ranks"]) if q["doc"] != "costituzione" and k == 0)
            nother = len(questions) - ncost
            out.write(f"{r['model']} | {r['mode']} | {r['width']} | {r['r1']}/{r['n']} | {r['r3']}/{r['n']} | "
                      f"{r['r5']}/{r['n']} | {r['mrr']:.3f} | {const_hits}/{ncost} | {other}/{nother} | "
                      f"{r['latency']:.2f} | {r['tokens']} | {r['cost']:.5f}\n")
        out.write("\nper-question rank of the first correct passage (0 = top; - = not in the 86):\n")
        for i, q in enumerate(questions):
            cells = ["-" if "error" in r or r["ranks"][i] is None else str(r["ranks"][i]) for r in rows]
            out.write(f"{i:2} {q['doc'][:12]:12} {' '.join(f'{c:>3}' for c in cells)}  {q['q'][:60]}\n")
    print(open(os.path.join(HERE, "results.txt"), encoding="utf-8").read())


if __name__ == "__main__":
    main()
