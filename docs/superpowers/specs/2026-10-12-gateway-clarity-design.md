# Gateway clarity: what an approval names, what a refusal looks like, how the daemon stops

Agreed with the operator on 2026-10-12, at `6a47560b8`. It closes step 2 of the consolidation's
order of landing (`2026-10-09-consolidation-best-of-design.md`): the six gateway and cockpit
findings that the wave-1 runs left to the operator. Wave 2 triage and briefing will run through
the same gateway. The operator made three decisions; the other three findings are defects. After
the plan's three-way validation, the same day, the operator decided three more questions: where
the target lives, what shutdown does to a running turn, and whether a rejection is a refusal.

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

- **A tool may describe the target of a withheld call, and the model never sees it** (operator,
  2026-10-12). `ApprovalDescriber.DescribeForApproval(ctx, args) (string, error)` is a new
  optional interface beside `SessionEvictor` (`internal/agent/tools/evict.go`).
  - **When it runs.** The gateway calls it in `routeApprove`, right before the challenge
    (`internal/gateway/approve.go:146`). By then the profile, the policy, the no-responder deny,
    the standing grants and the operator's one-shot accept have all decided. `execTool` hands
    the gateway the call's describer through the context, as it does the reservation key. A call
    that never reaches a prompt is never described and pays nothing.
  - **On error.** The call is refused as not executed, before any prompt. Its error text is
    Aura's own and never quotes third-party content.
  - **Where the text goes.** The challenge keeps the text and the withheld call's
    `tool_call_id`. When the pause is saved (`internal/runner/runner_persist_pause.go:45`), the
    runner looks up the challenge behind the relayed `resume_context` (conversation, tool,
    `args_sha256`) through a seam the composition root wires, as it wires the resume hook. It
    writes `target` and `tool_call_id` into that context as root fields, replacing whatever the
    model copied there. A miss writes neither, and the model's copies are dropped all the same.
    `deriveGateway` reads `target` from those fields,
    so `approvaltext.FromContext` still accepts a presentation only when it equals a fresh
    derivation. The cockpit (`web/src/approvals/approvalQuestion.ts`) and Telegram
    (`RenderItalian`) show it.
  - *Amended 2026-10-13 after the code review.* Every reader of these fields reads exact keys,
    through `approvaltext.RootString`, as Postgres's `->>` reads them: the stamp,
    `deriveGateway`, the cockpit's `withheld_tool_call_id` and the declined-pause query. The
    cockpit's reader was a struct decode, which folds case, so a context the model wrote as
    `"TYPE"` or `"Tool_Call_ID"` slipped past the stamp and named a call of the model's choosing.
    The stamp also drops the model's `target` and `tool_call_id` under any spelling.
  - **The question does not change.** The model copies today's question, built from the tool
    name, the tier and the argument names, so no third-party text reaches the model as Aura's
    words, and the byte-for-byte resume never depends on the model copying operator Unicode.
  - **The board** answers for `update`, `move` and `delete`: the card's label and its column,
    joined by ` · `; for `move`, `label · from → to`. It answers `""` for the other actions.
    Every other tool is unchanged.
  - *Amended 2026-10-12 after the plan's validation (operator).* The first text put the target
    in the question, as a `target:` line. The adversarial review showed what the sanitizer cannot
    remove: a plain-language instruction in a card label, today copied from a web page and in
    wave 2 from mail. That instruction would reach the model as trusted text. And a model that
    straightens `’` to `'` would fail the exact comparison at `approvals.go:173`, which brings
    back the second prompt.
- **The board checks seen ids before the prompt** (operator, 2026-10-12). Its
  `DescribeForApproval` applies `requireSeen` first. After a restart, the unseen id is refused
  with the existing "run board list or board search first", and no approval is spent. The seen
  set stays in memory; `Execute` keeps its own check.
  - *Amended 2026-10-12 after validation.* The check runs where the description runs, in
    `routeApprove`, so a `deny` policy, a grant or the operator's accept decides first. Before
    `Decide` it would have preempted them, and it would have cost two database calls on every
    board update, under every profile.
