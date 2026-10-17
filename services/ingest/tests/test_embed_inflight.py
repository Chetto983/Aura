"""How many embedding requests are in the air, and that it is never more than the sidecar serves.

An ingest that times out on its embedding calls looks the same whether one request was slow
or thirty were sent at once; only the count at the moment of sending tells the two apart. And a
request the sidecar has no slot for waits inside it with its deadline already running, so the
bound is the sidecar's own slot count.
"""

import io
import json
import re
import threading
import time
import urllib.error
import urllib.request

import cocoindex as coco
import pytest

from ingest import embed
from ingest.tests import fake_embedder

LINE = re.compile(
    r"^\[embed\] (?P<time>\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}\.\d{3}\+00:00) "
    r"inflight=(?P<inflight>\d+) waiting=(?P<waiting>\d+) (?P<chunks>\d+) chunk\(s\) in one request$"
)


def _slots(count) -> tuple[int, bytes]:
    return 200, json.dumps({"total_slots": count}).encode()


@pytest.fixture
def fake(monkeypatch):
    """fake_embedder.install() replaces urllib.request.urlopen outright; this restores it."""
    monkeypatch.setattr(urllib.request, "urlopen", urllib.request.urlopen)
    return fake_embedder


def _all_at_once(target, args_list):
    threads = [threading.Thread(target=target, args=args) for args in args_list]
    for thread in threads:
        thread.start()
    for thread in threads:
        thread.join(timeout=30)
    assert not any(thread.is_alive() for thread in threads), "a caller never returned"


def test_the_embed_line_carries_time_and_inflight(fake, monkeypatch, capsys):
    fake.install(props=_slots(2))
    answer = urllib.request.urlopen
    both_arrived = threading.Barrier(2, timeout=10)

    def held_until_both_arrive(req, *args, **kwargs):
        if req.full_url.endswith("/v1/embeddings"):
            both_arrived.wait()
        return answer(req, *args, **kwargs)

    monkeypatch.setattr(urllib.request, "urlopen", held_until_both_arrive)

    _all_at_once(embed._embed_batch, [(["a"],), (["b", "c"],)])

    lines = [LINE.match(line) for line in capsys.readouterr().out.splitlines() if line.startswith("[embed]")]
    assert all(lines), "an [embed] line does not match the format"
    assert sorted(m["chunks"] for m in lines) == ["1", "2"]
    assert max(int(m["inflight"]) for m in lines) == 2


def test_a_lone_request_counts_itself(fake, capsys):
    fake.install(props=_slots(1))

    embed._embed_batch(["a"])

    line = LINE.match(capsys.readouterr().out.strip())
    assert (line["inflight"], line["waiting"]) == ("1", "0")


def test_requests_queued_for_the_one_slot_are_counted_as_waiting(fake, capsys):
    """With one slot, in flight reads 1 however many requests are queued behind it, so only
    the waiting count can show a burst."""
    fake.install(props=_slots(1), hold_s=0.3)

    _all_at_once(embed._embed_batch, [([f"chunk {i}"],) for i in range(4)])

    lines = [LINE.match(line) for line in capsys.readouterr().out.splitlines() if line.startswith("[embed]")]
    assert all(lines) and len(lines) == 4, "an [embed] line does not match the format"
    assert {m["inflight"] for m in lines} == {"1"}
    assert max(int(m["waiting"]) for m in lines) >= 1
    assert embed._waiting == 0


def test_a_failed_request_raises_and_leaves_the_count_where_it_found_it(fake, monkeypatch):
    fake.install(props=_slots(1))
    monkeypatch.setenv("SPACE_THAT_REFUSES", embed.SPACE)

    with pytest.raises(embed.EmbedRequestError):
        embed._embed_batch([fake_embedder.REFUSED])

    assert (embed._inflight, embed._waiting) == (0, 0)


@pytest.mark.parametrize("slots", [1, 2])
def test_requests_never_exceed_the_sidecars_slots(fake, slots):
    fake.install(props=_slots(slots), hold_s=0.2)

    _all_at_once(embed._embed_batch, [([f"chunk {i}"],) for i in range(8)])

    assert fake.requests() and fake.peak() == slots


@pytest.mark.parametrize("props", [
    pytest.param(None, id="refused"),
    pytest.param((401, b"Unauthorized"), id="401"),
    pytest.param((200, b"<html>not json</html>"), id="malformed"),
    pytest.param((200, b'{"model_path": "embeddinggemma.gguf"}'), id="missing"),
    pytest.param(_slots("4"), id="not-an-int"),
    pytest.param(_slots(True), id="a-bool"),
    pytest.param(_slots(0), id="zero"),
    pytest.param(_slots(-3), id="negative"),
])
def test_a_failed_props_bounds_to_one(fake, monkeypatch, props):
    if props is None:
        # Nothing listens on port 9 here, so this is a real refused connection.
        monkeypatch.setattr(embed, "BASE_URL", "http://127.0.0.1:9")
    fake.install(props=props)

    assert embed._total_slots() == 1


