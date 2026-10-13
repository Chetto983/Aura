# Gateway clarity: lab-VM acceptance

Acceptance of `docs/superpowers/specs/2026-10-12-gateway-clarity-design.md`, items 1–6, run on
2026-10-13 on the lab VM (`192.168.101.158`). The plan is
`docs/superpowers/plans/2026-10-12-gateway-clarity.md`.

All runs used the operator's own identity under `single_user_hardened`, through the cockpit API
and the operator's Telegram chat. The operator signed in with the repository's Authula helper;
no credential is recorded here. Every result below comes from a read-back (AG-UI frames,
`/api/approvals`, the thread snapshot, SQL on `aura-postgres`, the aura log), never from a reply.

The primary model was ChatGPT until about 07:40 UTC. Its `server_is_overloaded` errors were
intermittent and stopped turns before they reached the browser. From then on the operator ran
`ollama gemma4:31b-cloud`.

## Images

Each image is a child of `aura:lab-browser-keepalive-20261012` that replaces only
`/usr/local/bin/aura`. The binary was built from a clean `git archive` of the commit, with the SPA
rebuilt. Each overlay adds `stop_grace_period: 30s` and keeps the running sandbox image. The
updater timer stayed stopped throughout.

| Image `aura:lab-gateway-clarity-20261013…` | Commit | Used for |
|---|---|---|
| (before) `aura:lab-browser-keepalive-20261012` | older than the plan | item 5 "before" |
| `-2` | `ecb4f5c00` | item 5 after-1, after-2 |
| `-3` | `fcf52a184` | items 1, 4, 6 |
| `-4` | `db3d5e89d` | item 2, the Telegram commands |
| `-5` | `05f9a3c6b` | item 5 |
| `-6` | `7b6cdc630` | items 3, 5, 6, final |

The final image, `-6`, is image `sha256:59c7a0cf7969cdd4769dc7a29a423ac9fcbe5d91154517d238fdcd3bb99a9a70`.
Its binary is sha256 `96d9bdfd83a07cd9e96dea326780a034c787c458af3bdaf8987cb7bbb2242615`, read
inside the running container. The plan's code is `7c9c7287f`..`12cda7ae1`, plus the review's
fixes (`d095804ab`..`deffda241`) and the fixes below.

## Results

| Item | Result |
|---|---|
| 1. Target on the approval, not in the question | Pass, cockpit and Telegram. On Telegram the first try failed (see below) and the second passed. |
| 2. Unknown id refused after a restart | Pass |
| 3. Relayed question byte-identical, accept on the first try | Pass |
| 4. Refusal and failure, live and after a reload | Pass |
| 5. `docker stop` mid-turn | Pass on `-6`, after three fixes this run found |
| 6. `policies_enforced` | Pass |

### 1. The target is on the approval, not in the question

**Cockpit** (`-3`). The run created card "Prova accettazione gateway", then asked to delete it.
The delete `call_v2y83lnj` was withheld. `/api/approvals` showed:
- presentation `target`: `"Prova accettazione gateway · Da fare"`;
- question: `"Approve board (risk=destructive)?\nThis mutating action is WITHHELD until you accept.\nargs: action, id"`.

The question carries neither the label nor the column. The pause row's `resume_context` held the
stamped `target` and `tool_call_id`. After the accept, the re-drive deleted the card
(`call_jbvefqog`, 07:57:49), and `aura.board_cards` no longer had it.

