import { mkdirSync, readFileSync, writeFileSync } from 'node:fs';
import { basename, join } from 'node:path';
import { expect, test, type Page } from '@playwright/test';
import { gotoAuthenticated } from './auth';
import { createConversation, sameOriginFetch, streamFrames } from './live';

// craft-live.spec.ts gives the real agent, on a live stack, one real file per Craft MCP server and
// one task it can only do through that server (docs/superpowers/specs/2026-10-18-craft-mcp-design.md).
// The file goes in the way the cockpit attaches one, the turn runs over /agent/run, and the result
// must come back through send_file as a new asset of the conversation. The output and a screenshot
// of the conversation land in AURA_E2E_CRAFT_OUT, where the verification checks them outside the
// servers.
//
//   AURA_E2E_CRAFT=1
//   AURA_E2E_CRAFT_FILES=<folder with the real files, spikes/craft-mcp/README.md>
//   AURA_E2E_CRAFT_OUT=<folder for the outputs and screenshots>
//   AURA_E2E_CRAFT_ONLY=<server>   (optional: one case)

const live = process.env.AURA_E2E_CRAFT === '1';
const filesDir = process.env.AURA_E2E_CRAFT_FILES ?? '';
const outDir = process.env.AURA_E2E_CRAFT_OUT ?? '';
const only = process.env.AURA_E2E_CRAFT_ONLY ?? '';

interface CraftCase {
  readonly server: string;
  readonly file: string;
  readonly mime: string;
  readonly output: string;
  readonly prompt: string;
}

const cases: readonly CraftCase[] = [
  {
    server: 'gridcraft',
    file: 'fcffginzu.xls',
    mime: 'application/vnd.ms-excel',
    output: '.xlsx',
    prompt:
      'Ti ho allegato fcffginzu.xls, il modello di valutazione di Eli Lilly. Alza il tasso risk-free da 4,5% a 5% e dimmi come cambia il valore per azione. Salva il risultato in xlsx con un nome nuovo e mandamelo.',
  },
  {
    server: 'pdfcraft',
    file: 'anac-nota-illustrativa.pdf',
    mime: 'application/pdf',
    output: '.pdf',
    prompt:
      'Ti ho allegato anac-nota-illustrativa.pdf. Oscura per davvero ogni occorrenza della parola "ANAC" (non un rettangolo sopra: il testo deve sparire) e mandami il PDF oscurato.',
  },
  {
    server: 'wordcraft',
    file: 'bando-tipo-anac-2.docx',
    mime: 'application/vnd.openxmlformats-officedocument.wordprocessingml.document',
    output: '.docx',
    prompt:
      'Ti ho allegato bando-tipo-anac-2.docx. Compila gli estremi della determina a contrarre: numero 125/2026 del 12 ottobre 2026, mantenendo la formattazione del documento. Salvalo in docx con un nome nuovo e mandamelo.',
  },
  {
    server: 'deckcraft',
    file: 'HWC20_PPT_IT.pptx',
    mime: 'application/vnd.openxmlformats-officedocument.presentationml.presentation',
    output: '.pptx',
    prompt:
      'Ti ho allegato HWC20_PPT_IT.pptx. Aggiungi in fondo una slide con il titolo "Domande?" e il testo "Grazie per l\'attenzione", salva la presentazione in pptx con un nome nuovo e mandamela.',
  },
  {
    server: 'photocraft',
    file: 'ponte-vecchio-dusk.jpg',
    mime: 'image/jpeg',
    output: '.jpg',
    prompt:
      "Ti ho allegato ponte-vecchio-dusk.jpg. Ridimensionala a 2400 pixel di larghezza, schiariscila un po', esportala in JPEG mantenendo i dati EXIF e mandamela.",
  },
  {
    server: 'filmcraft',
    file: 'sintel-trailer-1080p.mp4',
    mime: 'video/mp4',
    output: '.mp4',
    prompt:
      'Ti ho allegato sintel-trailer-1080p.mp4. Tieni solo i primi 10 secondi ed esportali in un mp4 a 720p, con qualità costante (CRF 23) così il file resta piccolo, e mandamelo.',
  },
];

interface AssetRow {
  readonly id: string;
  readonly file_name: string;
  readonly status: string;
}

/** upload attaches a file to the conversation through the cockpit's own presign, PUT, finalize. */
async function upload(page: Page, threadID: string, file: string, mime: string): Promise<string> {
  const bytes = readFileSync(join(filesDir, file)).toString('base64');
  return page.evaluate(
    async ({ thread, name, type, data }) => {
      const body = Uint8Array.from(atob(data), (c) => c.charCodeAt(0));
      const presign = await fetch('/api/assets/presign', {
        method: 'POST',
        credentials: 'same-origin',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          thread_id: thread,
          file_name: name,
          mime_type: type,
          size_bytes: body.byteLength,
        }),
      });
      if (!presign.ok)
        throw new Error(`presign ${name}: HTTP ${String(presign.status)} ${await presign.text()}`);
      const presigned = (await presign.json()) as {
        readonly asset: { readonly id: string };
        readonly upload: {
          readonly upload_url: string;
          readonly method?: string;
          readonly required_headers?: Record<string, string>;
        };
      };
      const put = await fetch(presigned.upload.upload_url, {
        method: presigned.upload.method ?? 'PUT',
        headers: presigned.upload.required_headers ?? {},
        body,
      });
      if (!put.ok) throw new Error(`upload ${name}: HTTP ${String(put.status)}`);
      const finalize = await fetch(
        `/api/assets/${encodeURIComponent(presigned.asset.id)}/finalize`,
        {
          method: 'POST',
          credentials: 'same-origin',
        },
      );
      if (!finalize.ok)
        throw new Error(
          `finalize ${name}: HTTP ${String(finalize.status)} ${await finalize.text()}`,
        );
      return presigned.asset.id;
    },
    { thread: threadID, name: file, type: mime, data: bytes },
  );
}

