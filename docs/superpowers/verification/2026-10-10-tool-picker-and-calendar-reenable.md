# Tool policy picker and calendar re-enable on the lab VM

Date: 2026-10-10. Lab VM `192.168.101.158`, operator identity.

- **Before:** `4403b8989`.
- **After:** `facb60b1b` (fixes `caf0ba3d3` and `facb60b1b`), started 18:25:55 UTC.

Login used the operator credentials from `.env.google` through
`scripts/musr_live_run_authula_helpers.sh`. No credential value was printed.

## Tool policy picker

The check drove the cockpit's Identities & access pane in Chromium 1243 (headless) over the
VM's HTTPS origin. It opened the Tool picker, read the computed style of the `current_time`
row, and clicked it. It saved no policy.

| Image | `pointer-events` | `opacity` | `data-disabled` | `aria-disabled` | Click |
|---|---|---|---|---|---|
| `4403b8989` | `none` | `0.5` | `"false"` | `"false"` | timed out after 5 s |
| `facb60b1b` | `auto` | `1` | `"false"` | `"false"` | the trigger read `current_time` |

The run on the old image shows that the check can fail. `GET /api/approvals/policies/tools`
answered 200 on both images, and its list included the bridged MCP tools (`browser__*`).

## Calendar re-enable

At 17:55 calendar's grant had been expired since 17:06:32, the board read "dial failed",
and the sidecar was healthy. After the update the operator clicked Disable, then Enable,
once.

| Time (UTC) | Event |
|---|---|
| 18:28:32-35 | `token refresh failed` for calendar: the expired grant, read before the enable |
| 18:28:36 | `aura.mcp_server` row: `enabled=true`, `profiles={default}` |
| 18:28:36.49 | `mcp oauth: self-issued a first-party grant` for calendar, expiring 18:43:36 |
| 18:28:36.68 | `mcp live mounted` calendar |
| 18:29:40 | `aura mcp doctor calendar` answered `ok: calendar started; 1 tool` |

The board's probe answered `ok (1 tools)`. One tool is calendar's shape: `calendar__calendar`
is a single multiplexed tool. `GET /api/connect/pim/accounts` listed one Google account
with every permission effective.

## What this does not prove

- No agent turn called calendar after the repair. No paid run was made.
- The keeper's renewal of the new grant before 18:43:36 was not observed.
- No run toggled whatsapp or browser, though they are default-on the same way.
- The picker check ran desktop Chromium only, on one row.
