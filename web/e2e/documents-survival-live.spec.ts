import { expect, test, type Page } from '@playwright/test';
import { sameOriginFetch } from './live';
import {
  api,
  DocsMCP,
  documentsNamed,
  ensureLabel,
  openDocuments,
  type LiveRow,
} from './documentsLive';

// Acceptance 4 of docs/superpowers/specs/2026-10-18-document-fields-design.md, on the live stack:
// an operator value and its history survive a rename, a move and an edit of the bytes, which the
// ingest reads again, each made through the file manager's own routes, as the cockpit makes them.
// The same bytes uploaded again are left out: measured 2026-10-18, the ingest does not read them
// again, so the step only waited a scan for nothing. After each, the document is looked for three
// ways: its detail, the list filtered by the value (the projection on the indexed row), and the
// agent's search under the same filter (the projection on the passages), which must also show the
// current name.
//
//   AURA_E2E_ORIGIN=http://127.0.0.1:9080 AURA_E2E_LIVE_DOCUMENTS=1 \
//     npx playwright test e2e/documents-survival-live.spec.ts --project=chrome

const live = process.env.AURA_E2E_LIVE_DOCUMENTS === '1';

// The ingest scans the bucket every AURA_INGEST_INTERVAL_SEC, 60 s on the dev stack, so a file
// operation reaches the index within one scan and its processing.
const INDEXED_WITHIN_MS = 90_000;

interface Detail extends LiveRow {
  readonly indexed_at: string;
  readonly history: readonly { readonly action: string }[];
}

async function upload(page: Page, folder: string, name: string, text: string): Promise<void> {
  const status = await page.evaluate(
    async ({ url, fileName, body }) => {
      const form = new FormData();
      form.append('file', new File([body], fileName, { type: 'text/plain' }));
      const response = await fetch(url, { method: 'POST', body: form, credentials: 'same-origin' });
      return response.status;
    },
    { url: `/api/filemanager/upload?id=${encodeURIComponent(folder)}`, fileName: name, body: text },
  );
  expect(status, `upload ${folder}${name}`).toBe(200);
}

async function fileWrite(page: Page, method: string, path: string, body: unknown): Promise<void> {
  const response = await sameOriginFetch(page, path, { method, body: JSON.stringify(body) });
  expect(response.status, `${method} ${path}: ${response.text}`).toBe(200);
}

/** indexedAs waits until a document of that name is indexed under a key holding keyPart, and
 * reads its detail. */
async function indexedAs(page: Page, name: string, keyPart = '', newerThan = ''): Promise<Detail> {
  let found: Detail | undefined;
  await expect
    .poll(
      async () => {
        const row = (await documentsNamed(page, name)).find((r) => r.source_key.includes(keyPart));
        if (row === undefined) return false;
        found = await api<Detail>(page, 'GET', `/api/documents/${encodeURIComponent(row.id)}`);
        return found.indexed_at > newerThan;
      },
      { timeout: INDEXED_WITHIN_MS, intervals: [2_000] },
    )
    .toBe(true);
  if (found === undefined) throw new Error(`${name} never indexed`);
  return found;
}

