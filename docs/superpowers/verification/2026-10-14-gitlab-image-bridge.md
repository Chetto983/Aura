# GitLab image bridge: lab VM acceptance

Date: 2026-10-14. Plan: `docs/superpowers/plans/2026-10-13-gitlab-image-bridge.md`.
Spec: `docs/superpowers/specs/2026-10-13-gitlab-image-bridge-design.md`.

**Verdict: passed, within the limits under "What this does not establish".**
- **The patch.** It moved a box frozen on ghcr to GitLab, first as it was at `6b082c343`, then in
  its fixed form from GitLab raw master.
- **A clean install.** One from npm pulls only from GitLab.
- **The fixes.** The final review's fixes and the three bugs the box surfaced were published and
  then applied by the box's own updater. The last of them landed in a quiet spell, with nobody
  pressing anything.
- **The scheduled goal.** It now closes every ledger row it opens, so it no longer holds that
  apply back.

## The published set (Task D1)

- **Publisher run.** `scripts/publish_images.sh` ran at `e04aefcad` from the operator's PC. It
  finished with rc=0 in 1633 s: 9 images published and 11 moving tags moved, with `aura:edge`
  last. The whisper pin was mirrored with `skopeo copy --preserve-digests`.
- **Anonymous read-back from the lab VM.** The read-back ran with an empty `DOCKER_CONFIG`, so it
  used no credential. All ten references inspected:
  - `aura`, `aura-caddy`, `aura-cloudflared`, `aura-ingest`, `aura-arcadedb-mcp`,
    `aura-sandbox` and `aura-egress`, each at `:edge`;
  - `aura-pim-mcp:sidecar`;
  - `whatsapp-mcp:latest`;
  - `aura-whisper-server:2026.10.08.1056`, at digest
    `sha256:c6c6311af66c2fda59645745628edbda1737c619dcd30ce7ea2c00e5e5a1d9d6`.
- **The published binary.** `docker run --rm --entrypoint aura …/aura:edge version` printed
  `commit: e04aefcad319e4f4f99dd85f3457ab3075757d72`. The read-back pulled `aura:edge` and then
  removed it, so the VM met the patch with the image absent, as Andrea's box will.

### What a move costs on disk

The plan sized the patch's floor on a 7.5 GB set that left the sandbox out. The real costs:

- **The set as published.** `docker image ls` on the PC that built it gives 10.73 GB, or
  9.99 GiB. The sandbox alone is 4.82 GB.
- **Why each layer counts twice.** The lab VM, like any fresh Docker install, uses the containerd
  image store (`docker info` driver `overlayfs`). That store keeps each layer both compressed and
  extracted.
- **What the move adds on the VM.** The PC's layer chain IDs were compared against the VM's
  snapshots and blobs. On the VM the move adds 9.19 GiB: 6.83 GiB extracted and 2.36 GiB
  compressed. Some layers it already shares with the ghcr images.
- **Free space.** The VM had 10.49 GiB free. `df -BG` rounds that up to 11G.
- **What a host with no shared layer would get.** Its disk would be full under running Postgres
  and ArcadeDB before the first apply reclaims the old images.
- **The fix.** `6b082c343` raised `MIN_FREE_GB` to 12: the worst case plus 2 GiB, test-first. A
  new refusal at 11 GB failed first.

## Local gates (no CI ran on GitHub)

GitHub Actions has been disabled on the account since 2026-10-11.

- **`make quality`** ran on a clean checkout of `6b082c343` (`scripts/pushed_tree.sh`). It passed
  in 258 s:
  - deadcode, vet, file-size, the capability and model contracts, and lint (0 issues);
  - `go test -race` over 98 packages;
  - govulncheck (0 vulnerabilities reachable);
  - build.
- **The pushes.** The pushes of `6b082c343` to `gitlab` and `origin` passed every pre-push gate:
  payload-manifest, build, deadcode and tagged-tier-compile.
