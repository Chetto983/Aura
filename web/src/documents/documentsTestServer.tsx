import { render } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { vi } from 'vitest';
import DocumentsWorkspace from './DocumentsWorkspace';
import type { DocumentRow, DocumentsPage, Label } from './documentsApi';

// The documents API as the view's tests see it: two invoices, the operator's lists, and a server
// that records what the view asks for.

export const LABELS: Readonly<Record<string, readonly Partial<Label>[]>> = {
  type: [
    { id: 't-fattura', kind: 'type', name: 'Fattura' },
    { id: 't-contratto', kind: 'type', name: 'Contratto' },
  ],
  counterparty: [
    { id: 'c-gialli', kind: 'counterparty', name: 'Gialli Import' },
    { id: 'c-rossi', kind: 'counterparty', name: 'Rossi SRL' },
  ],
  tag: [{ id: 'g-ricambi', kind: 'tag', name: 'ricambi' }],
};

function row(
  id: string,
  fileName: string,
  values: DocumentRow['values'],
  needsReview: boolean,
): DocumentRow {
  return {
    id,
    source_key: `chat/${fileName}`,
    file_name: fileName,
    size_bytes: 1024,
    indexed_at: '2026-10-18 10:00:00.000',
    record_id: values.length > 0 ? `r-${id}` : undefined,
    values,
    needs_review: needsReview,
    ...(needsReview || values.length === 0 ? {} : { reviewed_at: '2026-10-18T11:00:00Z' }),
  } as DocumentRow;
}

const value = (field: string, key: string, setBy: string) => ({
  field,
  value_key: key,
  set_by: setBy,
  accepted: false,
  set_at: '2026-10-18T10:00:00Z',
});

export const PAGE: DocumentsPage = {
  rows: [
    row(
      'd1',
      'fattura_2026_031.pdf',
      [
        value('type', 't-fattura', 'operator'),
        value('counterparty', 'c-gialli', 'operator'),
        value('document_date', '2026-03-27', 'operator'),
        value('tag', 'g-ricambi', 'operator'),
      ],
      false,
    ),
    row(
      'd2',
      'fattura_2026_044.pdf',
      [
        value('type', 't-fattura', 'rule:t-fattura'),
        value('document_date', '2026-04-14', 'extractor'),
      ],
      true,
    ),
  ],
  total: 24,
  facets: {
    type: { 't-fattura': 20 },
    counterparty: { 'c-gialli': 9, 'c-rossi': 6 },
    tags: { 'g-ricambi': 4 },
    unassigned: { type: 4, counterparty: 3, tags: 20 },
  },
};

function urlOf(input: RequestInfo | URL): string {
  if (typeof input === 'string') return input;
  if (input instanceof URL) return input.href;
  return input.url;
}

/** A response the bulk route gives, in order; the last one repeats. */
export interface BulkAnswer {
  readonly status: number;
  readonly body: unknown;
}

/** stubServer answers the documents API, records every list request's query and every bulk
 * request's body, and answers the bulk route with the given responses in turn. */
export function stubServer(
  reviewCount = 7,
  bulk: readonly BulkAnswer[] = [{ status: 200, body: { applied: 1 } }],
) {
  const lists: URLSearchParams[] = [];
  const posts: unknown[] = [];
  vi.stubGlobal(
    'fetch',
    vi.fn((input: RequestInfo | URL, init?: RequestInit) => {
      const url = new URL(urlOf(input), 'http://cockpit.test');
      if (url.pathname === '/api/documents/bulk') {
        posts.push(typeof init?.body === 'string' ? (JSON.parse(init.body) as unknown) : undefined);
        const answer = bulk[Math.min(posts.length, bulk.length) - 1] ?? { status: 200, body: {} };
        return Promise.resolve(
          new Response(JSON.stringify(answer.body), { status: answer.status }),
        );
      }
      let body: unknown;
      if (url.pathname === '/api/documents/labels') {
        body = { labels: LABELS[url.searchParams.get('kind') ?? ''] ?? [] };
      } else if (url.pathname === '/api/documents/custom-fields') {
        body = { fields: [] };
      } else if (
        url.searchParams.get('needs_review') === 'true' &&
        url.searchParams.get('limit') === '1'
      ) {
        body = { rows: [], total: reviewCount, facets: {} };
      } else {
        lists.push(url.searchParams);
        body = PAGE;
      }
      return Promise.resolve(new Response(JSON.stringify(body), { status: 200 }));
    }),
  );
  return { lists, posts };
}

export function renderDocuments() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={client}>
      <DocumentsWorkspace />
    </QueryClientProvider>,
  );
}
