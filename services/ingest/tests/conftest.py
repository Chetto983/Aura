import pytest

from ingest import embed


@pytest.fixture(autouse=True)
def unread_bound(monkeypatch):
    """The bound is read once per process, so without this a test would inherit whatever
    /props answered first in the run, and pass or fail by the order it runs in."""
    monkeypatch.setattr(embed, "_bound", None)