**Telegram** (`-3`), first try, 07:59. Gemma relayed `args_sha256` `15f64ea65f7db52d59a484ac…`
for the real `15f64ea65f7db52d529a484ac…`, dropping one digit. The gateway logged
`gateway approval ledger miss … reason no_pending_challenge`. The stamp missed and, as
specified, dropped both `target` and `tool_call_id`, so the Telegram message had no "oggetto:"
line (operator's screenshot). The operator's accept at 08:00:50 was recorded on the pause, but
the resume hook failed with `challenge … not found`, and the card stayed.

Second try, 08:02:50. Pause `01a13449-3931…` carried `target` `"Prova Telegram 2 · Da fare"` and
`tool_call_id` `call_lfnk320q`. The operator accepted at 08:02:59 (`gateway approval recorded`,
scope `once`), and the re-drive deleted the card at 08:03:10.

### 2. An id is refused after a restart until the board is read again

On `-4`, card "Prova riavvio" (`47fc15ad…`) was created at 07:58. The restart was the container
recreate onto `-4`. After it, a delete by id was refused:
`card 47fc15ad-… is not on the board you listed: run board list or board search first`
(`call_mmxh9iyo`). The conversation had no approval and no pause at that point.

The model then listed the board on its own (`call_9u3onscx`, a `list` rather than a `search`) and
retried. Only that retry was withheld (`call_b5fp0tnr`): exactly one approval and one pause, with
target `"Prova riavvio · Da fare"`. After the accept, `call_jvjsimbi` deleted the card.

### 3. The relayed question equals the challenge, and the accept resumes on the first try

On `-6`, conversation `01a13477-6b0f…`, card "Prova domanda". Three strings were compared with
Python string equality, read from SQL:
- the withheld result's `question` field (the challenge);
- the `question` argument of the model's `ask_user` call;
- the pause row's `question`.

Both comparisons against the challenge were `True`. The question is the one quoted under item 1,
and the target `"Prova domanda · Da fare"` is only in the presentation. The accept returned
`{"outcome":"continue"}`. The first call after it, `call_n4nlo75g`, deleted the card. The
conversation had one pause in total.

The board question contains no character that HTML escaping changes, so this equality would also
have held before `7c9c7287f`. That fix is pinned by its unit tests, not by this run.

### 4. Refusal and failure, live and after a reload

On `-3`:

| Case | Live | After reload (snapshot) | Ledger |
|---|---|---|---|
| `deny` policy on `web_search` (`call_4ok38vcs`) | `aura.tool_outcome` `refused` | `refused=True isError=True` | `end`, `error`, `meta.gateway_verdict=deny` |
| `web_fetch` of `https://pagina-inesistente.invalid/notizie` (`call_nkwm6lr2`) | `aura.tool_outcome` `error` | `refused=None isError=True` | `end`, `error` |
| Operator's decline of a withheld delete (`call_w3b3v8u7`) | — | `refused=True isError=True` | pause `resumed_answer` = decline, `tool_call_id` stamped |

The policy was cleared right after, and `/api/approvals/policies` is empty. The labels "Negata"
and red come from those flags in the cockpit, and vitest covers them. They were not looked at in
a browser.

### 5. `docker stop` with a live view open and a turn streaming

The driver (scratchpad `drive_stop.py`) opens a cockpit turn and the browser live view on the
session that turn opened. While the turn streams, it runs a timed `docker stop aura` over ssh,
starts aura again, and lists every `start` row of the last 15 minutes that has no `end` row.

Times "after the stop" are measured from the moment the driver issues the stop. ssh adds about
1 s before the stop actually runs. The VM shares the workstation's CPU, and the load average was
between 1.4 and 3.6.

| Image | Run | Stop | Exit | Live view | Turn stream | `start` without `end` |
|---|---|---|---|---|---|---|
| before | operator's run | 10.35 s | 137 | — | — | — |
| before | 2 | 10.44 s | 137 | — | — | — |
| before | 3 | 0.37 s | 0 | — | — | — |
| before | 4 | 0.81 s | 0 | — | cut with no terminal frame | 3 |
| before | 5 | 10.39 s | 137 | open until the SIGKILL | — | — |
| `-2` | after-1 | 0.39 s | 0 | closed +1.20 s | `RUN_ERROR` | 0 |
| `-2` | after-2 | 1.64 s | 0 | — | — | **1**, fixed by `fcf52a184` |
| `-4` | after-1 | 0.82 s | 0 | closed +1.74 s | **ended on `CUSTOM`**, fixed by `05f9a3c6b` | 0 |
| `-5` | 1 | 1.47 s | 0 | closed +4.25 s | `RUN_ERROR` +4.31 s | 0 |
| `-5` | 2 | 1.15 s | 0 | closed +2.71 s | `RUN_ERROR` +2.63 s | 0 |
| `-5` | 3, Telegram turn in flight | 20.65 s | 0 | closed +1.65 s | `RUN_ERROR` **+16.30 s**, fixed by `7b6cdc630` | 0 |
| `-6` | Telegram turn in flight | 17.58 s | 0 | not measured | `RUN_ERROR` +1.75 s | 0 |
| `-6` | 1 | 0.59 s | 0 | not measured | `RUN_ERROR` +1.02 s | 0 |
| `-6` | 2 | 0.49 s | 0 | 38 frames, closed +1.21 s | `RUN_ERROR` +1.25 s | 0 |

In the two `-6` runs marked "not measured", the live view opened before the browser call had
created its session, and the relay answered `no_such_session`. For run 2 the driver was changed to
retry, as the cockpit does.

In the `-6` run with a Telegram turn in flight, the drain still joined the Telegram turn on a
live context, as the spec intends. That turn finished its answer about 9 s after the stop (3,858
characters, `content_stop`), and its `web_search` rows all have their `end`. After
`graceful shutdown complete`, `Close` of the conversation projector timed out at its 5 s
(`conversation projector shutdown incomplete`). That accounts for most of the remaining time to
17.58 s.

### 6. `policies_enforced`

`/api/me` returned `policies_enforced: true` on `-3` and again on `-6`. On the "before" image the
field was absent. The notice for `false` is covered by vitest and was not looked at in a browser.

## Bugs this acceptance found, and their fixes

Each fix was test-first (RED, then GREEN), with the touched packages' vet, race tests and
golangci-lint green, and was deployed and measured again on the VM.

| Commit | What the VM showed |
|---|---|
| `ecb4f5c00` | The cockpit's "retry" anchors its branch on an empty assistant turn. The rerun sent it to the model, and Ollama answered 400 for every later message in that conversation. The model's copy now leaves such turns out. |
| `fcf52a184` | A tool that ended because its turn was cancelled had its `end` row and its tool turn written with that same cancelled context, and both were lost (`after-2` on `-2`). |
| `db3d5e89d` | Telegram commands ran with no identity. `/clear` answered "Conversazione cancellata" and deleted nothing: the history since 2026-10-09 stayed, including a roleplay prompt Gemma kept following. `/search` always answered "Nessun risultato". `/cost` used the deployment's model profile. `/whoami` was hard-coded to "utente locale". |
| `05f9a3c6b` | A detached run's own `RUN_ERROR` was appended with its cancelled context, and the delivery `select` picked the cancellation about half the time. The cockpit's stream ended on `CUSTOM`. |
| `7b6cdc630` | The drain cancelled the detached runs (the cockpit's default path) only after joining the channel turns, so a Telegram turn held the cockpit turn for 12 s. |

`a3777b3bf` came from the coverage gate, not from the VM: a test deleted the local identity, and
a later test's insert then failed with FK 23503.

The Telegram fix was checked live on `-4` by the operator:
- `/whoami` named the linked account;
- `/search prova` returned 4 pages;
- `/clear`, then "ciao": the conversation was re-created at 08:25:55 with exactly those 2 turns,
  and no turn mentions "Axiom".

## Found and left open

1. **The runner takes the challenge's coordinates from the model's copy of the fingerprint.**
   One wrong digit (item 1, first Telegram try) loses the target and makes the resume fail. A
   follow-up spec should take them from the withheld result of the same round.
2. **A pause is marked resolved even when its resume hook fails.** The operator's accept at
   08:00:50 set `resumed_at` and `resumed_answer`. Nothing ran, and nothing told the operator.
3. An empty re-drive while an approval is pending makes the model relay a second pause for the
   same withheld call (item 1 in the cockpit, `call_3541h0no` beside `call_c6ihmc68`).
4. "Always" on a duplicate pause whose challenge was already consumed created no grant. Only the
   two grants from 2026-10-11 exist.
5. Telegram's status line shows a withheld call as "❌ board - errore".
6. `/cost` falls back to an estimate summed over every identity, because `aura.cache_metrics` is
   control-plane. The operator has to decide.
7. `/cancel`, `/reset` and `/stop` are three names for the same cancel.
8. With the roleplay prompt still in its history, Gemma created a duplicate card instead of
   relaying the approval. That is model behaviour, and it is gone since `/clear` works.
9. At shutdown the conversation projector can time out right after a turn finishes. The
   projection is rebuilt from Postgres, but whether the replay after the restart picked that turn
   up was not measured.

## Closing gates

On `7b6cdc630`. The Go gates and coverage ran in WSL (scratchpad `closing-gates.sh`); vitest ran
on Windows. All passed:

| Gate | Result |
|---|---|
| `go vet ./cmd/... ./internal/...` | ok |
| `go test -race` on 12 packages (`cmd/aura`, `agent`, `agent/tools`, `agui`, `approvaltext`, `askuser`, `board`, `channels/telegram`, `conversations`, `gateway`, `runner`, `setup`) | all `ok` |
| golangci-lint on the same packages | `0 issues.` |
| `scripts/check-file-size.sh` | all 4143 tracked source files within 600 LOC |
| `scripts/deadcode_gate.sh` | no findings |
| `sqlc generate` | byte-identical to the committed code |
| `tsc --noEmit`; oxlint and prettier on the 29 web files the work touched | clean; `Found 0 warnings and 0 errors.` |
| vitest: `src/chat src/approvals src/settings src/admin src/i18n` and `readabilityTokens` | 204 files, 1801 tests passed, 61 s |
| `scripts/coverage_docker.sh` (disposable `aura_cov`, 116 migrations) | 46,603 / 52,802 = 88.26% ≥ 85%; package-local policy passed |

vitest ran on Windows, with Node 24.16. `web/node_modules` is a Linux install, so the three
native bindings vitest needs on Windows were downloaded into a separate cache and linked in as
directory junctions:
- `@rolldown/binding-win32-x64-msvc` 1.2.12;
- `@tailwindcss/oxide-win32-x64-msvc` 4.3.3;
- `lightningcss-win32-x64-msvc` 1.32.0.

The lockfile and the Linux bindings were left as they are.

The same suite failed in WSL on `/mnt/d`. Two `readabilityTokens` cases each took 7–8 s to read
every source file through 9p. They pass on Windows, with the same code.

## What this does not establish

- Everything in the spec's "What this does not establish".
- **Mutation testing and CI.** The GitHub account has been blocked since 2026-10-11, so CI runs
  nothing, and mutation testing runs only there. The local gates above were the only gate.
- **What Telegram rendered on the second try.** The operator answered in 9 seconds and took no
  screenshot. The "oggetto:" line is inferred from the stamped pause through
  `localizedApprovalQuestion` and `RenderItalian`, which are unit-tested, not observed.
- **The cockpit's own rendering.** "Negata", the red tone, the immediate mark after a decline, and
  the absence of the `policies_enforced` notice were read through the API flags they derive from
  and through vitest, not in a browser.
- **Precise live-view timing.** Times come from the driver's clock, which includes the ssh hop.
  "Closed at once" means within the stop, not a measured delay from the drain's first step.
- **Repeatability.** Each image ran a handful of runs on a host whose CPU is shared, against a
  cloud model whose behaviour varies from one turn to the next.
