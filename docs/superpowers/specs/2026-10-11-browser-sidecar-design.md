# Browser sidecar: the logged-in browser lives where the model's shell does not

Proposed on 2026-10-11 at `290b782`. The operator agreed decision 12 (`eval` with approval) on
2026-10-11 and asked for downloads to be measured, which decision 8 now records. Every other
decision is still a proposal for review. It rests on the measurements in
`docs/superpowers/verification/2026-10-10-browser-profile-cookie-isolation.md`, including the
2026-10-11 follow-up and download sections, and on the lab VM run in
`docs/superpowers/verification/2026-10-11-browser-sidecar-lab-vm.md`: real sites, three browser
boxes at once, cold start, and the isolation invariant behind the appliance's own egress floor.
Nothing here has been built or measured as built.

Risk tier: **high**. This changes a tenancy boundary. No Postgres migration is involved.

## The problem, measured

Today each identity has one box (`aura-box-<id>`), plus its egress sidecar. The model's shell and
the logged-in browser both live in that box, and both run as root. The shell is root because D-12
chose root for `shell_exec` parity. The browser is root because it runs as `docker exec` into the
same box.

Measured on the shipped image `aura-sandbox@sha256:1dd37f37…`:
- **Cookie jar.** The root shell reads `Default/Cookies`, which is mode 0600, and lists the live
  jar.
- **Vault key.** The shell also reads `/run/aura/agent-browser.key`. Aura writes it 0600
  (`cmd/aura/sandbox_box_files.go:37`), and root reads it anyway.
- **Other users and permissions do not help.** A browser running as uid 1001 behind a 0700
  directory is still read by the root shell.
- **DevTools Protocol.** Chrome's CDP listens on `127.0.0.1`, port 38791 in the measured run.
  Any process in the box's network namespace can drive it, and CDP hands back cookies already
  decrypted without touching a file.
- **Control socket.** agent-browser's daemon socket sits on the workspace volume, at
  `/workspace/.agent-browser-home/.agent-browser/<session>.sock`.
- **Hand-built sidecar.** A browser in its own box, whose profile volume the shell box never
  mounts, left 0 cookie databases, 0 browser pids and no docker socket reachable from the shell.

Hermes Agent ships the same exposure on purpose and documents it. LibreChat avoids it by never
letting credential state and model-executed code share a host. The comparison is in the ledger.

## Goal

The goal is one invariant. **No filesystem, process or network path leads from the shell box to
the browser's profile, its vault, its control socket or its CDP port.** The model reaches the
browser only through Aura: the MCP bridge and the operator's live view.

Non-goals:
- **Hiding page content from the model.** Snapshots, reads and screenshots return what the page
  shows, which is the purpose of the browser. If PII on those pages lands in transcripts or
  memory, `internal/redact` and any future classifier deal with it, not this spec.
- **Taking browsers away from the shell box.** The shell box keeps its Playwright Chromium for
  artifact validation, on throwaway profiles with no logins. The operator can only log in through
  the live view, and the live view reaches only the browser box.
- **Making the agent shell non-root.** D-12 stays.

## Decisions (proposed)

1. **One browser box per identity, never shared.** The container is `aura-browser-<id>` and its
   volume is `aura-browser-<id>`. The id is the identity UUID, as it is for `aura-box-<id>`. No
   user-supplied string is ever part of a name. The volume is mounted at `/workspace` in the
   browser box and nowhere else. The browser box mounts none of the identity's cache volumes
   (uv, npm, pip).
2. **Its own network namespace.** The browser box must not join `container:aura-box-<id>`,
   because CDP and the dashboard listen on loopback. It gets its own egress sidecar,
   `aura-egress-browser-<id>`, launched by the existing `launchEgress` path with the same floor:
   RFC1918, cloud metadata and the shared-services bridge are dropped. As a result, the shell box
   cannot reach the browser box at the network layer either.
3. **Box files follow the browser.** The vault key `/run/aura/agent-browser.key` is written only
   into the browser box, on each start. The shell box no longer receives it. With no key, the image
   entry point already refuses to run (exit 78). Any `agent-browser` started from `shell_exec` is
   therefore refused by construction, not by a rule in the prompt.