- **GitLab CI.** The push started GitLab pipeline #1 (`.gitlab-ci.yml`, `make quality`, from
  `1f8ff1f7a`). Pipeline `#2945793839` ended in `success` on GitLab.com shared runners. It was
  the first CI run since GitHub Actions stopped.
- **Coverage.** `scripts/coverage_docker.sh` ran on `6b082c343` against the memory stack and a
  disposable `aura_cov`, with 116 migrations applied. `owned_internal` covered 46600/52802
  statements, 88.3% against the 85% floor, and the package-local policy passed (1884 s).

## The lab VM takes Andrea's path (Task D2)

**Before (07:57Z):**
- All 9 `.env` image keys named `ghcr.io/chetto1983`.
- `aura` ran the lab overlay `aura:lab-aura-connect-20261013-2`, and the identity's browser box
  ran a lab sandbox image.
- 10.49 GiB were free.
- The updater timer was active. Every tick failed on ghcr's `denied`.
- `auth.log` showed only this session's logins.

**The patch, run exactly as Andrea will:**
`curl -fsSL https://gitlab.com/Chetto1983/aura/-/raw/master/deploy/aura-registry-bridge.sh | sudo bash`

- **First run.** It refused, as it must: `Docker has 11 GB free, 12 GB are needed -- nothing was
  changed.`
- **Making room.** The operator approved removing lab and spike leftovers: three unused
  `aura:lab` images, four directories under `/home/aura`, and the apt cache. That brought free
  space to 12.16 GiB.
- **Second run.** It printed `pulling registry.gitlab.com/chetto1983/aura/aura:edge`, then
  `AURA_IMAGE now names the GitLab bridge.`, and started the updater.

**The apply:**
- **The old updater.** It pulled `aura:edge` from GitLab, skipped every ghcr sidecar, and recorded
  `update: e04aefcad waits for an admin, a quiet spell or 2026-10-15T08:01:22Z.`
- **No quiet spell came.** Aura reported `live_runs=12`; see "Found on the way".
- **The operator pressed Update now** at 08:12Z.
- **The payload.** The sync installed the new updater, which re-executed itself.
- **The moves.** Between 08:17:18 and 08:19:32 it moved the eight remaining keys: sandbox, PIM,
  WhatsApp, caddy, cloudflared, ingest, arcadedb-mcp and egress.
- **The stack.** It brought `aura` and then the stack up on the payload, and removed the box that
  ran a superseded image.
- **The reclaim.** It removed the eight ghcr images and logged
  `Aura image update completed; the appliance is healthy.` at 08:21:57Z.

**Read-back:**
- 9 of 9 `.env` image keys name `registry.gitlab.com/chetto1983/aura`.
- Every own container runs a GitLab image: aura, arcadedb-mcp, cloudflared, caddy, ingest, PIM,
  WhatsApp, the STT server, and the recreated browser box with its egress.
- `docker exec aura aura version` prints `commit: e04aefcad319e4f4f99dd85f3457ab3075757d72`.
- The journal has 8 `moved` lines, one per key the patch did not move. The service reports
  `Result=success` and `ExecMainStatus=0`.
- No `ghcr.io/chetto1983` image is left. The whisper pin survives only under its GitLab name, with
  the same ID `c6c6311af66c`.
- The operator's cockpit MCP board lists every server as healthy: browser (29 tools), calendar
  (1, its single nested tool), elevenlabs (160), linear (68), memory (13) and whatsapp (15).
- Free space is 5.40 GiB. The gap to the 10.49 GiB before is the two lab images the update left
  unused (`aura:lab-aura-connect-20261013-2` and the lab sandbox, 8.01 GB reclaimable). They are
  not pins, so the updater rightly leaves them. A box without lab images has nothing like them.

**Second tick:** 0 `moved` lines, `Result=success`, exit 0, in 32 s. The first tick took 1 min
52 s of failing ghcr pulls. The unit's description now reads "from its image home".

## Fresh installs are reachable (Task D3)

