#!/bin/bash
# Builds the corpus and runs measure.py. The operator's OpenRouter key is read from the repo's
# .env.google (OPEN_ROUTE_API_KEY) into this process's environment only; it is never echoed.
set +x
H=$(cd "$(dirname "$0")" && pwd)
key=$(sed -n 's/^OPEN_ROUTE_API_KEY=//p' "$H/../../.env.google" 2>/dev/null | head -1 | tr -d '\r' | sed 's/^["'"'"']//; s/["'"'"']$//')
cd "$H" && python3 -I corpus.py || exit 1
if [ -n "$key" ]; then OPENROUTER_API_KEY="$key" python3 -I measure.py "$@"; else python3 -I measure.py "$@"; fi
rc=$?
unset key
exit $rc
