# Browser sidecar always on: lab VM, 2026-10-12

Lab VM 192.168.101.158, operator identity `448ddbe1-…`, profile `single_user_hardened`, isolation
on. GitHub is blocked (the account was flagged on 2026-10-11), so this was a manual update with no
CI behind it.

## Before

- From 07:38 to 07:39 UTC, conversation `01a12ef9-7a25-…` made four browser calls. All four ended
  in `mcp transport error … browser isolation: browser migration verification mismatch; legacy
  source retained`, after 27.4, 34.8, 42.2 and 18.6 s (`aura.tool_invocations` end rows). The three
  parallel calls queued on the lifecycle lock, and each one retried the migration.
- `aura-browser-448ddbe1-…` had been `Exited (143)` since 2026-10-11 16:13 UTC, and nothing ever
  started it again. The board's probe of `browser` read "dial failed".
- Cause: `Resolve` ran the legacy profile migration before every box start, for both box kinds.
  The copy back from a browser volume that already held its own profile never equalled the
  shell's profile, so `Resolve` failed every time.

## Change

- `150cf3f65` removes the legacy migration. Starting a box touches only that box's own objects
  (`TestResolveBrowserNeverTouchesTheShellBox`, against an in-process Docker API).
- `6a3290e2f` keeps the browser box running:
  - the reaper tracks only shell boxes;
  - `AURA_SANDBOX_BROWSER_IDLE_TTL_SEC` is gone, and so are the browser's hold and touch;
  - a strict, isolated `aura serve` starts every existing browser box at boot
    (`StartBrowserBoxes`).

  Chrome still closes after its own 180 s.
- The decision is in `docs/superpowers/specs/2026-10-11-browser-sidecar-design.md`, decision 5 and
  acceptance items 5 and 8, as dated amendments.

## Update, 08:08 UTC

- Image `aura:lab-browser-keepalive-20261012`:
  - a child of the running `aura:lab-scheduler-20261012-02`, with `/usr/local/bin/aura` replaced;
  - the binary is `6a3290e2f`, sha256 `fdb21042ae0d31a1357c40308a167b9ca0093436bc43a8cf94b9e961292d1aee`;
  - it contains the other session's scheduler and ask_user fixes, which are on master.
- The SPA was rebuilt from a clean export of HEAD, because the committed `internal/webui/dist`
  predates `1c1427468` (file highlighting).
- Unchanged: sandbox `aura-sandbox:lab-browser-sidecar-20261011-03`. The updater timer stays
  inactive.
- Overlay: `/opt/aura/lab-browser-keepalive-20261012/compose.lab.yaml`.

## Measured after

- Boot log:
  - the derived admission limit is 3;
  - `sandbox: recreating a stale box, volumes kept … reason="an older browser configuration"`;
  - `aura serve: browser boxes started count=1`.

  This happened within one second of start. The browser box (sandbox `-03`) and its egress sidecar
  came up running.
- MCP handshake inside the box (`docker exec -i … agent-browser mcp`, the command the bridge
  runs): `initialize` answered `agent-browser 0.38.2`, protocol `2025-06-18`, and `tools/list`
  returned 29 tools, in about 40 ms.
- Operator, in the cockpit: "il browser adesso funziona".

## Afterwards, by another session

At 08:11 UTC, `/home/aura/vm-image-cleanup-apply-20261012.py` removed 11 obsolete lab images. It
first checked that `aura` ran this update's image. The removed images include
`aura:lab-scheduler-20261012-02`, so no lab image is left to roll back to. A rollback now means
rebuilding a child image.

## What this does not establish

- The changed `docker_integration` tests compile but were not run:
  - `TestBrowserSidecarKeepsRunningWhenTheShellIdles`;
  - `TestBrowserSidecarInventorySeesAStoppedBox`;
  - `TestBrowserSidecarRetirementPurgesCredentialsNotWork`;
  - `TestBrowserRelayDrivesTheBoxBrowser`.
- The boot start was seen on an `aura` recreate. It was not seen after a VM reboot or a Docker
  daemon restart.
- The shell box `aura-box-448ddbe1-…` still has the container from the old `edge` sandbox image.
  It is stopped, and it is recreated at its next shell call. Its volume still holds the legacy
  `.agent-browser-home`, which the shell can read. What to do with it is the operator's decision.
- Admission: boxes no longer stop, so at most 3 identities on this host can hold a browser box.
  A fourth is refused until one is deprovisioned. With one identity this was not exercised.
- No CI ran. The local pre-commit gates (gofmt, vet, golangci-lint, file size) and the package
  tests under `-race` were the only gate.