The plan's probe ran in a clean `ubuntu:24.04` container with no credentials:
- **Install files.** All 35 that a fresh install fetches answered from GitLab raw.
- **Images.** 10 of 11 answered `skopeo inspect --no-creds` directly.
- **Whisper.** The eleventh, compose's whisper fallback, is written `tag@digest`, a form skopeo
  cannot parse. Docker and compose accept it. Probed by its digest alone it answered, and the
  tag resolves to that same digest.

**npm.** The operator published `create-aura-appliance@0.2.12` from WSL with
`npm publish --provenance=false`; npm's web 2FA needs a person, and provenance needs a CI OIDC
identity the PC lacks. Read back from the registry: version 0.2.12, `repository.url` on
gitlab.com, a 63-file tarball whose installer is byte-identical to the local build. Unpacked
(`--noexec --target`; makeself compresses the payload, so a grep of the `.run` finds nothing),
the installer holds 36 files, `DEFAULT_IMAGE` names the GitLab home, and ghcr appears only in the
updater's `IMAGE_HOMES`.

**A clean install from npm.** With the operator's approval the lab VM was wiped down to the OS:
Aura, its volumes and Docker itself. The operator ran `npx create-aura-appliance@0.2.12 --mode
remote` from WSL against it:
- it installed Docker 29.9.0, with the containerd image store;
- it pulled everything from GitLab and printed the setup wizard's URL at 09:11:48Z;
- read-back: 9 of 9 `.env` image keys on the GitLab home, every own container on a GitLab image
  and healthy, no ghcr image, `aura version` `e04aefcad`, the appliance posture strict, the
  updater timer active.

**The databases come back.** At the operator's request the 2026-10-13 backup was restored into
the fresh install (`docs/BACKUP-RESTORE.md`). The old `.env` secrets were kept and the fresh
image keys put in. Postgres from `pg_dumpall`: 2 identities, 100 conversations, migration 141.
ArcadeDB from its native backup: 20 entities, 24 facts, 99 conversations, 329 turns. Not in that
backup: Garage objects, the PIM Google tokens, the WhatsApp session. ElevenLabs' refresh token
had rotated after the backup (`invalid_grant`, Riautorizza in the cockpit), and the ChatGPT plan
route needs its sign-in again.

**Smoke.** After the update the operator sent a chat message from the cockpit and it was
answered. Telegram was left out: the lab network intercepts its TLS.

## The final review and its fixes

A fresh reviewer (opus, read-only) went over `6ae832907..6b082c343`: 0 Critical, 7 Important,
about 12 Minor. All seven Important findings were fixed test-first in `82e9b3b5d..1a5c4137c`:

| Finding | Fix | Proof |
|---|---|---|
| The move pulled the whole set inside the apply: 10 of the unit's 15 minutes on the VM | `deploy/aura-image-home.sh`: `fetch_image_homes` downloads on every tick, before consent; `move_image_registry` never pulls | `aura_image_home_test.sh`; the old behaviour put back fails it |
| A key whose pull failed waited for the next publish | `update_pending` counts a key whose home copy arrived | same suite, case 5 |
| The updater had no free-space check | the fetch stops below 12 GiB, read in KiB | same suite, case 2 |
| The move ran after the payload sync | it runs before `sync_payload` | same suite, case 6 |
| The patch pulled `aura` only, leaving 8 images to the unit's deadline | the patch pulls every ghcr key's GitLab copy and refuses if one fails | `aura_registry_bridge_test.sh`; a mutant fails it |
| `df -BG` rounds 11.9 GiB up to 12 | `df -k`, compared in KiB | same suite; a mutant fails it |
| The publisher published any commit | it refuses a commit that is not on `gitlab/master`, before building | `publish_images_test.sh`, red first |
| No pipeline ran the shell suites | `.gitlab-ci.yml` job `appliance-shell`, 11 suites | run in its image as root with `CI=true`; GitLab pipeline #2 |

On the way, `cloudflare_update_test.sh`'s docker stub still accepted the image list format
`9049e10f5` replaced, so the cleanup failed silently inside that suite; it was brought up to
date. The minors are deferred in the plan's ledger.

## The fixes reach the box through its own updater

