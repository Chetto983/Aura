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
# every image .env names under ghcr has pulled from the bridge. The updater moves the other
# keys only to images already on the host, so the whole set is fetched here, where no service
# timeout runs. Removed from the repo on the return trip to ghcr.

set -euo pipefail

GHCR_HOME=ghcr.io/chetto1983
BRIDGE_HOME=registry.gitlab.com/chetto1983/aura
# A move adds up to 10 GiB on a host sharing no layer with the new images (the published set,
# measured 2026-10-14; the containerd image store keeps each layer compressed and extracted),
# and the old images stay on disk until the first apply finishes. The rest is the databases'.
# Compared in KiB: df -BG rounds up, so 11.9 GiB would pass as 12.
MIN_FREE_GB=12

say() { echo "aura-registry-bridge: $*"; }
die() {
  say "$* -- nothing was changed." >&2
  exit 1
}

# "Aura applies it after 15 idle minutes" needs the timer's ticks, and the lab VM's timer was found
# inactive on 2026-10-13. It is started before .env changes, so a timer that cannot start
# refuses the move.
enable_timer() {
  systemctl enable --now aura-image-update.timer >/dev/null 2>&1 || die "aura-image-update.timer does not start"
}

start_updater() {
  systemctl start --no-block aura-image-update.service
  say "the updater is running: journalctl -fu aura-image-update.service"
  say "the cockpit will offer the update; 'Update now' applies it, or Aura applies it after 15 idle minutes."
}

# Prints the free KiB of the tightest filesystem holding images, and its directory; fails when
# Docker does not answer. With the containerd image store, images live under containerd's own
# root, "a separate storage path from the Docker data directory" that `docker info` does not
# report (docs.docker.com/engine/storage/containerd); containerd's config names it. The same
# measure is image_store_free in deploy/aura-image-home.sh, which this one-time script, piped
# from curl, cannot source.
image_store_free() {
  local info dir free least='' where=''
  local -a dirs
  info="$(docker info --format '{{.DockerRootDir}} {{json .DriverStatus}}' 2>/dev/null)" && [[ -n "${info}" ]] || return 1
  dirs=("${info%% *}")
  if [[ "${info}" == *io.containerd.snapshotter* ]]; then
    dir="$(containerd config dump 2>/dev/null | sed -n "s/^root = [\"']\(.*\)[\"']$/\1/p")"
    dirs+=("${dir:-/var/lib/containerd}")
  fi
  for dir in "${dirs[@]}"; do
    free="$(df -P -k "${dir}" | awk 'NR == 2 { print $4 }' | tr -dc 0-9)" && [[ -n "${free}" ]] || return 1
    if [[ -z "${least}" ]] || ((free < least)); then least="${free}" where="${dir}"; fi
  done
  echo "${least} ${where}"
}

# Prints the bridge copy of every image .env names under ghcr, AURA_IMAGE's first.
bridge_images() {
  local value
  while IFS='=' read -r _ value; do
    value="$(tr -d "\"'" <<<"${value}")"
    [[ "${value}" != "${GHCR_HOME}/"* ]] || echo "${BRIDGE_HOME}/${value#"${GHCR_HOME}"/}"
  done < <(grep -E '^AURA_IMAGE=' "$1"; grep -E '^AURA_[A-Z0-9_]+_IMAGE=' "$1" || true)
}

main() {
  local env_file updater current target store free where need image defaults="${AURA_DEFAULTS_FILE:-/etc/default/aura}"
  [[ "$(id -u)" == 0 ]] || die "run it as root (sudo)"
  # shellcheck source=/dev/null
  [[ ! -f "${defaults}" ]] || source "${defaults}"
  env_file="${INSTALL_DIR:-/opt/aura}/.env"
  updater="${AURA_IMAGE_UPDATE_BIN:-/usr/local/sbin/aura-image-update.sh}"
  [[ -f "${env_file}" ]] || die "no ${env_file}"
  current="$(sed -n 's/^AURA_IMAGE=//p' "${env_file}" | tail -n 1 | tr -d "\"'")"
  case "${current}" in
    "${BRIDGE_HOME}/"*)
      say "AURA_IMAGE already names the GitLab bridge."
      enable_timer
      start_updater
      return 0
      ;;
    "${GHCR_HOME}/"*) ;;
    *) die "AURA_IMAGE names neither ${GHCR_HOME} nor ${BRIDGE_HOME}" ;;
  esac
  grep -q '^sync_payload()' "${updater}" 2>/dev/null ||
    die "${updater} predates the payload sync and could never install its successor"
  store="$(image_store_free)" || die "Docker does not answer: start it (systemctl start docker) and run this again"
  read -r free where <<<"${store}"
  need=$((MIN_FREE_GB * 1048576))
  ((free >= need)) ||
    die "${where} has $((free / 1048576)) GiB free, ${MIN_FREE_GB} GiB are needed: free $(((need - free + 1048575) / 1048576)) GiB there and run this again"
  while read -r image; do
    say "pulling ${image}"
    docker pull -q "${image}" >/dev/null || die "${image} does not pull"
  done < <(bridge_images "${env_file}")
  enable_timer
  target="${BRIDGE_HOME}/${current#"${GHCR_HOME}"/}"
  sed -i "s|^AURA_IMAGE=.*|AURA_IMAGE=${target}|" "${env_file}"
  say "AURA_IMAGE now names the GitLab bridge."
  start_updater
}

[[ "${BASH_SOURCE[0]:-$0}" != "$0" ]] || main "$@"
