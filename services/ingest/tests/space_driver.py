"""One catch-up pass of the production indexing path, for test_embed_space_reruns.py and
test_row_passages.py.

    python -m ingest.tests.space_driver <database> <name>=<text> | <name>=@<path> ...

Production reads each object from Garage; here the objects come from argv, and that is the
only part replaced. `<name>=<text>` is a text file; `<name>=@<path>` is that file's bytes, with
its suffix and the content type its name implies. _extract, chunking, embed.embed_text with the
space as its dependency, and the stamped rows the ArcadeDB target writes are production code.
The embedding server is fake_embedder, so the test can count requests and refuse one input on
purpose.
"""

import mimetypes
import pathlib
import sys

import cocoindex as coco
from cocoindex.connectors import neo4j

from ingest import app, arcade, embed
from ingest.tests import fake_embedder

_database = ""


@coco.lifespan
async def _lifespan(builder: coco.EnvironmentBuilder):
    # Replaces app.py's lifespan (CocoIndex keeps the last one registered): same target, a
    # disposable database, and no S3 client, since nothing here reads the bucket.
    builder.provide(app.KG_DB, neo4j.ConnectionFactory(
        uri=app.ARCADE_BOLT, auth=("root", app.ARCADE_PASSWORD), database=_database))
    yield


@coco.fn(memo=True)
async def process_object(
    item: tuple[str, bytes, str, str | None], table: neo4j.TableTarget[app.Passage],
    documents: neo4j.TableTarget[app.IndexedDocument],
) -> None:
    name, content, suffix, content_type = item
    extracted = app._extract(content, suffix, name, content_type)
    await app.index_object(app._S3_CONFIG.identity_id, name, name, content, extracted, table, documents)


@coco.fn
async def driver_main(objects: dict[str, tuple[bytes, str, str | None]]) -> None:
    table, documents = await app.mount_targets()
    await coco.mount_each(
        process_object, [(name, (name, *found)) for name, found in objects.items()], table, documents,
    )


def _object(arg: str) -> tuple[str, tuple[bytes, str, str | None]]:
    name, value = arg.split("=", 1)
    if not value.startswith("@"):
        return name, (value.encode("utf-8"), ".txt", "text/plain")
    path = pathlib.Path(value[1:])
    # None for a type the name does not imply, as source.object_facts gives for an object
    # stored without one: .xlsx is such a name in this image, which ships no mime.types.
    return name, (path.read_bytes(), path.suffix, mimetypes.guess_type(path.name)[0])


if __name__ == "__main__":
    _database = sys.argv[1]
    objects = dict(_object(arg) for arg in sys.argv[2:])
    fake_embedder.install()
    arcade.ensure_schema(app.ARCADE_HTTP, _database, ("root", app.ARCADE_PASSWORD), embed.DIMENSIONS)
    coco.App(coco.AppConfig(name="space-driver"), driver_main, objects).update_blocking()
    print(f"REQUESTS {fake_embedder.requests()}", flush=True)
