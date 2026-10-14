#!/usr/bin/env bash
#
# One-time bootstrap for an Aura appliance whose updater cannot pull its successor: GitHub
# blocks ghcr.io/chetto1983 since 2026-10-11. It points AURA_IMAGE at the GitLab bridge and
# starts the updater; the aura image pulled from there carries the updater that moves every
# other image key on its own (docs/superpowers/specs/2026-10-13-gitlab-image-bridge-design.md).
#
#   curl -fsSL https://gitlab.com/Chetto1983/aura/-/raw/master/deploy/aura-registry-bridge.sh | sudo bash
#
# It refuses, and changes nothing, unless it runs as root on an appliance whose updater syncs
# its payload, whose AURA_IMAGE is under a known home, with room for the new images, and once
# the new aura image has pulled. Removed from the repo on the return trip to ghcr.

set -euo pipefail

GHCR_HOME=ghcr.io/chetto1983
BRIDGE_HOME=registry.gitlab.com/chetto1983/aura
# A move adds up to 10 GiB on a host sharing no layer with the new images (the published set,
# measured 2026-10-14; the containerd image store keeps each layer compressed and extracted),
# and the old images stay on disk until the first apply finishes. The rest is the databases'.
MIN_FREE_GB=12

say() { echo "aura-registry-bridge: $*"; }
die() {
  say "$* -- nothing was changed." >&2
  exit 1
}

start_updater() {
  systemctl start --no-block aura-image-update.service
  say "the updater is running: journalctl -fu aura-image-update.service"
  say "the cockpit will offer the update; 'Update now' applies it, or Aura applies it after 15 idle minutes."
}

main() {
  local env_file updater current target free
  [[ "$(id -u)" == 0 ]] || die "run it as root (sudo)"
  # shellcheck source=/dev/null
  [[ ! -f /etc/default/aura ]] || source /etc/default/aura
  env_file="${INSTALL_DIR:-/opt/aura}/.env"
  updater="${AURA_IMAGE_UPDATE_BIN:-/usr/local/sbin/aura-image-update.sh}"
  [[ -f "${env_file}" ]] || die "no ${env_file}"
  current="$(sed -n 's/^AURA_IMAGE=//p' "${env_file}" | tail -n 1 | tr -d "\"'")"
  case "${current}" in
    "${BRIDGE_HOME}/"*)
      say "AURA_IMAGE already names the GitLab bridge."
      start_updater
      return 0
      ;;
    "${GHCR_HOME}/"*) ;;
    *) die "AURA_IMAGE names neither ${GHCR_HOME} nor ${BRIDGE_HOME}" ;;
  esac
  grep -q '^sync_payload()' "${updater}" 2>/dev/null ||
    die "${updater} predates the payload sync and could never install its successor"
  free="$(df -P -BG "$(docker info --format '{{.DockerRootDir}}')" | awk 'NR == 2 { print $4 }' | tr -dc 0-9)"
  ((${free:-0} >= MIN_FREE_GB)) || die "Docker has ${free:-0} GB free, ${MIN_FREE_GB} GB are needed"
  target="${BRIDGE_HOME}/${current#"${GHCR_HOME}"/}"
  say "pulling ${target}"
  docker pull -q "${target}" >/dev/null || die "${target} does not pull"
  sed -i "s|^AURA_IMAGE=.*|AURA_IMAGE=${target}|" "${env_file}"
  say "AURA_IMAGE now names the GitLab bridge."
  start_updater
}

[[ "${BASH_SOURCE[0]:-$0}" != "$0" ]] || main "$@"