4. **Aura is the only way in.** The browser recipe's stdio MCP server (`agent-browser mcp`) is
   started in the browser box. The live-view relay (`browser-relay.mjs`, `internal/agui/browser_live.go`)
   is exec'd in the browser box too. Both still key on the caller's identity from `identityctx`, so
   a caller can only ever reach their own browser box.
5. **Created lazily, stopped when idle.** No browser box exists until an identity's first browser
   call. It is not created at `Resolve`. It has its own idle TTL, separate from the shell box's
   30 minutes. When that TTL passes, it is **stopped**, which returns its memory, and the profile
   volume keeps the login (prd.md §12). The next browser call starts it again. Suspending the shell
   box never touches the browser box, and the reverse also holds.
   - *Amended 2026-10-12 by the operator: the sidecar is always running, not the browser.* The
     box is still created at the identity's first browser call. After that it is never stopped
     for idleness. `aura serve` starts every existing browser box at boot, egress sidecar included
     (`StartBrowserBoxes`). Then the browser MCP server answers the first call and the board's
     probe instead of showing "dial failed". Chrome inside still closes after its own 180 s
     (`AGENT_BROWSER_IDLE_TIMEOUT_MS`). The box it leaves holds 3.7 MiB and 4 pids (measured
     2026-10-11, open question 2), so the cost is the admission slot.
     `AURA_SANDBOX_BROWSER_IDLE_TTL_SEC` and the browser's hold and touch on the reaper's clock
     are gone with the idle stop.
6. **Its own resource caps.** The browser box gets `AURA_SANDBOX_BROWSER_MEMORY_LIMIT`,
   `_PIDS_LIMIT` and `_CPU_LIMIT`. They default to today's per-box envelope of 2 GiB, 1024 pids and
   2 CPUs, the envelope three browsers were measured inside. The shell box keeps its own caps.
   `maxOpenBrowsers` stays at 3 per identity.
7. **Host-wide admission, with no eviction across identities.** `AURA_SANDBOX_BROWSER_MAX_ACTIVE`
   bounds how many browser boxes run at once on the host. When the limit is reached, a browser call
   from an identity with no running browser box is refused with a tool error the model reads,
   saying it should try later. Aura never stops another identity's browser box to make room. One
   user's question must not close another user's logged-in session or live view. Section *RAM*
   below has the default.
   - *Amended 2026-10-15 by the operator: a slot is a Chrome, not a box.* With the box always
     running (decision 5, amended), a slot counted per running box was held forever: the fourth
     identity on a host admitting three was refused until another was deprovisioned (final
     review, Important 2). Starting a box, at boot or on any call, now takes no slot. A slot is
     asked for before each browser call that can open a Chrome, and before the ChatGPT sign-in.
     It counts the identities whose browser was called within agent-browser's 180 s idle close,
     plus those whose running box holds a process launched with `--remote-debugging-port`, so a
     Chrome left open across an Aura restart still counts. The identity's own open browser
     always holds its slot. The refusal says the slots are open for other members and that one
     closes 3 minutes after its last call. Measured on 2026-10-15 by the docker_integration tests,
     one with a stand-in process and the relay test with the image's real Chrome: a second
     identity is refused while a Chrome is open, and admitted once it closes, its box still
     running.
     What this does not show:
     - the lab VM: the change was not deployed there;
     - many identities calling at once;
     - a browser launched without that flag, which would not count.

     A Chrome hung past its idle close keeps its slot until its process ends.
