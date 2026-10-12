# Scheduled job repair plan and run ledger

**Goal:** Execute the noleggio-sociale scheduled check without recursively scheduling
it or passing a technical session to UUID-backed persistence.

**Architecture:** Reuse LlmAgent construction, tool-call context and the cron handler's
existing message history. Pass a durable origin independently from technical sessions;
seed the worker history with scheduled execution context.

**Tech Stack:** Existing Go agent/tools/cron/swarm, WSL, authenticated Cockpit API,
lab VM Docker Compose. No new dependency, migration or environment variable.

**Spec:** `docs/superpowers/specs/2026-10-12-scheduled-job-origin-design.md`.

## Global Constraints

- Preserve unrelated `.claude/settings.json` and `Aura_analisi_codebase.md` changes.
- Work on existing master; no task branch or worktree.
- Preserve the latest browser-sidecar lab image and sandbox configuration.
- Do not send unsolicited Telegram notifications or leave recurring test jobs active.
- Do not claim legal-content accuracy, full coverage closure or green remote CI.

## Review Focus

Origin must reach tools after the technical tool-call context is installed. The stable
shared system prefix and exact stored goal must remain intact. Auto-reject must retain
the execution instruction. Explicit follow-up scheduling must not be disabled globally.

## File structure

| Files | Responsibility |
|---|---|
| agent/llm_agent{,_construct,_tool}.go | Agent origin configuration and tool context |
| agent/tools/{result,task}.go | Context access and durable task owner |
| cron/handlers/{agentjob,handler}.go | Worker context and cron origin |
| swarm/swarm.go | Worker origin propagation |
| cron/handlers/agentjob{,_context}_test.go; swarm/worker_identity_test.go | Regression coverage |

## Tasks and evidence

- [x] Reproduce on the real stack with the exported goal. Run
  `01a12ee6-c00a-7257-a11c-de02d003fc93` failed six task calls with invalid UUID length 46.
- [x] Propagate durable origins and cover cron/swarm construction with regression tests.
- [x] Add the execution-context regression; observe RED (message role user, not system),
  then add the scheduled instruction and observe GREEN, including auto-reject continuation.
- [x] Run affected package tests and race/goleak in WSL; all four packages pass.
- [x] Compile an isolated WSL source snapshot with the freshly built SPA; install a
  binary-only child of the latest lab image and verify running image/binary hashes.
- [x] Replay the exact exported goal, cron, timezone and step budget 12. Run
  `01a12ef2-2e67-7ca9-8fb8-54e0e9d25374` completed in 23 seconds with research calls,
  no task calls and no duplicate task. Cancel the isolated test task and archive its conversation.
- [x] Finish vet/build/lint/deadcode evidence and final diff review.
- [x] Commit measured PRD/spec record separately (`796bdc2a9`).
- [ ] Commit the repair and verification.
- [ ] Push using the repository WSL recipe and report remote CI availability honestly.

## Decisions taken during the run

The Cockpit edit API does not accept step_budget. Only the newly created test task's
row was updated, guarded by its exact task and conversation UUIDs, to reproduce budget 12.
Notification was redirected to stdout; Telegram delivery is unmeasured.

The VM updater timer was unexpectedly active. It was stopped (not disabled) to keep
the manual candidate from being replaced by edge while the registry account is blocked.
