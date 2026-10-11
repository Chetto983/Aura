# Measurement — browser sidecar on the lab VM

Run on 2026-10-11 on the lab VM `192.168.101.158`, for the parts of
`docs/superpowers/specs/2026-10-11-browser-sidecar-design.md` that a light page on another host
could not answer: memory and pids on real sites, the host admission default (open question 3),
and the browser box's idle stop against its cold start (open question 2). The isolation
invariant was measured again, this time with the appliance's own egress image and floor. The
operator's real box was read for what it exposes today and for what decision 9 would migrate.

The spec's gVisor question, open question 4 when this run started, was not measured, and the
spec now drops it. The operator retired gVisor the same day: runc works
well, and gVisor caused problems in the past. `runsc` is installed on this VM but unused: every
container runs on `runc`, and `AURA_RUNTIME` is unset in `/opt/aura/.env`.

## Setup

- **Host.** VMware guest, 8 vCPUs, `MemTotal` 15.07 GiB, 4 GiB swap, kernel `7.0.0-38-generic`,
  Docker 29.8.2, cgroup v2 with the systemd driver. The full appliance stack was running (21
  containers). The VM had booted at 11:53 UTC, and the runs started at 12:01. With the stack up,
  `MemAvailable` was 8184 MiB and 193 MiB of swap was in use.
- **Images.** `ghcr.io/chetto1983/aura-sandbox:edge` at `sha256:1268a4a5…` and
  `ghcr.io/chetto1983/aura-egress:edge` at `sha256:6a773f98…`. The sandbox image differs from the
  `1dd37f37…` of the 2026-10-10 ledger. It runs the same Chrome, `151.0.7922.34`.
- **Boxes.** These were built by hand to match `DockerBackend.createBox` and `toHostConfig`:
  hostname `aura-box`, Docker's init, 2 GiB, 2 CPUs, 1024 pids, a named volume at `/workspace`, a
  tmpfs at `/workspace/.scratch`, and `tail -f /dev/null`. The shell box also mounted its own
  uv, npm and pip cache volumes, while the browser box mounted none. Each box had its own egress
  sidecar (`--network container:<box>`, `NET_ADMIN`), carrying the floor ruleset copied from the
  operator's live sidecar. A random 64-hex key was written 0600 to `/run/aura/agent-browser.key`,
  in the browser box only. The names start with `meas-`, so nothing Aura reconciles by its
  `aura-box-` and `aura-egress-` names saw them, and all of them were removed afterwards.
- **Driver.** `agent-browser mcp` ran over `docker exec -i -w /workspace`, with stderr sent to
  `/tmp/aura-mcp-browser.log`, as `ExecStream` runs it. Every call carried `session` and
  `extraArgs: ["--profile", "~/profiles/<session>"]`, as the bridge sends them.
- **Memory.** It was read from the box's cgroup every second: `memory.current`, `anon` and
  `file` from `memory.stat`, `pids.current` and `memory.events`. `docker stats` shows
  `memory.current` minus `inactive_file`. Page cache is charged to the first cgroup that reads a
  page, so the Chromium binary counts against whichever box ran it first. **Anon is the cost
  that compares across boxes.** The `current` and `docker stats` columns depend on the order the
  boxes started.

## The operator's real box, as shipped

Box `aura-box-448ddbe1…` was read at 12:03. Only modes, counts and sizes were taken. Nothing was
read out of a cookie database, the key or a profile.

- The key `/run/aura/agent-browser.key` is 0600, owned by root, and the box's root shell can
  read it.
- `/workspace/.agent-browser-home` holds 20 profiles, 19 `Cookies` databases (all 19 readable by
  the shell), 19 `Login Data` files and 3 control sockets.
- The tree is 553 MB in 4013 files, of which 117 MB lies outside Chrome's cache directories.
  That is what decision 9 would move once.

The operator's own red-team run that day counted 38 cookie and login files in the same place,
consistent with these 19 and 19.

## Isolation with the appliance's egress floor

A session on ansa.it was open in the browser box. It was probed from a shell box built like
`aura-box-<id>`.

**Inside the browser box**, as a positive control:
- 17 Chrome processes ran.
- Two listeners were bound to `127.0.0.1`. Port 40151 answered `/json/version` as
  `Chrome/151.0.7922.34`, so it was CDP. Port 45195 was not.
- Chrome is launched with `--remote-debugging-port=0`, so the CDP port is random on each launch.

**From the shell box:**

| Probe | Result |
|---|---|
| `find /` (skipping `/proc` and `/sys`) for `Cookies`, `Login Data` | 0 |
| `*.sock` under any `*agent-browser*` path | 0 |
| Volumes in `/proc/self/mountinfo` | only its own: workspace and the 3 caches |
| `/run/aura/agent-browser.key` | absent |
| Processes whose command line contains `chrom` | 0 (5 processes in total) |
| `agent-browser --version` | exit 78 |
| Its own listeners | none |
| `127.0.0.1:40151` and `127.0.0.1:45195` | refused |
| Browser box address `172.17.0.3:40151` and `:45195` | timed out after 5 s |
| `example.com:443` | open in 0.08 s |

