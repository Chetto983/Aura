"""What the vault in ArcadeDB 26.10.1 actually guarantees, over RESP and over HTTP.

Runs against the databases measure.py created (pii_identity_0 and pii_identity_1).
Usage: python3 -I probe.py [after-restart]
"""
import hashlib, json, os, statistics, sys, time

HERE = os.path.dirname(os.path.abspath(__file__))
sys.path.insert(0, HERE)
from vault import ENTRY, Resp, RespError, Vault, http  # noqa: E402

PW = open(os.path.join(HERE, "root.pw")).read().strip()
BASE = "http://127.0.0.1:2490"
TRICKY = ["Nicolò De Luca", "Chiara D'Angelo", 'Mario "il Baffo" Rossi', "Studio {Conti} & Associati",
          "riga uno\nriga due", "Zoë Ångström-Øberg", "李小龙", "Ana 😀 Lima", "  spazi  attorno  ", "x" * 5000,
          "HGET pii_identity_1.PiiEntry[token] PII_X", "\\u0000 e \\\\ barre"]


def connect(seed):
    upw = hashlib.sha256(f"{PW}/{seed}".encode()).hexdigest()[:24]
    key = hashlib.sha256(f"identity-key/{seed}/{PW}".encode()).digest()
    return Vault(Resp("127.0.0.1", 6390, f"identity_{seed}", upw), f"pii_identity_{seed}", key), upw


def check(name, ok, detail=""):
    print(f"{'PASS' if ok else 'FAIL'}  {name}" + (f"  -- {detail}" if detail else ""))
    return ok


def main():
    after_restart = sys.argv[1:] == ["after-restart"]
    v0, pw0 = connect(0)
    v1, _ = connect(1)
    state_path = os.path.join(HERE, "probe-state.json")

    if after_restart:
        state = json.load(open(state_path, encoding="utf-8"))
        back = [v0.get("token", t) for t in state["tokens"]]
        check("entries survive a server restart, byte-exact",
              all(b is not None and b["value"] == val for b, val in zip(back, state["values"])),
              f"{sum(1 for b in back if b)}/{len(back)} found")
        count = http(BASE, "identity_0", pw0, "/api/v1/command/pii_identity_0",
                     {"language": "sql", "command": f"SELECT count(*) AS n FROM {ENTRY}"})["result"][0]["n"]
        check("the vault still holds every entry", count == state["count"], f"{count} vs {state['count']}")
        try:
            gone = v0.r.call("GET", "probe-transient")
        except RespError as err:
            gone = f"error {err}"
        check("a transient key does NOT survive the restart (why the vault never uses it)", gone is None, repr(gone))
        return

    tokens, values = [], []
    for i, value in enumerate(TRICKY):
        e = v0.entry("PROBE", value, lambda k, val, a: f"probe-surrogate-{i}-{a}")
        back = v0.get("token", e["token"])
        tokens.append(e["token"])
        values.append(value)
        check(f"tricky value {i} round-trips byte-exact", back is not None and back["value"] == value, repr(value[:30]))

    e1 = v1.entry("PROBE", "solo dell'identità 1", lambda k, val, a: "probe-surrogate-other")
    try:
        leaked = v0.r.call("HGET", f"pii_identity_1.{ENTRY}[token]", e1["token"])
        check("identity 0 cannot read identity 1's vault", False, f"read {leaked!r}")
    except RespError as err:
        check("identity 0 cannot read identity 1's vault", "NOPERM" in str(err) or "permission" in str(err).lower(), str(err)[:120])
    try:
        v0.r.call("HSET", "pii_identity_1", ENTRY, json.dumps({"token": "PII_X_" + "0" * 32, "digest": "f" * 64}))
        check("identity 0 cannot write identity 1's vault", False)
    except RespError as err:
        check("identity 0 cannot write identity 1's vault", "NOPERM" in str(err) or "permission" in str(err).lower(), str(err)[:120])

    first = v0.get("token", tokens[0])
    clash = dict(first)
    clash.pop("@rid", None), clash.pop("@type", None), clash.pop("@cat", None)
    clash["value"] = "un altro valore"
    try:
        v0.r.call("HSET", v0.db, ENTRY, json.dumps(clash, ensure_ascii=False))
        check("a second entry with the same digest is refused, never overwrites", False)
    except RespError as err:
        again = v0.get("token", tokens[0])
        check("a second entry with the same digest is refused, never overwrites",
              again["value"] == TRICKY[0], str(err)[:120])

    # The docs call the transient map shared; measured 2026-10-20 on 26.10.1, another user's connection
    # does not see the key. Either way it is RAM only (see after-restart), so the vault never uses it.
    v0.r.call("SET", "probe-transient", "visibile a tutti?")
    seen = v1.r.call("GET", "probe-transient")
    check("a transient key set by identity 0 is not visible to identity 1's connection", seen is None, repr(seen))

    # The HTTP `redis` language splits at a line break: one GET whose argument carries a line break
    # answers TWO results, because the rest of the line ran as a second command.
    cmd = "GET probe-transient\nSET injected-by-a-value yes"
    out = http(BASE, "identity_0", pw0, "/api/v1/command/pii_identity_0", {"language": "redis", "command": cmd})
    replies = out["result"][0]["value"]
    check("HTTP redis: a line break in an argument runs a second command (keys must be hex)",
          isinstance(replies, list) and len(replies) == 2 and replies[1] == "OK", json.dumps(replies))

    lat = []
    for i in range(1000):
        t = time.perf_counter()
        v0.get("token", tokens[i % len(tokens)])
        lat.append((time.perf_counter() - t) * 1000)
    lat.sort()
    print(f"HGET by token over RESP, 1000 calls: p50 {statistics.median(lat):.3f} ms, p95 {lat[949]:.3f} ms, max {lat[-1]:.3f} ms")

    count = http(BASE, "identity_0", pw0, "/api/v1/command/pii_identity_0",
                 {"language": "sql", "command": f"SELECT count(*) AS n FROM {ENTRY}"})["result"][0]["n"]
    json.dump({"tokens": tokens, "values": values, "count": count}, open(state_path, "w", encoding="utf-8"), ensure_ascii=False)
    print(f"{count} entries in pii_identity_0; restart the container and run: probe.py after-restart")


if __name__ == "__main__":
    main()
