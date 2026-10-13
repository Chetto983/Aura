
# GitLab image bridge: appliances keep updating while GitHub is blocked

Agreed with the operator on 2026-10-13, at `003835005`. GitHub flagged the operator's account on
2026-10-11. Since then nobody but the owner can pull from `ghcr.io/chetto1983`, so no appliance
can update, and no new one can be installed. Andrea's appliance is frozen on the images it
pulled before that day. This spec moves the images, and the installer's sources, to GitLab
until GitHub returns, if it does:
- running appliances move there;
- fresh installs start there;
- every host comes back to ghcr the same way when GitHub returns.

Neither trip needs a person on the machine, except Andrea's one-time patch.

## What was measured on 2026-10-13

- **Who is blocked.** An anonymous manifest request for `ghcr.io/chetto1983/aura:edge` answers
  403. Third-party images on ghcr.io still pull: llama.cpp, SearXNG and Kokoro.
- **The lab VM's updater** (`aura-image-update.timer`) is active and fails every tick on
  `docker compose pull aura`. In the last 24 hours its journal has 328 lines with `403` or
  `denied` and 12 DNS lookup timeouts. The VM's resolver answers now, and the retry in
  `pull_retrying` covers the occasional timeout. The failure is the 403.
- **What .env pins on the VM.** `/opt/aura/.env` pins nine images, all under
  `ghcr.io/chetto1983/`:
  - `AURA_IMAGE`
  - `AURA_SANDBOX_IMAGE`
  - `AURA_SANDBOX_EGRESS_IMAGE`
  - `AURA_PIM_MCP_IMAGE` (`:sidecar`)
  - `AURA_WHATSAPP_MCP_IMAGE` (`:latest`)
  - `AURA_CADDY_IMAGE`
  - `AURA_CLOUDFLARED_IMAGE`
  - `AURA_INGEST_IMAGE`
  - `AURA_ARCADEDB_MCP_IMAGE`

  The tenth `chetto1983` image is the whisper mirror, which `compose.yaml` pins by digest
  (`aura-whisper-server:2026.10.08.1056@sha256:c6c6311a…`). It runs in no profile, so every
  appliance already holds it.
  - Docker Hub's `hwdsl2/whisper-server:latest` is still that digest, published 2026-10-08
    10:56, and the digest still answers 200 there.
  - `scripts/mirror_whisper_server.sh` takes its destination from `DEST_REPO` and copies with
    `skopeo --preserve-digests`. So a GitLab copy keeps the tag and the digest.
  - skopeo is not installed in WSL.
- **Fresh installs: every entry point is blocked.**

  | Entry point | Files come from | Images |
  |---|---|---|
  | `npx create-aura-appliance` (npm 0.2.10, published 2026-10-11; the repo is at 0.2.11) | the payload inside the package | ghcr; the preflight requires `https://ghcr.io` (`packages/create-aura/src/local.ts:40`, `remote.ts:71`) |
  | `curl …raw.githubusercontent.com/chetto1983/Aura/…/install.sh \| bash` | `RAW_BASE`, GitHub raw (`scripts/install.sh:88`), which answers 404 | `DEFAULT_IMAGE`, ghcr (`install.sh:97`) |
  | `npx github:chetto1983/Aura` | GitHub, which answers 404 | ghcr |

  `install_env.sh:171-197` writes the sidecar defaults, all under ghcr. `compose.yaml` falls
  back to ghcr for pim, whatsapp and whisper. Several tests pin these references:
  - `install_lib_test.sh`, `cloudflare_update_test.sh` and `install_remote_access_test.sh`;
  - `distribution_artifacts_test.go`;
  - the "ours" rule in `image_pin_artifacts_test.go:91`;
  - create-aura's `local.test.ts` and `remote.test.ts`.
- **The GitLab registry.** The public mirror `gitlab.com/Chetto1983/aura` has its container
  registry on. From the PC and from the VM, an anonymous pull token answers 200 (0.44 s from
  the VM), and the repository list is empty. GitLab documents that a public project whose
  registry is visible to "Everyone With Access" serves anonymous pulls. It also documents that
  registry storage is outside the 10 GiB free-tier limit, which counts only the repository and
  LFS.
