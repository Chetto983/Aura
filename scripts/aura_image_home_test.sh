#!/usr/bin/env bash
#
# Drives deploy/aura-image-home.sh, sourced with the updater, against a stubbed docker and df.
# The download of a host's image homes happens on every tick, before any consent, and stops
# below the free-space floor; the apply only repoints keys whose home copy is already here,
# and does so before the payload is read from AURA_IMAGE (GitLab bridge, 2026-10-13/14).

set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
fixture="$(mktemp -d)"
trap 'rm -rf "$fixture"' EXIT
fail() {
  echo "FAIL: $*" >&2
  exit 1
}

# shellcheck source=/dev/null
source "$repo_root/deploy/aura-image-update.sh"
# shellcheck source=/dev/null
source "$repo_root/deploy/aura-image-home.sh"
# shellcheck source=/dev/null
source "$repo_root/deploy/aura-update-consent.sh"
set +e # the updater enables errexit when sourced; the cases below test return codes

calls="$fixture/calls.log"
H=registry.gitlab.com/chetto1983/aura
free_kib=$((40 * 1024 * 1024)) containerd_free_kib=$((40 * 1024 * 1024)) docker_down="" local_images="" pull_fails=""
# pull_retrying waits 10 s between tries; the test does not.
sleep() { :; }
# The shape `df -P -k` prints: the free space is the fourth field of the second line, in KiB.
# containerd's root sits on a filesystem of its own.
df() {
  local kib="$free_kib"
  [[ "${*: -1}" != /srv/containerd ]] || kib="$containerd_free_kib"
  printf 'Filesystem 1024-blocks Used Available Capacity Mounted on\n/dev/sda2 59768832 17825792 %s 30%% /\n' "$kib"
}
containerd() { [[ "$*" == 'config dump' ]] && echo "root = '/srv/containerd'"; }
docker() {
  echo "docker $*" >>"$calls"
  case "$1 $2" in
    'info --format') [[ -z "$docker_down" ]] && echo '/var/lib/docker [["driver-type","io.containerd.snapshotter.v1"]]' ;;
    'pull -q') [[ " ${pull_fails} " != *" $3 "* ]] && local_images+=" $3" ;;
    'image inspect') [[ " ${local_images} " == *" ${!#} "* ]] && echo "sha256:${!#}" ;;
    *) fail "unexpected docker $*" ;;
  esac
}

reset() { # a host whose keys are half on ghcr, nothing of the home on disk yet
  INSTALL_DIR="$fixture/host"
  rm -rf "$INSTALL_DIR"
  mkdir -p "$INSTALL_DIR"
  cat >"$INSTALL_DIR/.env" <<'ENV'
POSTGRES_PASSWORD=do-not-print-me
AURA_IMAGE=ghcr.io/chetto1983/aura:edge
AURA_CADDY_IMAGE="ghcr.io/chetto1983/aura-caddy:edge"
AURA_INGEST_IMAGE=ghcr.io/chetto1983/aura-ingest:edge
AURA_SANDBOX_IMAGE=registry.gitlab.com/chetto1983/aura/aura-sandbox:edge
AURA_STT_IMAGE=hwdsl2/whisper-server:latest
ENV
  chmod 600 "$INSTALL_DIR/.env"
  cp "$INSTALL_DIR/.env" "$fixture/env.before"
  IMAGE_HOME=$H free_kib=$((40 * 1024 * 1024)) containerd_free_kib=$((40 * 1024 * 1024)) docker_down="" local_images="" pull_fails=""
  : >"$calls"
}

# 1. The tick downloads every key's home copy and changes nothing that runs.
reset
fetch_image_homes >"$fixture/fetch.out"
pulled="$(sed -n 's/^docker pull -q //p' "$calls" | sort | tr '\n' ' ')"
[[ "$pulled" == "$H/aura-caddy:edge $H/aura-ingest:edge $H/aura:edge " ]] || fail "fetched: $pulled"
cmp -s "$INSTALL_DIR/.env" "$fixture/env.before" || fail "fetching changed .env"
echo "ok: a tick fetches every key's home copy and leaves .env alone"