test.describe('document fields survive what the cockpit does to a file, live', () => {
  test.skip(!live, 'set AURA_E2E_LIVE_DOCUMENTS=1 against a rebuilt Aura');

  test('rename, move and new bytes keep the values', async ({ page }, testInfo) => {
    test.setTimeout(10 * 60_000);
    const run = Date.now().toString(36);
    const folder = `/e2e-sopravvivenza-${run}/`;
    const first = `sopralluogo-${run}.txt`;
    const renamed = `sopralluogo-rinominato-${run}.txt`;
    const text = `Verbale di sopralluogo ${run}. Il cantiere di via Roma è stato ispezionato.`;
    await openDocuments(page);
    const typeId = await ensureLabel(page, 'type', `Sopralluogo ${run}`);
    const tagId = await ensureLabel(page, 'tag', `e2e-${run}`);
    const mcp = await DocsMCP.start();
    try {
      await fileWrite(page, 'POST', `/api/filemanager/files/${encodeURIComponent('/')}`, {
        name: folder.slice(1, -1),
        type: 'folder',
      });
      await upload(page, folder, first, text);
      let doc = await indexedAs(page, first);
      await api(page, 'PATCH', `/api/documents/${encodeURIComponent(doc.id)}`, {
        set: [
          { field: 'type', value_key: typeId },
          { field: 'document_date', value_key: '2026-10-18' },
          { field: 'tag', value_key: tagId },
        ],
        remove: [],
      });
      const held = (d: Detail) =>
        d.values
          .map((v) => `${v.field}=${v.value_key}`)
          .sort()
          .join(' ');
      doc = await api<Detail>(page, 'GET', `/api/documents/${encodeURIComponent(doc.id)}`);
      const values = held(doc);
      expect(values).toBe(`document_date=2026-10-18 tag=${tagId} type=${typeId}`);
      let events = doc.history.length;

      // A move or a rename reaches the index when the ingest reconciles the bucket: until then
      // the old key's row stays, holding no record. Each step waits for the index to converge
      // on the new key and records how long that took.
      const survives = async (name: string, step: string, keyPart = '') => {
        const started = Date.now();
        const detail = await indexedAs(page, name, keyPart);
        expect(held(detail), `${step}: the values`).toBe(values);
        expect(detail.history.length, `${step}: the history`).toBeGreaterThanOrEqual(events);
        events = detail.history.length;
        const filtered = `/api/documents?type=${encodeURIComponent(typeId)}&include_unreviewed=false&limit=5`;
        await expect
          .poll(
            async () =>
              (await api<{ rows: readonly LiveRow[] }>(page, 'GET', filtered)).rows.map(
                (r) => r.source_key,
              ),
            { timeout: INDEXED_WITHIN_MS, intervals: [2_000], message: `${step}: the list filter` },
          )
          .toEqual([detail.source_key]);
        testInfo.annotations.push({
          type: step,
          description: `the index held only the new key after ${String(Date.now() - started)} ms`,
        });
        await expect
          .poll(
            async () =>
              (
                await mcp.search({ query: 'verbale di sopralluogo', type: [`Sopralluogo ${run}`] })
              ).documents.map((d) => d.title),
            { timeout: INDEXED_WITHIN_MS, intervals: [2_000], message: `${step}: the search` },
          )
          .toEqual([name]);
        return detail;
      };

      await fileWrite(page, 'PUT', `/api/filemanager/files/${encodeURIComponent(folder + first)}`, {
        operation: 'rename',
        name: renamed,
      });
      doc = await survives(renamed, 'rename');
      expect(
        doc.history.some((e) => e.action === 'moved'),
        'rename: a moved event',
      ).toBe(true);

      await fileWrite(page, 'POST', `/api/filemanager/files/${encodeURIComponent(folder)}`, {
        name: 'archivio',
        type: 'folder',
      });
      await fileWrite(page, 'PUT', '/api/filemanager/files', {
        operation: 'move',
        ids: [folder + renamed],
        target: `${folder}archivio`,
      });
      doc = await survives(renamed, 'move', '/archivio/');
      expect(doc.source_key, 'move: the new key').toContain('/archivio/');

      await upload(page, `${folder}archivio/`, renamed, `${text} Aggiunta una nota a margine.`);
      await indexedAs(page, renamed, '', doc.indexed_at);
      await survives(renamed, 'new bytes');
    } finally {
      mcp.close();
      await sameOriginFetch(page, '/api/filemanager/files', {
        method: 'DELETE',
        body: JSON.stringify({ ids: [folder.slice(0, -1)] }),
      });
      for (const id of [typeId, tagId]) {
        await sameOriginFetch(page, `/api/documents/labels/${encodeURIComponent(id)}`, {
          method: 'DELETE',
        });
      }
    }
  });
});