- **The two forks.** Both are public forks on GitHub, readable with the owner's credentials. The
  images the VM runs match each fork's `main` head, which has not moved since 2026-09-24.
  - `aura-pim-mcp`:
    - fork of MarimerLLC/calendar-mcp, .NET;
    - `main` at `6734385c4`;
    - built by `aura-publish-image.yml` as `:sidecar`, `:latest` and `:<sha>`.
  - `whatsapp-mcp`:
    - fork of lharries/whatsapp-mcp, Go and Python;
    - `main` at `1ec023314`;
    - built by `publish-image.yml` as `:latest`, `:sidecar` and `:sha-<short>`;
    - the image's `org.opencontainers.image.revision` label confirms the commit.

  Both build from a root `Dockerfile`, for amd64 only.
- **Build sources on the operator's PC.** WSL's Docker has a working `default` builder, no Aura
  image, and no registry login. WSL has Go 1.27.2 and glab v1.122.0, installed with `go install`
  into `~/go/bin`.
- **The lab overlay the VM runs on.** It sets `stop_grace_period: 30s`, which `compose.yaml`
  already has, and a lab-built sandbox image. Leaving it for `edge` loses nothing master lacks.
  The VM has 12 GB free.

This does not show what Andrea's host holds: its disk, its updater's version, or whether its
whisper image is present. The patch checks what it depends on.

## Decisions (operator, 2026-10-13)

- **The bridge lasts until GitHub returns, and it covers fresh installs.** The first draft left
  the installer on ghcr. The operator rejected that: GitHub has not answered, and nobody knows
  when it will. Every default the repo ships therefore names GitLab while the bridge lasts:
  - the installer's image and the place it fetches its files from;
  - the sidecar defaults;
  - the `compose.yaml` fallbacks;
  - create-aura;
  - the install docs.

  The return trip reverses all of it.
- **Images are published from the operator's PC,** through a script run in WSL. There is no
  GitLab CI.
- **The updater moves each host, and moves it back** (approach A). The return needs no second
  patch. A standalone switch script run on every host (approach B) was rejected because Andrea
  would have to run it twice.
- **The forks are rebuilt from source,** not copied from the VM's images. Each fork also gets a
  public GitLab mirror, so its source stays reachable without GitHub. The two projects are
  created with glab, which the operator authorized for autonomous use on 2026-10-13. Once the
  mirrors exist, the publisher clones from them, with no credentials.

## Design

### Registry layout

Every image lives under the one project's registry, one level deep:
`registry.gitlab.com/chetto1983/aura/<name>`. The names stay the ones ghcr uses: `aura`,
`aura-caddy`, `aura-cloudflared`, `aura-ingest`, `aura-sandbox`, `aura-egress`,
`aura-arcadedb-mcp`, `aura-pim-mcp`, `whatsapp-mcp` and `aura-whisper-server`. Moving between the
two registries is a prefix swap:
`ghcr.io/chetto1983/<name>:<tag>` ⇄ `registry.gitlab.com/chetto1983/aura/<name>:<tag>`.
Tags, and the whisper digest, are unchanged.

### Publisher: `scripts/publish_images.sh`

The script takes the Aura commit and, for each fork, a git URL and a ref (`main` by default).
It builds the nine images and pushes them to `AURA_PUBLISH_REGISTRY`, which defaults to the
GitLab path.

1. **Export.** Each source is exported at its commit into a fresh directory: `git archive` for
   Aura, a clone checked out at the commit for a fork. Dirty or untracked files never reach an
   image.
2. **Build.** Every image is amd64, with the Dockerfile, the build args (`VCS_REF` for aura) and
   the `org.opencontainers.image.source` / `.revision` labels that its CI workflow uses today.
   - Aura's seven images follow `publish-aura-edge.yml` and `publish-arcadedb-mcp.yml`.
   - The forks follow their own publish workflows.
   - cloudflared's arm64 variant is not built.
3. **Contract tests.** Before any push, the repo's image contract tests run on the local builds:
   `cloudflared_image_contract_test.sh`, `ingest_image_contract_test.sh` and
   `sandbox_image_contract_test.sh`. A red contract pushes nothing.