Every build below was published with `scripts/publish_images.sh`, which moves `aura:edge` last.

- **`1a5c4137c` (the review fixes), 10:13:54Z.** The operator pressed Update now.
  - The payload brought `deploy/aura-image-home.sh` and the new updater, which re-executed itself.
  - `aura version` then printed `commit: 1a5c4137c`, and the box was healthy at 10:15:25.
  - GitLab pipeline #2 on this commit: `appliance-shell` 39 s, `quality` 626 s.
- **The loop it exposed.** From 10:21Z the updater re-applied that same build on every idle tick
  ("applying 1a5c4137c: nobody is using Aura", every key `X -> X`, ~7 min apart, until 12:50).
  See "Found on the way". Fixed in `4144693ec`.
- **`82a49761c` (the three VM fixes), 13:00:55Z.** This time nobody pressed anything:
  - **The apply.** A tick applied it in a quiet spell: "applying 82a49761c: nobody is using Aura".
  - **The updater.** The payload brought the `4144693ec` updater, which re-executed itself.
  - **The result.** The box printed `commit: 82a49761c` and was healthy at 13:03:12.
  - **The next tick.** The 13:08:39 tick ended in 36 s without a single decision line
    (`state=current`), so the loop is gone.
  - **GitLab CI.** Pipeline #3 on this commit: `appliance-shell` 41 s, `quality` 785 s.
- **A publish and a tick overlapped.** The 12:50:06 tick pulled while the `82a49761c` publish was
  moving its tags, which finished at 12:51:31.
  - **What it ran.** It recreated arcadedb-mcp, caddy, ingest and cloudflared from the new
    commit's images, and re-applied the old `aura`.
  - **Why the order matters.** `aura:edge` moves last, and that is what kept the box from running
    an `aura` whose sidecars were not yet published. The reverse can happen: for one tick, the
    sidecars ran ahead of `aura`.

## The fixed patch on a frozen box

The lab VM was put back into the state Andrea's box is in, and the patch ran again from GitLab
raw master (sha256 `bf1336baa5ce`, the `09ac56630` form).

**Before (17:39:11Z):**
- All 9 `.env` image keys named `ghcr.io/chetto1983`.
- `aura` ran `1a5c4137c`, recorded as applied, while `aura:edge` on GitLab named `379f41c06`.
- The updater was `9049e10f5`'s, from before the bridge, and `deploy/aura-image-home.sh` was
  absent.
- 20.12 GiB were free.
- Everything this replaced was backed up under `/opt/aura/backups/e2e-andrea-20261014T173911Z`.

**The patch, 17:40:14-17:40:39Z**, run as Andrea will:
- It pulled all nine GitLab copies, `aura:edge` first.
- It printed `AURA_IMAGE now names the GitLab bridge.` and started the updater. Exit 0.

**The tick it started, 17:40:39-17:45:59Z:**
- **The old updater.** Its ghcr sidecar pulls were denied, as they are on Andrea's box. At
  17:42:14 it decided `update: applying 379f41c06: nobody is using Aura.`
- **The payload.** It brought `aura-image-home.sh`, the consent library and the new updater,
  which re-executed itself.
- **The moves.** At 17:44:10-11 it moved the eight other keys to GitLab.
- **The apply.** `aura` went from `5677f56…` to `d501c15…`, and `aura version` printed
  `commit: 379f41c06`. The stack came up on the payload.
- **The reclaim.** `ghcr.io/chetto1983/aura:edge` was removed as superseded, and the updater
  logged `the appliance is healthy`.

No one pressed anything: the box was idle, so the old updater's own quiet-spell rule applied it.
One pull of `whatsapp-mcp:latest` got `failed to fetch anonymous token` from registry.gitlab.com
twice, at 17:45:18. The updater tolerated it, and that container already ran the new image.

**Read-back (17:46Z):**
- 9 of 9 keys name the GitLab home.
- Every own container runs a GitLab image and is healthy. WhatsApp runs `215b0107e`, with its
  platform manifest equal to its tag's.
