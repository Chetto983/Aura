# Wave 1 on the lab VM: tool approval policy and the work board

Date: 2026-10-10. Lab VM `192.168.101.158`, the appliance stack under `/opt/aura`, profile
`single_user_hardened`, `AURA_MUSR_ISOLATION=true`. Image `9574e11c3` until 13:43 UTC, then
`db4e9ec19`, which the updater applied mid-run. Model: `gemma4:31b-cloud` through Ollama, the
deployment-wide route the operator chose for this run. Identity: the operator's own.

The driver was an SSE script against `POST /agent/run` with the operator's session cookies, one
new conversation per run. It answered approval cards through `/api/approvals`. Policies were
set and cleared through `/api/approvals/policies`. The board's failure case ran in Playwright
against the cockpit. Mail stayed read-only, except for one email the operator confirmed
themselves, sent to their own address (policy 2).

## What this run does not prove

- **Two models.** Every "on two models of the release's matrix" criterion ran on one model.
- **The cockpit's approval cards.** The driver resolved them through the API. Only board 3 and
  the operator's own confirmation of the email draft went through the UI.
- **Board acceptance 1 and 5.** These are the board on a phone and a week of real use. Both
  need the operator.
- **Browser takeover acceptance 1-5.** They need the operator in the live view.
- **Real pacing.** Policy 4 asked ten questions about 25 s apart. That burst is what exhausted
  the box's browsers (below), and nothing in this run says an operator works that way.

## Tool approval policy (`docs/superpowers/specs/2026-10-09-tool-approval-policy-design.md`)

| # | Criterion | Result |
|---|---|---|
| 1 | `ask` on `browser__agent_browser_open`: a card with `once` and `session` only, accept runs the open, 3/3 | **2/3.** Runs 2 and 3 (13:38) showed the card with `once` and `session`, and accepting `once` ran the open. In run 1 (13:37) the model called `get_title` alongside the open, on a session my baseline run had left open in the box. It answered from that and never relayed the card. |
| 2 | `deny` on calendar send_email: the model says the tool is disabled, no retry loop | **The deny is bypassed by design.** A confirmed draft passes it, as the operator decided afterwards. See below. The criterion now names a tool that is not drafted, and policy 4 measured one: no retry, and 2 of 10 turns told the operator. |
| 3 | `always` on `shell_exec`, then `ask`: the next call prompts and the grant is gone | **Premise wrong on this profile.** `shell_exec` ran with no prompt at all (13:39), so there was no grant to make. Redone on `board` `delete` (13:40): the card offered `once`, `session` and `always`. Answering `always` stored the grant. `PUT ask` answered `revoked_grant: true`, and the next delete prompted with `once` and `session` only. |
| 4 | The rounds a denied tool costs over ten turns | **One call each.** `web_search` denied, ten questions, ten conversations (13:49-13:54). Every turn called `web_search` exactly once and never retried it. See below for what the model did next. |

**Policy 2, the bypass.** `deny` was set on tool `calendar__calendar`, action `send_email`. The
spec's "calendar send_email" is not a subject: the PIM email tool is the multiplexed
`calendar__calendar`. For trusted outbound tools the agent drafts the message before the gateway
(`withholdOutboundMessage`), so the model never hears about the policy. Once the draft is
confirmed, `Decide` allows the send through `decideReviewedMessage`, which runs before the
policy check (`internal/gateway/decide.go`). On the VM, the operator confirmed the draft of run
3. The `tool_invocations` start row at 13:45:43 carries `gateway_verdict: allow`, `approved:
true` and `gateway_tier: destructive`, and the draft went to `sent` at 13:45:44, while the deny
policy was in force. The email went to the operator's own address. The operator decided
afterwards that this stands: a drafted message leaves only on their own confirmation, so a
confirmed draft passes a `deny` (prd.md §5, 2026-10-10).

**Policy 4, what the model did after the refusal.** It said the tool was disabled in 2 of 10
turns: q2 stopped there, and q7 said so after failing on the browser too. In the other 8 it
answered anyway: through the agent-browser in 7 (3-12 more calls per turn), and through
`web_fetch` alone in 1. A deny blocks one tool, not the capability. The spec's threshold for
hiding a denied tool from `tool_search` ("more than one round per denied call") was not reached
on this model.

## Work board (`docs/superpowers/specs/2026-10-09-work-board-design.md`)

