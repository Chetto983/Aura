
# aura-connect moves to Cloudflare Pages, for good

Agreed with the operator on 2026-10-13, at `13c442330`. aura-connect publishes the fixed OAuth
addresses every Aura install shares:
- Google's redirect relay, for calendar and email accounts;
- Aura's Client ID Metadata Document (CIMD) and its relay, for remote MCP servers that accept no
  other kind of client, ElevenLabs for one.

It lived on GitHub Pages. Since the GitHub account was flagged on 2026-10-11, all three
addresses answer 404 to everyone, so no install can connect a Google account or a CIMD-only MCP
server. Unlike the image bridge (`2026-10-13-gitlab-image-bridge-design.md`), this move does not
go back when GitHub returns.

## What was measured on 2026-10-13

- **The old addresses.** `https://chetto1983.github.io/aura-connect/mcp/client-metadata.json`
  answers 404 anonymously. The owner's credentials still read the repo, whose `main` is at
  `4fa7f65`.
- **The site is static and relative.** The repo holds:
  - `google/callback/` and `mcp/callback/`, each an `index.html` and a `relay.js`;
  - `lib/relay.js` and `lib/relay.css`;
  - `mcp/client-metadata.json`;
  - three `node --test` suites, run by a GitHub Actions workflow.

  Every page loads its assets by relative path (`../../lib/relay.js`, `./relay.js`). Each
  carries its own CSP and `no-referrer` as `<meta>` tags. The absolute addresses appear only
  in these places:
  - `client-metadata.json`: `client_id`, `redirect_uris`, and a `client_uri` that points at
    the GitHub repo;
  - `test/client-metadata.test.mjs`: `PUBLISHED_AT` and `RELAY`;
  - the README.
- **The Pages name.** `aura-connect.pages.dev` is taken: it answers 200 with a third party's
  "AURA-Connect" page. `chetto1983-aura-connect.pages.dev` does not resolve, so it is free.
- **What Cloudflare documents.**
  - Pages Git integration supports GitHub and GitLab.com, and not self-managed GitLab.
  - A project created by Direct Upload cannot switch to Git integration later.
  - A production deployment is served at `<project>.pages.dev`.
- **Where Aura names the old addresses.**
  - `internal/mcp/oauth_cimd.go:34` names `auraClientMetadataURL`, and `:41` names
    `auraOAuthRelayURL`.
  - `internal/agui/connect_pim_providers_api.go:18` names `PIMGoogleRelayRedirectURI`.
  - Two web tests name it: `CalendarConnect.test.tsx:74` and `PimProviderAppsPanel.test.tsx:22`.
  - Comments name it in `cmd/aura/serve_webui.go:262`, `internal/agui/connect_pim_api.go:35`,
    `:171` and `web/src/governance/pimApi.ts:127`.
  - The operator docs in `docs/mcp-manager.md:250-280` and the record in PRD §13 name it too.

  The Go tests compare against the constants, never against a literal.
- **The PIM sidecar names the Google relay itself.** This was found while writing the plan, the
  same day. The sidecar builds the consent URL and exchanges the code with its own
  `GoogleOAuthRelayUrl`.
  - Its default is the old address:
    `chetto1983/aura-pim-mcp@main:src/CalendarMcp.Core/Configuration/CalendarMcpConfiguration.cs:36`.
  - `PIMGoogleRelayRedirectURI` only shows that address in the admin panel. Its comment says
    it "must equal the sidecar's GoogleOAuthRelayUrl default", and nothing checks it.
  - The sidecar loads its configuration with `AddEnvironmentVariables("CALENDAR_MCP_")`. Its
    own docs give `CALENDAR_MCP_CalendarMcp__ExternalBaseUrl` as the environment form of the
    sibling setting, so `CALENDAR_MCP_CalendarMcp__GoogleOAuthRelayUrl` sets this one.
    `compose.yaml` sets neither today.

This does not show whether Google accepts a `pages.dev` redirect URI in the operator's client,
or how long an authorization server caches a CIMD it fetched. The acceptance measures the first.
The second is listed below.

## Decisions (operator, 2026-10-13)

- **The site goes to Cloudflare Pages through Git integration with a GitLab mirror.** Direct
  Upload was rejected for two reasons: every change would need a manual deploy, and Cloudflare
  never lets such a project move to Git.
- **The move is permanent.** Each move of these addresses costs every operator a manual step:
  a new redirect URI in their Google client, and a new login on each CIMD server. Coming back
  to github.io would charge that a second time for nothing.

## Design

### The site

1. **GitLab project.** A public `gitlab.com/Chetto1983/aura-connect` is created with glab and
   receives the full history of `main` from the GitHub clone. It becomes the source of the
   site. The GitHub repo is left as it is.