- The updater is the new one, and `deploy/aura-image-home.sh` is present. The status reads
  `state=current`.
- The timer and `aura-update-request.path` are active.
- 20.88 GiB are free.

## A scheduled goal closes its ledger rows

The finding below ("A scheduled goal pins `live_runs`") was fixed in `779db66af`. Proving it on
the box took three runs of the operator's daily news goal (task `01a12efb`), each started with ▶
on the Scheduler board.

- **10:22Z, on `1a5c4137c`: inconclusive.**
  - **What failed.** Every gateway row hit `tool_invocations_conversation_id_fkey`.
  - **Why.** At 10:01 the operator had bulk-deleted their conversations, among them the one the
    task was scheduled from.
  - **Every `web_fetch` failed too**, on `session_id "agent_job:<run>" contains invalid character
    ':'`. The sidecar grammar is `[A-Za-z0-9_-]`.
  - **The fixes.**
    - The operator ruled that a task outlives its conversation. `4c0dbe8e6` moves such a task into
      a conversation of its own, titled "⏰ <goal>".
    - `82a49761c` names the session `agent_job-<run>`.
- **12:55Z: started before `82a49761c` landed.** It failed the same way, so it is not evidence
  either way.
- **13:10:19-13:11:34Z, on `82a49761c`: passed.**
  - **The task moved.** Aura logged "cron: the task's conversation is gone; it answers in a
    conversation of its own". Task `01a12efb` moved from the deleted `01a12ef9…` to `01a13a89…`.
    That conversation has the same owner, is active, and is titled "⏰ Cerca le notizie principali
    del giorno in italiano, riass…". The task's `origin_conversation_id` now names it.
  - **The ledger rows.** There were 12 tool calls: `tool_search`, `current_time`, 3 `web_search`
    and 7 `web_fetch`. That gives 12 `start` rows and 12 `end` rows. Every end has `status=ok` and
    `meta {"unobserved": true}`, written in the same second as its start or the next.
  - **No failures.** No write failed and no tool returned an error.
  - **The answer.** The goal's answer landed in the new conversation at 13:11:35.
  - **`live_runs`.** It read 1 while the job ran, which is its own heartbeat by design: a running
    job holds the quiet-spell apply back. It read 0 at the activity write right after the run, at
    about 13:11:50.

## Found on the way

**Reinstalling in the same boot leaves the timer with nothing to count from.** After the wipe and
the clean install the updater never ran: `systemctl list-timers` showed `NEXT` as `-`
(`NextElapseUSecMonotonic=infinity`). `OnBootSec=2min` had already fired at 07:32, for the
install the wipe removed, and the reinstalled service unit had never been inactive, so
`OnUnitInactiveSec=5min` had no point to count from. On a host installed once per boot the
documented rule applies instead: an `OnBootSec` already in the past "will immediately elapse"
when the timer is activated (systemd.timer(5)). The updater was started once by hand at 09:54Z,
as the patch does; the timer chain resumed and the first tick enabled
`aura-update-request.path`.

**The first tick after an install re-applies the installed build.** `install.sh` writes neither
`update/applied-image` nor `payload_manifest.applied`. The first tick therefore finds an update
pending and applies it without consent ("aura already runs it (the host restarted)"), and
counts every payload file as changed, so it restarts every service that mounts one: searxng,
arcadedb, prometheus, garage, grafana, tempo, caddy. Measured at 09:54Z on the fresh install. It
happens once, but on a new host it lands while the operator may be in the setup wizard.
- **The fix.** `9c10e3da7` adds `record_applied_update` to `install.sh`, called once compose is
  up. It records the image's ID and the image's own payload manifest, but only when that manifest
  verifies against the payload on disk. Otherwise it records nothing and drops any stale record.
- **Unit proof.** `install_lib_test.sh` pins the record names to the updater's own. It kills
  three mutants: the record written after the timer, stale records kept, and no payload check.
