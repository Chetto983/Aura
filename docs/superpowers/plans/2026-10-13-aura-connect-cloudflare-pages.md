# aura-connect on Cloudflare Pages

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Every Aura install can again connect a Google account and a CIMD-only MCP server:
- the three shared OAuth addresses answer from `https://chetto1983-aura-connect.pages.dev/`;
- Aura names them;
- the PIM sidecar is sent through them.

**Architecture:** three repositories move together.
- **aura-connect** gets a public GitLab mirror. One commit moves its addresses, and Cloudflare
  Pages builds it from GitLab, with `node --test` as the deploy gate.
- **Aura** moves its two CIMD constants and its Google relay constant. `compose.yaml` hands the
  Google relay to the PIM sidecar, which is what actually builds Google's consent URL, and a Go
  test keeps the two equal.
- **The aura-pim-mcp fork** moves its own default, so a standalone run names no dead page.

**Tech Stack:** static HTML/JS with `node --test` (Node 24 in WSL); Go 1.27; React + vitest;
.NET 10 (the fork, tested in the `mcr.microsoft.com/dotnet/sdk:10.0` container); glab v1.122.0;
Cloudflare Pages Git integration.

**Spec:** `docs/superpowers/specs/2026-10-13-aura-connect-cloudflare-pages-design.md`. It was
agreed on 2026-10-13 and amended the same day: the PIM sidecar names the Google relay itself.

## Global Constraints

- **The new base is `https://chetto1983-aura-connect.pages.dev/`.**
  - Never `aura-connect.pages.dev`: that name answers with a third party's site.
  - The three addresses are exactly:
    - `https://chetto1983-aura-connect.pages.dev/google/callback/`
    - `https://chetto1983-aura-connect.pages.dev/mcp/client-metadata.json`
    - `https://chetto1983-aura-connect.pages.dev/mcp/callback/`
- **The relay shape, the callback paths, the CSP and the PKCE do not change.**
- **Go runs only in WSL.** Never `go build ./...` or `go test ./...` locally. After every Go
  edit, run `go vet` and `go test -race` on the touched packages.
- **Web runs vitest on Windows** through `node node_modules/vitest/vitest.mjs run …` from
  `web/`. Prettier and oxlint run on the touched files.
- **Commits.** One commit per task, with the why. Commit from WSL with an explicit pathspec:
  `LEFTHOOK_BIN=$HOME/go/bin/lefthook git -c core.hooksPath=.git/hooks commit -F <msg> -- <paths>`.
  Never `--no-verify`.
- **`compose.yaml` is a payload file.** Its change is committed first. Then
  `make payload-manifest` writes `scripts/payload_manifest.txt` in a second commit, because the
  gate refuses to freeze a manifest of uncommitted bytes.
- **The operator does every login and every console change.** That covers the Cloudflare
  dashboard, Google Cloud, the Google consent and the ElevenLabs consent. Claude never handles
  their credentials.
- **No CI while the GitHub account is flagged.** The local gates are the only gate, and the
  ledger says so.

## Review Focus

1. **The panel shows one Google address and the sidecar sends another.** The operator would
   register a URI Google is never sent, and the consent would fail with
   `redirect_uri_mismatch`. Pinned by `TestPIMSidecarIsToldTheGoogleRelay` (Task 3), and live
   by acceptance step 2, which reads the consent URL before any login.
2. **The served CIMD names a `client_id` other than its own URL.** Every authorization server
   answers `invalid_client`, and that is all an install sees. Pinned by aura-connect's
   `client_id is the exact URL the document is published at` (Task 1), and live by the
   anonymous read in Task 2, which compares the served `client_id` with the URL it came from.
3. **The document is served with a non-JSON `Content-Type`.** A strict authorization server
   refuses it. Task 2's read asserts `application/json`.
4. **A relay page loads but its script does not.** The pages load `../../lib/relay.js`
   relative to their own path. Task 2 reads `/lib/relay.js` and `/lib/relay.css` from the root,
   expecting 200, and checks that each page still carries its CSP and `no-referrer`.
