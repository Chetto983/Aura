"""Does pseudonymizing the embedding input lose anything? Clear vs three representations.

Corpus: the 86 passages the lab VM's ingest wrote (embed-eval dump, operator's Garage files)
plus the 33 synthetic documents of corpus.py. Questions: the 35 of embed-eval plus 64 synthetic.
Every value of corpus.json's entities, and the given name in esempio.md, goes through the
ArcadeDB vault (vault.py) before any text is embedded.

Ranking follows Aura's fused passage read (internal/arcadedb/document_retrieval.go
fusedStatement): a dense leg (cosine distance <= 0.72, 200 neighbours) and a lexical leg fused by
RRF 1/(60+rank), one passage per document, the top max(4*limit, 20) re-ranked by cosine, kept
at cosine >= 0.32, limit 8. The lexical leg runs on the ORIGINAL passage text with the ORIGINAL
query, as it does in Aura: the full-text index is local and never leaves. BM25 stands in for
ArcadeDB's Lucene index; it is an approximation of that leg, not the leg.

Usage: python3 -I measure.py [model ...]   (OPENROUTER_API_KEY for the cloud models)
"""
import collections, hashlib, json, math, os, re, sys, time, urllib.error, urllib.request

HERE = os.path.dirname(os.path.abspath(__file__))
EVAL = os.path.join(os.path.dirname(HERE), "embedding-cloud-models")
sys.path.insert(0, HERE)
from vault import Resp, Vault, create_identity  # noqa: E402
from pseudo import Pseudonymizer  # noqa: E402

QUERY_PREFIX = "task: search result | query: "
DOC_PREFIX = "title: none | text: "
WIDTH = 768
SEEDS = 5
REPS = ["clear", "openrouter_redact", "token_whole", "token_parts"] + [f"surrogate#{k}" for k in range(SEEDS)]
LOCAL = "local/embeddinggemma-300M-Q8_0"
MODELS = [LOCAL, "perplexity/pplx-embed-v1-4b", "google/gemini-embedding-2"]
SEED_CACHE = {"perplexity/pplx-embed-v1-4b": "perplexity__pplx-embed-v1-4b.json",
              "google/gemini-embedding-2": "google__gemini-embedding-2.json"}
DUMP = os.path.join(EVAL, "dump-mem_448ddbe1_96ea_405d_8219_4a3d52a425c0.json")


def norm(s):
    return re.sub(r"\s+", " ", s).lower()


def cosine(a, b):
    dot = sum(x * y for x, y in zip(a, b))
    na, nb = math.sqrt(sum(x * x for x in a)), math.sqrt(sum(y * y for y in b))
    return dot / (na * nb) if na and nb else 0.0


def truncate(v):
    if len(v) <= WIDTH:
        return v
    head = v[:WIDTH]
    n = math.sqrt(sum(x * x for x in head))
    return [x / n for x in head]


