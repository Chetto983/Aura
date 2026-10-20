#!/bin/bash
# Probes the vault, restarts the throwaway ArcadeDB, and checks what survived. Run after run.sh.
H=$(cd "$(dirname "$0")" && pwd)
cd "$H" && python3 -I probe.py || exit 1
pw=$(cat "$H/root.pw")
docker restart pii-spike-arcadedb >/dev/null
for i in $(seq 1 60); do
  curl -s -o /dev/null -w '%{http_code}' -u "root:$pw" http://127.0.0.1:2490/api/v1/server 2>/dev/null | grep -q 200 && break
  sleep 1
done
sleep 2
python3 -I probe.py after-restart