# 2. Below the floor nothing is fetched: the old images stay until an apply reclaims them.
reset
free_kib=$((12 * 1024 * 1024 - 1))
fetch_image_homes >"$fixture/fetch.out"
! grep -q '^docker pull' "$calls" || fail "fetched with $free_kib KiB free"
grep -q 'nothing fetched' "$fixture/fetch.out" || fail "the refusal was not logged: $(cat "$fixture/fetch.out")"
# With the containerd image store, images live under containerd's own root, not Docker's.
reset
containerd_free_kib=$((5 * 1024 * 1024))
fetch_image_homes >"$fixture/fetch.out"
! grep -q '^docker pull' "$calls" || fail "fetched with containerd's filesystem at 5 GiB"
grep -q '/srv/containerd has 5 GiB free' "$fixture/fetch.out" || fail "the refusal did not name containerd's root: $(cat "$fixture/fetch.out")"
reset
docker_down=1
fetch_image_homes >"$fixture/fetch.out" || fail "a tick with Docker down failed the fetch"
! grep -q '^docker pull' "$calls" || fail "fetched with Docker down"
grep -q 'Docker does not answer' "$fixture/fetch.out" || fail "Docker down was not logged: $(cat "$fixture/fetch.out")"
echo "ok: a host under 12 GiB free where its images live fetches nothing"

# 3. The apply repoints only keys whose home copy is here, and never downloads.
reset
pull_fails="$H/aura-ingest:edge"
fetch_image_homes >/dev/null
: >"$calls"
move_image_registry >"$fixture/move.out"
! grep -q '^docker pull' "$calls" || fail "the apply downloaded: $(grep '^docker pull' "$calls")"
[[ "$(cat "$INSTALL_DIR/.env")" == "POSTGRES_PASSWORD=do-not-print-me
AURA_IMAGE=$H/aura:edge
AURA_CADDY_IMAGE=$H/aura-caddy:edge
AURA_INGEST_IMAGE=ghcr.io/chetto1983/aura-ingest:edge
AURA_SANDBOX_IMAGE=$H/aura-sandbox:edge
AURA_STT_IMAGE=hwdsl2/whisper-server:latest" ]] || fail "the move left .env as: $(cat "$INSTALL_DIR/.env")"
[[ "$(stat -c %a "$INSTALL_DIR/.env")" == 600 ]] || fail "the move widened .env"
grep -qx "env: AURA_IMAGE moved to $H." "$fixture/move.out" || fail "a move was not logged"
grep -q '^env: AURA_INGEST_IMAGE stays put' "$fixture/move.out" || fail "a key left behind was not logged"
! grep -q 'do-not-print-me\|ghcr.io/chetto1983/aura-ingest\|hwdsl2' "$fixture/move.out" ||
  fail "the move logged .env values: $(cat "$fixture/move.out")"
echo "ok: the apply moves the keys whose home copy is here, and only those"

# 4. Flipping IMAGE_HOME brings every key back, never touching another registry.
IMAGE_HOME=ghcr.io/chetto1983
local_images+=" ghcr.io/chetto1983/aura:edge ghcr.io/chetto1983/aura-caddy:edge ghcr.io/chetto1983/aura-sandbox:edge"
move_image_registry >/dev/null
[[ "$(grep -c '=ghcr.io/chetto1983/' "$INSTALL_DIR/.env")" == 4 ]] ||
  fail "flipping IMAGE_HOME did not bring every key back: $(cat "$INSTALL_DIR/.env")"
grep -qx 'AURA_STT_IMAGE=hwdsl2/whisper-server:latest' "$INSTALL_DIR/.env" || fail "the return trip touched another registry"
echo "ok: flipping the home brings every key back"

# 5. A key left behind is pending work: the next tick fetches it and offers the apply again.
reset
pull_fails="$H/aura-ingest:edge"
fetch_image_homes >/dev/null
move_image_registry >/dev/null
env_value() { sed -n "s/^$1=//p" "$INSTALL_DIR/.env" | tail -n 1; }
applied_image() { image_id "$(env_value AURA_IMAGE)"; }
service_behind() { return 1; }
stale_boxes() { :; }
(cd "$INSTALL_DIR" && update_pending) && fail "a key with no home copy yet counted as pending"
local_images+=" $H/aura-ingest:edge"
(cd "$INSTALL_DIR" && update_pending) || fail "a key whose home copy arrived is not offered as an update"
echo "ok: a key whose home copy arrives later is offered on the next tick"

# 6. The apply moves AURA_IMAGE before reading the payload from it: image and compose file
# must come from the same commit (sync_payload's own rule).
reset
local_images=" $H/aura:edge"
(
  record_status() { :; }
  on_apply_exit() { :; }
  container_image_id() { echo before; }
  sync_payload() {
    echo "sync $1" >>"$calls"
    exit 0
  }
  apply_update
) >/dev/null
grep -qx "sync $H/aura:edge" "$calls" || fail "the payload was read from $(sed -n 's/^sync //p' "$calls"), not the moved AURA_IMAGE"
echo "ok: the apply moves AURA_IMAGE before it syncs the payload"
