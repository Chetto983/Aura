"""The disposable ArcadeDB databases the integration tests write, read and drop.

NEVER aura_memory or any mem_<uuid>: those are production identities.
"""

import os
import subprocess
import sys

from ingest.arcade import ArcadeSchemaError, _post

ARCADE_HTTP = os.environ.get("ARCADE_HTTP", "http://arcadedb:2480")
# ArcadeDB requires the rootPassword baked into the running container -- see
# test_arcade_integration.py's docstring for how to read it without hardcoding it.
AUTH = ("root", os.environ["ARCADEDB_PASSWORD"])


def drop(database: str) -> None:
    try:
        _post(ARCADE_HTTP, "/api/v1/server", {"command": f"drop database {database}"}, AUTH, 30.0)
    except ArcadeSchemaError as exc:
        if "not exist" not in str(exc).lower():
            raise


def query(database: str, statement: str, params: dict | None = None) -> list[dict]:
    body = _post(
        ARCADE_HTTP, f"/api/v1/query/{database}",
        {"language": "sql", "command": statement, "params": params or {}}, AUTH, 30.0,
    )
    return body["result"]


def space_driver_pass(database: str, objects: list[str], state, **env: str) -> str:
    """One catch-up pass of space_driver over one CocoIndex state; its stdout."""
    done = subprocess.run(
        [sys.executable, "-m", "ingest.tests.space_driver", database, *objects],
        env=dict(os.environ, COCOINDEX_DB=str(state), **env),
        check=False, capture_output=True, text=True, timeout=600,
    )
    assert done.returncode == 0, done.stdout + done.stderr
    return done.stdout