8. **Downloads cross only through Aura, as files the model is told about.** Measured 2026-10-11:
   - a click on an attachment or `download` link through `agent-browser mcp` saves the file under
     the server's name, in `$HOME/Downloads` by default, or in `AGENT_BROWSER_DOWNLOAD_PATH` when
     that is set;
   - the click returns in about 0.02 s and its result does not mention the download, so today the
     model is never told a file arrived;
   - the file grows as `<name>.crdownload` and is renamed to `<name>` only when complete.

   The proposal:
   - The browser box's entry point sets `AGENT_BROWSER_DOWNLOAD_PATH=/workspace/downloads`, so
     every download lands in one known directory.
   - After each browser tool call, the bridge lists that directory. It copies every completed
     regular file it has not copied before into the shell box's `/workspace/downloads/`, using
     `CopyArtifactOut` and `WriteFileStream` (`internal/sandbox/usersandbox/router_tools.go`).
     "Completed" means no `.crdownload` suffix. A file is identified by name, size and mtime.
   - The bridge appends one line per copied file to the tool result, naming its shell-box path. A
     `.crdownload` still growing gets a line saying a download is in progress, with its size so
     far. The model collects the finished file on its next browser call, `agent_browser_wait_ms`
     included.
   - Names are reduced to their base name. A name already used in the shell box gets a numeric
     suffix. Symlinks and non-regular files are refused. The size is bounded, and the plan
     picks the bound. The nearest existing ones are `send_file`'s 50 MiB
     (`internal/agent/tools/send_file.go:46`) and `AURA_ASSET_MAX_DOCUMENT_BYTES`.
   - *Amended 2026-10-15 (final review, Important 5):* nothing removed a copied file from the
     browser volume. Past 1,000 files the listing failed on every call, and a file over the
     limit was announced on every call. Now:
     - A copied file leaves the browser volume on the next browser call, through the directory's
       fd, and only while it is the same inode, size and mtime that was copied.
     - A file over the limit is announced once, and stays where it is.
     - The listing takes 1,000 files per call and no longer fails past them.

     Measured by the docker_integration tests on 2026-10-15. What this does not show: the lab
     VM, where it was not deployed.

   Upload exists only as the CLI's `upload`, and no MCP tool exposes it. Its crossing stays an open
   question.
9. **Existing logins move once, then leave the shell box.** On an identity's first browser-box
   start, Aura copies `/workspace/.agent-browser-home` from the shell volume into the browser
   volume. After the copy is verified, Aura deletes it from the shell volume. If the copy were left
   behind, every existing user would keep a cookie jar the shell can read. On the lab VM the
   operator's box holds 553 MB in 4013 files there, 117 MB of it outside Chrome's caches: 20
   profiles, 19 cookie databases and 19 `Login Data` files, all readable by its shell
   (measured 2026-10-11).
   - *Amended 2026-10-15 by the operator: the legacy home is deleted, never copied.* The copy was
     retired on 2026-10-12 (acceptance item 8), and with it every path that removed the home. The
     final review found that every appliance upgraded from the earlier layout still keeps its
     logged-in profile on the shell volume, where one shell command drives it past the browser
     box's guards (Critical 1). At boot, Aura now removes `/workspace/.agent-browser-home` from
     every shell workspace volume on the host. It uses a short-lived helper container over the
     volume (`aura-sweep-<id>`), so a volume whose box is gone is reached too. The removal is
     delete-only and idempotent. A shell box does not start until its identity's home is gone:
     `Resolve` runs the same sweep once per identity per process and fails closed if it fails.
     Whoever kept logins only there signs in again once, in the browser box. Measured on
     2026-10-15 by the docker_integration test: a synthetic profile is removed both from a volume
     whose box is gone and under a running box, the shell's own files stay, and no helper is left.
     What this does not show: the lab VM and the appliances, where it was not deployed. `SandboxRouter.Destroy`, called by the
    deprovision saga (`cmd/aura/serve_provisioning.go:322`), also removes `aura-browser-<id>`, its
    egress sidecar and its volume. A login must not outlive its identity. The step stays
    fail-closed and resumable like the rest of the purge.
11. **Tool arguments become an allowlist.** Today the bridge refuses only `--profile` in
    `extraArgs` (`internal/agent/mcptools/bridge_browser_profile.go`). Flags such as `--state`,
    `--restore` and `--auto-connect` move session state. Every one of the 29 tool schemas also
    takes `restore`, `restoreSave`, `restoreCheck*`, `caCert`, `clearCaCert`, `namespace` and
    `idleTimeout` as first-class arguments (measured from `tools/list`, 2026-10-11).
    `agent_browser_open` also takes `allowedDomains`, `headed`, `timeoutMs`, `webgpu` and
    `webmcp` (lab VM image, 2026-10-11). The bridge
    should pass only the arguments and flags the recipe needs and drop the rest. The list comes
    from the plan, after reading the installed CLI's flags.
12. **`agent_browser_eval` asks the operator. Agreed 2026-10-11.** JavaScript in the page reads
    every cookie that is not HttpOnly and the whole of Web Storage. Many sites keep bearer tokens
    there, and Google's `SAPISID` is not HttpOnly. `eval` is graded `MCPActionDestructive` in
    `bridge_risk_browser.go`, so the operator approves each call. Removing it would have cost
    scraping power, and leaving it ungated would have left those values one call away.