async function threadAssets(page: Page, threadID: string): Promise<readonly AssetRow[]> {
  const response = await sameOriginFetch(
    page,
    `/api/assets?thread_id=${encodeURIComponent(threadID)}`,
  );
  expect(response.status, response.text).toBe(200);
  const parsed = JSON.parse(response.text) as
    { readonly assets?: readonly AssetRow[] } | readonly AssetRow[];
  return Array.isArray(parsed)
    ? parsed
    : ((parsed as { readonly assets?: readonly AssetRow[] }).assets ?? []);
}

async function download(page: Page, assetID: string): Promise<Buffer> {
  const data = await page.evaluate(async (id) => {
    const response = await fetch(`/api/assets/${encodeURIComponent(id)}/download`, {
      credentials: 'same-origin',
    });
    if (!response.ok) throw new Error(`download ${id}: HTTP ${String(response.status)}`);
    const bytes = new Uint8Array(await response.arrayBuffer());
    let binary = '';
    for (let i = 0; i < bytes.length; i += 0x8000)
      binary += String.fromCharCode(...bytes.subarray(i, i + 0x8000));
    return btoa(binary);
  }, assetID);
  return Buffer.from(data, 'base64');
}

test.describe('Craft MCP servers, live, with the real agent', () => {
  test.skip(
    !live,
    'set AURA_E2E_CRAFT=1 with AURA_E2E_CRAFT_FILES and AURA_E2E_CRAFT_OUT against a live stack',
  );

  for (const craft of cases.filter((c) => only === '' || c.server === only)) {
    test(`${craft.server}: ${craft.file}`, async ({ page }, testInfo) => {
      test.setTimeout(45 * 60_000);
      expect(filesDir, 'AURA_E2E_CRAFT_FILES').not.toBe('');
      expect(outDir, 'AURA_E2E_CRAFT_OUT').not.toBe('');
      mkdirSync(outDir, { recursive: true });
      await gotoAuthenticated(page, '/');
      const threadID = await createConversation(page, `Craft live: ${craft.server}`);
      const uploaded = await upload(page, threadID, craft.file, craft.mime);

      const started = Date.now();
      const response = await sameOriginFetch(page, '/agent/run', {
        method: 'POST',
        body: JSON.stringify({
          threadId: threadID,
          messages: [{ id: `user-${String(started)}`, role: 'user', content: craft.prompt }],
          aura: { attachment_ids: [uploaded] },
        }),
      });
      const seconds = Math.round((Date.now() - started) / 1000);
      expect(response.status, response.text.slice(0, 2_000)).toBe(200);
      const frames = streamFrames(response.text);
      const tools = frames
        .filter((frame) => frame.type === 'TOOL_CALL_START')
        .map((frame) => (typeof frame.toolCallName === 'string' ? frame.toolCallName : ''));
      const answer = frames
        .filter((frame) => frame.type === 'TEXT_MESSAGE_CONTENT')
        .map((frame) => (typeof frame.delta === 'string' ? frame.delta : ''))
        .join('');
      const errors = frames.filter((frame) => frame.type === 'RUN_ERROR');
      writeFileSync(
        join(outDir, `${craft.server}.turn.json`),
        JSON.stringify({ threadID, seconds, tools, answer, errors }, null, 1),
      );
      expect(errors, JSON.stringify(errors.slice(0, 3))).toHaveLength(0);
      expect(frames.some((frame) => frame.type === 'RUN_FINISHED')).toBe(true);
      expect(
        tools.some((name) => name.startsWith(`${craft.server}__`)),
        tools.join(', '),
      ).toBe(true);
      expect(tools, tools.join(', ')).toContain('send_file');

      const delivered = (await threadAssets(page, threadID)).filter(
        (asset) => asset.id !== uploaded && asset.file_name.toLowerCase().endsWith(craft.output),
      );
      expect(
        delivered.map((asset) => asset.file_name),
        'send_file delivered the result',
      ).not.toHaveLength(0);
      const result = delivered.at(-1);
      if (result === undefined) throw new Error('send_file delivered nothing');
      const bytes = await download(page, result.id);
      expect(bytes.byteLength).toBeGreaterThan(0);
      const saved = join(outDir, `${craft.server}-${basename(result.file_name)}`);
      writeFileSync(saved, bytes);
      await testInfo.attach(result.file_name, { path: saved });

      await page.goto(`/c/${threadID}`, { waitUntil: 'domcontentloaded' });
      await expect(page.getByText(result.file_name, { exact: true }).first()).toBeVisible({
        timeout: 30_000,
      });
      const shot = join(outDir, `${craft.server}-cockpit.png`);
      await page.screenshot({ path: shot, fullPage: true });
      await testInfo.attach(`${craft.server}-cockpit.png`, {
        path: shot,
        contentType: 'image/png',
      });
    });
  }
});
