# ask_user visible response: lab verification

## Baseline and repair

The operator's Cockpit screenshot and the original authenticated scheduled-job
replays ended after approval without assistant prose. Resolve returned
`{outcome:approved, remaining:0}`: activation succeeded, but the channel did not
continue the agent. Scheduled approvals now return Continue; the existing tool
answer retains task-state and no-rescheduling instructions.

At the operator's additional request, `Runner.answerTurn` now adds a visible-response
instruction to interactive accept/decline answers for clarification, choice and
approval: explain the outcome or next step, do not silently end after tools, and do
not claim success unless the action succeeded. Cancel, expiry, invalid blank input
and worker-owned continuation remain unchanged.

## Local checks (WSL, 2026-10-12)

- Scheduled-approval continuation regression observed RED before the classifier fix.
- All six kind/decision combinations observed RED before adding the tool-answer instruction.
- Full unit tests and race/goleak passed for runner, AG-UI and Telegram.
- The expiry regression caught an overly broad initial guard; restricting it to
  human accept/decline preserves the existing expiry marker. All affected tests pass.
- Full runner `db_integration` suite with race passed (52.326 s) against a disposable
  PostgreSQL 18 instance and the real migration chain, including activation,
  rejection, foreign-origin isolation, atomicity and worker resume tests.
- The earlier focused scheduled-approval integration run also executed all three cases.
- Full native WSL source-snapshot vet/build and deadcode passed; runner lint: 0 issues.
- Existing assertions were changed only where the operator superseded the old
  terminal-scheduled/raw-only-answer contract. Worker, cancel and expiry assertions stayed.
- No migration, env, dependency or frontend runtime change.

Full repository coverage, mutation, independent review and remote CI are not claimed.
An instruction is not a universal guarantee of model compliance. Actual Telegram
delivery, news accuracy and browser-click automation are outside this test.

## Live candidate

Binary-only child of `aura:lab-scheduler-20261012`, retaining the prior scheduler
repair, freshly built SPA and browser-sidecar lineage. Sandbox remains
`aura-sandbox:lab-browser-sidecar-20261011-03`.

- Candidate: `aura:lab-scheduler-20261012-02`.
- Local binary SHA-256:
  `129c7419a4e7506a02b524dd5256c692303490f8d6f9fe183abdc09189d9765b`.
- Build stamp: `bf9586a3d-approval-ack-fix` (candidate before source commit).

- Running image matches the built tag:
  `sha256:de4171d2c1c49ff798be23bc86a45729c88a99284b99d76ffa3f9d75e9cc0390`.
- SHA-256 matched after transfer and inside the healthy running container.
- Overlay: `/opt/aura/lab-scheduler-20261012-02/compose.lab.yaml`, after base and CPU.

## Real-agent daily-news approval

Authenticated through the repository Authula helpers and the operator's existing
Cockpit credentials. No login credentials or cookies are recorded in evidence.

- Conversation: `01a12f13-e2ae-7f3f-b971-a36130424e6f`.
- Task: `01a12f14-49eb-7258-b9d1-0b655180ee7b`.
- Approval resolution: `{outcome:continue, remaining:0}`.
- Continued run: `run-ca181f56-59aa-42fc-8854-87a436d449e6`, `RUN_FINISHED`, success.
- Actual visible answer: “Il task è stato creato con successo: ogni giorno alle
  **9:00** riceverai un riassunto delle notizie principali su **Telegram**.”
- Read-back before cleanup: exactly one task, active; exactly one persisted RoleTool
  answer containing ACTIVE, do-not-reschedule and the new visible-response instruction.
- Continuation made no tool calls and created no second task.
- Cleanup/read-back: task cancelled, conversation archived. No scheduled notification fired.

The first turn performed the news trial, then asked where to deliver despite the
prompt already saying chat. A follow-up retried the request; the model first omitted
schedule_kind, received a validation error and corrected it within that turn. The actual
task chose Telegram, not the requested chat route. These existing model/tool-selection
limitations are not counted as proof of delivery-choice compliance or news quality.
The task was immediately cancelled; actual Telegram notification delivery is untested.

The driver resolves through the real approval API and issues the same continuation
request as Cockpit on Continue; it does not automate a browser click. Local evidence:
`.planning/tmp/scheduler-conversation-20261012-094634/`, including SSE, resolution and
machine-checked task/answer assertions. The requested response-after-approval behavior
is observed; universal model compliance is not claimed.
