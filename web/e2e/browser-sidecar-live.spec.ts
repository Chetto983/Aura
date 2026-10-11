import { randomUUID } from 'node:crypto';
import { expect, test, type Page } from '@playwright/test';
import { gotoAuthenticated } from './auth';
import { runTurn } from './identities';
import { createConversation, sameOriginFetch } from './live';

const enabled = process.env.AURA_E2E_BROWSER_SIDECAR === '1';
test.use({ trace: 'off', video: 'off', screenshot: 'off' });
const browserPrefix = 'browser__agent_browser_';
type Frame = Readonly<Record<string, unknown>>;

interface ToolEvidence {
  readonly name: string;
  readonly args: Record<string, unknown>;
  readonly result: string;
}

interface PendingApproval {
  readonly token: string;
  readonly conversation_id: string;
  readonly presentation?: {
    readonly params: { readonly tool?: string; readonly risk?: string };
  };
}

function toolEvidence(frames: readonly Frame[]): readonly ToolEvidence[] {
  return frames.flatMap((frame) => {
    if (frame.type !== 'TOOL_CALL_START' || typeof frame.toolCallName !== 'string') return [];
    const argsText = frames
      .filter((part) => part.type === 'TOOL_CALL_ARGS' && part.toolCallId === frame.toolCallId)
      .map(({ delta }) => (typeof delta === 'string' ? delta : ''))
      .join('');
    const content = frames.find(
      (part) => part.type === 'TOOL_CALL_RESULT' && part.toolCallId === frame.toolCallId,
    )?.content;
    return [
      {
        name: frame.toolCallName,
        args: JSON.parse(argsText || '{}') as Record<string, unknown>,
        result: typeof content === 'string' ? content : '',
      },
    ];
  });
}

function requireSuccessfulTurn(frames: readonly Frame[]) {
  expect(frames.some((frame) => frame.type === 'RUN_ERROR')).toBe(false);
  expect(
    frames.some(
      (frame) =>
        frame.type === 'RUN_FINISHED' &&
        (frame.outcome as { type?: string } | undefined)?.type === 'success',
    ),
  ).toBe(true);
}

async function withConversation(
  page: Page,
  title: string,
  operation: (id: string) => Promise<void>,
) {
  const id = await createConversation(page, title);
  try {
    await page.goto(`/c/${id}`, { waitUntil: 'domcontentloaded' });
    await operation(id);
  } finally {
    const response = await sameOriginFetch(page, `/api/conversations/${id}`, { method: 'DELETE' });
    expect([200, 204]).toContain(response.status);
  }
}

async function authenticateLive(page: Page) {
  expect(process.env.AURA_E2E_ORIGIN, 'set AURA_E2E_ORIGIN to the deployed appliance').toBeTruthy();
  await gotoAuthenticated(page, '/', { isolatedSession: true });
  await page.unroute('**/api/onboarding/status');
  await page.unroute('**/healthz');
  await page.unroute('**/readyz');
  const expectedIdentity = process.env.AURA_E2E_BROWSER_IDENTITY_ID;
  if (expectedIdentity) {
    const response = await sameOriginFetch(page, '/api/me');
    expect(response.status).toBe(200);
    expect((JSON.parse(response.text) as { identity_id?: string }).identity_id).toBe(
      expectedIdentity,
    );
  }
}