The shell box's floor counted the dropped packets: 25 on its `172.16.0.0/12` rule. The timeouts
came from the floor dropping them. A probe from the host to the browser box timed out as well,
not refused, because the browser box's own floor drops its replies to `172.16.0.0/12`, which it
counted 10 times. **Every port a box answers on is therefore unreachable from the private
address space, the host included.** Separately, the CDP and dashboard listeners are bound to
loopback.

Public egress stays open, by design (D-04). A file the shell can read can leave the box.

One correction to the 2026-10-10 ledger. It said the browser volume was absent from
`/proc/mounts`, but that check cannot fail: `/proc/mounts` shows a volume as its device and mount
point, never by name. This run read `/proc/self/mountinfo`, where the volume's name is part of
the source path. The other 2026-10-10 checks are unaffected: `find`, the missing path, the pids
and the docker socket.

## Real sites, one box

Box `meas-browser-a`, sessions opened one after another, each followed by 30 s of sampling.
Peak values are over those 30 s:

| Sessions open | Site added | `open` | Peak current | Peak anon | Peak pids | File at end |
|---|---|---|---|---|---|---|
| 1 | ansa.it | 14.4 s | 1013.7 MiB | 495.8 MiB | 250 | 479.0 MiB |
| 2 | + lastampa.it | 17.9 s | 1323.2 MiB | 731.0 MiB | 437 | 533.8 MiB |
| 3 | + youtube.com | 15.7 s | 1757.9 MiB | 1080.9 MiB | 645 | 555.6 MiB |

Boxes `b` and `c` opened the same three sites afterwards. Their anon matched box `a` within
about 35 MiB at every step: 464 to 478 MiB with one session, and 1086 to 1096 MiB with three.
Their file memory was 55 to 215 MiB, because the Chromium binary was already charged to box `a`.

- **Three heavy sessions use about 1.1 GiB of anon memory and about 640 pids.** That is under
  the 2 GiB and 1024-pid caps, and no box recorded a `memory.events` max or an OOM kill.
- **The spec's RAM table understates real sites.** Its 251.7 MiB per session came from a light
  page. A real news site costs 460 to 500 MiB of anon on its own.
- **An "idle box" is 4.3 MiB only before its first browser.** Ten seconds after three
  `agent_browser_close` calls, box `a` was back to 3.3 MiB anon and 4 pids. Its `memory.current`
  was 322.7 MiB, though, 311.7 MiB of it page cache, and `docker stats` showed 185.8 MiB. The
  "39 pids after closing all three" of the 2026-10-11 follow-up (in the 2026-10-10 ledger) did
  not reproduce on this image: no daemon
  outlived its close. Whether the newer image explains the difference was not checked.
- **The first `open` in a fresh volume took 12.6 s on a file:// page.** That ran eight minutes
  after the VM booted, at load 4.5. Later first opens took 2.5 to 3 s (see the cold start below).

## Three boxes at once: the admission default

After box `a`, boxes `b` and `c` were filled with the same three sites. All three were then held
open together for 90 s, nine Chromiums in all, with a keep-alive call every 30 s.

| Box | Peak current during the hold | Peak anon | Peak pids |
|---|---|---|---|
| `a` | 1485.8 MiB | 957.2 MiB | 619 |
| `b` | 1345.2 MiB | 1068.9 MiB | 607 |
| `c` | 1315.2 MiB | 1047.6 MiB | 622 |

The host over the run:

| Point | `MemAvailable` | Swap in use |
|---|---|---|
| Stack up, no browser | 8184 MiB | 193 MiB |
| Box `a` with 3 sessions | 7224 MiB | 186 MiB |
| Box `b` with 3 sessions | 5960 MiB | 187 MiB |
| Box `c` with 3 sessions | 3713 MiB | 792 MiB |
| Lowest point of the hold | 3482 MiB | 835 MiB at most |

No box was OOM-killed. After the run, the kernel log had no OOM line and every stack container
reported healthy. One open timed out: lastampa.it in box `c`, at 30.0 s.

Between t=285 s and t=371 s, `MemAvailable` fell 2157 MiB while box `c`'s first session added
about 530 MiB to its cgroup. About 1.6 GiB of that drop came from outside the measured boxes, and
it is not attributed here. The operator's red-team conversation was running on the same stack at
the time, and the load average reached 7.8 at 12:09.

**What this means for the default.** The spec proposes
`max(1, floor(0.5 × MemTotal / AURA_SANDBOX_BROWSER_MEMORY_LIMIT))`, which is 3 on this VM. Three
boxes at the bridge's limit of three heavy sessions did fit, but the host had started swapping
and kept 3.4 GiB in hand. A fourth box was not tried. By arithmetic only, one more box of the
same weight, about 1.1 GiB of anon plus its page cache, leaves the host about 2 GiB. The formula's 3 matched
here because half of `MemTotal` (7.5 GiB) happens to be close to what this stack leaves free
(8.2 GiB). The formula does not see the stack, and a host whose stack is heavier or lighter would
diverge.