13. **The skill loses its shell fallback.** `browser-aura/SKILL.md` stops teaching
    `agent-browser …` through `shell_exec`, and stops saying downloads land in `/workspace`. It
    describes the crossing from decision 8 instead.

## Multi-user

- **Isolation between users is the same boundary as today, applied twice.** There is one shell box
  and one browser box per identity, with names derived from the identity UUID and volumes that
  are never shared. The floor drops traffic between boxes.
- **No shared browser.** Two members never share a jar, even inside one organisation.
- **Fairness.** Each identity holds at most one browser box with three sessions. The host limit
  refuses new identities. It never takes capacity away from identities already browsing.
- **Operators and admins.** The live view stays restricted to the caller's own identity, as today.
  No admin path into another identity's browser box is added.
- **Deletion.** Deprovisioning removes the logins (decision 10). Retiring the seeded `local`
  identity moves Postgres rows only (`identityctx`). Whether its browser box is renamed, migrated
  or destroyed is an open question.

## RAM

Measured on the lab VM on 2026-10-11 under runc, with the 2 GiB, 1024-pid cap, on ansa.it,
lastampa.it and youtube.com
(`docs/superpowers/verification/2026-10-11-browser-sidecar-lab-vm.md`). Memory is the box
cgroup's anon. Page cache is left out because it is charged to whichever box first read a file,
the Chromium binary included, and is reclaimable.

| What | Anon | PIDs |
|---|---|---|
| Box before its first browser | 0.6 to 0.7 MiB | 4 |
| Idle egress sidecar | 1.2 MiB (2026-10-11, light-page run) | 1 |
| One session (ansa.it) | 464 to 496 MiB | 243 to 250 |
| Three sessions (+ lastampa.it, youtube.com) | 1081 to 1096 MiB | 641 to 645 |
| 10 s after closing all three | 3.3 to 3.8 MiB | 4 |
| Running box 200 s after its last call, Chrome closed by its idle timeout | 3.7 MiB | 4 |

The 2026-10-11 light-page figures (251.7 MiB per session, 692.5 MiB for three) understate real
sites by about half. prd.md §12 records up to 868 MiB for one ANSA browser on 2026-10-10.

What follows from the table:
- **The sidecar barely costs anything.** While the browser box runs with no browser in it, it
  adds a few MiB per identity, egress sidecar included. When stopped, it adds nothing. The
  browser's own memory moves to another box but does not grow.
- **The per-identity cap does grow.** Today one 2 GiB cap covers the shell and the browsers
  together. With decision 6, the worst case per identity becomes 2 GiB for the shell plus 2 GiB
  for the browser. Docker memory limits are caps, not reservations, so the real host load is the
  sum of live Chromiums plus live shell work.
- **Separation also ends an interference.** Today browsers and shell work share one cgroup. Its
  measured symptom is pids: a fifth browser exhausted the box's 1024 (prd.md §12, 2026-10-10).
  The same sharing applies to memory, where a browser and a LibreOffice or pip job could push each
  other into the OOM killer. That was not measured. After the split, neither can starve the other.
- **What bounds the host is admission plus idle stop.** Proposed default for
  `AURA_SANDBOX_BROWSER_MAX_ACTIVE`: when unset or 0, derive
  `max(1, floor(0.5 × host MemTotal / AURA_SANDBOX_BROWSER_MEMORY_LIMIT))` from
  `docker info` at startup, and log the derived value. On the lab VM it gives 3. Measured
  there on 2026-10-11 with the full stack up, three boxes of three heavy sessions each fit:
  - no box reached its cap, and none was OOM-killed;
  - host `MemAvailable` went from 8184 MiB to a low of 3482 MiB;
  - swap in use rose from 193 to 835 MiB.

  The 3 matched only because half of `MemTotal` (7.5 GiB) is close to what this stack leaves
  free (8.2 GiB). The formula does not see the stack. Open question 3 keeps the choice.
