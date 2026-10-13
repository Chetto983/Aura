#!/usr/bin/env bash
#
# Drives scripts/publish_images.sh against a stubbed docker and git. The images it builds must
# be the ones the GitHub workflows build, nothing may be pushed when a build or an image
# contract fails, and no moving tag may move before every per-commit tag is in the registry --
# with aura:edge last, as publish-aura-edge.yml does since 2026-09-27.

set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
fixture="$(mktemp -d)"
trap 'rm -rf "$fixture"' EXIT
fail() {
  echo "FAIL: $*" >&2
  exit 1
}

# shellcheck source=/dev/null
source "$repo_root/scripts/publish_images.sh"
declare -F main >/dev/null || fail "main undefined after sourcing the publisher"

# The workflows are the reference while GitHub runs them: same Dockerfiles, same edge order.
edge_workflow="$repo_root/.github/workflows/publish-aura-edge.yml"
workflow_files="$(sed -n 's/^ *file: *//p' "$edge_workflow" | sort -u | tr '\n' ' ')"
script_files="$(for e in "${EDGE_IMAGES[@]}"; do echo "${e#*:}"; done | sort -u | tr '\n' ' ')"
[[ "$workflow_files" == "$script_files" ]] || fail "edge Dockerfiles differ: workflow [$workflow_files] script [$script_files]"
workflow_order="$(sed -n 's/^ *for image in \(.*\); do$/\1/p' "$edge_workflow")"
script_order="$(for e in "${EDGE_IMAGES[@]}"; do printf '%s ' "${e%%:*}"; done)"
[[ "$workflow_order " == "$script_order" ]] || fail "edge tag order differs: workflow [$workflow_order] script [$script_order]"
[[ "$(sed -n 's/^ *file: *//p' "$repo_root/.github/workflows/publish-arcadedb-mcp.yml")" == "${ARCADEDB_MCP#*:}" ]] ||
  fail "arcadedb-mcp Dockerfile differs from its workflow"
echo "ok: the publisher builds what the GitHub workflows build, in the edge workflow's order"

calls="$fixture/calls.log"
src="$fixture/src"
mkdir -p "$src/scripts" "$src/fork"
for s in cloudflared_image_contract_test.sh ingest_image_contract_test.sh sandbox_image_contract_test.sh; do
  printf '#!/usr/bin/env bash\n' >"$src/scripts/$s"
done
touch "$src/fork/Dockerfile"
failing_build=""
failing_contract=""
git() {
  echo "git $*" >>"$calls"
  case "$*" in
    *' rev-parse '*) echo 0123456789abcdef0123456789abcdef01234567 ;;
    *' archive '*) tar -c -C "$src" . ;;
    'clone --quiet '*) mkdir -p "${*: -1}" && cp -R "$src/fork/." "${*: -1}" ;;
    *' checkout --quiet '*) ;;
    *) fail "unexpected git $*" ;;
  esac
}
docker() {
  echo "docker $*" >>"$calls"
  if [[ "$1" == build && -n "$failing_build" && "$*" == *"/$failing_build:"* ]]; then return 1; fi
  case "$1 ${2:-}" in
    'build '* | 'push '* | 'buildx imagetools') ;;
    *) fail "unexpected docker $*" ;;
  esac
}
bash() {
  echo "contract $*" >>"$calls"
  [[ -z "$failing_contract" || "$1" != *"$failing_contract"* ]]
}
export AURA_PUBLISH_REGISTRY=registry.example/aura

: >"$calls"
main HEAD >"$fixture/out" || fail "a clean publish failed: $(tail -5 "$calls")"
pushes="$(grep -c '^docker push ' "$calls")"
[[ "$pushes" == 9 ]] || fail "expected 9 per-commit pushes, saw $pushes"
last_push="$(grep -n '^docker push ' "$calls" | tail -1 | cut -d: -f1)"
first_move="$(grep -n '^docker buildx imagetools create' "$calls" | head -1 | cut -d: -f1)"
((last_push < first_move)) || fail "a moving tag moved before every per-commit tag was pushed"
grep '^docker buildx imagetools create' "$calls" | tail -1 | grep -q -- '--tag registry.example/aura/aura:edge ' ||
  fail "aura:edge did not move last: $(grep '^docker buildx' "$calls" | tail -1)"
grep -q '^docker build .*--build-arg VCS_REF=0123456789abcdef0123456789abcdef01234567' "$calls" ||
  fail "aura was not stamped with its commit"
grep -q -- '--tag registry.example/aura/aura-pim-mcp:sidecar ' "$calls" || fail "the pim fork's sidecar tag did not move"
grep -q -- '--tag registry.example/aura/whatsapp-mcp:latest ' "$calls" || fail "the whatsapp fork's latest tag did not move"
echo "ok: every per-commit tag is pushed before any moving tag moves, and aura:edge moves last"

for case in build contract; do
  : >"$calls"
  if [[ "$case" == build ]]; then failing_build=aura-ingest; else failing_contract=sandbox_image_contract_test.sh; fi
  if main HEAD >"$fixture/out" 2>&1; then fail "a failed $case still published"; fi
  ! grep -q '^docker push ' "$calls" || fail "a failed $case pushed: $(grep '^docker push ' "$calls")"
  failing_build="" failing_contract=""
done
echo "ok: a failed build or image contract pushes nothing"