| # | Criterion | Result |
|---|---|---|
| 2 | "add a card to call the supplier tomorrow and move the invoice card to done": two writes, no prompt, 3/3 | **3/3 on one model** (13:46-13:47). Each run added the card and moved the fixture invoice card, with no prompt. Run 2 first tried the column `fatto (done)`. "Tomorrow" was stored as 09:00Z. The board's visible refresh was not measured: API only. |
| 3 | `POST /cards` answers 500: what the widget shows, the temporary card, the server id on success | **Measured (Playwright, 13:50).** The provider shows "The board is not available right now." and reloads. The editor stays open on a phantom "New card" that stays in the widget until the page is reloaded; the server holds nothing. A successful add then reconciles: the edit's `PUT` carried the server's id. |
| 4 | "delete the card about the old server": a card, the delete after accept | **Passed** (13:40, twice, the policy 3 runs). The card reads "Approve board (risk=destructive)? … args: action, id". It names neither the card nor its label. |

**The "Discuss in chat" draft** (`f013b582a`, measured on `db4e9ec19`, 13:48, three runs). The
draft now names the board, the card's id and its column. `board` was the model's second call in
every run, after `tool_search`, and no run reached for Linear. Input tokens were 57k, 82k and
68k, against 104k for the morning's turn with the old draft.

## Found

1. A confirmed message draft passes a `deny` policy (policy 2). Kept by the operator's
   decision: the confirmation is the gate.
2. The policy API accepts a tool that does not exist: `PUT` with `calendar__send_email`
   answered `set: true`.
3. The spec's and the CLI's subject "calendar send_email" does not match the gateway's, which is
   `calendar__calendar` plus action `send_email`.
4. The `always` grant was stored with `granted_by: "local"`, not the operator's identity.
5. `shell_exec` is not gated under `single_user_hardened` on the VM, so policy criterion 3 as
   written cannot run there.
6. A `deny` stops a tool, not what it does: 8 of 10 turns answered through another tool, and
   only 2 told the operator the tool was disabled.
7. Two policy 2 runs ended with an empty final text after the turn paused on the draft.
8. Board approval cards name the action and the id, not the card.
9. After a failed `POST /cards`, the editor stays open on a phantom card until the page is
   reloaded.
10. The board's search for "fattura fornitore" misses "fattura del fornitore": the match is a
    substring.
11. The updater failed a whole run (exit 1) at 13:38. It had pulled `whatsapp-mcp:latest`, but
    the `up` after the pull went back to the registry (`pull_policy: always`), stalled, and
    `set -e` ended the tick. The cockpit's modal showed an alarming error. At 14:29 the box
    image's pull stalled and was skipped. Fixed in `bcd5d8420`: one retry per pull, and
    `--pull missing` on the apply's `up`. Not yet applied on the VM. The idle check also
    reported nobody using Aura while API turns were running.
12. **Five browsers exhausted the box** during policy 4: prd.md §12, "Five browsers in two
    minutes exhaust a box", and the section below.
