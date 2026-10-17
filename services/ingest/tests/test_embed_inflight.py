"""The embed line says when a request went out and how many were in the air with it.

An ingest that times out on its embedding calls looks the same whether one request was slow
or thirty were sent at once; only the count at the moment of sending tells the two apart.
"""

import re
import threading
import urllib.request

from ingest import embed
from ingest.tests import fake_embedder

LINE = re.compile(
    r"^\[embed\] (?P<time>\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}\.\d{3}\+00:00) "
    r"inflight=(?P<inflight>\d+) (?P<chunks>\d+) chunk\(s\) in one request$"
)


def test_the_embed_line_carries_time_and_inflight(monkeypatch, capsys):
    monkeypatch.setattr(urllib.request, "urlopen", urllib.request.urlopen)
    fake_embedder.install()
    answer = urllib.request.urlopen
    both_arrived = threading.Barrier(2, timeout=10)

    def held_until_both_arrive(req, *args, **kwargs):
        if req.full_url.endswith("/v1/embeddings"):
            both_arrived.wait()
        return answer(req, *args, **kwargs)

    monkeypatch.setattr(urllib.request, "urlopen", held_until_both_arrive)

    threads = [
        threading.Thread(target=embed._embed_batch, args=(["a"],)),
        threading.Thread(target=embed._embed_batch, args=(["b", "c"],)),
    ]
    for thread in threads:
        thread.start()
    for thread in threads:
        thread.join()

    lines = [LINE.match(line) for line in capsys.readouterr().out.splitlines() if line.startswith("[embed]")]
    assert all(lines), "an [embed] line does not match the format"
    assert sorted(m["chunks"] for m in lines) == ["1", "2"]
    assert max(int(m["inflight"]) for m in lines) == 2


def test_a_lone_request_counts_itself(monkeypatch, capsys):
    monkeypatch.setattr(urllib.request, "urlopen", urllib.request.urlopen)
    fake_embedder.install()

    embed._embed_batch(["a"])

    assert LINE.match(capsys.readouterr().out.strip())["inflight"] == "1"


def test_a_failed_request_leaves_the_count_where_it_found_it(monkeypatch):
    monkeypatch.setattr(urllib.request, "urlopen", urllib.request.urlopen)
    fake_embedder.install()
    monkeypatch.setenv("SPACE_THAT_REFUSES", embed.SPACE)

    try:
        embed._embed_batch([fake_embedder.REFUSED])
    except embed.EmbedRequestError:
        pass

    assert embed._inflight == 0
