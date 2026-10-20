# Read-only, runs inside aura-ingest on the lab VM: every Passage and IndexedDocument of one tenant
# database with its stored vector, as JSON on stdout. Uses the container's own ArcadeDB settings.
import base64, json, os, sys, urllib.request

base = os.environ.get("ARCADE_HTTP", "http://aura-arcadedb:2480").rstrip("/")
auth = "Basic " + base64.b64encode(f"root:{os.environ['ARCADEDB_PASSWORD']}".encode()).decode()

def get(path):
    req = urllib.request.Request(base + path, headers={"Authorization": auth})
    return json.loads(urllib.request.urlopen(req, timeout=60).read())

def query(db, sql):
    req = urllib.request.Request(
        f"{base}/api/v1/query/{db}",
        data=json.dumps({"language": "sql", "command": sql}).encode(),
        headers={"Authorization": auth, "Content-Type": "application/json"})
    return json.loads(urllib.request.urlopen(req, timeout=120).read())["result"]

dbs = get("/api/v1/databases")["result"]
want = sys.argv[1] if len(sys.argv) > 1 else ""
if not want:
    print(json.dumps({"databases": dbs}))
    sys.exit(0)
out = {"database": want}
out["passages"] = query(want, "SELECT source_key, ordinal, heading_path, text, embed_space, embedding FROM Passage ORDER BY source_key, ordinal")
cols = [p["name"] for p in query(want, "SELECT expand(properties) FROM schema:types WHERE name = 'IndexedDocument'")]
out["card_columns"] = cols
out["cards"] = query(want, "SELECT * FROM IndexedDocument")
print(json.dumps(out))