2. **One commit in aura-connect** moves the addresses to
   `https://chetto1983-aura-connect.pages.dev/`:
   - `client-metadata.json`:
     - `client_id` becomes `https://chetto1983-aura-connect.pages.dev/mcp/client-metadata.json`;
     - `redirect_uris` becomes `["https://chetto1983-aura-connect.pages.dev/mcp/callback/"]`;
     - `client_uri` becomes the public GitLab mirror of Aura, because the GitHub repo is
       unreachable for everyone else;
   - the test's `PUBLISHED_AT` and `RELAY`;
   - the README:
     - every address;
     - the hosting-log sentence, which now names Cloudflare;
     - the development note, since GitHub Actions no longer runs the tests.

   The relay logic, the CSP and the allowed target paths do not change.
3. **The Pages project.** The operator creates `chetto1983-aura-connect` in the Cloudflare
   dashboard: Connect to Git, then GitLab, then `Chetto1983/aura-connect`. The settings:
   - production branch `main`;
   - no framework;
   - build command `node --test`;
   - output directory, the repo root.

   The tests are therefore the deploy gate: a red suite publishes nothing. GitHub Pages also
   served the repo root, README and tests included, so nothing new becomes public.

### Aura

- **One commit changes the addresses.**
  - It changes the three constants to the new addresses.
  - It changes the two web tests.
  - It changes the comments that name GitHub Pages or the old host.
  - It changes the operator guide `docs/mcp-manager.md`.
- **The sidecar is told the address.** `compose.yaml` sets
  `CALENDAR_MCP_CalendarMcp__GoogleOAuthRelayUrl` on `aura-pim-mcp` to the new Google relay.
  - This works with the pim image appliances already run, so no rebuild is needed.
  - A Go test beside the existing compose artifact tests asserts the value equals
    `PIMGoogleRelayRedirectURI`. The "must equal" in the comment becomes a check.
  - The fork's own default moves to the new address too, in one commit on its `main`, so a
    standalone run of the fork names no dead page. The bridge's fork build picks it up.
- **Nothing else changes.** The relay shape stays: `state` as `<nonce>.<base64url(target)>`.
  So do the callback paths and the PKCE.
- **PRD §13 records the new address after the acceptance,** citing the ledger. The plans in
  `docs/superpowers/plans/` are history and stay as they are.

### What each operator does once

- **Google.**
  - Add `https://chetto1983-aura-connect.pages.dev/google/callback/` as an Authorized redirect
    URI in their Google Cloud Web client.
  - Add `chetto1983-aura-connect.pages.dev` to the consent screen's authorized domains if
    Google asks.
  - Google accounts already connected keep working: a redirect URI matters only at consent.
- **CIMD servers.** Sign in again to each MCP server connected through the CIMD. The client_id
  changes, so to the authorization server Aura is a new client. Since 2026-10-11 those servers
  could not fetch the old document anyway.

The operator does these steps for the lab VM's client, and Andrea for his. The cockpit already
shows the redirect URI, read from the constant, in the provider panel. So the panel names the
new address as soon as the new image runs.

## Tests

- **aura-connect.** `node --test` passes locally with the new addresses, and the metadata suite
  asserts `client_id` equals the published URL. It runs again as the Pages build.
- **Aura.**
  - The Go tests under `internal/mcp` and `internal/agui` follow the constants.
  - The two vitest files take the new literal.
  - The Gate 2 checks run on the touched packages: vet, build, test and race.

## Acceptance

1. **Anonymous reads of the three addresses on `chetto1983-aura-connect.pages.dev`:**
   - `/mcp/client-metadata.json` answers 200 with `Content-Type: application/json`, and its
     `client_id` equals its own URL;
   - both relay pages answer 200, and their CSP and referrer `<meta>` tags are intact.
2. **Google, end to end on the lab VM,** on an image carrying the new constants and a compose
   that sets the sidecar's relay. Until the image bridge lands, both reach the VM through the
   lab overlay, as on 2026-10-13. Before any login, the start endpoint's consent URL must carry
   the new `redirect_uri`.
   1. The operator adds the redirect URI.
   2. The operator connects a Google calendar account from the cockpit, driving the login
      themselves.
   3. Read back: the account is listed with its status by `/admin/accounts`.
3. **CIMD, end to end:** a CIMD-only server (ElevenLabs) connects, and its grant is stored. The
   operator drives the login.
4. **The ledger** is a dated file in `docs/superpowers/verification/`.

## What this does not establish

- **Other operators' Google clients.** Each must register the new URI. Nothing in Aura can do
  it for them.
- **Cached documents.** How long an authorization server keeps a CIMD it fetched before. A
  server that cached the old document still names the old relay until it fetches again.
- **Cloudflare's logs.** The code passes through Cloudflare's infrastructure as it passed
  through GitHub's. It cannot be redeemed without the install's secret or PKCE verifier.
- **The `aura-connect.pages.dev` squatter.** The name is someone else's, and a link to it is
  never to be used.
