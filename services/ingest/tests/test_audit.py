"""The reconciliation that turns a lost document into a failure instead of a silence.

An ingest pass that drops a file still exits 0: CocoIndex catches the component failure,
prints "component build failed" and carries on. On 2026-08-09 two separate defects did
exactly that on the same 130-document corpus and both were found by comparing the bucket
against the index BY HAND. These tests cover that comparison without a daemon, because
the parts that can be wrong are pure: which keys count as expected, and how the index's
answer is parsed.
"""

from types import SimpleNamespace

import pytest

from ingest import app, arcade, source
from ingest.tests import fake_s3


def test_expected_keys_excludes_the_prefixes_the_walker_excludes(monkeypatch):
    # RESERVED_PATTERNS keeps Aura's own layout out of the pipeline, so counting those
    # objects here would invent a discrepancy on every single pass: they are never
    # supposed to produce a document row.
    fake_s3.install(monkeypatch, [{"Contents": [
        {"Key": "laws/statute.pdf"},
        {"Key": "identity/abcd/original"},
        {"Key": "share/token/thing.pdf"},
        {"Key": "laws/table.csv"},
    ]}])

    assert source.expected_keys(fake_s3.config()) == {"laws/statute.pdf", "laws/table.csv"}


def test_expected_keys_skips_folder_markers(monkeypatch):
    # A zero-byte key ending in "/" is what a UI writes so an empty prefix appears in a
    # listing. The walker never offers one to the pipeline, so expecting a row for it
    # invents a discrepancy that can never clear. MEASURED 2026-08-16: one "test/" marker
    # made every catch-up run exit 1.
    fake_s3.install(monkeypatch, [{"Contents": [
        {"Key": "test/"},
        {"Key": "test/statute.pdf"},
    ]}])

    assert source.expected_keys(fake_s3.config()) == {"test/statute.pdf"}


def test_expected_keys_still_counts_an_empty_file(monkeypatch):
    # The test is the trailing slash, not the size: an empty file a person uploaded is a
    # document, gets a row, and must still be audited.
    fake_s3.install(monkeypatch, [{"Contents": [{"Key": "vuoto.txt", "Size": 0}]}])

    assert source.expected_keys(fake_s3.config()) == {"vuoto.txt"}


def test_expected_keys_spans_pages(monkeypatch):
    fake_s3.install(monkeypatch, [
        {"Contents": [{"Key": "a.pdf"}]},
        {"Contents": [{"Key": "b.pdf"}]},
        {},
    ])

    assert source.expected_keys(fake_s3.config()) == {"a.pdf", "b.pdf"}


def test_indexed_source_keys_reads_the_rows(monkeypatch):
    captured = {}

    def fake_post(base_url, path, payload, auth, timeout_s):
        captured.update(base_url=base_url, path=path, payload=payload)
        return {"result": [{"source_key": "laws/a.pdf"}, {"source_key": "laws/b.csv"}]}

    monkeypatch.setattr("ingest.arcade._post", fake_post)

    keys = arcade.indexed_source_keys("http://arcadedb:2480", "mem_x", ("root", "pw"), 60.0)

    assert keys == {"laws/a.pdf", "laws/b.csv"}
    assert captured["path"] == "/api/v1/query/mem_x"
    assert arcade.DOCUMENT_TYPE in captured["payload"]["command"]


@pytest.mark.parametrize("body", [{}, {"result": []}, {"result": [{"source_key": None}]}])
def test_indexed_source_keys_treats_an_empty_index_as_empty_not_as_success(monkeypatch, body):
    # An empty answer must read as "nothing is indexed", never as "nothing is missing".
    # Returning a full set here would make the audit pass loudest exactly when the whole
    # pass has failed.
    monkeypatch.setattr("ingest.arcade._post", lambda *a, **k: body)

    assert arcade.indexed_source_keys("http://x", "mem_x", ("root", "pw"), 1.0) == set()


def snapshot(num_errors: int | None):
    total = SimpleNamespace() if num_errors is None else SimpleNamespace(num_errors=num_errors)
    return SimpleNamespace(stats=SimpleNamespace(total=total))


def catch_up(monkeypatch, last_snapshot, expected: set[str], indexed: set[str]) -> int:
    async def run():
        return last_snapshot

    monkeypatch.setattr(app, "_update_publishing_status", run)
    monkeypatch.setattr(app, "_bucket_versus_index", lambda: (expected, indexed))
    return app._catch_up()


def test_a_catch_up_with_cocoindex_errors_exits_one(monkeypatch):
    # MEASURED 2026-10-17: CocoIndex published errors=19 for an upgrade, the audit counted
    # 33 objects, 33 indexed, 0 missing, and the process exited 0. The index being
    # complete does not make the run clean.
    assert catch_up(monkeypatch, snapshot(2), {"a.pdf"}, {"a.pdf"}) == 1


def test_a_clean_catch_up_exits_zero(monkeypatch):
    assert catch_up(monkeypatch, snapshot(0), {"a.pdf"}, {"a.pdf"}) == 0


def test_a_catch_up_with_a_missing_object_still_exits_one(monkeypatch):
    assert catch_up(monkeypatch, snapshot(0), {"a.pdf"}, set()) == 1


@pytest.mark.parametrize("last", [None, snapshot(None)])
def test_no_snapshot_or_no_error_count_counts_as_none(monkeypatch, last):
    assert catch_up(monkeypatch, last, {"a.pdf"}, {"a.pdf"}) == 0


def test_a_snapshot_without_stats_fails_loudly_instead_of_reading_clean(monkeypatch):
    # If CocoIndex's snapshot shape drifts, a silent 0 would be the false clean again.
    with pytest.raises(AttributeError):
        catch_up(monkeypatch, SimpleNamespace(), {"a.pdf"}, {"a.pdf"})


def test_the_audit_line_prints_the_error_count(monkeypatch, capsys):
    monkeypatch.setattr(app, "_bucket_versus_index", lambda: ({"a.pdf", "b.pdf"}, {"a.pdf"}))

    app.audit_pass(19)

    assert (
        f"[audit] 2 objects in {app._S3_CONFIG.bucket}, 1 indexed, 1 missing, 19 errors"
        in capsys.readouterr().out
    )
