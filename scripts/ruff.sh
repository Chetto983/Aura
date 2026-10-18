#!/usr/bin/env bash
# Run ruff with the repository's ruff.toml, wherever it is installed: the `ruff` binary (CI, pipx),
# or the module of whichever Python has it (`pip install --user`, Windows included). The version is
# pinned in ruff.toml (required-version), so a different one refuses to run instead of linting by
# other rules. Called by lefthook pre-commit with `check --force-exclude {staged_files}` and by
# `make py-lint`; safe standalone: scripts/ruff.sh check .
set -euo pipefail

cd "$(dirname "$0")/.."

if command -v ruff >/dev/null 2>&1; then
  exec ruff "$@"
fi
for python in python3 python; do
  if "$python" -m ruff --version >/dev/null 2>&1; then
    exec "$python" -m ruff "$@"
  fi
done
echo "ruff is not installed: pip install --user ruff==0.17.1 (the version ruff.toml requires)" >&2
exit 1
