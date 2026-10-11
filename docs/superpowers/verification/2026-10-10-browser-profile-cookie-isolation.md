# Measurement — browser cookie-store isolation inside the per-identity box

Run on 2026-10-10 against the shipped box image, before any spec or amendment (CLAUDE.md
PRD-first: misura, poi emenda, poi implementa). Driven by the operator's pentest finding:
"i database dei cookie risultavano leggibili dal processo root della sandbox".

## Setup (real stack, not a toy)

- Image: `aura-sandbox@sha256:1dd37f37d433c868a68ac4ee2067618db6d7e1b81a0d2c80bc9f41729a9635dd`
  (`ghcr.io/chetto1983/aura-sandbox:edge`, 4.85 GB, pulled — the local build is blocked here
  only by the agent proxy's untrusted CA on apt-over-HTTPS, not by the Dockerfile).
- Docker 29.8.2, runc. Containers created faithfully to `DockerBackend.createBox`:
  `aura-sandbox:latest`, named `/workspace` volume, `WorkingDir=/workspace`, `sleep infinity`,
  **no `--user`, no `--privileged`** (host-exposure is unrepresentable in `SandboxSpec`).
- The model's shell is modelled as `docker exec` with no `User` field — exactly what
  `docker_backend_exec.go` issues for `shell_exec`. It runs as `uid=0(root)`.
- The browser is the real one: `/usr/local/bin/agent-browser` (the image entrypoint),
  `HOME=/workspace/.agent-browser-home`, profile `~/profiles/default`. A real Chromium wrote
  a real `Default/Cookies` SQLite DB. One **sentinel** cookie (`measure_sentinel=NOT_A_SECRET_fixture`)
  was set via a loopback server inside the box. No real account, no credential, nothing decrypted.

## Results

| Topology | Model shell reach to the cookie store |
|---|---|
| **Baseline** (shipped: one box, shell + browser co-resident as root) | **EXPOSED** |
| **Weak mitigation** (same box, browser as uid 1001, profile `0700`/`0600`) | **STILL EXPOSED** to the root shell |
| **Sidecar** (browser in its own box; shell box does not mount the profile volume) | **HIDDEN** — 0 reachable |

### Baseline — EXPOSED

- `Default/Cookies` is `-rw------- root root` (0600). The model shell reads it anyway, because
  it **is** root: `head -c16` returns the `SQLite format 3\0` header; read succeeds.
- The shell enumerates the live jar: one row, `name=measure_sentinel host=127.0.0.1
  encrypted_value_len=67`. The jar is live and listable from the shell.
- The agent-browser vault key `/run/aura/agent-browser.key` is readable by the model shell.
  Aura writes it `0600` on every `Resolve` (`cmd/aura/sandbox_box_files.go:37`); the root shell
  reads it regardless. (This run wrote the key by hand and got `0666`: that mode is an artefact of
  the hand-built setup, not Aura's behaviour, and is not part of the finding.)

### Weak mitigation — STILL EXPOSED

- Profile dir `0700` owned by `aurabrowser` (uid 1001), cookie file `0600`. The root model shell
  `cat`s the file and reads the content. DAC does not constrain root; the operator's own note
  ("0600 non ferma root") is confirmed on the real box. Running the browser as a non-root uid and
  tightening directory permissions does **not** close the finding while the shell stays root.

### Sidecar — HIDDEN

- Browser runs in its own box with the profile on its own volume; the shell box mounts only its
  own `/workspace`. From the shell box:
  - `find /` for `Cookies`/`Login Data`: **0 hits**.
  - the profile path does not exist (`No such file or directory`).
  - **0** Chromium/agent-browser pids visible (separate PID namespace — no `nsenter` path).
  - no `/var/run/docker.sock`, and the browser volume is absent from `/proc/mounts`: root in the
    shell box has no primitive to attach it.
- The browser box keeps its persistent `Default/Cookies`, so the per-session login survives
  (prd.md §12). The isolation does not cost the feature.

## What this measurement proves

- The finding is **intra-box, same-root**, not cross-tenant. Different identity = different
  container + different volume + host-exposure unrepresentable in `SandboxSpec`; no path.
- Of the operator's proposed measures, only "profili in sandbox distinte" closes it. Non-root
  browser user, 0700 dirs, and 0600 files do not, as long as the agent shell shares root with the
  browser in the same box.
- Putting the browser in its own box, with the profile volume absent from the shell box, removes
  every filesystem and process path the model shell has to the cookie store.

## What this measurement does NOT prove

- Not measured: Chrome's at-rest cookie encryption strength. `encrypted_value` is present; the
  decryption key (profile `Local State`) was not read and nothing was decrypted. The claim is
  about **reachability of the files by the model shell**, not plaintext recovery.
- Not measured on a live login: no real Google/site session was created. The sentinel is a
  fixture. A real jar would differ only in row contents, not in file reachability.
- Not an end-to-end run of the sidecar as Aura would wire it: this used two hand-built boxes to
  isolate the invariant. It does not measure the daemon changes a sidecar needs (a second box per
  identity, the browser bridge targeting it, lifecycle/suspend/reaper doubling, egress netns for
  the browser box). Those are the spec's job and must be measured when built.
- Not a host-escape test. Consistent with the operator's report: `/proc/1/root` is the box's own
  filesystem, no docker/containerd socket is mounted; no path off the box was demonstrated or
  attempted.
- gVisor (runsc) not covered: measured under runc only.

## How LibreChat and Hermes Agent handle it (source read, not run)

Read the same day from shallow clones, only reading and grepping. Neither was executed.

**Hermes Agent** (`NousResearch/hermes-agent` @ `b406d25`): **same shape as Aura, no isolation.**
- `bot_desktop.placement: auto` puts the browser *inside* the terminal backend's container on
  purpose, and browser commands become `docker exec` into that container
  (`tools/bot_desktop/placement.py:1-14,49-63`, `tools/browser_tool_session.py:654-695`).
- The shell runs `docker exec` with no `-u`, as root. Root keeps `DAC_OVERRIDE`, and the browser
  user `pn` has passwordless sudo (`tools/environments/docker.py:300-313,1078-1094`,
  `docker/sandbox-desktop.Dockerfile:77-82`).
- The only guards run inside the agent process. `read_file`/`write_file` deny some credential
  paths, and the deny is labelled "not a security boundary; the terminal tool can still bypass"
  (`agent/file_safety.py:249,337-339,352-366`). The terminal approval patterns have no entry for
  browser profiles (`tools/approval_detection.py:17-39`).
- The docs state the risk: the profile is reachable by "the bot's own `terminal` tool", and the
  lease is "a tool-level fence… not an OS one" (`website/docs/user-guide/features/bot-screen.md:20-35`).
  `SECURITY.md` says the OS is the only boundary (`SECURITY.md:60-65`).
- Real separation exists only with a cloud browser backend such as Browserbase, where cookies stay
  remote.

**LibreChat** (`danny-avila/LibreChat` @ `e1dfc10`): **does not have the problem, because of its
architecture rather than an added guard.**
- Model-executed code runs on a separate Code API service set by `LIBRECHAT_CODE_BASEURL`. Each
  call carries a JWT bound to user and tenant, with a TTL of at most 300 s
  (`packages/api/src/agents/execution.ts:3,304`, `packages/api/src/auth/codeapi.ts:66-98,325-330`).
  Its docs describe NsJail/microVM workers with no network. That sandbox's source is in another
  repo and was not verified.
- LibreChat has no browser of its own. Web search calls APIs from the server. Puppeteer exists only
  as an admin-only stdio MCP server, which runs as a child of the LibreChat server: outside the code
  sandbox's reach, but shared between users (`packages/api/src/mcp/connection.ts:1735-1745`,
  `packages/data-provider/src/mcp.ts:534-537`).
- Credentials stay server-side. MCP OAuth tokens sit in MongoDB, AES-encrypted with keys taken from
  the server environment (`packages/api/src/mcp/oauth/tokens.ts:432,774`). No path to the code
  sandbox was found.

**Reading.** Neither project offers a pattern to copy for keeping a *persistent, logged-in* browser
profile next to a root shell. Hermes documents the same exposure Aura has, and accepts it. LibreChat
avoids the exposure by never letting credential-holding state and model-executed code share a
filesystem. That is the same invariant the sidecar measurement above shows working.

## Follow-up 2026-10-11: footprint, listeners and tool surface

Same image, same Docker 29.8.2 and runc. The box was capped as Aura caps it: 2 GiB, 1024 pids,
2 CPUs. The egress sidecar ran with an empty ruleset. The page was a light 20 KB page on a
loopback server inside the box.

| State | Memory (`docker stats`) | PIDs |
|---|---|---|
| Idle box (`sleep infinity`) | 4.3 MiB | 1 |
| Idle egress sidecar | 1.2 MiB | 1 |
| One browser session | 251.7 MiB | 174 |
| Three browser sessions | 692.5 MiB | 529 |
| After closing all three | 256.6 MiB | 39 |

Listeners while one session ran, all bound to `127.0.0.1`:
- Port 38791 answered `/json/version` as `Chrome/151.0.7922.34`: this is the DevTools Protocol
  (CDP). Any process in the box's network namespace can drive it.
- Port 35831 served agent-browser's HTML dashboard.
- The daemon's control socket sits at `/workspace/.agent-browser-home/.agent-browser/s1.sock`,
  on the workspace volume.

`agent-browser mcp` lists 29 tools. None of them reads cookies, storage or saved state.
`agent_browser_eval` runs arbitrary JavaScript in the page, and the bridge grades it
`MCPActionMutate` (`internal/agent/mcptools/bridge_risk_browser.go:34`). The CLI has `cookies`,
`storage`, `state` and `auth` commands, which the MCP surface does not expose. Download and upload
exist only in the CLI.

This follow-up does not prove the following:
- The memory column includes page cache, which was not separated from anonymous memory. The 39 pids
  after closing are agent-browser daemons waiting out their idle timeout.
- A light page is not a real site. prd.md §12 records 137 to 263 pids and up to 868 MiB per
  browser on a real site on the lab VM.
- Reaching CDP was shown with `/json/version`. No cookie was read through CDP.
- Reading `document.cookie` or Web Storage through `eval` follows from browser semantics. It was
  not measured here.
