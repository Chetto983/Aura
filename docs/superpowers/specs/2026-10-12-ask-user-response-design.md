# ask_user answers require visible closure

## Measured failure and operator decision

On 2026-10-12, authenticated lab replays returned `outcome:approved` after a scheduled
task approval. The task activated, but the agent did not continue. The operator supplied
the same silent ending in Cockpit and explicitly requested changing the tool answer so
the agent always explains what it did.

`classifyResolve` made scheduled approvals terminal. `scheduledApprovalAnswer` already
instructed the model to confirm activation, but the model never read that answer.
Ordinary answers contain only the person's text or the decline marker.

## Contract

- Resolved interactive scheduled approvals return Continue after the existing hook.
- Interactive accept/decline tool answers preserve the answer or task outcome, followed
  by an instruction to always give a visible assistant response explaining the outcome
  or next step. Do not silently finish after tools or claim unverified success.
- Retain scheduled-task ACTIVE/cancelled and no-rescheduling instructions.
- Pending FIFO groups stay pending; cancellation and expiry do not resume the agent.
- Worker-owned approvals retain their separate continuation and raw answer contract.
- Invalid empty/whitespace accepts stay invalid; policy and atomic claim gates remain.

## Scope and acceptance

Medium risk, restricted to runner response construction and continuation classification.
Reuse channel Continue handling; no new protocol, dependency, migration or env variable.
Update old tests only where the operator explicitly supersedes terminal/raw-only behavior.

Test every question kind and decision, task outcome, worker ownership, cancellation,
expiry, empty input, remaining pauses and duplicate resolution. Run affected package
unit/race/goleak, real disposable PostgreSQL migrations and scheduled-approval round trips,
vet/build/lint/deadcode, then the daily-news request and approval on the real lab VM.
Require visible final text and exactly one scheduled task; clean up only test resources.

This measures continuation and an instruction, not universal LLM compliance, content
quality, actual Telegram delivery, or browser-click automation.