class Embedder:
    def __init__(self, model):
        self.model = model
        self.path = os.path.join(HERE, "cache", model.replace("/", "__") + ".json")
        self.cache = json.load(open(self.path, encoding="utf-8")) if os.path.exists(self.path) else {}
        self.cost, self.tokens, self.calls = 0.0, 0, 0
        if not self.cache and model in SEED_CACHE:
            self._seed(os.path.join(EVAL, "cache", SEED_CACHE[model]))

    def _seed(self, path):
        """Reuse the vectors embed-eval already paid for: same input, same prefix, same width."""
        if not os.path.exists(path):
            return
        old = json.load(open(path, encoding="utf-8"))
        passages = [p["text"] for p in json.load(open(DUMP, encoding="utf-8"))["passages"]]
        questions = json.load(open(os.path.join(EVAL, "questions.json"), encoding="utf-8"))
        for t, v in zip(passages, old["docs"]):
            self.cache[self.key(DOC_PREFIX + t)] = truncate(v)
        for q, v in zip(questions, old["queries"]):
            self.cache[self.key(QUERY_PREFIX + q["q"])] = truncate(v)

    @staticmethod
    def key(text):
        return hashlib.sha256(text.encode("utf-8")).hexdigest()

    def embed(self, texts):
        missing = list(dict.fromkeys(t for t in texts if self.key(t) not in self.cache))
        for i in range(0, len(missing), 16):
            batch = missing[i:i + 16]
            for t, v in zip(batch, self._post(batch)):
                self.cache[self.key(t)] = truncate(v)
        if missing:
            os.makedirs(os.path.dirname(self.path), exist_ok=True)
            json.dump(self.cache, open(self.path, "w", encoding="utf-8"))
        return [self.cache[self.key(t)] for t in texts]

    def _post(self, batch):
        if self.model == LOCAL:
            url, body, headers = "http://127.0.0.1:8081/v1/embeddings", {"input": batch, "model": "embeddinggemma"}, {}
        else:
            url = "https://openrouter.ai/api/v1/embeddings"
            body = {"model": self.model, "input": batch, "dimensions": WIDTH}
            headers = {"Authorization": "Bearer " + os.environ["OPENROUTER_API_KEY"]}
        req = urllib.request.Request(url, data=json.dumps(body).encode(), headers={**headers, "Content-Type": "application/json"})
        for attempt in range(6):
            try:
                data = json.loads(urllib.request.urlopen(req, timeout=300).read())
                if "data" not in data:
                    raise RuntimeError(f"no data: {str(data)[:300]}")
                u = data.get("usage") or {}
                self.cost += float(u.get("cost") or 0)
                self.tokens += u.get("prompt_tokens") or u.get("total_tokens") or 0
                self.calls += 1
                return [d["embedding"] for d in sorted(data["data"], key=lambda d: d["index"])]
            except urllib.error.HTTPError as err:
                detail = err.read().decode("utf-8", "replace")[:300]
                if err.code == 429 or err.code >= 500:
                    time.sleep(5 * (attempt + 1))
                    continue
                raise RuntimeError(f"HTTP {err.code}: {detail}") from None
        raise RuntimeError("gave up after 6 attempts")


WORD = re.compile(r"\w+", re.UNICODE)


class BM25:
    def __init__(self, texts, k1=1.2, b=0.75):
        self.docs = [collections.Counter(WORD.findall(t.lower())) for t in texts]
        self.len = [sum(d.values()) for d in self.docs]
        self.avg = sum(self.len) / len(self.len)
        df = collections.Counter(w for d in self.docs for w in d)
        n = len(texts)
        self.idf = {w: math.log(1 + (n - c + 0.5) / (c + 0.5)) for w, c in df.items()}
        self.k1, self.b = k1, b

    def rank(self, query):
        terms = WORD.findall(query.lower())
        scores = []
        for i, d in enumerate(self.docs):
            s = 0.0
            for w in terms:
                if w in d:
                    tf = d[w]
                    s += self.idf[w] * tf * (self.k1 + 1) / (tf + self.k1 * (1 - self.b + self.b * self.len[i] / self.avg))
            if s > 0:
                scores.append((s, i))
        return [i for _, i in sorted(scores, reverse=True)]


def aura_read(qv, query, pvecs, groups, bm25, limit=8):
    cos = [cosine(qv, v) for v in pvecs]
    dense = [i for i in sorted(range(len(pvecs)), key=lambda i: -cos[i]) if 1 - cos[i] <= 0.72][:200]
    lexical = bm25.rank(query)
    rrf = collections.defaultdict(float)
    for leg in (dense, lexical):
        for r, i in enumerate(leg, 1):
            rrf[i] += 1 / (60 + r)
    best = {}
    for i, s in sorted(rrf.items(), key=lambda x: -x[1]):
        best.setdefault(groups[i], i)
    pool = sorted(best.values(), key=lambda i: -rrf[i])[:max(limit * 4, 20)]
    ranked = [i for i in sorted(pool, key=lambda i: -cos[i]) if cos[i] >= 0.32]
    # The variant: same pool and the same abstention floor, ordered by the fusion instead of the cosine.
    fused = [i for i in pool if cos[i] >= 0.32]
    return ranked[:limit], fused[:limit], cos


def first_hit(order, ok):
    return next((r for r, i in enumerate(order) if ok(i)), None)


def summarize(ranks):
    n = len(ranks)
    at = lambda k: sum(1 for r in ranks if r is not None and r < k)
    return {"r1": at(1), "r3": at(3), "r8": at(8), "n": n,
            "mrr": round(sum(1 / (r + 1) for r in ranks if r is not None) / n, 4)}