## Idle stop against cold start

These runs used box `meas-browser-a` and its session `c1`. Each was timed on a `file://` page
inside the box, which keeps the network out of the figure, and on ansa.it. The states were:
- **Hot:** Chrome open.
- **Warm:** the box running, Chrome closed by agent-browser's 180 s idle timeout.
- **Cold:** the box and its egress sidecar stopped as `Suspend` stops them, then started as
  `Resolve` starts them, followed by a fresh `agent-browser mcp`.

Three runs back to back, 12:17 to 12:31. The load average was 2.9 to 5.4, and nothing else of
the measurement ran during them.

| Run | Hot `open` (file) | Warm `open` (file) | Warm `open` (ansa) | Box start | + egress | Handshake | Cold `open` (file) | Cold total (file) | Cold `open` (ansa) |
|---|---|---|---|---|---|---|---|---|---|
| 1 | 0.46 s | 2.53 s | 11.6 s | 0.59 s | 1.36 s | 0.29 s | 2.55 s | 4.20 s | 8.1 s |
| 2 | 0.34 s | 2.57 s | 7.8 s | 0.51 s | 1.27 s | 0.30 s | 2.52 s | 4.09 s | 7.8 s |
| 3 | 0.32 s | 2.69 s | 8.3 s | 0.55 s | 1.37 s | 0.31 s | 2.83 s | 4.52 s | 8.9 s |

"+ egress" is the box start plus the key rewrite plus the egress sidecar's start. "Cold total"
runs from `docker start` to the end of the first `open`.

What a running browser box held, 200 s after its last call (agent-browser's 180 s idle timeout
plus 20 s):

| Run | Chrome processes | Anon | Pids | `memory.current` |
|---|---|---|---|---|
| 1 | 0 | 3.6 MiB | 4 | 350.9 MiB (337.1 MiB file) |
| 2 | 0 | 3.7 MiB | 4 | 9.0 MiB |
| 3 | 0 | 3.7 MiB | 4 | 6.3 MiB |

With the browser open on ansa.it, the same box held 384 to 401 MiB of anon and 200 to 207 pids.

- **A cold start costs about 1.6 s more than a warm one.** The local page opened in 4.1 to 4.5 s
  cold against 2.5 to 2.7 s warm. Box start, egress start and the MCP handshake make up the
  difference. On a real site the network dominates: ansa.it took 7.8 to 11.6 s warm and 7.8 to
  8.9 s cold, so the difference is lost in the noise.
- **Once Chrome has closed, a running browser box holds almost nothing.** agent-browser's own
  timeout had already closed every Chrome process, and the box was down to 3.7 MiB of anon and 4
  pids. Stopping it then frees those few MiB and its egress sidecar's 1.2 MiB, and nothing more.
- **Run 1's 337 MiB of file memory was the Chromium binary,** charged to this box when it first
  ran during the fill. Stopping a container ends its cgroup, so after the first restart (runs 2
  and 3) the box no longer carried that charge.
- **The memory argument for the box's idle TTL is therefore weak. The admission argument is
  not.** Decision 7 counts running browser boxes, so a box whose Chrome has closed still holds an
  admission slot until its own TTL stops it.

## The tool schema: decision 11

`agent_browser_open` takes 17 arguments. Besides `session`, `url` and `extraArgs`, it takes those
decision 11 lists (`restore`, `restoreSave`, `restoreCheckFn`, `restoreCheckText`,
`restoreCheckUrl`, `caCert`, `clearCaCert`, `namespace`, `idleTimeout`). It also takes five more
that the spec does not name: `allowedDomains`, `headed`, `timeoutMs`, `webgpu` and `webmcp`.

## What this measurement proves

- The sidecar topology holds on the appliance's own egress image and floor. The shell box
  reaches no browser file, process, socket, key, CDP port or dashboard.
- The shipped wiring exposes the operator's real logins to the box shell today: 19 cookie
  databases and the vault key.
- A browser box with three heavy sessions costs about 1.1 GiB of anon memory and about 640 pids.
  Three such boxes fit on this VM with its full stack, at the edge where swapping begins.

## What it does not prove

- **It is not the sidecar as Aura would wire it.** The boxes were built by hand, and no Aura
  code was changed or run for them.
- **The figures cover three sites.** Heavier pages, video playback and logged-in sessions were
  not measured. prd.md §12 records up to 868 MiB for one ANSA browser on 2026-10-10.
- **The admission run shared the host with another workload.** About 1.6 GiB of the drop is
  unattributed. The VM is also a guest on the workstation, so its CPU timings are shared.
- **Timings on the VM's network are slow**, as prd.md §12 recorded on 2026-09-27. A site's `open`
  measures that network as much as the browser.
- **gVisor was not measured.** It was retired, so the question no longer applies.
- **Page cache was not forced out.** Whether the kernel reclaims a stopped browser's cache in
  time under pressure was not measured.
