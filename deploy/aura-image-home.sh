#!/usr/bin/env bash
#
# Sourced by deploy/aura-image-update.sh: how a host's image keys reach IMAGE_HOME, the home the
# updater names (docs/superpowers/specs/2026-10-13-gitlab-image-bridge-design.md). The download
# runs on every tick, before any consent, so the apply only repoints keys at images already on
# the host: pulling the set inside the apply took 10 of the service's 15 minutes on the lab VM
# (2026-10-14), which a slower link would not finish. Removed with the bridge.

# The published set takes up to 10 GiB on a host that shares no layer with it, because the
# containerd image store keeps each layer compressed and extracted (measured 2026-10-14). The old
# images stay until the apply reclaims them, so the rest is left to the databases.
MOVE_MIN_FREE_KIB=$((12 * 1024 * 1024))

# Prints "KEY TARGET" for every AURA_*_IMAGE key of .env that names a repo-built image under
# another home, TARGET being the same name and tag under IMAGE_HOME.
image_keys_elsewhere() {
  local env_file="${INSTALL_DIR}/.env" key value home
  [[ -f "${env_file}" ]] || return 0
  while IFS='=' read -r key value; do
    value="$(tr -d "\"'" <<<"${value}")"
    for home in "${IMAGE_HOMES[@]}"; do
      [[ "${home}" != "${IMAGE_HOME}" && "${value}" == "${home}/"* ]] || continue
      printf '%s %s\n' "${key}" "${IMAGE_HOME}/${value#"${home}"/}"
    done
  done < <(grep -E '^AURA_([A-Z0-9_]+_)?IMAGE=' "${env_file}" || true)
}

docker_free_kib() {
  df -P -k "$(docker info --format '{{.DockerRootDir}}')" | awk 'NR == 2 { print $4 }'
}

# Downloads the home copy of every key still elsewhere. It changes nothing that runs, so a tick
# killed halfway loses only time, and a pull that fails is simply tried again next tick.
fetch_image_homes() {
  local keys key target free
  keys="$(image_keys_elsewhere)"
  [[ -n "${keys}" ]] || return 0
  free="$(docker_free_kib)"
  if ((${free:-0} < MOVE_MIN_FREE_KIB)); then
    echo "images: $((${free:-0} / 1048576)) GiB free, $((MOVE_MIN_FREE_KIB / 1048576)) GiB needed; nothing fetched from ${IMAGE_HOME}."
    return 0
  fi
  while read -r key target; do
    pull_retrying docker pull -q "${target}" >/dev/null || echo "images: ${key} does not pull from ${IMAGE_HOME} yet."
  done <<<"${keys}"
}

# The keys an apply would move now: their home copy is on the host.
movable_image_keys() {
  local key target
  while read -r key target; do
    [[ -n "${key}" ]] && image_id "${target}" >/dev/null && echo "${key}"
  done < <(image_keys_elsewhere)
  return 0
}

# Repoints each key whose home copy is on the host and leaves the others for a later tick. Like
# retire_env_keys it logs key names only; GNU sed keeps .env's mode and owner.
move_image_registry() {
  local key target
  while read -r key target; do
    [[ -n "${key}" ]] || continue
    if image_id "${target}" >/dev/null; then
      sed -i "s|^${key}=.*|${key}=${target}|" "${INSTALL_DIR}/.env"
      echo "env: ${key} moved to ${IMAGE_HOME}."
    else
      echo "env: ${key} stays put until its image is fetched from ${IMAGE_HOME}."
    fi
  done < <(image_keys_elsewhere)
}
