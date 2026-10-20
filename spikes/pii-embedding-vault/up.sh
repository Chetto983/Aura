#!/bin/bash
# A throwaway ArcadeDB 26.10.1 (the compose pin) with the Redis plugin: HTTP on 127.0.0.1:2490,
# RESP on 127.0.0.1:6390, no volume. The root password is generated into root.pw (git-ignored).
H=$(cd "$(dirname "$0")" && pwd)
[ -s "$H/root.pw" ] || head -c 24 /dev/urandom | base64 | tr -dc 'A-Za-z0-9' > "$H/root.pw"
pw=$(cat "$H/root.pw")
docker rm -f pii-spike-arcadedb >/dev/null 2>&1
docker run -d --name pii-spike-arcadedb -p 127.0.0.1:2490:2480 -p 127.0.0.1:6390:6379 --memory 1g \
  -e ARCADEDB_OPTS_MEMORY="-Xms512M -Xmx512M" \
  -e JAVA_OPTS="-Darcadedb.server.rootPassword=$pw -Darcadedb.server.plugins=Redis:com.arcadedb.redis.RedisProtocolPlugin" \
  arcadedata/arcadedb@sha256:01506549bcb4a6d16af678ee43718850eba605430a3720a6b0398cb0470c710d >/dev/null
for i in $(seq 1 60); do
  curl -s -o /dev/null -w '%{http_code}' -u "root:$pw" http://127.0.0.1:2490/api/v1/server 2>/dev/null | grep -q 200 && { echo "arcadedb up after ${i}s"; exit 0; }
  sleep 1
done
echo "arcadedb did not answer in 60 s"; exit 1
