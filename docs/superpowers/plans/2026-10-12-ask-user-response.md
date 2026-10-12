# ask_user response repair plan

**Goal:** After the person's ask_user answer, the agent explains the outcome instead
of silently ending, including scheduled-task approvals.

**Architecture:** Reuse runner RoleTool answer construction and the channels' Continue
path. Preserve task-state/no-rescheduling answers and worker/cancel/expiry semantics.

**Tech Stack:** Existing Go runner, AG-UI, Telegram, PostgreSQL; WSL and lab VM Compose.

**Spec:** `docs/superpowers/specs/2026-10-12-ask-user-response-design.md`.

## Global Constraints

Master directly, preserve unrelated user edits, no migration/env change. Keep the latest
browser-sidecar image lineage and sandbox. Do not leave recurring test jobs active or
send unsolicited notifications. No mutation testing per operator restriction recorded
for this run; no full coverage or remote-CI pass claim without a measurement.

## Review Focus

Both relayed and sweep-minted scheduled approvals have wire-valid history. Continue
must not duplicate scheduling. Appended prose must not validate an otherwise empty
answer or change cancel, expiry, worker-owned or policy/fencing behavior.

## File structure

- `internal/runner/runner_resume.go`: actual ask_user answer instruction.
- `internal/runner/runner_resume_directive.go`: scheduled approval continuation.
- Runner answer/directive/scheduled-approval unit and integration tests.
- Dated verification ledger for real-agent confirmation and deployment hashes.

## Tasks

- [x] Measure the silent ending on the running VM before recording the contract.
- [x] RED scheduled acknowledgement test; remove scheduled terminal classification.
- [x] RED visible-answer tests across all three ask_user kinds and both decisions.
- [x] Append the visible-response instruction; preserve cancel/expiry/worker/empty input.
- [x] Verify affected packages, race/goleak, real disposable PostgreSQL, static gates.
- [x] Deploy the binary-only lab child; replay daily news and approve with the real agent.
- [x] Verify final assistant text, one schedule, and test-resource cleanup.
- [ ] Review diff; commit the measured spec before the source commit, then push and check CI.

## Decisions taken during implementation

The response is built by `Runner.answerTurn`, not `AskUser.Execute`: Execute must retain
the awaiting-input sentinel. Adding normal return text there would break the pause.
Expiry is a separate non-human action and must retain its existing terminal marker.