13. After `agent-browser mcp` panicked, the bridge's redial failed with `materialize clear
    "/skills": rm -rf "/skills" exited 128`, an error that names neither the full box nor
    the browser.
14. The bridge's list of browsers outlives the box's idle shutdown. The cap's first three closes
    were no-ops on browsers already gone, and the model was told it had closed them.
15. Neither gemma run closed a browser session: none of the 7 opens on `16602b330` was
    followed by a close, even with the close line in the result.

## The browsers of policy 4

Eight turns opened a browser, each under a new session name, and none closed it. Four were
still up under the 10-minute idle shutdown when the fifth launched at 13:51:49. The box's
cgroup then counted 136 forks refused at its 1024 pids (`pids.events`), and memory at the 2 GiB
cap 2403 times, with no OOM kill. Chrome died with `pthread_create: Resource temporarily
unavailable`, and `agent-browser mcp` panicked. All seven browser calls from 13:52:16 to
13:54:12 failed. The operator saw it in the cockpit ("è andato in errore il browser").

Read back while the box drained:

| UTC | Chromium | pids | memory |
|---|---|---|---|
| ~13:55 | 5 | 972 | 2066 MiB |
| 13:56:58 | 4 | 743 | 1581 MiB |
| 13:59:17 | 3 (`news-it` closed by the operator through Aura) | 491 | 927 MiB |
| 14:00:17 | 2 (`weather`, idle 10.5 min) | 316 | 733 MiB |
| 14:01:17 | 1 (`finance`, idle) | 149 | 513 MiB |
| 14:02:18 | 0 (`strikes_check`, idle) | 12 | 336 MiB |

Three agent-browser daemons whose browser never launched were still in the box at 14:02.

Fixed in `16602b330`: the bridge keeps at most three browsers per box and closes the least
recently used one, never one the operator holds. `agent_browser_open` carries the close rule in
its description and its result. Idle browsers close after three minutes.

## The same runs on `16602b330`

The updater applied `aura` at 14:29:44, but its pull of the box image timed out at 14:29:27
(GHCR, `timeout awaiting response headers`) and was skipped, so the box kept the old script.
The 14:37 tick pulled the image, found the box on a superseded image, and removed it at
14:38:01. aura recreated it at 14:38:13, with `AGENT_BROWSER_IDLE_TIMEOUT_MS` defaulting to
`180000`. A sampler read the box every 10 s from 14:39:24 to 14:48:34. Model:
`gemma4:31b-cloud`, as in the first runs.

**Policy 4 again** (14:39-14:41, the same deny and the same ten questions). Every turn called
`web_search` once and never retried. This time 6 of 10 told the operator they could not
search, and 2 went to the browser: q7 opened `strikes_search` and `strikes_search_ddg`, q10
`f1_results`. Every open's result ended with `Session "…" is open. Close it with
browser__agent_browser_close when the site is done.`, and the model closed none. The same
model and the same questions gave 2 and 6 refusals told to the operator, and 8 and 2 browser
detours, an hour apart. The spread is the model's, not the policy's.

**The cap** (14:45, one turn): "open these four sites, each in its own browser session, s1 to
s4, and tell me each page's title". Each open answered with its page's title. The bridge
closed four sessions to make room. Three were the policy-4 browsers the box had already shut
down when idle, so those closes were no-ops, but the model still read "Closed browser session
… to make room" for each. The fourth was `s2`, when `s4` opened. The box ended with `s1`,
`s3` and `s4`.

| Box, 14:39-14:48 | Value |
|---|---|
| Chromium browsers at once, most | 3 |
| `pids.peak` | 625 of 1024 |
| forks refused (`pids.events max`) | 0 |
| `memory.peak` | 1204 MiB of 2 GiB |
| memory at its cap (`memory.events max`) | 0 |

Each browser closed 3:00-3:10 after its last call: `s1`, last driven at 14:45:13, was gone at
14:48:13. Across the seven opens the model never called `agent_browser_close`.

**The live view.** At 14:46:39 the operator took `s4` over in the cockpit, released it at
14:46:56, and took it again at 14:49:24 (aura's log, "browser session taken over by the
operator"). `s4` was still up at 14:49:29, 4:11 after the agent's last call. The operator
reported the takeover working ("funziona, l'ho già testato io"). That is their own check, not
a scripted acceptance run.

**The second model.** The operator switched the chat route to `gpt-6-sol` at 14:45:30. One
policy-4 turn ran on it (14:51, q1: one denied `web_search`, then `web_fetch`) before the run
was stopped at the operator's word. One turn is not a measurement: the second model is still
open for every two-model criterion.

## The lab VM's network

Read on 2026-10-10, between 14:45 and 14:50: a VMware guest (`e1000` NIC) behind VMware's NAT,
whose `192.168.101.2` is both gateway and DNS. At that moment it was not slow:

- 0% loss over 40 pings each to the gateway, `1.1.1.1` and GHCR;
- RTT 27 ms to `1.1.1.1` and 35 ms to GHCR;
- path MTU 1500;
- HTTPS 0.3-0.5 s from the host and from inside the box alike;
- a 20 MB download at 22 MB/s.

Since boot (2.4 h), though, the kernel counted 18 SYN retransmissions and 18 TCP timeouts in
71k segments. These are connections whose first packet went unanswered for a second, the shape
of the five GHCR stalls of the day. Not shown: where the stalls happen. The NAT on the Windows
host is the first suspect, and nothing here measures it.

## Cleanup

- Policies: `GET /api/approvals/policies` answered `[]` at 13:54 and at 14:41, after each
  policy-4 run cleared its deny. The stopped GPT run left its deny in place; it was cleared
  by hand at 14:53 and read back as `[]`, with no grants.
- The board fixture card (`2aefa265…`) was deleted at 14:13. The four "Isolamento Tenant"
  cards are the operator's own, added by their 12:54 conversation, and were left alone.
- The two drafts of the first policy 2 runs (13:41:44, 13:43:34) are still `pending` and expire
  on their own. Nothing was sent from them.
- Test conversations stay in the operator's history.
