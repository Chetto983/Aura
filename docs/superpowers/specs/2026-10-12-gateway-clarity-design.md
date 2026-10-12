# Gateway clarity: what an approval names, what a refusal looks like, how the daemon stops

Agreed with the operator on 2026-10-12, at `6a47560b8`. It closes step 2 of the consolidation's
order of landing (`2026-10-09-consolidation-best-of-design.md`): the six gateway and cockpit
findings that the wave-1 runs left to the operator. Wave 2 triage and briefing will run through
the same gateway. The operator made three decisions; the other three findings are defects.

## The findings, read in the tree on 2026-10-12

1. **The delete prompt names no card.** `gatewayApprovalQuestion`
   (`internal/gateway/approve.go:195`) renders `Approve %s (risk=%s)?` plus the sorted argument
   keys. It never renders values, on purpose, because they can carry secrets. The operator
   approves `board` `delete` without knowing which card (wave-1 ledgers of 2026-10-09 and
   2026-10-10, finding 8).
2. **A restart costs a second prompt.** The board's seen-ids set (`internal/agent/tools/board.go:24`)
   lives in process memory, keyed by conversation. `requireSeen` runs inside `Execute`, which
   comes after `Decide` (`internal/agent/llm_agent_retry.go:125`). After a restart a legitimate
   id is refused only after the operator approved it, and a second prompt follows.
3. **Policies are inert under `dev` and `local_trusted`, silently.** `Decide` returns early
   when `!profile.Strict()` (`internal/gateway/decide.go:52`). Standing grants are read after
   that line, so they are inert too. No API exposes the profile, and the cockpit's policy and
   grant panels look active.
4. **A refused call looks completed, and so does every failed call while it streams.** A
   `deny` becomes `*gateway.ErrDenied`, so `ToolInvocation.Status = "error"`
   (`internal/agent/llm_agent_events.go:127`). The translator emits only `TOOL_CALL_RESULT`
   (`internal/agui/translator.go:468`), which has no error field, and the reducer
   (`web/src/chat/sseAdapter.ts:198`) sets only `result`. Every failed tool is green until a
   reload. After a reload only the crash-recovery placeholder is red
   (`internal/agui/server_display.go:92`).
5. **The approval request reaches the model HTML-escaped.** `gatewayApprovalRequiredResult`
   (`approve.go:164`) returns a result with no provenance. The withheld tool's name is not on the
   trusted list, so `renderToolResultForPrompt` wraps it in `<tool_output>` and HTML-escapes it
   (`internal/agent/trust.go:71`). The model must copy `question`, `options` and
   `resume_context` verbatim from text full of `&#34;`. `shellApprovalRequiredResult`
   (`internal/agent/tools/shell_approval.go:180`) has the same shape and the same problem.
   `cmd/aura/gateway_approval_roundtrip_test.go:186` unescapes it to pass.
6. **`aura serve` outlives SIGTERM.** The `http.Server` has no `BaseContext` and no
   `RegisterOnShutdown`, so streaming handlers see only `r.Context()`. `Shutdown` waits for them
   for its 10 s, then logs, with no `Close` (`cmd/aura/serve_drain.go:120`). The setup server
   waits another 10 s after that. The streams that do not end on shutdown are the browser live
   view, both swarm event streams, the non-detached `/agent/run`, the branch rerun and the setup
   events. Detached runs already end through `runRegistry.Close`.

## Decisions

- **A tool may describe the target of a withheld call** (operator, 2026-10-12). This is a new
  optional interface beside `SessionEvictor` (`internal/agent/tools/evict.go`):
  `ApprovalDescriber.DescribeForApproval(ctx, args) (string, error)`. `execTool` calls it before
  `Decide`.
  - An error refuses the call as not executed, before any prompt.
  - A non-empty text becomes one line of the question, `target: <text>`, rendered by the gateway
    after the `args:` line.
  - The board answers for `update`, `move` and `delete`: the card's label and its column name,
    joined by ` · `, read with `Store.Card`. It answers `""` for the other actions.
  - Every other tool is unchanged: argument names only.
- **The board checks seen ids before the prompt** (operator, 2026-10-12). Its
  `DescribeForApproval` applies `requireSeen` first. After a restart, the unseen id is refused
  with the existing "run board list or board search first", and no approval is spent. The seen
  set stays in memory; `Execute` keeps its own check.
- **The cockpit says when approvals do nothing** (operator, 2026-10-12). `GET /api/me` gains
  `approvals_enforced`, the deployment's `profile.Strict()`. The standing-approvals panel, which
  holds both grants and policies, shows a notice in English and Italian when it is false.
  Behaviour under `dev` and `local_trusted` does not change.
