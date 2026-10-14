#!/usr/bin/env bash
#
# Drives deploy/aura-registry-bridge.sh against a stubbed id, docker, df and systemctl. Every
# refusal must leave .env byte-identical; a run that proceeds changes AURA_IMAGE and nothing
# else, and starts the updater; a second run changes nothing and only starts the updater.

set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
fixture="$(mktemp -d)"
trap 'rm -rf "$fixture"' EXIT
fail() {
  echo "FAIL: $*" >&2
  exit 1
}

# shellcheck source=/dev/null
source "$repo_root/deploy/aura-registry-bridge.sh"
declare -F main >/dev/null || fail "main undefined after sourcing the patch"

calls="$fixture/calls.log"
export INSTALL_DIR="$fixture/aura" AURA_IMAGE_UPDATE_BIN="$fixture/updater.sh"
# main sources the appliance's defaults, which on an appliance name the real INSTALL_DIR: run as
# root there, a stubbed pull would move the real .env. The test reads its own file instead.
export AURA_DEFAULTS_FILE="$fixture/defaults"
: >"$AURA_DEFAULTS_FILE"
uid=0 free_kib=$((40 * 1048576)) broken_pull=''
id() { echo "$uid"; }
# What `df -P` prints in the unit asked for: the free space is the fourth field of the second
# line, and -BG rounds it up to a whole GiB.
df() {
  local free="${free_kib}K"
  [[ "$1 $2" != '-P -BG' ]] || free="$(((free_kib + 1048575) / 1048576))G"
  printf 'Filesystem Blocks Used Available Capacity Mounted on\n/dev/sda2 57G 17G %s 30%% /\n' "$free"
}
docker() {
  echo "docker $*" >>"$calls"
  case "$1" in
    info) echo /var/lib/docker ;;
    pull) [[ "$3" != "${broken_pull}" ]] ;;
    *) fail "unexpected docker $*" ;;
  esac
}
systemctl() { echo "systemctl $*" >>"$calls"; }

reset() {
  rm -rf "$INSTALL_DIR"
  mkdir -p "$INSTALL_DIR"
  printf 'POSTGRES_PASSWORD=do-not-print-me\nAURA_IMAGE=ghcr.io/chetto1983/aura:edge\nAURA_CADDY_IMAGE=ghcr.io/chetto1983/aura-caddy:edge\n' >"$INSTALL_DIR/.env"
  chmod 600 "$INSTALL_DIR/.env"
  printf 'sync_payload() {\n  :\n}\n' >"$AURA_IMAGE_UPDATE_BIN"
  cp "$INSTALL_DIR/.env" "$fixture/env.before"
  : >"$calls"
  uid=0 free_kib=$((40 * 1048576)) broken_pull=''
}

refuses() { # <why> <its message> : main must fail for that reason and leave .env byte-identical
  # A refusal exits the script, as it must when run from curl, so it runs in a subshell here.
  if (main) >"$fixture/out" 2>&1; then fail "$1: the patch went on"; fi
  grep -q "$2" "$fixture/out" || fail "$1: refused for another reason: $(cat "$fixture/out")"
  cmp -s "$INSTALL_DIR/.env" "$fixture/env.before" || fail "$1: .env changed"
  ! grep -q '^systemctl' "$calls" || fail "$1: the updater was started"
}

reset; uid=1000; refuses "not root" "as root"
reset; printf 'AURA_IMAGE=example.invalid/aura:edge\n' >"$INSTALL_DIR/.env"; cp "$INSTALL_DIR/.env" "$fixture/env.before"; refuses "AURA_IMAGE under neither home" "names neither"
reset; printf 'echo too old\n' >"$AURA_IMAGE_UPDATE_BIN"; refuses "an updater with no payload sync" "predates the payload sync"
reset; free_kib=$((3 * 1048576)); refuses "too little free space" "3 GiB free"
# The lab VM had 11 GiB free; a move adds up to 10 GiB, which would leave its databases no room.
reset; free_kib=$((11 * 1048576)); refuses "free space without a margin over the measured set" "11 GiB free"
# df -BG rounds up: 11.9 GiB free printed as 12G once let a host through.
reset; free_kib=$((12 * 1048576 - 1)); refuses "free space just under the floor" "11 GiB free"
reset; broken_pull=registry.gitlab.com/chetto1983/aura/aura:edge; refuses "the aura image does not pull" "aura:edge does not pull"
# The updater moves every other key without pulling, inside a 15-minute start; the patch is the
# one place with no deadline, so it fetches the whole set before it moves anything.
reset; broken_pull=registry.gitlab.com/chetto1983/aura/aura-caddy:edge; refuses "a sidecar image does not pull" "aura-caddy:edge does not pull"
echo "ok: every refusal leaves .env byte-identical and starts nothing"

reset
main >"$fixture/out"
[[ "$(cat "$INSTALL_DIR/.env")" == "POSTGRES_PASSWORD=do-not-print-me
AURA_IMAGE=registry.gitlab.com/chetto1983/aura/aura:edge
AURA_CADDY_IMAGE=ghcr.io/chetto1983/aura-caddy:edge" ]] || fail "the patch left .env as: $(cat "$INSTALL_DIR/.env")"
[[ "$(stat -c %a "$INSTALL_DIR/.env")" == 600 ]] || fail "the patch widened .env"
grep -qx 'docker pull -q registry.gitlab.com/chetto1983/aura/aura:edge' "$calls" || fail "the patch did not pull first"
grep -qx 'docker pull -q registry.gitlab.com/chetto1983/aura/aura-caddy:edge' "$calls" || fail "the patch left a sidecar image to the updater's deadline"
grep -qx 'systemctl start --no-block aura-image-update.service' "$calls" || fail "the updater was not started"
! grep -q 'do-not-print-me' "$fixture/out" || fail "the patch printed .env values"
echo "ok: the patch moves AURA_IMAGE alone and starts the updater"

cp "$INSTALL_DIR/.env" "$fixture/env.before"
: >"$calls"
main >"$fixture/out"
cmp -s "$INSTALL_DIR/.env" "$fixture/env.before" || fail "a second run changed .env"
! grep -q '^docker pull' "$calls" || fail "a second run pulled again"
grep -qx 'systemctl start --no-block aura-image-update.service' "$calls" || fail "a second run did not start the updater"
echo "ok: a second run only starts the updater"

reset
mkdir -p "$fixture/from-defaults"
cp "$fixture/env.before" "$fixture/from-defaults/.env"
printf 'INSTALL_DIR=%s\n' "$fixture/from-defaults" >"$AURA_DEFAULTS_FILE"
(main) >"$fixture/out"
grep -qx 'AURA_IMAGE=registry.gitlab.com/chetto1983/aura/aura:edge' "$fixture/from-defaults/.env" ||
  fail "the patch did not take INSTALL_DIR from AURA_DEFAULTS_FILE"
cmp -s "$INSTALL_DIR/.env" "$fixture/env.before" || fail "the patch moved a .env its defaults do not name"
: >"$AURA_DEFAULTS_FILE"
echo "ok: the patch reads its defaults from AURA_DEFAULTS_FILE"
