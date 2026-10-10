# MCP installer and default-on lifecycle on the lab VM

Date: 2026-10-10. Lab VM `192.168.101.158`, operator identity, Aura image revision
`2e16bdb8b6e9ce07a432a16e5d0e8486789be663`. The updater applied the edge image at
16:51 UTC. Authentication used the operator credentials from `.env.google`; no credential
value was printed or attached.

The Playwright driver used the real HTTPS cockpit and real governance API. It did not mock a
route. One Chromium test passed in 3.2 seconds (4.3 seconds including runner setup).

## Measured

- The MCP board selected the implicit default-on `calendar` server.
- Its detail exposed `Disable server` and no exact `Remove server` action.
- Clicking Disable changed the control to `Enable server`.
- After a full page reload and reopening Governance, `calendar` still exposed Enable: the
  explicit `enabled=false` override had persisted through the real backend.
- Clicking Enable restored the original state and the detail exposed Disable again.
- The Add MCP server panel exposed exactly `Custom (stdio)` and `Remote (HTTP)`.
- Custom stdio was selected by default, the Command field was visible, and there was no exact
  Recipe mode button. The `recipe:*` origin badges in the server list remained visible.

The final read-back left `calendar` enabled. No MCP server was created or removed, and the
temporary live-only Playwright spec was removed after the run.

## What this run does not prove

- Removal of an operator-installed custom or remote server was not exercised on the live VM.
  Unit and mocked browser tests cover the removable-row path and the visible rejection path.
- Recipe installation through the CLI/API was not exercised; this change intentionally removes
  recipes only from the cockpit installer.
- The run used desktop Chromium only; the permanent mocked governance spec remains in the normal
  desktop and mobile Chrome project matrix.