- **The cockpit says when policies do nothing** (operator, 2026-10-12). `GET /api/me` gains
  `policies_enforced`, the deployment's `profile.Strict()`. The standing-approvals panel, which
  holds both grants and policies, shows a notice in English and Italian when it is false: "Tool
  policies and standing approvals below are saved but not applied on this profile (dev or
  local_trusted)." Behaviour under `dev` and `local_trusted` does not change.
  - *Amended 2026-10-12 after validation.* The field was `approvals_enforced`, and the notice
    said Aura asks for no approvals. Both overclaimed: under `dev`, destructive shell commands
    (`cmd/aura/main.go:191`) and outbound message review (`decide.go:49`) still ask.
- **One sanitizer for operator-visible text.** Argument keys, the scope labels built from the
  model's `action` (`scope.go:52`) and the described target are cut to one line: control and
  format characters, `<`, `>`, `&`, `"` and backticks are removed, and the result is at most 120
  runes.
  - *Amended 2026-10-12 after validation.* This stops markup and line forgery, nothing more. It
    is not an injection defence. Keeping the target out of the model's text is what keeps a label
    from speaking with Aura's voice.
- **Approval requests are Aura's own text.** `gatewayApprovalRequiredResult` and
  `shellApprovalRequiredResult` carry `Provenance{Trust: TrustTrusted}`. Their content is:
  - the tool name from the registry;
  - the tier and a fingerprint;
  - server-generated options, with sanitized labels;
  - the question.

  The question carries no third-party text. The shell request echoes the model's own `command`
  and `cwd`, which `json.Marshal` keeps free of envelope markup. The envelope keeps escaping
  everything else.
- **A refusal is its own state, and a failure is red as it happens.** `ToolInvocation` gains
  `Refused`, set when the gateway refused by decision: a `deny` policy (`decide.go:66`) or no
  interactive approver (`approve.go:84`). `Verdict` and `ErrDenied` carry a `Refusal` flag set at
  those two sites, and only there. After `TOOL_CALL_RESULT` the
  translator emits one CUSTOM `aura.tool_outcome` event `{tool_call_id, outcome}` with
  `error` or `refused`. The reducer sets the tool part's `isError` and `refused`. A reload reads
  the same outcome from the call's `end` row in `aura.tool_invocations`. `status = 'error'` is a
  failure. `meta.gateway_verdict = 'deny'` is a refusal: `recordDenyFact`
  (`internal/gateway/approve.go:246`) writes it at the same two sites. The cockpit gains a `refused` status
  ("Refused" / "Negata", neutral tone, not the danger red). It also applies to a group's dot.
  - *Amended 2026-10-12, while planning, read at `0bcb0d88e`.* The first text said every
    `*gateway.ErrDenied` is a refusal. The gateway has ten `Deny` returns, and `execTool` turns
    each into an `ErrDenied`. Only the two above write the deny fact. The other eight are the
    gateway's own bookkeeping and guards: `reserve.go:59, 62, 66, 76, 78, 260, 275` and
    `message_draft.go:36`. Under the first rule a failed reservation would stream as "Refused"
    and reload as "Error", so a fault would pass for a decision. Those eight are failures.
  - *Amended 2026-10-12.* The key was `toolCallId`. Aura's own CUSTOM payloads use snake_case
    (`aura.display` `tool_call_id`, `internal/agent/display/payload.go:42`); camelCase belongs
    to the AG-UI events.
  - *Amended 2026-10-12 after validation.* The runner stamps `meta.gateway_verdict = 'deny'` on
    its own `end` row when `Refused` is set. Both writers insert with `ON CONFLICT DO NOTHING`
    (`internal/db/queries/tool_invocations.sql:15`), so whichever row lands first is the one a
    reload reads, and both must say the same thing.
