# aura-connect on Cloudflare Pages: lab VM acceptance

Date: 2026-10-13. Plan: `docs/superpowers/plans/2026-10-13-aura-connect-cloudflare-pages.md`.
Spec: `docs/superpowers/specs/2026-10-13-aura-connect-cloudflare-pages-design.md`.

**Verdict:** the three shared OAuth addresses answer from
`https://chetto1983-aura-connect.pages.dev/`. On the lab VM (`192.168.101.158`) a Google account
linked through the new relay, and ElevenLabs signed in through the new CIMD. The run found two
defects, both fixed the same day with tests that fail without the fix:
- `108603dc2`: the cockpit carried one MCP server's authorization onto another;
- `e6222a6c1`: an authorization could not replace a grant the provider refuses.

What the VM re-measured for each is narrower, and the last section says where.

## The site (Task 2)

- The Pages project `chetto1983-aura-connect` is built by Git integration from
  `gitlab.com/Chetto1983/aura-connect`. The operator authorized Cloudflare Workers on GitLab, with
  scope `api`.
- The build at aura-connect `a9896e4` ran `node --test` (21/21) and uploaded 13 files.
- Read back anonymously:
  - `mcp/client-metadata.json` is served as `application/json`, and its `client_id` equals its
    own URL;
  - all six paths answer 200;
  - both relays carry their CSP and `no-referrer`.

## The lab VM

| Image | Built from | Binary sha256 | What it carried |
|---|---|---|---|
| `aura:lab-aura-connect-20261013-1` | `c31fd6155` | `47d3b0898f58…a1ee4` | the new addresses |
| `aura:lab-aura-connect-20261013-2` | `e6222a6c1` | `88933470f026…40aac` | the two fixes below |

- Both images are children of `aura:lab-gateway-clarity-20261013-6`, with only
  `/usr/local/bin/aura` replaced. The SPA was rebuilt from `web/src` in each build.
- Each overlay (`/opt/aura/<tag>/compose.lab.yaml`) also sets
  `CALENDAR_MCP_CalendarMcp__GoogleOAuthRelayUrl` on `aura-pim-mcp`. In the container,
  `printenv` gives the new address.
- After each switch, `aura` was `healthy`, its image id matched the tag, and the binary's sha256
  inside the container matched the build.
- `auth.log` showed only this session's logins. The updater timer was stopped for the run.

## Google, end to end

- **Before any login**, `GET /api/connect/pim/accounts/{id}/google/start` answered
  `redirectUri` = `https://chetto1983-aura-connect.pages.dev/google/callback/`. The `authUrl`'s
  `redirect_uri` was the same address.
- The operator added that URI to the Google Cloud Web client.
- **First link.** The first attempt did not consent: the token file kept its 2026-10-10 date.
  The operator then consented on a fresh `google/start` link. The sidecar logged "Exchanging
  authorization code" and "Google OAuth tokens stored" at 13:43:43 UTC, and the browser showed
  "Account '…__personale' is now linked".
- **Second link.** The operator removed the account at 14:02:03 and added it again at 14:02:18.
  The consent was stored at 14:02:30.
- **Read-back.** `GET /api/connect/pim/accounts/{id}/status` answered `linked: true` at 14:03.
  The token file `Google.Apis.Auth.OAuth2.Responses.TokenResponse-user` is dated 14:02:30.

## ElevenLabs (CIMD), end to end

- **The first Connect on `-1` failed.** The flow ended in `invalid_client: Client metadata
  document returned HTTP 404`. The stored grant had been issued to the old document
  (`chetto1983.github.io/…`). The flow restored that grant and died refreshing it, before any
  consent.
- **This is the refresh leg PRD §13 had not measured.** ElevenLabs re-fetches the document named
  by the grant's `client_id` on refresh. With that document gone, it refuses every refresh.
  Background mount retries logged the same refusal every 5 s until they gave up.
- **Recovery on `-1`.** The operator pressed Disconnect, then Connect, and consented. The
  consent URL carried `client_id` = `https://chetto1983-aura-connect.pages.dev/mcp/client-metadata.json`
  and `redirect_uri` = `.../mcp/callback/`. The grant was recreated at 13:44:55, and the server
  mounted 160 tools at 13:44:57.
- **The fix (`e6222a6c1`).** A session that brings a Fetcher (the cockpit flow, `aura mcp
  login`) no longer restores the stored grant, because it exists to replace it. The regression
  test is `TestAuthorizationReplacesAGrantTheProviderNoLongerHonours`: a refused grant for the
  old document, then a fresh consent and a stored grant for the current one.
- **Re-measured on `-2`.**
  - At 14:01, an authorization started for the already-authorized ElevenLabs answered
    `authorization_required`, with a consent for the current document and relay. On `-1` the
    same request would have reused the stored grant without a consent.
  - The unattended boot mount still restored the stored grant: 160 tools at 14:01:04, no
    consent.

## The cockpit defect found on the way (`108603dc2`)

- **What happened.** With ElevenLabs' consent tracked by the authorization panel, the operator
  selected Linear. Linear's panel showed ElevenLabs' consent URL and no Connect button.
- **Two causes.**
  - `McpBoard` mounted the detail pane without a key, so its state survived a change of
    selection. That state includes an open env form, which would have saved one server's keys
    to another.
  - Disconnect cleared the polled flow, but not the start response that still held the spent
    consent URL.
- **Linear's grant was deleted.** Disconnect was pressed while Linear was selected. Only that
  route and `aura mcp logout` delete a grant, and the latter was not run.
- **Re-measured on `-2`.** The operator reloaded and connected Linear. The grant was stored at
  14:01:33, and 68 tools mounted at 14:01:33.
- **Tests.** Three vitest tests pin the fix: two in `McpBoard.test.tsx` and one in
  `McpAuthorizationPanel.test.tsx`. All three failed before the fix and pass after it. All 285
  `src/governance` tests pass.

## What this does not establish

- **Other operators' Google clients.** Each one must register the new URI; Aura cannot do it
  for them. Andrea's install has not been touched.
- **CIMD grants at other installs.** A grant issued to the old document stays refused until it
  is replaced. With `e6222a6c1`, Re-authorize replaces it. Without that build, the way back is
  Disconnect and then Connect.
- **Cached documents.** It is not known how long an authorization server keeps a document it
  fetched before.
- **Cloudflare's logs.** Codes pass through Cloudflare's infrastructure as they passed through
  GitHub's. A code cannot be redeemed without the install's PKCE verifier.
- **The `aura-connect.pages.dev` squatter.** That name belongs to someone else and is never
  linked.
- **A successful ElevenLabs refresh.** Only the refusal of a dead document's refresh was
  observed.
- **The fork's new default.** The PIM sidecar on the VM runs `ghcr.io/chetto1983/aura-pim-mcp:sidecar`
  with the relay set by the overlay. The fork's new default (`32d0c43`) is not in any published
  image yet.
- **The panel's change of selection.** The case where a consent is still pending when another
  server is selected was not driven on the VM. Disconnect followed by Connect was not driven
  either: Linear was connected after a page reload. The vitest tests cover both.
- **Re-authorize completing on `-2`.** The VM showed that an authorization on `-2` asks for a
  fresh consent with the current document. Nobody completed that consent. The new grant
  replacing the refused one is shown by `TestAuthorizationReplacesAGrantTheProviderNoLongerHonours`,
  and by the `-1` consent after Disconnect. That consent ran the same exchange, but it inserted
  the row instead of overwriting it.
- **CI.** No CI ran: the GitHub account is flagged and Actions does not run. The gates are the
  local ones named above, plus the pre-commit hooks.