4. **Per-commit tags.** These go first, all nine before any moving tag:
   - `master-<sha>` for the edge set;
   - `<sha>` for arcadedb-mcp and for the forks.
5. **Moving tags.** They move only after every per-commit push succeeded:
   - the forks' `sidecar` and `latest`;
   - arcadedb-mcp's `edge`;
   - the edge set in the workflow's order, ending with `aura:edge`.

   They are registry-side copies (`docker buildx imagetools create --prefer-index=false`), as in
   `publish-aura-edge.yml`. A run that dies half way never leaves `aura:edge` ahead of the
   images it needs. That is the 2026-09-27 failure the workflow's comment records.

Docker authenticates to the registry through glab, so no second registry token exists. The
operator logged in with `glab auth login` on 2026-10-13. `glab auth configure-docker`
registers `docker-credential-glab`, a credential helper that answers Docker from that login.
The script never reads or prints credentials. The project's registry was measured through the
API with that login: the prefix is `registry.gitlab.com/chetto1983/aura`, lowercase, and the
access level is `enabled`.

### Updater: `deploy/aura-image-update.sh`

- **Two constants.**
  - `IMAGE_HOMES`: `ghcr.io/chetto1983` and `registry.gitlab.com/chetto1983/aura`.
  - `IMAGE_HOME`: where images live now. During the bridge it is the GitLab path.
- **`move_image_registry`.** It runs in `apply_update` after `ensure_cloudflared_edge_env`, with
  the other `.env` edits, so it runs only once an update has been decided. It looks at every
  `AURA_*_IMAGE` line whose value starts with a home other than `IMAGE_HOME`:
  1. It pulls the same `<name>:<tag>` under `IMAGE_HOME`, through `pull_retrying`.
  2. On success, it rewrites that one line in place and logs the key name. GNU `sed -i` keeps
     the file's 0600 mode and owner.
  3. On failure, the key stays as it is and the tick goes on. A pull that fails today is
     retried on the next tick.

  Values are never logged, as with `retire_env_keys`. A value under any other registry is left
  alone, such as an operator's own `AURA_STT_IMAGE`.
- **The cloudflared default.** `ensure_cloudflared_edge_env` writes
  `${IMAGE_HOME}/aura-cloudflared:edge` instead of the ghcr literal.
- **Old images leave the disk.** `remove_superseded_images` already exists so that a pin that
  moved registry still supersedes the image it replaced: SearXNG moved from Docker Hub to ghcr on
  2026-10-07. Two changes extend it to this move:
  - `image_repository` folds every home in `IMAGE_HOMES` to one name. Today
    `chetto1983/aura-caddy` and `chetto1983/aura/aura-caddy` read as two repositories, so
    nothing would be superseded.
  - The two box images from `.env` (`AURA_SANDBOX_IMAGE`, `AURA_SANDBOX_EGRESS_IMAGE`) join
    the pinned set. They are not compose services, so today the function never sees them.
    `refresh_sandbox_images` runs before it, so no box still holds the old image.

  Without these changes about 6 GB of ghcr images would stay on the VM's disk, whisper's
  842 MB included. A ghcr name on the same image ID as its GitLab pin is skipped, as today.
  That happens to whisper, whose digest does not change, and it costs no disk.
- **Cosmetics.**
  - The service's `Description` stops saying "from GHCR".
  - The header's prerequisite list names `IMAGE_HOME`.
  - The payload manifest is regenerated, because the updater is a payload file.

The move needs nothing from the old updater beyond what it already does. The old updater pulls
the new aura and installs its payload. The new updater then re-executes and moves the other
keys before the stack is brought up.

### Andrea's patch: `deploy/aura-registry-bridge.sh`

It is a one-time script, run as root on the appliance. It reads `INSTALL_DIR` from
`/etc/default/aura`, as the updater does. It is not in the payload: `install.sh` does not
download it.

