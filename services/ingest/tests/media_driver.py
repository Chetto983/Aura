"""One catch-up pass of _extract over one media file, for test_billed_media_memo.py.

    python -m ingest.tests.media_driver <path> <content type>

The object comes from argv instead of Garage, and the vision and speech-to-text bridge is a
stub the test puts first on PATH so it can count calls. _extract, media.index_text and the
memos around the billed calls are production code.
"""

import pathlib
import sys

import cocoindex as coco

from ingest import app, media


@coco.lifespan
async def _lifespan(builder: coco.EnvironmentBuilder):
    # Replaces app.py's lifespan (CocoIndex keeps the last one registered): the media route
    # fingerprint, and nothing else, since nothing here reads the bucket or writes ArcadeDB.
    builder.provide(media.CONFIG_FINGERPRINT, media.fingerprint())
    yield


@coco.fn(memo=True)
async def process_object(item: tuple[str, str]) -> None:
    path, content_type = item
    source = pathlib.Path(path)
    app._extract(source.read_bytes(), source.suffix, source.name, content_type)


@coco.fn
async def driver_main(path: str, content_type: str) -> None:
    await coco.mount_each(process_object, [(pathlib.Path(path).name, (path, content_type))])


if __name__ == "__main__":
    coco.App(coco.AppConfig(name="media-driver"), driver_main, sys.argv[1], sys.argv[2]).update_blocking()
