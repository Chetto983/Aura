# Two defects of ArcadeDB's native image, reproduced

Date: 2026-10-17. The operator asked for proof ("Dimostralo") before sending the two findings to
Roberto Franchini.

**Image:** `arcadedata/arcadedb:latest-native` = `26.11.1-SNAPSHOT` at build `15958b48f`, linux/amd64.
**Reference:** `arcadedata/arcadedb:26.10.1`, the JVM image Aura pins.

Every run started a new container. The commands are in the transcripts quoted below, and the
script was `scratchpad/prove/prove.sh` of this session.

**Verdict:**
- **Defect 1.** The native image ships a multi-line `config/server-users.jsonl` that its own
  loader rejects.
  - It shows only when a new container starts on a databases directory that already holds a
    database. That is every recreation of a container whose `config/` is an anonymous volume, as
    in Aura's compose.
  - The log line names `server-groups.json`, while the failing loader is the users one.
  - The JVM image ships no such file and logs nothing.
- **Defect 2.** The amd64 image cannot run as a non-root user without being rebuilt `FROM scratch`.
  - A root-run native server leaves files a JVM server (UID 1000) cannot open, and that JVM then
    exits.
- **Correction.** The repro first drafted for Roberto, a clean start with `defaultDatabases`, does
  not reproduce defect 1: 0 SEVERE lines. It is replaced below by the measured trigger.

## Defect 1: the shipped users file

The file inside each image:
```
--- latest-native: /home/arcadedb/config/server-users.jsonl
{
  "root": {
    "access": [
      "updateSecurity"
    ],
    "databases": {
      "*": [
        "admin"
      ]
    }
  }
}
--- 26.10.1 (JVM): (no such file in image)
```

What triggers it, each row a new container with `-Darcadedb.server.rootPassword=…`:

| Run | databases directory | `defaultDatabases` | SEVERE lines |
|---|---|---|---|
| native, A | empty | `test[]` | 0 |
| native, B | holds `test` from run A | `test[]` | 1 |
| native, C | holds `test` | none | 1 |
| JVM 26.10.1, D | holds `test` | `test[]` | 0 |
| native, clean, nothing mounted | empty | `test[]` | 0 |
| native, clean, nothing mounted | empty | none | 0 |

Run B's log:
```
SEVER [SecurityGroupFileRepository] <ArcadeDB_0> Error on loading file 'server-groups.json', using the default configuration and saving the corrupt file as 'config/server-groups-error.json'
com.arcadedb.serializer.json.JSONException: Invalid JSON object format: {
	at com.arcadedb.serializer.json.JSONObject.<init>(JSONObject.java:89)
	at com.arcadedb.server.security.SecurityUserFileRepository.load(SecurityUserFileRepository.java:139)
	at com.arcadedb.server.security.SecurityUserFileRepository.getUsers(SecurityUserFileRepository.java:117)
Caused by: com.google.gson.JsonSyntaxException: java.io.EOFException: End of input at line 1 column 2 path $.
```

**The exact command sent to Roberto, run as written.**
- The image is `arcadedata/arcadedb:latest-native`, pulled from Docker Hub, digest
  `sha256:357caaf9381f…`, the same as the mirror copy above.
- `/tmp/dbs` did not exist before the first run.

```
docker run --rm -v /tmp/dbs:/home/arcadedb/databases arcadedata/arcadedb:latest-native -Darcadedb.server.rootPassword=prova12345 '-Darcadedb.server.defaultDatabases=test[]'
```

| Run | Log | SEVERE lines |
|---|---|---|
| 1 | `Creating default database 'test'...`, server started | 0 |
| 2, the same command after a stop | server started; the SEVERE above, with `SecurityUserFileRepository.load` | 1 |

After the fallback, root logs in (HTTP 200) and the default database exists. So the cost is the
SEVERE line, and the users file reset to its defaults.

## Defect 2: no non-root run on amd64

| Attempt | Result |
|---|---|
| upstream image, `--user 1000:1000`, nothing mounted | exit 1: `AccessDeniedException: ./log/arcadedb.log.0.lck`. The five anonymous volumes (`backups`, `config`, `databases`, `log`, `replication`) are created `0:0 755` |
| `FROM latest-native`, `COPY --chown=1000:1000` of `config/` and of new empty dirs, `USER 1000:1000` | the image keeps `home/arcadedb/config/` at `0/0`, because the existing directory keeps its owner. Exit 1: `AccessDeniedException: ./config/server-users.jsonl….tmp` |
| `FROM scratch`: `/etc/ssl`, then `/tmp` and `/home/arcadedb` with `--chown=1000:1000`, the upstream `WORKDIR`, `VOLUME`s and `ENTRYPOINT`, and `USER 1000:1000` | ready in 2 s, and `create database` answers `ok`. After a stop, 0 of 8 files are owned by anyone but 1000 |

**ArcadeDB's own documented non-root example fails the same way.**
- `docs/native-image.md` at `15958b48f`, "Running the container", lines 320 to 331, says the UID
  you choose "should own the host-side `databases` bind mount". With `databases` created and
  `chown 1000:1000`, the example was run verbatim on `arcadedata/arcadedb:latest-native`:
  ```
  docker run --rm -p 2480:2480 -p 5432:5432 \
    --user 1000:1000 \
    -v "$(pwd)/databases:/home/arcadedb/databases" \
    arcadedata/arcadedb:latest-native \
    -Darcadedb.server.rootPassword=PlayWithData123!
  ```
  Exit code 1, after:
  ```
  Exception in thread "main" com.arcadedb.server.ServerException …
  java.nio.file.AccessDeniedException: ./log/arcadedb.log.0.lck
  java.io.FileNotFoundException: ./log/server-event-log-….jsonl (Permission denied)
  ```
- **The cause is in `native/src/main/docker/Dockerfile.native.scratch`.**
  - Its `dirs` stage creates `databases`, `backups`, `replication` and `log` and runs
    `chown -R 0:0`.
  - `config/` is copied without `--chown`.
  - All five are declared `VOLUME`. So every directory that is not bind-mounted becomes a
    root-owned anonymous volume, and UID 1000 cannot write `log/` or `config/`.
  - The arm64 Dockerfile chowns the same directories to 65532 and runs as that user, so only amd64
    is affected.

**Why it matters for a JVM rollback.**
- The upstream native image ran as root on a directory owned by 1000 and created database
  `rootmade`. The database directory came out `root(0):root 755`, and its `schema.json`
  `root(0):root 600`.
- The JVM 26.10.1, which runs as `arcadedb` (UID 1000), then started on the same directory and
  exited 1:
  ```
  SEVER [LocalDatabase] Error on loading configuration from file './databases/rootmade/configuration.json'
  SEVER [TransactionManager] Error on WAL file management for file './databases/rootmade/txlog_0.wal'
  ```

## What this does not prove

- **arm64.** ArcadeDB's documentation says that image is distroless `:nonroot` (65532). It was not
  run.
- **26.11.1 as released.** This is a build of main.
- **Why run A logs nothing,** although its config is the same. The trigger is measured, the cause
  inside the loader is not.