def setup_vault(seed):
    """Identity `seed`: its own database, its own ArcadeDB user, its own HMAC key."""
    pw = open(os.path.join(HERE, "root.pw")).read().strip()
    base = "http://127.0.0.1:2490"
    db, user = f"pii_identity_{seed}", f"identity_{seed}"
    upw = hashlib.sha256(f"{pw}/{seed}".encode()).hexdigest()[:24]
    try:
        create_identity(base, pw, db, user, upw)
    except RuntimeError as err:
        if "exists" not in str(err):
            raise
    key = hashlib.sha256(f"identity-key/{seed}/{pw}".encode()).digest()
    return Vault(Resp("127.0.0.1", 6390, user, upw), db, key)


def main():
    corpus = json.load(open(os.path.join(HERE, "corpus.json"), encoding="utf-8"))
    dump = json.load(open(DUMP, encoding="utf-8"))
    garage_q = json.load(open(os.path.join(EVAL, "questions.json"), encoding="utf-8"))
    entities = corpus["entities"] + [{"kind": "GIVEN", "value": "Davide"}]

    t0 = time.time()
    pss = [Pseudonymizer(setup_vault(k), entities) for k in range(SEEDS)]
    setup_s = time.time() - t0

    texts = [p["text"] for p in dump["passages"]] + [d["text"] for d in corpus["docs"]]
    groups = [p["source_key"] for p in dump["passages"]] + [d["id"] for d in corpus["docs"]]
    n_garage = len(dump["passages"])
    questions = [{"cls": "G", "q": q["q"], "truth": q["truth"]} for q in garage_q] + corpus["questions"]

    report = {"setup_s": round(setup_s, 2), "entries": len(pss[0].v.by_digest), "reps": {}}
    pseudo_texts, pseudo_queries = {}, {}
    for rep in REPS:
        ps = pss[int(rep.split("#")[1])] if "#" in rep else pss[0]
        kind = rep.split("#")[0]
        t = time.time()
        pt = [ps.pseudonymize(x, kind) for x in texts]
        pq = [ps.pseudonymize(q["q"], kind) for q in questions]
        elapsed = time.time() - t
        pseudo_texts[rep] = [x for x, _ in pt]
        pseudo_queries[rep] = [x for x, _ in pq]
        leaks = [(i, ps.residue(x)) for i, x in enumerate(pseudo_texts[rep] + pseudo_queries[rep]) if ps.residue(x)]
        info = {"replaced_in_texts": sum(c for _, c in pt), "replaced_in_queries": sum(c for _, c in pq),
                "leaks": len(leaks) if rep != "clear" else None, "pseudonymize_s": round(elapsed, 3)}
        if rep in ("token_whole", "token_parts"):
            t = time.time()
            rebuilt = [ps.rebuild_tokens(x) for x in pseudo_texts[rep] + pseudo_queries[rep]]
            info["rebuild_s"] = round(time.time() - t, 3)
            original = texts + [q["q"] for q in questions]
            info["rebuilt_exact"] = sum(1 for a, b in zip(rebuilt, original) if a == b)
            info["rebuilt_of"] = len(original)
            info["rebuild_mismatch"] = [i for i, (a, b) in enumerate(zip(rebuilt, original)) if a != b][:5]
        if kind == "surrogate":
            synth = list(range(n_garage, len(texts)))
            rebuilt = [ps.rebuild_surrogates(pseudo_texts[rep][i]) for i in synth]
            info["surrogate_rebuilt_exact"] = sum(1 for i, r in zip(synth, rebuilt) if r == texts[i])
            info["surrogate_rebuilt_of"] = len(synth)
        report["reps"][rep] = info
        print(rep, info, flush=True)

    sample = {rep: pseudo_texts[rep][n_garage] for rep in REPS}
    json.dump(sample, open(os.path.join(HERE, "sample-D01.json"), "w", encoding="utf-8"), ensure_ascii=False, indent=1)

    bm25 = BM25(texts)
    def correct(qi):
        q = questions[qi]
        if q["cls"] == "G":
            truth = norm(q["truth"])
            return lambda i: i < n_garage and truth in norm(texts[i])
        return lambda i: groups[i] in q["truth"]

    wanted = sys.argv[1:] or MODELS
    if "OPENROUTER_API_KEY" not in os.environ:
        wanted = [m for m in wanted if m == LOCAL]
    results = {}
    for model in wanted:
        emb = Embedder(model)
        per = {}
        for rep in REPS:
            try:
                pv = emb.embed([DOC_PREFIX + t for t in pseudo_texts[rep]])
                qv = emb.embed([QUERY_PREFIX + q for q in pseudo_queries[rep]])
            except RuntimeError as err:
                per[rep] = {"error": str(err)}
                print(model, rep, err, flush=True)
                continue
            aura, rrf, dense = [], [], []
            for qi, q in enumerate(questions):
                order, fused, cos = aura_read(qv[qi], q["q"], pv, groups, bm25)
                aura.append(first_hit(order, correct(qi)))
                rrf.append(first_hit(fused, correct(qi)))
                dense.append(first_hit(sorted(range(len(pv)), key=lambda i: -cos[i]), correct(qi)))
            per[rep] = {"aura": aura, "rrf": rrf, "dense": dense}
        results[model] = {"per": per, "cost": round(emb.cost, 5), "tokens": emb.tokens, "calls": emb.calls}
        print(model, "cost", round(emb.cost, 5), "tokens", emb.tokens, flush=True)

    classes = sorted({q["cls"] for q in questions})
    lines = []
    for model, r in results.items():
        lines.append(f"\n## {model}  (cost ${r['cost']}, {r['tokens']} tokens, {r['calls']} calls)")
        lines.append("rep | leg | all r@1 r@3 r@8 MRR | " + " | ".join(f"{c} r@1/r@3" for c in classes) + " | worse than clear")
        for rep in REPS:
            if "error" in r["per"].get(rep, {"error": "missing"}):
                lines.append(f"{rep} | ERROR {r['per'].get(rep, {}).get('error', 'missing')[:160]}")
                continue
            for leg in ("aura", "rrf", "dense"):
                ranks = r["per"][rep][leg]
                s = summarize(ranks)
                cls = []
                for c in classes:
                    idx = [i for i, q in enumerate(questions) if q["cls"] == c]
                    cs = summarize([ranks[i] for i in idx])
                    cls.append(f"{cs['r1']}/{cs['r3']} of {cs['n']}")
                base = r["per"]["clear"][leg]
                worse = [i for i, (a, b) in enumerate(zip(ranks, base))
                         if (a is None and b is not None) or (a is not None and b is not None and a > b)]
                lines.append(f"{rep} | {leg} | {s['r1']} {s['r3']} {s['r8']} {s['mrr']:.3f} | " + " | ".join(cls) +
                             f" | {len(worse)}: " + ", ".join(f"{questions[i]['cls']}{i}" for i in worse[:12]))
        keys = [f"surrogate#{k}" for k in range(SEEDS) if "aura" in r["per"].get(f"surrogate#{k}", {})]
        for leg in ("aura", "rrf", "dense"):
            if not keys:
                break
            runs = [summarize(r["per"][k][leg]) for k in keys]
            base = summarize(r["per"]["clear"][leg])
            span = lambda f: f"{sum(x[f] for x in runs) / len(runs):.2f} [{min(x[f] for x in runs)}..{max(x[f] for x in runs)}]"
            lines.append(f"surrogate over {len(keys)} keys | {leg} | r@1 {span('r1')} r@3 {span('r3')} MRR {span('mrr')}"
                         f" | clear r@1 {base['r1']} r@3 {base['r3']} MRR {base['mrr']}")
            moved = []
            for i in range(len(questions)):
                b = r["per"]["clear"][leg][i]
                ranks = [r["per"][k][leg][i] for k in keys]
                worse = sum(1 for a in ranks if (a is None and b is not None) or (a is not None and b is not None and a > b))
                better = sum(1 for a in ranks if (b is None and a is not None) or (a is not None and b is not None and a < b))
                if worse or better:
                    moved.append(f"{questions[i]['cls']}{i} -{worse}/+{better}")
            lines.append(f"  per question, keys worse/better than clear: " + ", ".join(moved))
    out = "\n".join(lines)
    print(out)
    report["results"] = {m: {"cost": r["cost"], "tokens": r["tokens"], "per": r["per"]} for m, r in results.items()}
    report["questions"] = [{"cls": q["cls"], "q": q["q"] if q["cls"] != "G" else "(garage)"} for q in questions]
    json.dump(report, open(os.path.join(HERE, "results.json"), "w", encoding="utf-8"), ensure_ascii=False, indent=1)
    open(os.path.join(HERE, "results.txt"), "w", encoding="utf-8").write(out + "\n")


if __name__ == "__main__":
    main()