- **One sanitizer for operator-visible text the model copies.** `target` text and argument keys
  are cut to one line: control characters, `<`, `>`, `&`, `"` and backticks are removed, and the
  result is at most 120 runes. Card labels come from operators and, in wave 2, from mail, so this
  is what makes the next decision safe.
- **Approval requests are Aura's own text.** `gatewayApprovalRequiredResult` and
  `shellApprovalRequiredResult` carry `Provenance{Trust: TrustTrusted}`. Their content is the
  tool name from the registry, the tier, a fingerprint, server-generated options, and the
  sanitized question. The envelope keeps escaping everything else.
- **A refusal is its own state, and a failure is red as it happens.** `ToolInvocation` gains
  `Refused`, set when the error is a `*gateway.ErrDenied`. After `TOOL_CALL_RESULT` the
  translator emits one CUSTOM `aura.tool_outcome` event `{toolCallId, outcome}` with
  `error` or `refused`. The reducer sets the tool part's `isError` and `refused`. A reload reads
  the same outcome from the call's `end` row in `aura.tool_invocations`. `status = 'error'` is a
  failure. `meta.gateway_verdict = 'deny'` is a refusal: `recordDenyFact`
  (`internal/gateway/approve.go:246`) writes that on every refusal. The cockpit gains a `refused` status
  ("Refused" / "Negata", neutral tone, not the danger red). It also applies to a group's dot.
- **Streams end when the server stops.** The agui server takes a shutdown signal that
  `httpSrv.RegisterOnShutdown` closes. Every long-lived stream handler named in finding 6, and
  the setup server's, ends on it. A `Shutdown` that still times out is followed by `Close`, and
  the timeout is logged as an error.

## The approval question, before and after

```
Approve board (risk=destructive)?
This mutating action is WITHHELD until you accept.
args: action, id
target: Fattura fornitore · Fatto
```

The `target:` line is present only when the tool described the call. The challenge stores the
question and the resume compares it byte for byte (`internal/gateway/approvals.go:173`). The text
is computed once per withheld call, in `routeApprove`, exactly like today's question.
`approvaltext.deriveGateway` (`internal/approvaltext/presentation.go:103`) learns an optional
`target` parameter, bounded at 512 bytes, which 120 runes always fit. A question without the
line parses as
before. The cockpit (`web/src/approvals/approvalQuestion.ts`) and Telegram (`RenderItalian`)
render it.

## Acceptance on the lab VM

One dated file in `docs/superpowers/verification/`, on the operator's identity, profile
`single_user_hardened`. Each item is checked by a script or a read-back, not by a reply:

1. A `board` `delete` prompt shows `target:` with the card's label and column in the cockpit's
   approval card and in the Telegram approval message. Accepting deletes that card.
2. After `docker restart aura`, a delete of a card id listed before the restart is refused with
   "run board list or board search first". `aura.tool_invocations` and `/api/approvals` show no
   approval. After a search, exactly one prompt.
3. The stored tool row of a withheld call (`aura.conversation_turns`, role `tool`) contains
   `"question"` and no `&#34;`.
4. With `deny` on `web_search`, the call shows "Negata" live and again after a reload. A call that
   fails for another reason shows red live.
5. `docker stop aura` with a browser live view open: the stop time, measured before and after.
   After the fix it should be well under the 10 s stop timeout.
6. The `approvals_enforced` notice: the VM is strict, so it must not appear there. The `false`
   case is covered by the unit and vitest suites.

## Tests

Go, beside the existing suites:
- the describer and refusal order in `llm_agent_retry_gateway_test.go`;
- the question and `target` in `approve_test.go` and `presentation_test.go`;
- board describe and seen-before-prompt in `board_test.go`;
- the sanitizer, table-driven and fuzzed against `<tool_output>` forgery like `FuzzWrapUntrustedToolOutput`;
- provenance of both approval results;
- the `aura.tool_outcome` emission in `translator_test.go`, and the snapshot outcome;
- `/api/me` in `audit_api_test.go`;
- a shutdown test with an idle stream that must finish well inside the timeout.

Vitest:
- the reducer and `toolStatus` for `refused`, and `ToolActivityCard` and `ToolGroup`;
- the notice in a `StandingApprovalsPanel` test;
- `approvalQuestion` for the target.

Coverage and mutation follow CLAUDE.md. Without CI (the GitHub account is flagged) the local
pre-push gates are the only gate until it is lifted, and the closing report says so.

## What this does not establish

- That a described target is enough for every destructive tool. Only the board describes one
  here, and other tools keep argument names.
- That the seen set survives a restart. It does not: the refusal only moves ahead of the prompt.
- Anything about wave-1 findings outside these six: `granted_by: local`, the empty final text
  after a draft pause, the phantom card, substring search, and browsers the model never closes
  all stay open.