- **On the lab VM.** At 16:18:52 the VM was put back to what an install leaves, with no record.
  `record_applied_update`, taken from `install.sh` on GitLab raw master, then ran against the
  real `aura` image and payload. It wrote the same image ID and the same manifest the updater had
  recorded itself. The 16:24:11 tick ended in 33 s with `state=current` and no decision.

**A republish looked like an update forever.** This is pre-existing, and today's republish
surfaced it. Fixed in `4144693ec`.
- **The image.** On the containerd image store an image's ID is its index digest, and every
  attested publish writes a new index.
- **What the republish changed.** `whatsapp-mcp` was republished from the same commit `1ec0233`
  with the same created time. Its tag's index moved from `1f47f…` to `1dad3d…`, while its
  platform manifest stayed `f93f07…`.
- **What compose compares.** Compose compares the platform manifest, which it keeps in the
  container's `com.docker.compose.image` label (compose #13636). That is also the container's
  `ImageManifestDescriptor`. So compose kept the container.
- **What the updater compared.** `service_behind` compared index digests, so it saw the sidecar
  behind on every tick and re-applied.
- **The fix.** `runs_superseded_image` compares the platform manifest whenever the container
  carries a descriptor: `ImageManifestDescriptor`, API 1.48, against
  `docker image inspect --platform`, API 1.49. Otherwise it compares the image ID. `stale_boxes`
  uses it for the box and its egress.

**A scheduled goal pinned `live_runs`.** Fixed in `779db66af`, and proved on the box as described
under "A scheduled goal closes its ledger rows".
- **The rows.** The 12 live runs were the `start` rows of one scheduled-goal turn (07:33Z:
  `tool_search`, `current_time`, `web_search`, `web_fetch`). None of them had an `end` row,
  although the turn logged `agent turn end` at 07:34:14.
- **Why.** A scheduled goal runs outside a Runner, so nothing wrote the `end` rows until the
  reconciler stamped them `indeterminate` errors 30 minutes later. The query behind `live_runs`
  counted them for up to an hour, and the updater's quiet-spell apply waited for that hour.

## What this does not establish

- **How far the frozen box was frozen.** The second patch run was a simulation, and it differed
  from Andrea's box in three ways.
  - **Local copies.** GitLab's `aura:edge` was still on the disk, kept by the exited one-shot
    containers, so the patch's pulls cost little here.
  - **Old images.** The eight sidecar keys' ghcr names were tags on the very images GitLab names.
    So the reclaim of ghcr images that differ in content was not exercised here; D2 did exercise
    it. The aliases were untagged by hand afterwards.
  - **The refusal.** The space refusal was not hit: 20.12 GiB were free.
- **The install fix on a clean install.** `record_applied_update` ran on the lab VM's real image
  and payload, not inside a fresh install. A clean install from npm needs a release that carries
  `9c10e3da7`.
- **Andrea's box.** Its disk, its layers already shared with GitLab and its network are its own.
  The 12 GiB floor is the lab VM's worst case plus 2 GiB.
- **The quiet spell, from the updater's side.** At 13:00:55 the operator had been idle since
  ~10:02, and that apply came from the `1a5c4137c` updater. The `4144693ec` updater has since
  decided both ways:
  - at 17:06:26, `applying 82a49761c: nobody is using Aura`, for sidecars whose tags had moved;
  - at 17:14:48, `9c10e3da7 waits for an admin, a quiet spell or 2026-10-15T17:14:47Z`, after
    activity at 17:12.

  Neither decision was compared against a recorded log of the operator's own activity.
- **More than two ticks.** Only two ticks after the fix were read for the loop: 13:08:39 and
  13:14:16. Each took 36-37 s and logged no decision line.
- **A publish racing a tick.** Nothing stops a tick from running sidecars of a newer commit than
  `aura` for one tick, as happened at 12:50.
- **The re-home with no owner.** It ran for a task whose conversation was deleted by its owner.
  It did not run for an owner who no longer exists, for a conversation that was archived (that
  one stays where it is), or for a failed existence check (that one answers where it was).
- **Other platforms and limits.** arm64 hosts, and GitLab's anonymous pull limits under many
  boxes.
