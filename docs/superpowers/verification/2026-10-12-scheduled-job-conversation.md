# Noleggio-sociale conversation: real VM reproduction and repair

The operator requested reproduction of the exported conversation, a fix, and manual
startup of the lab stack. All runs used the operator's existing Cockpit account via
the repository Authula helpers and `.env.google`; no credential is recorded here.

## Before

The manually started cached edge binary reported commit
`64ec618203eaff6675d2371c6c04221903c4628c`, image ID
`sha256:7be277efd592053e1589368bea80d374e011c51c2102d77de75d8e078c7ece6b`.
Compose CPU overrides were required; the unavailable CUDA image was not pulled.

The original user scheduling request created a real approved recurring agent_job,
cron `0 12 * * 0`, timezone `Europe/Rome`, notify `telegram`. Its stored goal was then
replaced with the exact goal from task call #33 of the supplied Markdown export.
Only the final notification route was redirected to stdout, so no Telegram message
would be sent during the test. The first replay retained the newly created row's
default step budget (0).

- Conversation: `01a12ee6-2e0d-78ed-a299-3e439b40f3a4`.
- Task: `01a12ee6-45c1-7a4a-9575-ecb4f528e5e2`.
- Run: `01a12ee6-c00a-7257-a11c-de02d003fc93`.
- Invocation: `01a12ee6-c010-7c54-a0a2-08e8fb58d1ee`.
- UTC 06:57:51–06:58:11: six task calls failed with
  `task schedule: resolve origin identity: get conversation: invalid id "agent_job:01a12ee6-c00a-7257-a11c-de02d003fc93": invalid UUID length: 46`.

The run nevertheless completed with a summary promising a recurring check at 09:00,
claiming it had saved the commitment in memory, and explaining that scheduling was
temporarily broken. It did not perform the requested official-site check. The tool
ledger shows repeated task calls and two memory-upsert attempts.

An earlier replay with the newly generated, non-recurring goal completed research;
that was not counted as reproduction of the exported bug. Two initial provider
`server_is_overloaded` failures were likewise not counted.

## Repair and checks

The agent now passes its durable source conversation separately from its technical
session to conversation-bound tools. Cron and swarm roots supply the real UUID;
interactive construction retains its session default. Scheduled history adds an
execution instruction after the stable system prefix, before the unchanged goal:
execute the current occurrence, do not recreate its cadence, and return the result
for scheduler notification. Auto-reject continuation retains that context.

- Regression RED: `TestAgentJobExecutionContextSurvivesAutoReject` failed because
  the supposed execution instruction was actually the bare user goal.
- WSL `go test` and `go test -race` passed for `internal/agent`,
  `internal/agent/tools`, `internal/cron/handlers`, and `internal/swarm`.
  Package goleak gates ran as part of these tests.
- Cron task-origin and swarm-origin regression tests passed.
- Full `go vet ./...` and `go build ./...` passed on the isolated native WSL
  source snapshot. The initial full vet over the mounted working tree was stopped;
  it is not reported as a passing check.
- Scoped golangci-lint on those four packages: `0 issues`.
- Full native-snapshot `scripts/deadcode_gate.sh .../deadcode -test ./...`: no findings.
- Fresh vet and the execution-context regression passed after copying the new test
  into the isolated snapshot. `git diff --check` passed.
- Full coverage, mutation testing, independent review and remote CI are not claimed.

## Manual deployment

The candidate is a binary-only child of `aura:lab-browser-sidecar-20261011-04`,
preserving yesterday's browser work. Its SPA was freshly built in an isolated WSL
tree; the repository's committed dist and unrelated dirty files were not changed.

- Image: `aura:lab-scheduler-20261012`.
- Image ID, checked against the running container:
  `sha256:94c95776e861e2b331e9d4a507a8f3cb18038f13b752270d1d65125886da6cc7`.
- Binary SHA-256, checked locally, after transfer and inside the running container:
  `d30fdb98d191b3906877da444b0deb8d478bc5c27c826443a88bd02e8f8eec1f`.
- Binary stamp: `0bcb0d88e-scheduler-fix` (uncommitted candidate at build time).
- Overlay: `/opt/aura/lab-scheduler-20261012/compose.lab.yaml` after base and CPU compose.
- Sandbox remains `aura-sandbox:lab-browser-sidecar-20261011-03`.
- Existing database migration version: 141, not dirty; source maximum is also 141.
- Aura healthy. The updater timer was active and was stopped, not disabled, to
  prevent edge from replacing the manual candidate. The previous lab image remains
  available for rollback. No registry image was published.

## After: exact-goal replay

- Conversation: `01a12ef1-c37c-760f-8ef6-ac32ca1b2586`.
- Task: `01a12ef1-ea81-7011-8522-fd669a27a08d`.
- Run: `01a12ef2-2e67-7ca9-8fb8-54e0e9d25374`.
- Invocation: `01a12ef2-2e74-7e51-8b16-031c90da4bcd`.
- Exact exported goal, cron, timezone and step budget 12. The Cockpit edit API has
  no budget field, so only this test row was updated, guarded by its task UUID and
  source conversation UUID. Notification alone remained redirected to stdout.
- UTC 07:09:45.960–07:10:08.919: completed in 23 seconds, empty LastError, with a
  research report rather than a new scheduling promise.
- Invocation ledger: two `tool_search`, two `web_search`, one memory recall;
  **no task call, no duplicate task, no separately sent notification**.

The report's legal accuracy, its assertion about change since the prior week,
actual official-page fetching and Telegram delivery were not validated. A successful
search/report run is not evidence for those separate claims.

## Cleanup and reproducibility

All created test tasks were cancelled and conversations archived. SQL read-back
confirmed both exact replay task rows are cancelled and that no duplicates belong
to either test conversation. Audit rows were retained.

The failed baseline run supported one memory fact. `memory_forget` was first run
with dry_run and the exact source invocation UUID; only that source was detached.
A separate subsequent read-only dry_run returned facts=0, entities=0. Facts supported
by other sources were not targeted. The archived transcript remains the recovery
record for the discarded test commitment.

Raw, local-only evidence and driver: `.planning/tmp/scheduler-conversation-*`,
`scheduler-conversation-repro.{sh,py}`. These are not committed and contain no login
credentials. The driver sources the existing repository authentication helpers.