- **The operator's rejection is a refusal too** (operator, 2026-10-12). When the operator
  declines a gateway approval, the withheld call shows "Negata", at once in the cockpit that
  declined it, and after any reload.
  - **Durable carrier: the resolved pause.** A deny fact cannot be it. The withheld call already
    has its `end` row: the approval request came back as a normal result, `status = 'ok'`
    (`internal/agent/llm_agent_events.go:127`). The table takes one `end` per call
    (`tool_invocations_once_per_phase_idx`, migration 0011) and is append-only, so a second row
    would be dropped without a word. The pause row records the answer instead
    (`paused_states.resumed_answer`), and its `resume_context` names the withheld call through
    the `tool_call_id` the runner stamped from the challenge. A reload marks refused every call
    named by a declined `gateway_approval` pause of the conversation.
  - **Live: the cockpit that declined.** After a successful decline, the cockpit marks the tool
    part the approval names as refused. A decline from Telegram reaches an open cockpit at its
    next snapshot.
  - *Amended 2026-10-12, while amending.* The option the operator chose said the rejection
    "writes the deny fact too". Read after the choice, that row cannot land, for the reason
    above. The behaviour the operator chose is unchanged; only its carrier is.
  - *Amended 2026-10-12, while planning v2.* A reload decorates a call only when its id is
    unique in the conversation. This holds for an outcome from the ledger and for one from a
    declined pause. Ids do repeat: the blank-id fallback, `deterministicBlankCallID`, hashes only
    name, arguments and position (`internal/agent/llm_agent_call_dedup.go:93-96`), so the same call
    in two turns shares one. It is the rule `matchingEndFacts` already applies to MCP displays. A call left
    undecorated is better than the wrong call painted "Negata" or red.
  - *Amended 2026-10-13 after the code review.* The live mark follows the same rule: a decline
    marks the withheld call only when exactly one tool part of the thread carries its id. Before,
    it marked every carrier, so live and reload disagreed.
- **A turn is cancelled at shutdown, and its stream waits for it** (operator, 2026-10-12). The
  agui server takes a shutdown signal that `httpSrv.RegisterOnShutdown` closes.
  - The observer streams end on it at once: the browser live view, both swarm event streams and
    the setup events.
  - A turn's stream (the non-detached `/agent/run` and the branch rerun) runs the turn on a
    context derived from the same signal. Shutdown cancels the turn, and the stream keeps
    draining until the turn has written its records and sent its terminal frame, inside the
    10 s `Shutdown`.
  - A `Shutdown` that still times out is followed by `Close`, and the timeout is logged as an
    error.
  - `compose.yaml` gives the aura service `stop_grace_period: 30s`. Docker's default is 10 s, the
    same as the HTTP drain alone, and the setup server waits after it.
  - *Amended 2026-10-12 after validation.* The first text ended every stream at once, the turn's
    too. The pool closes after the drain, so a turn cut mid-write lost its end rows and left
    operation claims open.
  - *Amended 2026-10-13 after the code review.* The drain closes the signal first, before it
    joins the channel turns (`drainShutdown`). Those run on a context that stays live and can
    outlast the grace; a web turn cancelled only after them was cut at exit with its end rows
    unwritten. `RegisterOnShutdown` still closes it, a no-op by then.
  - *Amended 2026-10-13 after the lab-VM acceptance*
    (`docs/superpowers/verification/2026-10-12-gateway-clarity.md`, item 5). Three gaps showed
    on the VM, each fixed and measured again:
    - The rule above covers the detached runs too, which are the cockpit's default path.
      `RunRegistry.Close` now runs right after the signal (`7b6cdc630`). Before, a Telegram turn
      held a cockpit turn for 12 s.
    - A tool whose turn was cancelled writes its `end` row and its tool turn on a context
      without that cancellation (`fcf52a184`).
    - A detached run delivers its terminal frame, the translator's own included, on the bounded
      context the fallback already used (`05f9a3c6b`). With the cancelled run context, the
      viewer lost it about half the time.

## The approval, before and after

The question the model copies stays as it is today:

```
Approve board (risk=destructive)?
This mutating action is WITHHELD until you accept.
args: action, id
```

The presentation the operator reads gains one parameter:

```
{"key": "approval.gateway.mutation",
 "params": {"tool": "board", "risk": "destructive", "args": "action, id",
            "target": "Fattura fornitore · Fatto"}}
```

`target` is present only when the tool described the call. `approvaltext.deriveGateway`
(`internal/approvaltext/presentation.go:103`) learns it as an optional parameter, bounded at
512 bytes, which 120 runes always fit. A presentation without it validates as before.

*Amended 2026-10-12 after validation.* The first text showed a `target:` line in the question.
See the first decision for why it moved.

## Acceptance on the lab VM

One dated file in `docs/superpowers/verification/`, on the operator's identity, profile
`single_user_hardened`. Each item is checked by a script or a read-back, not by a reply:

1. A `board` `delete` approval shows the card's label and column in the cockpit's approval card
   and in the Telegram approval message, and the question the model relayed carries neither.
   Accepting deletes that card. *Amended 2026-10-12:* the first text looked for a `target:` line
   in the prompt.
2. After `docker restart aura`, a delete of a card id listed before the restart is refused with
   "run board list or board search first". `aura.tool_invocations` and `/api/approvals` show no
   approval. After a search, exactly one prompt.