1. **It refuses, and changes nothing**, when any of these is true:
   - `.env` is missing;
   - `AURA_IMAGE` is under neither home;
   - the installed updater predates the payload sync, because it has no `sync_payload` and so
     could never install its successor;
   - the free space under Docker's root is below the floor;
   - the pull of `registry.gitlab.com/chetto1983/aura/aura:<same tag>` fails.

   The floor is the measured size of the published set plus a margin, fixed in the plan.
2. **It rewrites `AURA_IMAGE` only.** The old ghcr `aura:edge` leaves the disk at the end of
   the first apply, through `remove_superseded_images`.
3. **It starts `aura-image-update.service`** and says what comes next:
   1. The updater downloads the new aura.
   2. The cockpit offers the update.
   3. "Update now" applies it, or Aura applies it on its own after 15 idle minutes, or at the
      deadline.
4. **It is idempotent.** On a host already on GitLab it says so and only starts the updater.

Andrea runs it from the public mirror:
`curl -fsSL https://gitlab.com/Chetto1983/aura/-/raw/master/deploy/aura-registry-bridge.sh | sudo bash`.
The operator can also send him the file.

### Fresh installs

Every default the repo ships names the current home, `registry.gitlab.com/chetto1983/aura`, and
fetches from the GitLab mirror:

- **`scripts/install.sh`.**
  - `DEFAULT_IMAGE` takes the home.
  - `RAW_BASE` defaults to `https://gitlab.com/Chetto1983/aura/-/raw/${RAW_REF}`.
  - The usage lines in the header follow.
- **`scripts/install_env.sh`.** The sidecar defaults take the home.
- **`compose.yaml`.**
  - The fallbacks for pim, whatsapp and whisper take the home.
  - The whisper pin keeps its tag and digest (`2026.10.08.1056@sha256:c6c6311a…`).
  - The comments that name the ghcr channel follow.
- **`.env.example`.**
- **create-aura.**
  - The required host is `https://registry.gitlab.com`, in `local.ts` and `remote.ts`.
  - The package version is bumped, and the operator publishes it to npm with their own login.
- **`docs/INSTALL.md` and `docs/cloudflare-remote-access.md`.**
  - The install command fetches from the GitLab raw URL.
  - The `npx github:chetto1983/Aura` line is dropped for as long as the bridge lasts, because
    nothing replaces it.

`docs/aura-quality-snapshot.md` is a ledger of past measurements, and it is not edited.

**Whisper is mirrored once,** by the existing `scripts/mirror_whisper_server.sh` with
`DEST_REPO=registry.gitlab.com/chetto1983/aura/aura-whisper-server`. Docker Hub still serves the
pinned digest, so the copy keeps it. The mirror needs skopeo in WSL and a `skopeo login`.

**An anti-drift test** keeps the shipped files in step. It reads `IMAGE_HOME` from the updater
and fails on any reference to another home in the files a host receives:
- `scripts/install.sh` and `scripts/install_env.sh`;
- the compose files;
- `.env.example`;
- `deploy/`;
- create-aura's sources.

GitHub raw counts as the ghcr home, and GitLab raw as the GitLab one. The rule for "ours" in
`image_pin_artifacts_test.go:91` accepts both homes. The tests that pin ghcr today move to the
home.

### Return trip

When GitHub reinstates the account:
1. One commit sets `IMAGE_HOME` back to ghcr, reverses the defaults above, and deletes
   `deploy/aura-registry-bridge.sh`. The anti-drift test fails until no GitLab reference is
   left in a shipped file.
2. CI publishes that commit to ghcr, and the operator publishes create-aura to npm.
3. `scripts/publish_images.sh` publishes the commit once more to GitLab.
4. Every bridged host pulls that GitLab `aura:edge` and installs its updater. The updater then
   moves each key back once the ghcr image pulls.
5. When no bridged host is left, the publisher script is deleted.

## Out of scope

- **The CI workflows**, which stay as they are and publish to ghcr whenever GitHub runs them.
- **arm64 cloudflared.**
- **A cleanup policy for the GitLab registry.** Its storage is outside the free limit.
- **The aura-connect page**, which is a separate design.

## Tests