function downloadFixture() {
  const url = process.env.AURA_E2E_BROWSER_FIXTURE_URL;
  const sha256 = process.env.AURA_E2E_BROWSER_DOWNLOAD_SHA256;
  const name = process.env.AURA_E2E_BROWSER_DOWNLOAD_NAME ?? 'browser-sidecar.bin';
  if (!url)
    throw new Error(
      'set AURA_E2E_BROWSER_FIXTURE_URL: a public HTTP(S) fixture page with a #download link to an 8 MiB attachment served over 8 s',
    );
  if (!sha256)
    throw new Error(
      'set AURA_E2E_BROWSER_DOWNLOAD_SHA256: the independently computed SHA-256 of the fixture attachment',
    );
  expect(url).toMatch(/^https?:\/\//);
  expect(sha256).toMatch(/^[a-f0-9]{64}$/);
  expect(name).toMatch(/^[A-Za-z0-9_-]+\.[A-Za-z0-9]+$/);
  return { url, sha256, name };
}

const isolationProbe = `python3 - <<'PY'
import glob,json,os
files=[]
for root,dirs,names in os.walk('/workspace',followlinks=False):
 for name in names:
  if name in ('Cookies','Login Data') or ('agent-browser' in root and name.endswith('.sock')): files.append(name)
processes=0
for path in glob.glob('/proc/[0-9]*/cmdline'):
 try:
  cmd=open(path,'rb').read()
  if b'chr'+b'om' in cmd: processes+=1
 except (FileNotFoundError,PermissionError,ProcessLookupError): pass
listeners=0
for path in ('/proc/net/tcp','/proc/net/tcp6'):
 for line in open(path).readlines()[1:]:
  if line.split()[3]=='0A': listeners+=1
print('AURA_ISOLATION='+json.dumps(dict(cookie_or_socket_files=len(files),key_exists=os.path.lexists('/run/aura/agent-browser.key'),browser_mount='aura-browser-' in open('/proc/self/mountinfo').read(),browser_processes=processes,listeners=listeners,docker_socket=os.path.exists('/var/run/docker.sock'))))
PY`;

test.describe('browser sidecar live acceptance', () => {
  test.skip(
    !enabled,
    'set AURA_E2E_BROWSER_SIDECAR=1 after the sidecar implementation is deployed',
  );
  test.describe.configure({ mode: 'serial' });

  test('real agent opens a live view, downloads a complete file, and cannot see browser credentials from shell', async ({
    page,
  }, testInfo) => {
    test.setTimeout(900_000);
    const fixture = downloadFixture();
    const session = `e2e-${randomUUID().slice(0, 8)}`;
    await authenticateLive(page);
    await withConversation(page, `Browser sidecar ${session}`, async () => {
      try {
        const opened = await runTurn(
          page,
          `Use browser__agent_browser_open with session ${session} and url ${fixture.url}, then browser__agent_browser_snapshot on the same session. Leave this browser open. Do not use shell_exec, eval, web_fetch or web_search.`,
        );
        requireSuccessfulTurn(opened);
        expect(
          toolEvidence(opened).some(
            (call) =>
              call.name === `${browserPrefix}open` &&
              call.args.session === session &&
              call.args.url === fixture.url,
          ),
        ).toBe(true);
        const live = page.getByRole('region', { name: 'Live browser' });
        await expect(live).toBeVisible({ timeout: 30_000 });
        await expect(live.getByAltText('The page open in your sandbox browser')).toBeVisible({
          timeout: 30_000,
        });

        // A second viewer takes over the first stream, so close it before the next agent turn.
        const viewer = await page.context().newPage();
        try {
          await viewer.goto(`/browser/${session}`, { waitUntil: 'domcontentloaded' });
          await expect(viewer.getByAltText('The page open in your sandbox browser')).toBeVisible({
            timeout: 30_000,
          });
        } finally {
          await viewer.close();
        }

        const clicked = await runTurn(
          page,
          `In browser session ${session}, use browser__agent_browser_click with selector #download exactly once. Then call browser__agent_browser_wait_ms with ms 10000 in the same session, and browser__agent_browser_get_title to collect completed downloads. Do not download with shell, fetch, eval, curl, wget or Python. Keep the session open.`,
        );
        requireSuccessfulTurn(clicked);
        const downloadCalls = toolEvidence(clicked);
        expect(
          downloadCalls.some(
            (call) => call.name === `${browserPrefix}click` && call.args.selector === '#download',
          ),
        ).toBe(true);
        const completed = downloadCalls
          .filter((call) => call.name.startsWith(browserPrefix))
          .map((call) => call.result)
          .join('\n');
        expect(completed).toMatch(/Download ready: \/workspace\/downloads\//);
        const clickResult = downloadCalls.find(
          (call) => call.name === `${browserPrefix}click`,
        )?.result;
        expect(clickResult).toContain('Download in progress:');
        expect(clickResult).not.toContain('Download ready:');
        const path = /Download ready: (\/workspace\/downloads\/[A-Za-z0-9_.-]+)/.exec(
          completed,
        )?.[1];
        expect(path, 'download footer names the transferred attachment').toBeTruthy();

        const checked = await runTurn(
          page,
          `Use shell_exec to execute the following read-only Python probe verbatim. Do not read or print any cookie, login database or key contents. Then use shell_exec to run: python3 -c 'import hashlib,os; p=${JSON.stringify(path)}; print("AURA_DOWNLOAD="+str(os.path.getsize(p))+":"+hashlib.sha256(open(p,"rb").read()).hexdigest())'. Report the outputs. Probe:\n${isolationProbe}`,
        );
        requireSuccessfulTurn(checked);
        const shellCalls = toolEvidence(checked).filter((call) => call.name === 'shell_exec');
        expect(shellCalls.length).toBeGreaterThanOrEqual(1);
        const shellResults = shellCalls.map((call) => call.result).join('\n');
        const isolation = /AURA_ISOLATION=(\{[^\n]+\})/.exec(shellResults)?.[1];
        if (!isolation) throw new Error('the read-only isolation probe did not execute');
        const isolationReport: unknown = JSON.parse(isolation);
        expect(isolationReport).toEqual({
          cookie_or_socket_files: 0,
          key_exists: false,
          browser_mount: false,
          browser_processes: 0,
          listeners: 0,
          docker_socket: false,
        });
        expect(shellResults).toContain(`AURA_DOWNLOAD=8388608:${fixture.sha256}`);
        await testInfo.attach('browser-sidecar-summary.json', {
          body: Buffer.from(
            JSON.stringify({
              session,
              liveView: true,
              downloadBytes: 8388608,
              downloadSHA256: fixture.sha256,
              isolation: isolationReport,
            }),
          ),
          contentType: 'application/json',
        });
      } finally {
        // This closes only the newly named session, never the operator's other browsers.
        await runTurn(
          page,
          `Use browser__agent_browser_close with session ${session}, all false. Do nothing else.`,
        );
      }
    });
  });

  test('real browser tool calls reject file URLs and screenshot filesystem destinations', async ({
    page,
  }) => {
    test.setTimeout(600_000);
    await authenticateLive(page);
    await withConversation(page, `Browser negative probes ${randomUUID()}`, async () => {
      for (const probe of [
        { tool: `${browserPrefix}open`, args: { url: 'file:///tmp/aura-e2e-nonexistent-probe' } },
        { tool: `${browserPrefix}open`, args: { url: 'http://127.0.0.1:1/aura-e2e-probe' } },
        { tool: `${browserPrefix}open`, args: { url: 'about:blank', restore: true } },
        {
          tool: `${browserPrefix}screenshot`,
          args: { screenshotDir: '/tmp/aura-e2e-forbidden-output' },
        },
        {
          tool: `${browserPrefix}screenshot`,
          args: { path: '/tmp/aura-e2e-forbidden-output.png' },
        },
      ]) {
        const frames = await runTurn(
          page,
          `Security acceptance: call ${probe.tool} exactly once with exactly these arguments: ${JSON.stringify(probe.args)}. The path is deliberately harmless. Report the tool refusal. Do not use alternate tools or retry.`,
        );
        requireSuccessfulTurn(frames);
        const evidence = toolEvidence(frames).find(
          (call) =>
            call.name === probe.tool &&
            Object.entries(probe.args).every(([key, value]) => call.args[key] === value),
        );
        expect(evidence, 'model actually attempted the forbidden argument').toBeDefined();
        expect(evidence?.result).toMatch(
          /not allowed|forbidden|refus|unsupported|public HTTP|extraArgs|argument/i,
        );
      }
    });
  });

  test('eval is withheld behind a destructive approval without reading any credentials', async ({
    page,
  }) => {
    test.setTimeout(300_000);
    await authenticateLive(page);
    await withConversation(page, `Browser eval approval ${randomUUID()}`, async (id) => {
      const marker = `AURA_EVAL_EXECUTED_${randomUUID()}`;
      const frames = await runTurn(
        page,
        `Call browser__agent_browser_eval once with script ${JSON.stringify(JSON.stringify(marker))}. Do not use other tools, do not request a standing grant. Stop when approval is required.`,
      );
      const approvals = await sameOriginFetch(page, '/api/approvals');
      expect(approvals.status).toBe(200);
      const pending = (JSON.parse(approvals.text) as PendingApproval[]).find(
        (item) =>
          item.conversation_id === id && item.presentation?.params.tool === `${browserPrefix}eval`,
      );
      expect(pending, 'eval reached the operator approval gate').toBeDefined();
      try {
        expect(pending?.presentation?.params.risk).toBe('destructive');
        expect(
          toolEvidence(frames)
            .filter((call) => call.name === `${browserPrefix}eval`)
            .some((call) => call.result.includes(marker)),
        ).toBe(false);
      } finally {
        if (pending !== undefined) {
          const canceled = await sameOriginFetch(page, `/api/approvals/${pending.token}/resolve`, {
            method: 'POST',
            body: JSON.stringify({ action: 'cancel' }),
          });
          expect(canceled.status).toBe(200);
        }
      }
    });
  });
});