def test_props_is_read_from_the_server_root_and_never_carries_a_key(fake, monkeypatch):
    """Only the local route reads /props, and that route has no credential; a key held for
    any other route must not reach the sidecar through this probe."""
    fake.install(props=_slots(1))
    answer = urllib.request.urlopen
    asked = []

    def recording(req, *args, **kwargs):
        if req.full_url.endswith("/props"):
            asked.append(req)
        return answer(req, *args, **kwargs)

    monkeypatch.setattr(urllib.request, "urlopen", recording)
    monkeypatch.setattr(embed, "BASE_URL", "http://sidecar:8081/v1/")
    monkeypatch.setattr(embed, "_API_KEY", "sk-provider")

    embed._total_slots()

    [req] = asked
    assert req.full_url == "http://sidecar:8081/props"
    assert req.get_header("Authorization") is None


def test_a_refused_props_closes_its_response(monkeypatch):
    body = io.BytesIO(b"Unauthorized")
    # Held so the error outlives the except block: CPython's refcounting would otherwise close
    # the body when the error is collected, and this would pass whether or not it is closed.
    raised = []

    def refused(req, *_args, **_kwargs):
        raised.append(urllib.error.HTTPError(req.full_url, 401, "Unauthorized", None, body))
        raise raised[-1]

    monkeypatch.setattr(embed.urllib.request, "urlopen", refused)

    assert embed._total_slots() == 1
    assert body.closed, "the refused /props left its connection open"


def test_props_is_read_once(fake):
    fake.install(props=_slots(2))

    _all_at_once(embed._embed_batch, [([f"chunk {i}"],) for i in range(6)])
    embed._embed_batch(["later"])

    assert fake.props_requests() == 1


def test_a_hosted_route_is_not_bounded_and_never_reads_props(fake, monkeypatch):
    fake.install(props=_slots(1))
    monkeypatch.setattr(embed, "MODEL", "vendor/embed")
    monkeypatch.setattr(embed, "_API_KEY", "sk-test-key")
    taken = threading.BoundedSemaphore(1)
    taken.acquire()
    monkeypatch.setattr(embed, "_bound", taken)

    _all_at_once(embed._embed_batch, [([f"chunk {i}"],) for i in range(4)])

    assert fake.requests() == 4
    assert fake.props_requests() == 0


def _fails_with(fake, monkeypatch, failure):
    """The fake's /props, and a /v1/embeddings that fails the way `failure` says."""
    answer = urllib.request.urlopen

    def urlopen(req, *args, **kwargs):
        if req.full_url.endswith("/v1/embeddings"):
            return failure(req)
        return answer(req, *args, **kwargs)

    monkeypatch.setattr(urllib.request, "urlopen", urlopen)


def _http_500(req):
    raise urllib.error.HTTPError(req.full_url, 500, "boom", None, io.BytesIO(b"boom"))


def _http_400(req):
    raise urllib.error.HTTPError(req.full_url, 400, "refused", None, io.BytesIO(b"refused"))


def _timed_out(_req):
    raise TimeoutError("timed out")


def _malformed(_req):
    return io.BytesIO(b"<html>502 Bad Gateway</html>")


@pytest.mark.parametrize("failure, raised", [
    pytest.param(_http_500, embed.EmbedRequestError, id="http-error"),
    pytest.param(_timed_out, TimeoutError, id="timeout"),
    pytest.param(_malformed, ValueError, id="malformed"),
])
def test_the_permit_is_released_on_every_failure(fake, monkeypatch, failure, raised):
    fake.install(props=_slots(1))
    _fails_with(fake, monkeypatch, failure)

    with pytest.raises(raised):
        embed._embed_batch(["a"])

    permits = embed._inflight_bound()
    assert permits.acquire(blocking=False), "the failed request kept its slot"
    permits.release()


def test_a_split_batch_gives_its_permit_back(fake, monkeypatch):
    """A refused input leaves through RetryWithSmallerBatch, and the split that follows needs
    the slot the refused request held."""
    fake.install(props=_slots(1))
    _fails_with(fake, monkeypatch, _http_400)

    with pytest.raises(coco.RetryWithSmallerBatch):
        embed._embed_texts(["uno", "due"])

    permits = embed._inflight_bound()
    assert permits.acquire(blocking=False), "the split batch kept its slot"
    permits.release()


def test_the_deadline_starts_after_the_wait(fake, monkeypatch):
    fake.install(props=_slots(1))
    answer = urllib.request.urlopen
    sent = []

    def recording(req, *args, **kwargs):
        if req.full_url.endswith("/v1/embeddings"):
            sent.append((time.monotonic(), kwargs.get("timeout")))
        return answer(req, *args, **kwargs)

    monkeypatch.setattr(urllib.request, "urlopen", recording)
    permits = embed._inflight_bound()
    permits.acquire()
    caller = threading.Thread(target=embed._embed_batch, args=(["a"],))
    caller.start()
    time.sleep(0.3)
    assert sent == [], "the request went out while the only slot was taken"
    freed_at = time.monotonic()
    permits.release()
    caller.join(timeout=10)

    [(sent_at, timeout)] = sent
    assert sent_at >= freed_at
    assert timeout == 120