5. **The squatter's name slips in.** Task 3's last check greps Aura, aura-connect and the fork
   for `aura-connect.pages.dev` without the `chetto1983-` prefix, and expects nothing.

## Decisions taken while writing this plan

- **The sidecar gap.** The plan's inventory found that the sidecar's `GoogleOAuthRelayUrl`
  builds Google's consent URL and code exchange. Aura's constant only displays it. The spec was
  amended the same day: `compose.yaml` sets
  `CALENDAR_MCP_CalendarMcp__GoogleOAuthRelayUrl`, which the fork's own Google setup guide
  names as the override (`docs/GOOGLE-SETUP.md`: "Override it with
  `CalendarMcp:GoogleOAuthRelayUrl`").
- **aura-connect's `.github/workflows/test.yml` is deleted** in the address commit. On GitLab
  nothing runs it, and the Pages build runs the same `node --test`.
- **Four comments stay.** `cmd/aura/serve_webui.go:262`, `internal/agui/connect_pim_api.go:35`
  and `:171`, and `web/src/governance/pimApi.ts:127` say "the aura-connect relay" without
  naming a host. They are still true.
- **The fork commit is pushed to GitHub,** where the owner can still push. Aura's `origin`
  took a push on 2026-10-13. The image bridge plan mirrors the fork to GitLab and builds from
  it.
- **The lab VM is checked through the overlay, not through the bridge.** The overlay carries a
  child image with the new binary and a `compose.lab.yaml` that adds the sidecar's variable, as
  on 2026-10-13. This unblocks the operator's own install before the bridge lands. Andrea gets
  the change through the bridge.

## File structure

| Repository | File | Change |
|---|---|---|
| aura-connect | `mcp/client-metadata.json` | `client_id`, `redirect_uris`, `client_uri` |
| aura-connect | `test/client-metadata.test.mjs` | `PUBLISHED_AT`, `RELAY` |
| aura-connect | `README.md` | the addresses, the Aura link, hosting logs, development |
| aura-connect | `.github/workflows/test.yml` | deleted |
| Aura | `internal/mcp/oauth_cimd.go:32-41` | two constants and their comment |
| Aura | `internal/agui/connect_pim_providers_api.go:14-18` | the constant and its comment |
| Aura | `compose.yaml` (`aura-pim-mcp` environment) | `CALENDAR_MCP_CalendarMcp__GoogleOAuthRelayUrl` |
| Aura | `cmd/aura/pim_relay_artifacts_test.go` | new: `TestPIMSidecarIsToldTheGoogleRelay` |
| Aura | `web/src/governance/__tests__/CalendarConnect.test.tsx:74`, `PimProviderAppsPanel.test.tsx:22` | fixture literal |
| Aura | `docs/mcp-manager.md:250,259,278,280` | the addresses and the repo link |
| Aura | `scripts/payload_manifest.txt` | regenerated, second commit |
| Aura | `docs/superpowers/verification/2026-10-13-aura-connect-cloudflare-pages.md` | new ledger |
| Aura | `prd.md` §13 (around line 2461) | the new address, after the acceptance |
| aura-pim-mcp | `src/CalendarMcp.Core/Configuration/CalendarMcpConfiguration.cs:36` | default |
| aura-pim-mcp | `src/CalendarMcp.Tests/Admin/GoogleOAuthRelayTests.cs:13` | fixture literal |
| aura-pim-mcp | `src/CalendarMcp.HttpServer/Admin/GoogleOAuthRelay.cs:10`, `docs/GOOGLE-SETUP.md:181`, `AURA-FORK.md:54` | the address and the repo |

Working copies, outside the Aura tree:
- aura-connect is cloned at
  `C:/Users/Davide/AppData/Local/Temp/claude/d--Aura/<session>/scratchpad/aura-connect-src/repo`;
- the fork is cloned at `…/scratchpad/forks/aura-pim-mcp`.

Both were cloned from GitHub with the owner's `gh` credentials. Below they are `$AC` and `$PIM`.

---

### Task 1: aura-connect gets a GitLab mirror and the new addresses

**Files:**
- Modify: `$AC/test/client-metadata.test.mjs:8-9`
- Modify: `$AC/mcp/client-metadata.json`
- Modify: `$AC/README.md:3,15,41,45,67-69,75-77`
- Delete: `$AC/.github/workflows/test.yml`

**Interfaces:**
- Produces: `gitlab.com/Chetto1983/aura-connect` (public, branch `main`) serving these files at
  these paths:
  - `/google/callback/`
  - `/mcp/callback/`
  - `/mcp/client-metadata.json`
  - `/lib/relay.js`
  - `/lib/relay.css`

  Task 2 connects Cloudflare Pages to it.

- [x] **Step 1: Create the GitLab project from a directory that is not a git repo**

```bash
export PATH="$HOME/go/bin:$PATH"
cd "$(mktemp -d)"
glab repo create Chetto1983/aura-connect --public --defaultBranch main \
  -d "Fixed OAuth addresses every Aura install shares: Google redirect relay and the MCP client metadata document."
```

Expected: the project URL `https://gitlab.com/Chetto1983/aura-connect` is printed. Run from a
neutral directory, glab adds no remote anywhere.

- [x] **Step 2: Push the full history of `main` to it**

```bash
cd "$AC"
git remote add gitlab git@gitlab.com:Chetto1983/aura-connect.git
git push gitlab main
git ls-remote gitlab main
```

Expected: `4fa7f65…  refs/heads/main`.

- [x] **Step 3: Write the failing test.** In `$AC/test/client-metadata.test.mjs`, lines 8-9
  become:

```js
const PUBLISHED_AT = "https://chetto1983-aura-connect.pages.dev/mcp/client-metadata.json";
const RELAY = "https://chetto1983-aura-connect.pages.dev/mcp/callback/";
```

- [x] **Step 4: Run it and watch it fail**

Run: `cd "$AC" && node --test`
Expected: FAIL in `client_id is the exact URL the document is published at` and in
`the relay is the only redirect, on the client_id's own host`, because the document still names
github.io.

- [x] **Step 5: Move the document.** `$AC/mcp/client-metadata.json` becomes:

```json
{
  "client_id": "https://chetto1983-aura-connect.pages.dev/mcp/client-metadata.json",
  "client_name": "Aura",
  "client_uri": "https://gitlab.com/Chetto1983/aura",
  "redirect_uris": ["https://chetto1983-aura-connect.pages.dev/mcp/callback/"],
  "grant_types": ["authorization_code", "refresh_token"],
  "response_types": ["code"],
  "token_endpoint_auth_method": "none"
}
```

- [x] **Step 6: Run the suites**

Run: `cd "$AC" && node --test`
Expected: PASS, all three files (`client-metadata`, `mcp-relay`, `relay`), 0 failures.

- [x] **Step 7: Move the README.** Exact replacements in `$AC/README.md`:
  - line 3: `[Aura](https://github.com/chetto1983/Aura)` → `[Aura](https://gitlab.com/Chetto1983/aura)`;
  - line 15: `https://chetto1983.github.io/aura-connect/google/callback/` →
    `https://chetto1983-aura-connect.pages.dev/google/callback/`;
  - line 41: `https://chetto1983.github.io/aura-connect/mcp/client-metadata.json` →
    `https://chetto1983-aura-connect.pages.dev/mcp/client-metadata.json`;
  - line 45: `` `https://chetto1983.github.io/aura-connect/mcp/callback/` `` →
    `` `https://chetto1983-aura-connect.pages.dev/mcp/callback/` ``;
  - the "Hosting logs" bullet: `passes through GitHub Pages' infrastructure` →
    `passes through Cloudflare Pages' infrastructure`;
  - the last line of "Development":
    `The tests in [`test/relay.test.mjs`](test/relay.test.mjs) run in CI on every push.` →
    `Cloudflare Pages runs \`node --test\` as this site's build command, so a red suite
    publishes nothing.`

  Then delete `.github/workflows/test.yml`:

```bash
cd "$AC" && git rm -q .github/workflows/test.yml
grep -rn "github.io\|github.com/chetto1983" . --exclude-dir=.git
```

Expected: no output.

- [x] **Step 8: Commit and push**

```bash
cd "$AC"
git add mcp/client-metadata.json test/client-metadata.test.mjs README.md
git commit -m "Move the shared OAuth addresses to Cloudflare Pages

GitHub flagged the account on 2026-10-11, and since then
chetto1983.github.io/aura-connect answers 404 to everyone: no Aura
install can connect a Google account or a CIMD-only MCP server.

The site now builds on Cloudflare Pages from this GitLab repo, at
chetto1983-aura-connect.pages.dev. The bare aura-connect.pages.dev
belongs to a third party. The move is permanent, because each move costs
every operator a new Google redirect URI and a new CIMD login.

The Pages build runs node --test, so the GitHub Actions workflow goes.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
git push gitlab main
```

Expected: the push names one new commit on `main`.

---

### Task 2: Cloudflare Pages serves it (operator), and the three addresses read back

**Files:** none in a repo. The read-back output goes into the ledger in the closing task.

**Interfaces:**
- Consumes: the GitLab project from Task 1.
- Produces: the live base `https://chetto1983-aura-connect.pages.dev/`, which Tasks 3 and 4
  name.

- [x] **Step 1: The operator creates the Pages project.** In the Cloudflare dashboard:
  1. Workers & Pages → Create → Pages → Connect to Git.
  2. Choose GitLab and authorize Cloudflare on the GitLab account.
  3. Pick `Chetto1983/aura-connect`.
  4. Set:
     - project name `chetto1983-aura-connect`;
     - production branch `main`;
     - framework preset `None`;
     - build command `node --test`;
     - build output directory `/`.
  5. Save and deploy.

  Expected: the first deployment's build log shows the `node --test` summary with 0 failures,
  and the production URL is `https://chetto1983-aura-connect.pages.dev`. If the dashboard
  offers only another name, stop: the base is a Global Constraint.

- [x] **Step 2: Read the three addresses anonymously**

```bash
B=https://chetto1983-aura-connect.pages.dev
curl -s -D - -o /tmp/cimd.json "$B/mcp/client-metadata.json" | grep -i -E '^HTTP|^content-type'
python3 -c "import json; d=json.load(open('/tmp/cimd.json')); print(d['client_id']); assert d['client_id']=='$B/mcp/client-metadata.json'; assert d['redirect_uris']==['$B/mcp/callback/']"
for p in google/callback/ mcp/callback/ lib/relay.js lib/relay.css google/callback/relay.js mcp/callback/relay.js; do
  printf '%s %s\n' "$(curl -s -o /dev/null -w '%{http_code}' "$B/$p")" "$p"
done
for p in google/callback/ mcp/callback/; do
  curl -s "$B/$p" | grep -c -E "Content-Security-Policy|name=\"referrer\" content=\"no-referrer\""
done
```

Expected:
- `HTTP/2 200`, and a content type starting with `application/json`;
- the `client_id` line prints, and neither assertion fires;
- six lines start with `200`;
- the last loop prints `2` twice.

- [x] **Step 3: No commit.** Keep the output for the ledger.

---

### Task 3: Aura names the new addresses, and the PIM sidecar is told the Google relay

**Files:**
- Create: `cmd/aura/pim_relay_artifacts_test.go`
- Modify: `compose.yaml` (the `aura-pim-mcp` `environment:` block, after `ASPNETCORE_URLS`)
- Modify: `internal/agui/connect_pim_providers_api.go:14-18`
- Modify: `internal/mcp/oauth_cimd.go:32-41`
- Modify: `web/src/governance/__tests__/CalendarConnect.test.tsx:74`
- Modify: `web/src/governance/__tests__/PimProviderAppsPanel.test.tsx:22`
- Modify: `docs/mcp-manager.md:250,259,278,280`
- Modify (second commit): `scripts/payload_manifest.txt`

**Interfaces:**
- Consumes: the base from Task 2.
- Produces:
  - `agui.PIMGoogleRelayRedirectURI = "https://chetto1983-aura-connect.pages.dev/google/callback/"`;
  - the compose line `CALENDAR_MCP_CalendarMcp__GoogleOAuthRelayUrl: <same>`.

  The closing task deploys both.

- [x] **Step 1: Write the failing test.** Create `cmd/aura/pim_relay_artifacts_test.go`:

```go
package main

import (
	"strings"
	"testing"

	"github.com/chetto1983/aura/internal/agui"
)

// The PIM sidecar builds Google's consent URL and exchanges the code with its own relay
// address, while the admin panel shows agui.PIMGoogleRelayRedirectURI as the URI to register.
// compose.yaml hands the sidecar that same constant, so the address an operator registers is
// the one Google is sent.
func TestPIMSidecarIsToldTheGoogleRelay(t *testing.T) {
	service := composeServiceBlock(t, readProjectFile(t, repoRootForTest(t), "compose.yaml"), "aura-pim-mcp")
	want := "CALENDAR_MCP_CalendarMcp__GoogleOAuthRelayUrl: " + agui.PIMGoogleRelayRedirectURI
	if !strings.Contains(service, want) {
		t.Fatalf("aura-pim-mcp is not told the Google relay the panel shows; want the line %q", want)
	}
}
```

- [x] **Step 2: Run it and watch it fail**

Run, in WSL from `/mnt/d/Aura`:
`go test -run TestPIMSidecarIsToldTheGoogleRelay ./cmd/aura/`
Expected: FAIL with `aura-pim-mcp is not told the Google relay the panel shows`.

- [x] **Step 3: Move the Google constant.** `internal/agui/connect_pim_providers_api.go:14-18`
  becomes:

```go
// PIMGoogleRelayRedirectURI is the one redirect URI every install registers in its Google Web
// client. The sidecar builds the consent URL and the code exchange with its own copy, which
// compose.yaml sets to this value (CALENDAR_MCP_CalendarMcp__GoogleOAuthRelayUrl), and
// TestPIMSidecarIsToldTheGoogleRelay keeps the two equal. The relay's address is fixed by
// design, which is why a constant is enough for the admin panel.
const PIMGoogleRelayRedirectURI = "https://chetto1983-aura-connect.pages.dev/google/callback/"
```

- [x] **Step 4: Tell the sidecar.** In `compose.yaml`, in the `aura-pim-mcp` service's
  `environment:` block, insert right after the line
  `      ASPNETCORE_URLS: http://0.0.0.0:8080`, with six spaces before each key:

```yaml
      # Google returns through the shared aura-connect relay, and this sidecar builds the consent
      # URL and the code exchange with this address. The admin panel shows the same one
      # (agui.PIMGoogleRelayRedirectURI); TestPIMSidecarIsToldTheGoogleRelay keeps them equal.
      CALENDAR_MCP_CalendarMcp__GoogleOAuthRelayUrl: https://chetto1983-aura-connect.pages.dev/google/callback/
```

- [x] **Step 5: Run the test**

Run: `go test -run TestPIMSidecarIsToldTheGoogleRelay ./cmd/aura/`
Expected: PASS.

- [x] **Step 6: Move the CIMD constants.** `internal/mcp/oauth_cimd.go:32-41` becomes:

```go
	// auraClientMetadataURL is Aura's metadata document, and so its client_id. One document
	// serves every install. Built by Cloudflare Pages from gitlab.com/Chetto1983/aura-connect.
	auraClientMetadataURL = "https://chetto1983-aura-connect.pages.dev/mcp/client-metadata.json"

	// auraOAuthRelayURL is the document's only redirect URI. An authorization server must
	// match a redirect exactly, and an install's own callback is neither known in advance
	// nor https on a LAN, so the server redirects to this fixed page, which forwards the
	// browser to the callback named in `state` (relayedFetcher). It accepts only the
	// cockpit's callback path and the CLI's loopback one, https or loopback http.
	auraOAuthRelayURL = "https://chetto1983-aura-connect.pages.dev/mcp/callback/"
```

- [x] **Step 7: Move the web fixtures.** In both
  `web/src/governance/__tests__/CalendarConnect.test.tsx:74` and
  `web/src/governance/__tests__/PimProviderAppsPanel.test.tsx:22`, replace
  `https://chetto1983.github.io/aura-connect/google/callback/` with
  `https://chetto1983-aura-connect.pages.dev/google/callback/`.

- [x] **Step 8: Move the operator guide.** In `docs/mcp-manager.md`:
  - lines 250, 278 and 280: replace `https://chetto1983.github.io/aura-connect/` with
    `https://chetto1983-aura-connect.pages.dev/`, so each address keeps its path;
  - line 259: replace `([chetto1983/aura-connect](https://github.com/chetto1983/aura-connect))`
    with `([Chetto1983/aura-connect](https://gitlab.com/Chetto1983/aura-connect), served by
    Cloudflare Pages)`.

- [x] **Step 9: Gate 2 on the touched packages, in WSL**

```bash
export PATH="$HOME/.local/bin:$HOME/go/bin:$PATH"
cd /mnt/d/Aura
go vet ./internal/mcp/ ./internal/agui/ ./cmd/aura/
CGO_ENABLED=1 go test -race ./internal/mcp/ ./internal/agui/
CGO_ENABLED=1 go test -race -run 'TestPIMSidecarIsToldTheGoogleRelay|TestCloudflaredPackagingBoundary|TestComposeAuraReadinessHealthcheckContract|TestComposeOnlyAdminAPIHint' ./cmd/aura/
```

Expected: no vet output, and `ok` for each package.

- [x] **Step 10: Web checks on the two touched files, from `web/` on Windows**

```bash
node node_modules/vitest/vitest.mjs run src/governance/__tests__/CalendarConnect.test.tsx src/governance/__tests__/PimProviderAppsPanel.test.tsx
node node_modules/prettier/bin/prettier.cjs --check src/governance/__tests__/CalendarConnect.test.tsx src/governance/__tests__/PimProviderAppsPanel.test.tsx
```

Expected: both files pass, and prettier reports nothing to fix.

- [x] **Step 11: The old host and the squatter are gone**

```bash
cd /mnt/d/Aura
git grep -n "chetto1983.github.io" -- ':!docs/superpowers/plans' ':!docs/superpowers/specs' ':!docs/superpowers/verification' ':!prd.md' ':!.planning'
git grep -n -E "(^|[^-])aura-connect\.pages\.dev" -- ':!docs/superpowers/specs/2026-10-13-aura-connect-cloudflare-pages-design.md' ':!docs/superpowers/plans/2026-10-13-aura-connect-cloudflare-pages.md'
```

Expected: no output from either grep. `prd.md` §13 is edited in the closing task.

- [x] **Step 12: Commit the change**

```bash
LEFTHOOK_BIN=$HOME/go/bin/lefthook git -c core.hooksPath=.git/hooks commit -F <msg> -- \
  cmd/aura/pim_relay_artifacts_test.go compose.yaml internal/agui/connect_pim_providers_api.go \
  internal/mcp/oauth_cimd.go web/src/governance/__tests__/CalendarConnect.test.tsx \
  web/src/governance/__tests__/PimProviderAppsPanel.test.tsx docs/mcp-manager.md
```

Message:

```
fix(oauth): send Google and CIMD consents through Cloudflare Pages

aura-connect's addresses on chetto1983.github.io answer 404 since the
GitHub account was flagged on 2026-10-11, so no install could connect a
Google account or a CIMD-only MCP server. They now live on
chetto1983-aura-connect.pages.dev.

The PIM sidecar builds Google's consent URL with its own relay address,
which the admin panel's constant only displays. compose.yaml now hands
the sidecar that constant, and TestPIMSidecarIsToldTheGoogleRelay keeps
the two equal.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
```

- [x] **Step 13: Regenerate the payload manifest and commit it**

```bash
make payload-manifest
git diff --stat scripts/payload_manifest.txt
LEFTHOOK_BIN=$HOME/go/bin/lefthook git -c core.hooksPath=.git/hooks commit \
  -m "chore(payload): record compose.yaml with the PIM sidecar's Google relay" \
  -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>" -- scripts/payload_manifest.txt
```

Expected: the diff changes exactly one line, `compose.yaml`'s hash.

---

### Task 4: the aura-pim-mcp fork names the new address

**Files (in `$PIM`):**
- Modify: `src/CalendarMcp.Core/Configuration/CalendarMcpConfiguration.cs:36`
- Modify: `src/CalendarMcp.Tests/Admin/GoogleOAuthRelayTests.cs:13`
- Modify: `src/CalendarMcp.HttpServer/Admin/GoogleOAuthRelay.cs:10`
- Modify: `docs/GOOGLE-SETUP.md:181`
- Modify: `AURA-FORK.md:54`

**Interfaces:**
- Consumes: the base from Task 2.
- Produces: a `main` commit on the fork, which the bridge plan's fork build packages.

- [x] **Step 1: Move the default and its mentions**
  - `CalendarMcpConfiguration.cs:36`: the default becomes
    `"https://chetto1983-aura-connect.pages.dev/google/callback/"`.
  - `GoogleOAuthRelayTests.cs:13`: `Relay` becomes the same string.
  - `GoogleOAuthRelay.cs:10`: `lives in github.com/chetto1983/aura-connect` →
    `lives in gitlab.com/Chetto1983/aura-connect, served by Cloudflare Pages`.
  - `docs/GOOGLE-SETUP.md:181`: the URI becomes the new one.
  - `AURA-FORK.md:54`: `(github.com/chetto1983/aura-connect)` →
    `(gitlab.com/Chetto1983/aura-connect, on Cloudflare Pages)`.

  Then check:

```bash
cd "$PIM" && grep -rn "chetto1983.github.io\|github.com/chetto1983/aura-connect" . --exclude-dir=.git
```

Expected: no output.

- [x] **Step 2: Run the fork's tests in the SDK container**

```bash
cd "$PIM"
docker run --rm -v "$PWD":/src -w /src mcr.microsoft.com/dotnet/sdk:10.0 \
  dotnet test src/CalendarMcp.Tests --nologo -v q
```

Expected: `Passed!`, with 0 failed.

- [x] **Step 3: Commit on `main` and push to GitHub**

```bash
cd "$PIM"
git add -A src/CalendarMcp.Core/Configuration/CalendarMcpConfiguration.cs \
  src/CalendarMcp.Tests/Admin/GoogleOAuthRelayTests.cs src/CalendarMcp.HttpServer/Admin/GoogleOAuthRelay.cs \
  docs/GOOGLE-SETUP.md AURA-FORK.md
git commit -m "Default the Google relay to aura-connect on Cloudflare Pages

The github.io relay answers 404 since GitHub flagged the account on
2026-10-11. Aura sets this through
CALENDAR_MCP_CalendarMcp__GoogleOAuthRelayUrl; the default moves too, so
a standalone run names no dead page.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
git push origin main
```

Expected: the push names one new commit on `main`.

---

### Closing task: the lab VM, the ledger, the PRD, the push

**Files:**
- Create: `docs/superpowers/verification/2026-10-13-aura-connect-cloudflare-pages.md`
- Modify: `prd.md` §13 (the CIMD paragraph around line 2461, and the relay sentence around line
  2419)

- [x] **Step 1: Deploy to the lab VM through the overlay.** Do this first:
  - Check `auth.log` and the running overlay first, as every VM deploy does.
  - Build the `aura-linux` binary in WSL from a clean export of the Task 3 commit.
  - Build a child image `FROM` the running `aura:lab-gateway-clarity-20261013-6` with
    `COPY aura-linux`, tagged `aura:lab-aura-connect-20261013-1`.

  The new `/opt/aura/<tag>/compose.lab.yaml` keeps the current overlay's `aura` block, with the
  image switched to the new tag. It adds:

```yaml
  aura-pim-mcp:
    environment:
      CALENDAR_MCP_CalendarMcp__GoogleOAuthRelayUrl: https://chetto1983-aura-connect.pages.dev/google/callback/
```

  Recreate `aura` and `aura-pim-mcp` with the overlay. Expected:
  - both are `healthy`;
  - `docker exec aura-pim-mcp printenv CALENDAR_MCP_CalendarMcp__GoogleOAuthRelayUrl` prints
    the new address;
  - the binary's sha256 inside the container equals the one built.

- [x] **Step 2: Read the Google consent URL before any login.** With the operator's cockpit
  session, call `GET /api/connect/pim/accounts/{id}/google/start` for a test account.

  Expected: `redirectUri` is `https://chetto1983-aura-connect.pages.dev/google/callback/`, and
  `authUrl` carries `redirect_uri=https%3A%2F%2Fchetto1983-aura-connect.pages.dev%2Fgoogle%2Fcallback%2F`.

- [x] **Step 3: Google, end to end (the operator drives).**
  1. The operator adds the redirect URI to the Google Cloud Web client.
  2. The operator connects the account from the cockpit.

  Read back through the integrations proxy: `/admin/accounts` lists the account as linked.

- [x] **Step 4: CIMD, end to end (the operator drives).**
  1. Find ElevenLabs' registry name with `GET /api/governance/mcp`. The built-in catalogue has
     no entry for it, so it is whatever name it was installed under. If it is not installed,
     the operator installs it from its URL, and the ledger says so.
  2. Call `POST /api/governance/mcp/{name}/authorization`. It answers `flowId`, and an
     `authorizationUrl` when the URL is ready. Otherwise poll
     `GET /api/governance/mcp/authorization/flow/{flowId}` until `authorizationUrl` appears
     (`internal/agui/mcp_oauth_api.go:228`).
  3. Check that URL: its `client_id` equals the new metadata URL, and its `redirect_uri`
     equals the new relay.
  4. The operator opens it and consents.
  5. Read back:
     `select server_name, updated_at from aura.identity_mcp_oauth where server_name = '<name>'`
     shows an `updated_at` after the start of step 4.

- [x] **Step 5: Write the ledger.**
  `docs/superpowers/verification/2026-10-13-aura-connect-cloudflare-pages.md` records:
  - what was measured: Task 2's output, the VM image and binary, steps 2-4;
  - what this does not establish: the spec's list, and that no CI ran.

- [x] **Step 6: Amend PRD §13.** The sentence naming the relay page and the CIMD paragraph name
  `https://chetto1983-aura-connect.pages.dev/…`. They cite the ledger and the date, and say
  that the move is permanent.

- [x] **Step 7: Commit the ledger and the PRD; push Aura to both remotes.** The message:

```
docs(verification): record aura-connect on Cloudflare Pages

The three shared OAuth addresses answer from
chetto1983-aura-connect.pages.dev. On the lab VM, Google's consent URL
named the new relay before any login, a Google account connected, and
ElevenLabs signed in through the new CIMD. PRD §13 records the address
and that the move is permanent.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
```

```bash
LEFTHOOK_BIN=$HOME/go/bin/lefthook git -c core.hooksPath=.git/hooks commit -F <file holding the message above> -- \
  docs/superpowers/verification/2026-10-13-aura-connect-cloudflare-pages.md prd.md
LEFTHOOK_BIN=$HOME/go/bin/lefthook git -c core.hooksPath=.git/hooks push origin master
LEFTHOOK_BIN=$HOME/go/bin/lefthook git -c core.hooksPath=.git/hooks push gitlab master
```

Expected: every pre-push gate is green on `origin`, and `gitlab` receives the same commits.
