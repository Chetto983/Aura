import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { expect, test } from '@playwright/test';
import {
  documentsNamed,
  DocsMCP,
  ensureLabel,
  openDocuments,
  reviewAs,
  type SearchAnswer,
} from './documentsLive';

// Acceptance 2 of docs/superpowers/specs/2026-10-18-document-fields-design.md, on the live stack:
// the "which documents" set, scored with today's search and then with the filters each question
// implies, on the office library as the set declares it reviewed. The review goes through the
// documents API as the signed-in operator; the searches through the agent's own document_search.
//
//   AURA_E2E_ORIGIN=https://localhost AURA_E2E_LIVE_DOCUMENTS=1 \
//     npx playwright test e2e/documents-filter-eval-live.spec.ts --project=chrome

const live = process.env.AURA_E2E_LIVE_DOCUMENTS === '1';

interface LibraryEntry {
  readonly type: string;
  readonly counterparty?: string;
  readonly document_date?: string;
  readonly tags: readonly string[];
}

interface Question {
  readonly qid: string;
  readonly query: string;
  readonly filters: Record<string, unknown>;
  readonly gold_docs: readonly string[];
}

interface QuestionSet {
  readonly library: Readonly<Record<string, LibraryEntry>>;
  readonly questions: readonly Question[];
}

function readSet(): QuestionSet {
  const path = resolve(
    process.cwd(),
    '..',
    'scripts/fixtures/document_retrieval_eval/document_filter_questions.json',
  );
  return JSON.parse(readFileSync(path, 'utf8')) as QuestionSet;
}

function recall(gold: readonly string[], answer: SearchAnswer): number {
  const titles = new Set(answer.documents.map((d) => d.title));
  return gold.filter((name) => titles.has(name)).length / gold.length;
}

test.describe('the which-documents set, live', () => {
  test.skip(
    !live,
    'set AURA_E2E_LIVE_DOCUMENTS=1 against a rebuilt Aura holding the office corpus',
  );

  test('filters raise set recall over today search and lose no gold', async ({
    page,
  }, testInfo) => {
    test.setTimeout(20 * 60_000);
    const set = readSet();
    await openDocuments(page);

    const ids = new Map<string, string>();
    const labelId = async (kind: 'type' | 'counterparty' | 'tag', name: string) => {
      const key = `${kind}:${name}`;
      const known = ids.get(key);
      if (known !== undefined) return known;
      const id = await ensureLabel(page, kind, name);
      ids.set(key, id);
      return id;
    };
    for (const [fileName, entry] of Object.entries(set.library)) {
      const single: Record<string, string> = { type: await labelId('type', entry.type) };
      if (entry.counterparty !== undefined) {
        single.counterparty = await labelId('counterparty', entry.counterparty);
      }
      if (entry.document_date !== undefined) single.document_date = entry.document_date;
      const tags = await Promise.all(entry.tags.map((tag) => labelId('tag', tag)));
      const docs = await documentsNamed(page, fileName);
      expect(docs.length, `${fileName} is indexed`).toBeGreaterThan(0);
      for (const doc of docs) await reviewAs(page, doc.id, { single, tags });
    }

    const mcp = await DocsMCP.start();
    const rows: {
      qid: string;
      gold: number;
      unfiltered: number;
      filtered: number;
      lost: string[];
      missedByBoth: string[];
      filters: SearchAnswer['filters'];
    }[] = [];
    try {
      for (const question of set.questions) {
        const today = await mcp.search({ query: question.query });
        const filtered = await mcp.search({ query: question.query, ...question.filters });
        const todayTitles = new Set(today.documents.map((d) => d.title));
        const filteredTitles = new Set(filtered.documents.map((d) => d.title));
        rows.push({
          qid: question.qid,
          gold: question.gold_docs.length,
          unfiltered: recall(question.gold_docs, today),
          filtered: recall(question.gold_docs, filtered),
          lost: question.gold_docs.filter((n) => todayTitles.has(n) && !filteredTitles.has(n)),
          missedByBoth: question.gold_docs.filter(
            (n) => !todayTitles.has(n) && !filteredTitles.has(n),
          ),
          filters: filtered.filters,
        });
      }
    } finally {
      mcp.close();
    }

    const mean = (pick: (row: (typeof rows)[number]) => number) =>
      rows.reduce((sum, row) => sum + pick(row), 0) / rows.length;
    const report = {
      questions: rows.length,
      mean_set_recall_unfiltered: mean((row) => row.unfiltered),
      mean_set_recall_filtered: mean((row) => row.filtered),
      rows,
    };
    await testInfo.attach('document-filter-eval.json', {
      body: JSON.stringify(report, null, 2),
      contentType: 'application/json',
    });
    expect(report.mean_set_recall_filtered).toBeGreaterThan(report.mean_set_recall_unfiltered);
    expect(rows.flatMap((row) => row.lost.map((name) => `${row.qid}: ${name}`))).toEqual([]);
  });
});
