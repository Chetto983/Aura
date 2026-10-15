# Browser sidecar: the final review's fix pass

Date: 2026-10-15. This file records the fixes for the Critical and Important findings of the final
review in [2026-10-15-browser-sidecar-check.md](2026-10-15-browser-sidecar-check.md). The operator
ruled on each finding the same day:
- **Critical 1:** delete the legacy home at boot.
- **Important 2:** count only live Chrome.
- **Importants 3, 4 and 5:** fix all.

Every fix has its own commit and a test seen failing before the fix. The executor's ledger
(`.superpowers/sdd/2026-10-11-browser-sidecar/progress.md`) records the rulings taken during
the fixes, the deferred minors, and a ruling on every line the reviewer declined to judge.

## The fixes

| Finding | Commit | What changed |
|---|---|---|
| Critical 1: an upgraded appliance keeps its logged-in profile on the shell volume | `c2a7fb63a` | `aura serve` removes `/workspace/.agent-browser-home` at boot from every shell workspace volume on the host. A short-lived helper container (`aura-sweep-<id>`) does it, so a volume whose box is gone is reached too. A shell box does not start until its identity's home is gone. `Resolve` runs the same sweep once per identity per process and fails closed. |
| Important 2: always-on boxes made admission slots permanent | `1738407f9` | Starting a box takes no slot. A slot is asked for before each browser call that can open a Chrome, and before the ChatGPT sign-in. Two kinds of identity hold one: those whose browser was admitted within agent-browser's 180 s idle close, and those whose running box holds a process launched with `--remote-debugging-port`. |
| Important 3: process-wide lifecycle locks serialized every identity | `fd11a0004` | The router, backend and download locks are per identity, striped by `internal/keylock`. That package is arcadedb's fact lock, extracted so it exists once. `SuspendIdle` stops one box at a time, under that identity's lock, after checking again that no call used it. |
| Important 4: a failed or cancelled `Resolve` stopped a box it had found running | `965bb3328` | `Resolve` stops a box on failure only in two cases: this call started the box, or the egress check failed while the caller's context was still live. |
| Important 5: downloads never left the browser volume | `db5336994` | A delivered file leaves the browser volume on the next browser call, and only while it is still the inode, size and mtime that was copied. A file over the limit is announced once, and stays. The listing takes 1,000 files per call instead of failing. |

The spec carries a dated amendment for each change of behaviour:
- decision 7 for admission;
- decision 9 and acceptance item 8 for the legacy home;
- decision 8 for downloads.

## Tests

**Setup.** WSL against Docker Desktop 29.8.1:
- the egress image was `registry.gitlab.com/chetto1983/aura/aura-egress:edge`;
- the relay test ran the image's real Chrome from `aura-sandbox:edge`;
- the other usersandbox tests run busybox and `python:3.13-alpine` boxes by design.

**Tests seen failing first.** Each failed before its fix and passed after it:

| Test | Kind | Failure before the fix |
|---|---|---|
| `TestResolveFailureStopsOnlyABoxItStarted` | unit, fake Docker API | 2 of 4 cases stopped a running box |
| `TestOneIdentityNeverWaitsForAnother` | unit | 5 s timeouts, for resolve and for download collection |
| `TestSuspendIdleLetsOtherIdentitiesWork` | unit | a 5 s timeout, and the reaper stopped a box used meanwhile |
| `TestResolveStartsABrowserBoxWithoutASlot` | unit | refused with "browser capacity exhausted" |
| `TestChatGPTLoginBrowserWaitsForRoomOnTheHost` | unit | the refused sign-in reached the box |
| `TestBrowserDownloadPlanAnnouncesAnOversizedFileOnce` | unit | the mark was not remembered |
| `TestBrowserDownloadsLeaveTheBrowserVolumeOnceDelivered` | docker_integration | the oversized file was announced again |

The new APIs failed first at compile time:
- admission in the bridge, the backend and the mount options;
- the legacy home sweep, both per volume and at boot;
- `internal/keylock`, now covered 100% by unit tests.

**docker_integration, all passing:**

| Test | What it shows |
|---|---|
| `TestBrowserSidecarAdmissionCountsOpenChromes` | A backend started with no memory of earlier calls still refuses a second identity beside an open stand-in Chrome. It admits that identity once the Chrome ends, while both boxes keep running. |
| `TestBrowserRelayDrivesTheBoxBrowser`, extended | With the image's real Chrome open and a single slot, a second identity is refused. It is admitted after `agent-browser close`. |
| `TestBrowserSidecarLegacyHomeLeavesTheShellVolume` | A synthetic profile is removed from a volume whose box is gone, and from under a running box by a fresh backend's `Resolve`. The shell's own file stays, and no helper is left. |
| The other browser sidecar tests and all 10 download tests | Their behaviour is unchanged. |

No test container was left behind.

**Gates on the touched packages:**
- `go vet`, also with the `docker_integration`, `arcadedb_integration` and `db_integration` tags;
- `go test -race`;
- golangci-lint: 0 issues.

## The delegated Docker coverage floor

`scripts/docker_coverage_gate.sh` ran at HEAD `db5336994`, set up as on the morning run:
- the edge images were tagged as the `:latest` names CI builds, and the tags were removed after;
- `scripts/sandbox_image_contract_test.sh` passed first.

The gate exited 0 in 259 s, with 0 skipped tests and no container left.

| Scope | Statements covered | Floor |
|---|---|---|
| docker_owned_internal, the gate's aggregate | 4478/5035 = 88.94% | 85% |
| `internal/sandbox/usersandbox`, package-local | **910/1058 = 86.01%** | 85% |
| `internal/agent/tools` | 3568/3977 = 89.72% | (aggregate only) |

The package figures are summed from the gate's own profile, and the two add up to the aggregate.
In the morning the package was at 832/961 = 86.58%.

## What this does not show

- **The lab VM and the appliances.** None of this is deployed: nothing is pushed yet, and the VM
  stopped answering during the session. Acceptance predicates 6 and 8 touch admission and the
  shell volume, so they need the operator's re-run after deployment.
- **The owned-surface `db_integration` gate** (`scripts/coverage_docker.sh`). It needs the local
  stack, which was down, so it was not run. That gate is the only check of the new
  `internal/keylock` entry in `scripts/coverage_package_policy.json` against the package
  inventory. It also measures the `mcptools` and `cmd/aura` changes.
- **Many identities calling at once**, beyond the two-identity lock tests.
- **A browser launched without `--remote-debugging-port`.** It would not count for admission.
- **Mutation testing.** It is forbidden in this workspace by the operator's ruling of
  2026-10-11.