- **Chrome returns its memory before the box stops.** agent-browser closes an idle Chrome after
  180 s, and `agent_browser_close` frees a session within 10 s. Either way the box is back to
  about 4 MiB of anon and 4 pids. Stopping the box then frees almost nothing more. What it does
  free is an admission slot (decision 7), because a running box counts whether or not Chrome is
  in it. A cold start costs about 1.6 s more than a warm one on the lab VM: 4.1 to 4.5 s against
  2.5 to 2.7 s on a local page.

A search of `internal` and `cmd` found no host-wide admission for shell boxes. That gap exists
already, and this spec does not widen it. It is noted, not fixed here.

## Shape

The files the plan will touch, all under 600 LOC today:
- `internal/sandbox/usersandbox/spec.go` (82). Adds a `BoxKind` enum, `Shell` or `Browser`,
  backed by an int like `RuntimeClass`. The spec must make "a shell box with the browser volume"
  unrepresentable, not merely rejected.
- `translate.go` (65). Picks mounts by kind. A browser box gets only its own volume and no cache
  mounts. Every dangerous `HostConfig` field stays pinned.
- `docker_backend.go` (284) and `docker_backend_lifecycle.go` (318). Add per-kind naming and
  per-kind limits; today `NewDockerBackend` takes one `Resources`. The browser create, start and
  stop path goes in a new `docker_backend_browser.go`, so neither file nears 600 lines.
- `router.go` (172), `router_provision.go` (66) and `reap.go` (68). Add `RouteBrowser`, the lazy
  create, admission, the browser idle sweep, and `Destroy` covering both boxes.
- `cmd/aura/sandbox_box_files.go` (39). Box files per kind: the key goes to the browser only.
- `cmd/aura/sandbox_mcp_box.go`, `internal/mcp/box_session.go` (118). The browser recipe launches
  in the browser box.
- `internal/agui/browser_live.go` (302). The relay opens in the browser box.
- `internal/agent/mcptools/bridge_browser_profile.go` (65), `bridge_call.go` (143) and
  `bridge_risk_browser.go`. The `extraArgs` allowlist, the `eval` grade and the file crossing.
- `internal/config/config_sandbox.go`. The new env vars and their catalog entries in prd.md.
- `internal/skills/embed/browser-aura/SKILL.md`. Decision 13.

The image does not change: the browser box runs the same `aura-sandbox` image, so it costs no
extra disk. Comments in `docker/aura-sandbox/Dockerfile` and `agent-browser.sh` that say the
browser and the shell share a box get updated.

### New env vars

| Var | Default | Meaning |
|---|---|---|
| `AURA_SANDBOX_BROWSER_MEMORY_LIMIT` | 2 GiB | Memory cap of a browser box |
| `AURA_SANDBOX_BROWSER_PIDS_LIMIT` | 1024 | PID cap of a browser box |
| `AURA_SANDBOX_BROWSER_CPU_LIMIT` | 2 | CPU cap of a browser box |
| ~~`AURA_SANDBOX_BROWSER_IDLE_TTL_SEC`~~ | removed 2026-10-12 | The browser box is never stopped for idleness (decision 5) |
| `AURA_SANDBOX_BROWSER_MAX_ACTIVE` | 0 = derived | Identities with an open Chrome allowed on the host (decision 7, amended 2026-10-15) |

## Acceptance

Integration tests under `docker_integration`, plus daemon-free unit tests for naming, spec,
translate and admission:
1. With a session open in the browser box, the shell box shows:
   - `find /` for `Cookies`, `Login Data` and `*.sock` under `.agent-browser*` finds nothing;
   - `/proc/self/mountinfo` has no `aura-browser-` volume. `/proc/mounts` shows only the device
     and the mount point, never the volume name, so a check against it passes vacuously;
   - no Chromium pid;
   - no loopback listener other than its own, and a connect to the browser box's address on the
     CDP port fails. Chrome is launched with `--remote-debugging-port=0`, so the test reads the
     port inside the browser box first (lab VM, 2026-10-11).
2. The key file is absent from the shell box and present, 0600, in the browser box.
3. `agent-browser` run through `shell_exec` exits 78.
4. A login cookie survives a browser-box stop and start.
5. No browser box exists before the first browser call. ~~It is stopped after its idle TTL, and
   its memory is measured before and after.~~ *Amended 2026-10-12:* the box is still running
   long after its last call. After `docker restart aura` it is running again before any browser
   call. The board's probe of `browser` reads "ok (29 tools)".
