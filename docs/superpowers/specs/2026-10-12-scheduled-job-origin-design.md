# Scheduled job conversation origin and execution context

## Measured failure

On 2026-10-12, the operator asked to reproduce and fix the exported noleggio-sociale
conversation on the running lab VM. Replaying its exact stored goal produced six
`task schedule` failures: `agent_job:<run UUID>` was passed to the conversation store
as a UUID. The model treated the recurring goal as a fresh scheduling request instead
of performing the current check. See the dated verification ledger for run IDs.

## Contract

- Technical sessions retain their existing run-isolation and telemetry keys.
- Conversation-bound tools receive the durable source conversation separately.
- Interactive agents default that origin to their session; cron and swarm roots
  explicitly provide their real source conversation.
- Scheduled workers receive a system-role execution instruction after the stable
  shared system prefix and before the unchanged stored goal. The instruction
  survives auto-rejected `ask_user` resumes.
- Cadence is already configured. The worker executes this occurrence and returns
  its final result for scheduler delivery, rather than duplicating the schedule or
  separately sending the same notification. Explicit task-management goals remain
  possible.

## Scope and acceptance

Medium risk: shared agent/tool context, bounded by existing construction seams.
No migration, environment variable, dependency or new notification transport.
Regression tests cover origin propagation in cron and swarm, unchanged interactive
fallback, and execution instructions across an auto-reject. Run affected packages
with race and goleak, then replay the original goal on the real authenticated VM.
The live run must finish without task-scheduling errors or duplicate jobs. Redirect
the test's final notification to its test conversation, and cancel created jobs.

This does not assert the legal accuracy of the generated report, compare prior weeks,
or validate actual Telegram delivery. No production account settings are changed.