3. In the live turn, the `ask_user` question the model relays for a withheld call equals the
   challenge's question byte for byte, and the accept resumes on the first try. *Amended
   2026-10-12, while planning:* the first text checked the stored row in
   `aura.conversation_turns`. That row stores the raw preview (`internal/runner/runner_persist.go:189`).
   It was never escaped, so the check already passed at `0bcb0d88e` and proved nothing. The
   envelope reached the model only through the live turn's in-memory history
   (`internal/agent/llm_agent_dispatch.go:143`).
4. With `deny` on `web_search`, the call shows "Negata" live and again after a reload. Declining a
   `board` `delete` approval in the cockpit shows the withheld call as "Negata" at once and after
   a reload. A call that fails for another reason shows red live.
5. `docker stop aura` with a browser live view open and a turn streaming, measured before and
   after. After the fix the live view closes at once, the turn's stream ends with its terminal
   frame, every `start` row of that turn has its `end`, and the stop finishes well inside the
   30 s grace period. *Amended 2026-10-13:* one run also has a Telegram turn in flight, and the
   web turn's `end` rows still land.
6. The `policies_enforced` notice: the VM is strict, so it must not appear there. The `false`
   case is covered by the unit and vitest suites.

## Tests

Go, beside the existing suites:
- the describer's place in `approve_test.go`: after a `deny` policy, a grant and the operator's
  accept, never for a call that does not reach the prompt, and never under a profile that does
  not prompt;
- the question unchanged and the target on the challenge in `approve_test.go`, and the target's
  round trip through `Enrich`, `FromContext` and `RenderItalian` in `presentation_test.go`;
- the runner's stamp in `runner_persist_pause` tests: a model-supplied `target` or `tool_call_id`
  is replaced from the challenge, and a miss leaves neither;
- board describe and seen-before-prompt in `board_test.go`;
- the sanitizer, table-driven and fuzzed against `<tool_output>` forgery like
  `FuzzWrapUntrustedToolOutput`, and applied to the scope labels;
- provenance of both approval results;
- the `aura.tool_outcome` emission in `translator_test.go`; the snapshot outcome for a deny fact,
  for the runner's own refused `end` row, and for a declined `gateway_approval` pause;
- `/api/me` in `audit_api_test.go`;
- shutdown: an observer stream ends at once; a running turn is cancelled, writes its `end` rows
  and sends its terminal frame before the drain returns.

Vitest:
- the reducer and `toolStatus` for `refused`, and `ToolActivityCard` and `ToolGroup`;
- a cockpit decline that marks the named tool part refused;
- the notice in a `StandingApprovalsPanel` test;
- `approvalQuestion` for the target.

Coverage and mutation follow CLAUDE.md. Without CI (the GitHub account is flagged) the local
pre-push gates are the only gate until it is lifted, and the closing report says so.

## What this does not establish

- That a described target is enough for every destructive tool. Only the board describes one
  here, and other tools keep argument names.
- That the seen set survives a restart. It does not: the refusal only moves ahead of the prompt.
- That a card label is safe to show the model. It never reaches the model here; the sanitizer
  stops markup and line forgery only.
- That a rejection shows live everywhere. An open cockpit learns of a decline from Telegram only
  at its next snapshot. A pause row deleted by the operator's manual purge
  (`cmd/aura/paused_states.go:96`) takes its "Negata" with it. A decline of an approval relayed
  from a swarm child names the child's call, which is no part of the parent conversation, so
  nothing there turns "Negata".
- That a pause declined before this change names the right call. Its `tool_call_id` is the
  model's copy of the one the gateway handed it, so a reload after the update marks that call
  "Negata" too, behind the same unique-id guard. Read in the code, not measured.
- That a turn always finishes inside the drain. A turn that ignores cancellation is cut by
  `Close`, as today. A channel turn is joined on a live context and cancelled only by the
  backstop once the grace is over, as before.
- That untrusted tool output stays wrapped after its own turn. Read while planning, not
  measured: each turn reloads history with `LoadManagedHistory`
  (`internal/runner/runner_context.go:33`), whose tool rows are raw previews, so the
  `<tool_output>` envelope covers a result only in the turn that produced it. Wave 2 mail makes
  this a decision of its own.
- Anything about wave-1 findings outside these six: `granted_by: local`, the empty final text
  after a draft pause, the phantom card, substring search, and browsers the model never closes
  all stay open.