6. With `MAX_ACTIVE=1`, a second identity's browser call is refused and the first identity's
   browser keeps running.
7. Deprovisioning an identity removes its browser container, egress sidecar and volume.
8. ~~A shell volume carrying `.agent-browser-home` is migrated. The shell box then holds no cookie
   database, and the login works in the browser box.~~ *Removed 2026-10-12 by the operator: the
   legacy migration is gone.* On the lab VM, `Resolve` ran it before every box start, for both
   kinds. It copied the shell's profile onto a browser volume that already held its own and
   compared the copy back, and the comparison failed on every call. Every browser call then ended
   after 18 to 42 s with `browser migration verification mismatch`, and the browser box never
   started. Starting a box now touches only that box's own objects
   (`TestResolveBrowserNeverTouchesTheShellBox`). ~~A profile left in a shell volume stays
   there.~~ *Amended 2026-10-15:* it is deleted at boot, and the shell does not start while it is
   there (decision 9, amended).
9. Arguments and `extraArgs` outside the allowlist are refused, and `eval` reaches the approval
   gate as `MCPActionDestructive`.
10. A slow download, the 2026-10-11 fixture of 8 MiB over 8 s, behaves as follows:
    - it never reaches the shell box while it is still `.crdownload`;
    - it arrives complete on the first browser call after it is renamed, byte-identical;
    - that call's result names its shell-box path.

    A second file under the same name gets a suffix, and a symlink planted in the downloads
    directory is refused.
11. Coverage stays at 85% or more across the tier matrix (CLAUDE.md).

Live end-to-end on the lab VM, with its dated file in `docs/superpowers/verification/`:
- the operator signs in to a real site through the live view;
- the agent works on the site and downloads a file into the shell workspace;
- the agent's `shell_exec` searches for the cookie store and the CDP port and finds neither;
- a second member's browser runs at the same time, with RAM and pids recorded for both;
- the host admission limit is hit once.

## Open questions

1. **Upload.** No MCP tool exposes upload, and the CLI's `upload` is out of the shell's reach. One
   option is a bridge-level tool that copies one named shell-workspace file into the browser box
   and runs `agent-browser upload` there. The other is to drop upload. Neither has been measured.
2. **Browser idle TTL.** agent-browser closes Chrome after 180 s of idleness
   (`AGENT_BROWSER_IDLE_TIMEOUT_MS`). Should the box stop at the same moment, or later so a resumed
   task does not pay a cold start? Measured on the lab VM, 2026-10-11:
   - after Chrome closes, a running box holds 3.7 MiB of anon and 4 pids, so memory does not
     decide this;
   - a cold start costs about 1.6 s more than a warm one;
   - a running box still holds an admission slot (decision 7).

   So the choice is between 1.6 s per resumed task and a slot kept by a box with no browser in
   it. A third option is to count running Chromes instead of running boxes in admission. That
   cannot be read from Docker, so it would mean a count kept by the bridge.
   *Closed 2026-10-12 by the operator:* the box keeps its slot and is never stopped for idleness
   (decision 5, amended). *2026-10-15:* the box is still never stopped, but it no longer keeps a
   slot; admission counts open Chromes, the third option (decision 7, amended).
3. **The admission default.** Half of `MemTotal` gave 3 on the lab VM, and three heavy boxes
   fit there with swap starting (RAM above). Should the default keep reading `MemTotal`, or read
   what the running stack leaves free, which this formula does not see?
4. **The seeded `local` identity.** What happens to its browser box when it is retired at first
   enrolment?

## What this spec does not prove

- None of it is built. The invariant was measured on two hand-built boxes, not on the router
  wiring proposed here.
- The RAM table comes from three real sites on the lab VM. Heavier pages, video playback and
  logged-in sessions were not measured, and the three-box run shared the host with an
  unattributed workload of about 1.6 GiB.
- gVisor is not covered. The operator retired it on 2026-10-11: runc works well, and gVisor
  caused problems in the past. Every box runs on runc.
- Nothing here protects what the page shows to the model. `eval` is gated by the operator's
  approval, not removed: an approved call still returns what the page's JavaScript can read.
- The sidecar does not bound shell-box memory across the host. That gap exists today and remains.