- **`scripts/aura_image_update_test.sh`** keeps its strict-stub harness and adds these cases:
  - a key moves only after its pull succeeded;
  - a failed pull leaves `.env` byte-identical;
  - a quoted value moves;
  - a key already at home, and a value under another registry, are untouched;
  - with `IMAGE_HOME` flipped, the move runs in the return direction;
  - the log carries key names, never values;
  - `.env` keeps its mode;
  - `image_repository` folds both homes to one name;
  - a ghcr image whose GitLab pin moved is removed, the box images included;
  - the cloudflared default follows `IMAGE_HOME`.
- **`scripts/aura_registry_bridge_test.sh`** (new) stubs `docker`, `systemctl` and `df`, and
  covers these cases:
  - each refusal leaves `.env` byte-identical;
  - only `AURA_IMAGE` changes;
  - a second run is a no-op that only starts the updater.
- **The anti-drift test** (new, Go, beside `image_pin_artifacts_test.go`) has three jobs:
  - it fails on a reference to the other home in any shipped file;
  - it proves it can fail, on a synthetic file that names the other home;
  - it reads `IMAGE_HOME` from the updater, never from a second copy.
- **Existing tests that move to the home:**
  - `install_lib_test.sh`, `cloudflare_update_test.sh` and `install_remote_access_test.sh`;
  - `distribution_artifacts_test.go`;
  - create-aura's `local.test.ts` and `remote.test.ts`.
- **`scripts/publish_images_test.sh`** (new) covers these cases:
  - the script's Aura images and Dockerfiles equal what the two workflows build, parsed from
    the YAML, so the two cannot drift;
  - with a stubbed `docker`, no moving tag moves before every per-commit push succeeded;
  - `aura:edge` moves last;
  - a failed build or contract pushes nothing.

## Acceptance on the lab VM

The VM runs the exact path Andrea will run, before he gets it.

1. **Publish.** Publish the commit that lands this work from the operator's PC, and mirror
   whisper. Then read back from the VM, anonymously:
   - all ten manifests are present;
   - whisper's digest is the pinned one;
   - `aura version` in the published image names the commit.
2. **Patch.** Run `deploy/aura-registry-bridge.sh` from the GitLab raw URL.
3. **Apply.** "Update now" comes from the operator's cockpit session.
4. **Read-backs:**
   - all nine keys are on the GitLab prefix (key names and prefix only);
   - every compose container and every box runs a GitLab image ID;
   - `aura version` names the published commit;
   - through the integrations proxy, the PIM `/admin/accounts` and the WhatsApp `/api/status`
     answer;
   - the updater's journal shows each move and exit 0;
   - the superseded ghcr images are gone, and free space is back.
5. **Second tick:** nothing to move, exit 0.
6. **Smoke:** one chat turn in the cockpit and one on Telegram.
7. **Fresh-install reachability, the cheap probe.** In a clean `ubuntu` container, with no
   credentials and an empty Docker config:
   - `install.sh` and every payload file it lists (`scripts/payload_files.sh`) download from
     the GitLab raw URL;
   - the manifest of every image a fresh install names resolves anonymously.

   It does not start a stack. The published create-aura tarball is also checked: its installer
   names the GitLab home.

The VM leaves the lab overlay for `edge`, which is intended. The run is recorded in
`docs/superpowers/verification/` under its date. When Andrea runs the patch, his updater's
journal is added to the same ledger. PRD §17 (Deployment, backup and recovery) records the
bridge after the VM run, citing the ledger.

## What this does not establish

- **The return trip.** It runs only in the tests until ghcr is reachable again.
- **A complete fresh install.** The probe shows that every file and image is reachable, not that
  a clean machine reaches a working chat. That needs a clean VM, and the operator chose the
  cheap probe.
- **Build provenance.** The images are built on the operator's PC, not in a clean runner. The
  provenance is a `git archive` of a named commit plus its revision label, not a hermetic build.
- **Andrea's host.** Anything the patch does not check, such as a full disk after the pull, or a
  host on a much older payload.
- **GitLab's pull limits.** Nothing beyond the handful of pulls the acceptance makes.
- **Release installs.** A pinned `vX.Y.Z` install names a release tag that only ghcr carries; the bridge mirrors the edge channel.
