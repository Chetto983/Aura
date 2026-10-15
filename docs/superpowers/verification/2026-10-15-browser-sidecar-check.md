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

## Still open in the plan's Task 5

- **The coverage floors.** The last measurement, in the plan's ledger on 2026-10-11, put
  `internal/sandbox/usersandbox` package-local at 928/1114 = 83.30%, under its 85% floor. It has
  not been re-measured since.
- **The final independent review.**
- **CI.** None has run since GitHub blocked the account.
