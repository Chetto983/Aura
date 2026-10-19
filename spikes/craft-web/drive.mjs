// Drives one Craft app through the spike host page in headless Chromium: start, the file sent
// on <app>:ready, the steps given, and a report of every message, policy violation and save.
//
//   node spikes/craft-web/drive.mjs --app pdfcraft --file /fixtures/documents/Appunti.pdf \
//     --steps 'wait:4000,shot:opened,click:0.5:0.5,key:Control+s,save,shot:saved' --out <dir>
//
// Steps: wait:<ms>, shot:<name>, click:<x>:<y> (fractions of the frame), key:<chord>,
// type:<text>, save (wait for an <app>:save), dirty (wait for <app>:dirty true).
import { createRequire } from 'node:module';
import { mkdirSync, writeFileSync } from 'node:fs';
import { join } from 'node:path';
import { parseArgs } from 'node:util';

const require = createRequire(new URL('../../web/package.json', import.meta.url));
const { chromium } = require('playwright');

const { values: args } = parseArgs({
  options: {
    app: { type: 'string' },
    file: { type: 'string' },
    flags: { type: 'string', default: '' },
    steps: { type: 'string', default: '' },
    out: { type: 'string' },
    base: { type: 'string', default: 'http://127.0.0.1:8790' },
    ready: { type: 'string', default: '90000' },
    locale: { type: 'string', default: 'en-US' },
  },
});
mkdirSync(args.out, { recursive: true });

const browser = await chromium.launch({
  executablePath: '/opt/pw-browsers/chromium',
  headless: true,
  args: ['--use-angle=swiftshader', '--enable-unsafe-swiftshader', '--ignore-gpu-blocklist'],
});
const context = await browser.newContext({ viewport: { width: 1440, height: 900 }, locale: args.locale });
await context.addInitScript(() => {
  document.addEventListener('securitypolicyviolation', (e) => {
    console.log(`CSP ${e.violatedDirective} ${e.blockedURI}`);
  });
});
const page = await context.newPage();
const consoleLines = [];
const requests = [];
// An error keeps its stack (a wasm panic names the function); other lines are cut short.
page.on('console', (m) => consoleLines.push(`[${m.type()}] ${m.text()}`.slice(0, m.type() === 'error' ? 4000 : 400)));
page.on('pageerror', (e) => consoleLines.push(`[pageerror] ${e.message}`.slice(0, 4000)));
page.on('response', async (r) => {
  const h = r.headers();
  requests.push({
    url: r.url().replace(args.base, ''),
    status: r.status(),
    type: h['content-type'],
    encoding: h['content-encoding'],
    length: Number(h['content-length'] ?? 0) || undefined,
  });
});

const events = () => page.evaluate(() => window.__craft?.events ?? []);
const saves = () => page.evaluate(() => window.__craft?.saves ?? []);
async function until(predicate, timeout, what) {
  const deadline = Date.now() + timeout;
  while (Date.now() < deadline) {
    if (predicate(await events())) return true;
    await page.waitForTimeout(200);
  }
  throw new Error(`timed out waiting for ${what}`);
}

const query = new URLSearchParams({ app: args.app, ...(args.file ? { file: args.file } : {}), ...(args.flags ? { flags: args.flags } : {}) });
const started = Date.now();
await page.goto(`${args.base}/?${query}`);
const report = { app: args.app, file: args.file, flags: args.flags, steps: [] };
let failure;
try {
  await until((e) => e.some((m) => m.type === `${args.app}:ready` || m.type === `${args.app}:failed`), Number(args.ready), 'ready');
  report.readyMs = Date.now() - started;
  const frameBox = await page.locator('#app').boundingBox();
  let expectedSaves = 0;
  for (const step of args.steps.split(',').filter(Boolean)) {
    const [kind, ...rest] = step.split(':');
    const t = Date.now();
    if (kind === 'wait') await page.waitForTimeout(Number(rest[0]));
    else if (kind === 'shot') await page.screenshot({ path: join(args.out, `${args.app}-${rest[0]}.png`) });
    else if (kind === 'click') await page.mouse.click(frameBox.x + frameBox.width * Number(rest[0]), frameBox.y + frameBox.height * Number(rest[1]));
    else if (kind === 'dblclick') await page.mouse.dblclick(frameBox.x + frameBox.width * Number(rest[0]), frameBox.y + frameBox.height * Number(rest[1]));
    else if (kind === 'key') await page.keyboard.press(rest.join(':'));
    else if (kind === 'type') await page.keyboard.type(rest.join(':'), { delay: 30 });
    else if (kind === 'save') {
      // The nth save step waits for the nth save, which may have landed during an earlier wait.
      expectedSaves += 1;
      const deadline = Date.now() + 60000;
      while ((await saves()).length < expectedSaves) {
        if (Date.now() > deadline) throw new Error('timed out waiting for a save');
        await page.waitForTimeout(200);
      }
    } else if (kind === 'filmexport') {
      // FilmCraft's Export button, through its page API: an H.264 export of the active sequence.
      const frame = page.frames().find((f) => f.url().includes('/apps/filmcraft/'));
      report.export = await frame.evaluate(() =>
        window.filmcraft.execute('file.exportMedia', { path: '/exports/edited.mp4', format: 'h264', bitrateMode: 'crf', crf: 23 }),
      );
    } else if (kind === 'dirty') await until((e) => e.some((m) => m.type === `${args.app}:dirty` && m.dirty === true), 30000, 'dirty');
    else throw new Error(`unknown step ${step}`);
    report.steps.push({ step, ms: Date.now() - t });
  }
} catch (e) {
  failure = String(e);
  await page.screenshot({ path: join(args.out, `${args.app}-failure.png`) }).catch(() => {});
}
report.failure = failure;
report.events = await events();
report.saves = await saves();
report.requests = requests;
report.console = consoleLines;
writeFileSync(join(args.out, `${args.app}.json`), JSON.stringify(report, null, 2));
console.log(JSON.stringify({ app: report.app, readyMs: report.readyMs, failure, events: report.events, saves: report.saves }, null, 2));
await browser.close();
process.exit(failure ? 1 : 0);
