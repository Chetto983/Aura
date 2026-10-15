# Browser sidecar: the credentials left the shell, and the unrun tests pass

Date: 2026-10-15. This file checks `docs/superpowers/plans/2026-10-11-browser-sidecar.md` against
what [2026-10-12-browser-sidecar-always-on.md](2026-10-12-browser-sidecar-always-on.md) left open.

## Lab VM 192.168.101.158 (read-only)

The VM runs aura `37e9e9a61` from the GitLab edge images. Only names, counts, sizes and modes were
read; nothing was read out of a cookie database, a login file or the key.

**The shell no longer holds browser credentials.** The volumes were inspected through a read-only
mount:

| Volume | `Cookies` | `Login Data` | `.agent-browser-home` |
|---|---|---|---|
| `aura-box-448ddbe1-…` (shell workspace) | 0 | 0 | absent |
| its npm, pip and uv cache volumes | 0 | 0 | absent |
| `aura-browser-448ddbe1-…` (browser box) | 1 | 1 | 7.9 MB |

On 2026-10-12 the shell volume still held the legacy home, which the shell could read; it is gone.
No shell box was running, so the invariant was not probed from inside a shell this time.

**The browser box comes back with Docker.**
- Docker restarted at 10:16:01 UTC, during the E2E in
  [2026-10-15-docker-restart-and-install-dir.md](2026-10-15-docker-restart-and-install-dir.md).
- aura then logged `derived browser admission limit max_active=3` at 10:16:22 and
  `aura serve: browser boxes started count=1` at 10:16:24.
- `aura-browser-448ddbe1-…` and `aura-egress-browser-448ddbe1-…` were running an hour later.

The 2026-10-12 file had seen this only on an `aura` recreate.

**MCP inside the browser box.** `agent-browser mcp` over `docker exec`, the command the bridge
runs, answered `initialize` as `agent-browser 0.38.2`, and `tools/list` returned 29 tools.

## The docker_integration tests the 2026-10-12 change never ran

**Setup.** WSL against Docker Desktop 29.8.1:
- the sandbox and egress images were `registry.gitlab.com/chetto1983/aura/aura-sandbox:edge` and
  `aura-egress:edge`, the ones published from `37e9e9a61`;
- the usersandbox tests run busybox boxes by design;
- the relay test runs the real sandbox image, with Chrome.

**Results.** Every test ran, and none was skipped:

| Test | Result |
|---|---|
| `TestBrowserSidecarLazyIsolationPersistenceAndPurge` | PASS, 7.85 s |
| `TestBrowserSidecarKeepsRunningWhenTheShellIdles` | PASS, 0.84 s |
| `TestBrowserSidecarInventorySeesAStoppedBox` | PASS, 0.49 s |
| `TestBrowserSidecarAdmissionSurvivesBackendRestart` | PASS, 0.82 s |
| `TestBrowserSidecarEgressFailureLeavesBoxStopped` | PASS, 0.80 s |
| `TestBrowserSidecarResumeRefusesMissingEgress` | PASS, 0.66 s |
| `TestBrowserSidecarRetirementPurgesCredentialsNotWork` | PASS, 1.01 s |
| `TestBrowserRelayDrivesTheBoxBrowser` (cmd/aura) | PASS, 1.61 s |

**What the relay test does.** Through the live-view relay it types `hi.1` into a page in the box's
own Chrome, and reads the value back from the DOM. It also checks that a `navigate` command sent
through the relay is dropped. No test container was left behind.

## The acceptance predicates

**The 11 acceptance predicates with real agent turns** need two identities, a live MAX_ACTIVE=1
refusal, a slow 8 MiB download and a real-site live view. The operator ran them, as the plan's
ledger had recorded they would, and confirmed them. This file holds no measurement of them
beyond that confirmation.

## The delegated Docker coverage floor

`internal/sandbox/usersandbox` delegates its package floor to `scripts/docker_coverage_gate.sh`.

**Setup.** It ran in WSL at HEAD `0df23b427`, the way CI's job runs it:
- the egress and sandbox images were named `aura-egress:latest` and `aura-sandbox:latest`, which
  CI builds. Here those names were given to the edge images published from `37e9e9a61`, whose
  source differs from HEAD only in docs, and removed after the run;
- `busybox:stable` and `python:3-slim` were pre-pulled;
- `scripts/sandbox_image_contract_test.sh` passed first.

**Results.** The gate exited 0 in 158 s, with 0 skipped tests and no test container left.

| Scope | Statements covered | Floor |
|---|---|---|
| docker_owned_internal, the gate's aggregate | 4400/4938 = 89.10% | 85% |
| `internal/sandbox/usersandbox`, package-local | **832/961 = 86.58%** | 85% |
| `internal/agent/tools` | 3568/3977 = 89.72% | (aggregate only) |

The package figure comes from the gate's own profile, summed per package. The two packages add up
to the aggregate.

On 2026-10-11 the plan's ledger measured the package at 928/1114 = 83.30%. The denominator fell
because `150cf3f65` removed the legacy migration, which was 510 deleted lines.

The owned-surface `db_integration` gate (`scripts/coverage_docker.sh`) was not re-run here. It
needs the local stack up.

## The final independent review

**Setup.** A read-only Claude Opus session (`claude -p --tools Read Grep Glob`) reviewed the five
commits:
- the diffs: `7f0242a2d`, `5c3104b81`, `f5e5b7b74`, `150cf3f65` and `6a3290e2f`;
- the code on master at `0df23b427`;
- the plan's Review Focus, line by line.

**Verdict: ship with fixes.** The new layout's isolation is sound and tested:
- the box kind decides the mounts;
- the key reaches only the browser box;
- the download crossing is no-follow;
- the browser arguments are allowlisted twice.

### Critical

**1. An upgraded appliance keeps its old logged-in profile on the shell volume.** No code path
removes `/workspace/.agent-browser-home` any more, and the spec's amendment says "A profile left in
a shell volume stays there". The shell box still ships Chromium and the raw agent-browser binary.
One shell command can therefore drive the old logged-in profile without the browser box's guards.
- The lab VM's legacy home is gone. Today's measurement does not say who removed it.
- Every other appliance that ran the earlier layout still has its legacy home.

Checked against the code before recording.

### Important

**2. Always-on boxes make admission slots permanent.** On a host with `max_active=3`, the fourth
identity to use the browser is refused until another is deprovisioned. The refusal reads like
momentary load.

**3. Two global lifecycle locks serialize every identity's tool calls.**
- Each warm call now also runs the egress floor exec under them.
- `SuspendIdle` holds them through the slow stops, about 2.2 s each.

**4. A failed or cancelled `Resolve` stops a box that was already running and secured.** Download
collection routes with the tool's near-deadline context after the call, so this can stop Chrome.

**5. Downloads are never removed from the browser volume.** After 1,000 files in its lifetime, the
listing fails on every call.

### Minor

There are eight: 6-13 in the review. In brief:
- the egress probe checks the sidecar's network namespace, not the box's;
- dead `WithBoxFiles` and `Resume` admission code;
- the volume is created before admission;
- `headed` is still allowed;
- the shell's reply is unbounded;
- tests encode suspension, which production no longer does;
- the `AURA_SANDBOX_BROWSER_*` variables are missing from the PRD's env catalog;
- the boot start does not check that the identity exists.

The full review is kept in the session scratchpad, not in the repo.

## Still open in the plan's Task 5

- **Rulings and a fix pass** for Critical 1 and Important 2-5. Critical 1 and Important 2 reverse
  parts of the operator's 2026-10-12 amendments, so they need the operator's ruling first.
- **CI.** None has run since GitHub blocked the account.
