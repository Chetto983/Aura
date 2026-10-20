"""The PII vault in ArcadeDB, spoken over the Redis wire protocol (RESP).

One database per identity, as Aura's memory already is: the server checks the connection's
ArcadeDB user against the database named in every command (RedisNetworkExecutor
getAuthorizedDatabase, 26.10.1), so one identity cannot read another's vault.

Only the persistent commands are used. The transient ones (SET/GET) live in one RAM map
shared by every user and reset on restart (arcadedb-docs redis.adoc).

Keys are hex digests, never the value: the HTTP `redis` language splits a command at any line
break and has no escapes inside quotes (RedisQueryEngine COMMAND_PATTERN), so an untrusted value
must never become an argument. RESP is length-prefixed, but the same rule keeps both paths safe.
"""
import base64, hashlib, hmac, json, socket, urllib.request

ENTRY = "PiiEntry"


class RespError(Exception):
    pass


class Resp:
    """A minimal RESP2 client: enough for AUTH, HSET, HGET, HMGET, HEXISTS, PING."""

    def __init__(self, host, port, user, password):
        self.sock = socket.create_connection((host, port), timeout=30)
        self.buf = b""
        self.call("AUTH", user, password)

    def call(self, *args):
        out = [b"*%d\r\n" % len(args)]
        for a in args:
            b = a if isinstance(a, bytes) else str(a).encode("utf-8")
            out.append(b"$%d\r\n%s\r\n" % (len(b), b))
        self.sock.sendall(b"".join(out))
        return self._read()

    def _line(self):
        while b"\r\n" not in self.buf:
            chunk = self.sock.recv(65536)
            if not chunk:
                raise RespError("connection closed")
            self.buf += chunk
        line, self.buf = self.buf.split(b"\r\n", 1)
        return line

    def _exact(self, n):
        while len(self.buf) < n + 2:
            chunk = self.sock.recv(65536)
            if not chunk:
                raise RespError("connection closed")
            self.buf += chunk
        data, self.buf = self.buf[:n], self.buf[n + 2:]
        return data

    def _read(self):
        line = self._line()
        kind, rest = line[:1], line[1:]
        if kind == b"+":
            return rest.decode()
        if kind == b"-":
            raise RespError(rest.decode("utf-8", "replace"))
        if kind == b":":
            return int(rest)
        if kind == b"$":
            n = int(rest)
            return None if n < 0 else self._exact(n).decode("utf-8")
        if kind == b"*":
            n = int(rest)
            return None if n < 0 else [self._read() for _ in range(n)]
        raise RespError(f"unexpected reply {line[:40]!r}")

    def close(self):
        self.sock.close()


def http(base, user, password, path, body=None, method=None):
    req = urllib.request.Request(base + path, data=None if body is None else json.dumps(body).encode(),
                                 method=method or ("POST" if body is not None else "GET"))
    req.add_header("Authorization", "Basic " + base64.b64encode(f"{user}:{password}".encode()).decode())
    req.add_header("Content-Type", "application/json")
    try:
        return json.loads(urllib.request.urlopen(req, timeout=60).read() or b"{}")
    except urllib.error.HTTPError as err:
        raise RuntimeError(f"HTTP {err.code}: {err.read().decode('utf-8', 'replace')[:400]}") from None


def create_identity(base, root_pw, db, user, password):
    """A database and a user that may open only it, the shape of Aura's per-identity tenant."""
    http(base, "root", root_pw, "/api/v1/server", {"command": f"create database {db}"})
    http(base, "root", root_pw, "/api/v1/server", {"command": "create user " + json.dumps(
        {"name": user, "password": password, "databases": {db: ["admin"]}})})
    script = (f"CREATE DOCUMENT TYPE {ENTRY};"
              f"CREATE PROPERTY {ENTRY}.token STRING; CREATE INDEX ON {ENTRY} (token) UNIQUE;"
              f"CREATE PROPERTY {ENTRY}.digest STRING; CREATE INDEX ON {ENTRY} (digest) UNIQUE;"
              f"CREATE PROPERTY {ENTRY}.sur_digest STRING; CREATE INDEX ON {ENTRY} (sur_digest) UNIQUE;"
              f"CREATE PROPERTY {ENTRY}.kind STRING; CREATE PROPERTY {ENTRY}.value STRING;"
              f"CREATE PROPERTY {ENTRY}.sur STRING;")
    http(base, user, password, f"/api/v1/command/{db}", {"language": "sqlscript", "command": script})


def fold(value):
    return " ".join(value.split()).casefold()


class Vault:
    """One identity's map: entity -> token and surrogate, both directions, persisted before use."""

    def __init__(self, resp, db, identity_key):
        self.r, self.db, self.key = resp, db, identity_key
        self.by_digest = {}

    def mac(self, domain, text):
        return hmac.new(self.key, f"{domain}\x00{text}".encode("utf-8"), hashlib.sha256).hexdigest()

    def digest(self, kind, value):
        return self.mac("pii-entry", kind + "\x00" + fold(value))

    def sur_digest(self, kind, sur):
        return self.mac("pii-surrogate", kind + "\x00" + fold(sur))

    def token(self, kind, value):
        return f"PII_{kind}_{self.mac('pii-token', kind + chr(0) + fold(value))[:32]}"

    def get(self, index, key):
        raw = self.r.call("HGET", f"{self.db}.{ENTRY}[{index}]", key)
        return None if raw is None else json.loads(raw)

    def entry(self, kind, value, surrogate_for):
        """The stored entry for (kind, value), creating it with the first free surrogate."""
        d = self.digest(kind, value)
        if d in self.by_digest:
            return self.by_digest[d]
        found = self.get("digest", d)
        if found is None:
            for attempt in range(64):
                sur = surrogate_for(kind, value, attempt)
                doc = {"token": self.token(kind, value), "digest": d, "kind": kind, "value": value,
                       "sur": sur, "sur_digest": self.sur_digest(kind, sur)}
                try:
                    self.r.call("HSET", self.db, ENTRY, json.dumps(doc, ensure_ascii=False))
                    found = doc
                    break
                except RespError as err:
                    if "sur_digest" in str(err) or "Duplicated" in str(err):
                        again = self.get("digest", d)
                        if again is not None:  # a concurrent writer stored the same entity
                            found = again
                            break
                        continue  # the surrogate is taken by another entity: next candidate
                    raise
            if found is None:
                raise RespError(f"no free surrogate for a {kind} after 64 tries")
        self.by_digest[d] = found
        return found
