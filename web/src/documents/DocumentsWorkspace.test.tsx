import { act, fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { afterEach, describe, expect, it, vi } from 'vitest';
import DocumentsWorkspace from './DocumentsWorkspace';
import type { DocumentRow, DocumentsPage, Label } from './documentsApi';
import i18n from '@/i18n/i18n';

const LABELS: Readonly<Record<string, readonly Partial<Label>[]>> = {
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

const PAGE: DocumentsPage = {
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

/** stubServer answers the documents API and records every list request's query. */
function stubServer(reviewCount = 7) {
  const lists: URLSearchParams[] = [];
  vi.stubGlobal(
    'fetch',
    vi.fn((input: RequestInfo | URL) => {
      const url = new URL(urlOf(input), 'http://cockpit.test');
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
  return lists;
}

function renderDocuments() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={client}>
      <DocumentsWorkspace />
    </QueryClientProvider>,
  );
}

afterEach(() => {
  vi.unstubAllGlobals();
});

describe('DocumentsWorkspace', () => {
  it('shows each value under its column, marked when the operator did not set it', async () => {
    await act(() => i18n.changeLanguage('it'));
    stubServer();
    renderDocuments();
    const suggested = (await screen.findByText('fattura_2026_044.pdf')).closest('.wx-row');
    if (!(suggested instanceof HTMLElement)) throw new Error('no row for the suggested invoice');
    expect(within(suggested).getByText('regola')).toBeTruthy();
    expect(within(suggested).getByText('estrattore')).toBeTruthy();
    expect(within(suggested).getByText('Da rivedere')).toBeTruthy();
    const reviewed = screen.getByText('fattura_2026_031.pdf').closest('.wx-row');
    if (!(reviewed instanceof HTMLElement)) throw new Error('no row for the reviewed invoice');
    expect(within(reviewed).getByRole('button', { name: 'Gialli Import' })).toBeTruthy();
    expect(within(reviewed).getByText('27/03/2026')).toBeTruthy();
    expect(within(reviewed).queryByText('regola')).toBeNull();
    expect(within(reviewed).getByText('Rivisto')).toBeTruthy();
    expect(screen.getByText('24 documenti')).toBeTruthy();
  });

  it('counts each value of a list in the result, with Include, Exclude and Non assegnata', async () => {
    await act(() => i18n.changeLanguage('it'));
    const lists = stubServer();
    renderDocuments();
    await screen.findByText('fattura_2026_044.pdf');
    fireEvent.click(screen.getByRole('button', { name: 'Controparte' }));
    const menu = await screen.findByRole('dialog', { name: 'Controparte' });
    expect(within(menu).getByRole('checkbox', { name: /Non assegnata\s*3/ })).toBeTruthy();
    expect(within(menu).getByRole('checkbox', { name: /Gialli Import\s*9/ })).toBeTruthy();
    fireEvent.click(within(menu).getByRole('checkbox', { name: /Gialli Import/ }));
    await waitFor(() => {
      expect(lists.at(-1)?.getAll('counterparty')).toEqual(['c-gialli']);
    });
    fireEvent.click(within(menu).getByRole('radio', { name: 'Escludi' }));
    fireEvent.click(within(menu).getByRole('checkbox', { name: /Rossi SRL/ }));
    await waitFor(() => {
      expect(lists.at(-1)?.getAll('counterparty_not')).toEqual(['c-rossi']);
    });
    fireEvent.click(within(menu).getByRole('checkbox', { name: /Non assegnata/ }));
    await waitFor(() => {
      expect(lists.at(-1)?.getAll('unassigned')).toEqual(['counterparty']);
    });
  });

  it('filters by a value clicked in a row, and Azzera filtri clears every filter', async () => {
    await act(() => i18n.changeLanguage('it'));
    const lists = stubServer();
    renderDocuments();
    const reviewed = (await screen.findByText('fattura_2026_031.pdf')).closest('.wx-row');
    if (!(reviewed instanceof HTMLElement)) throw new Error('no row');
    fireEvent.click(within(reviewed).getByRole('button', { name: 'Gialli Import' }));
    await waitFor(() => {
      expect(lists.at(-1)?.getAll('counterparty')).toEqual(['c-gialli']);
    });
    expect(screen.getByRole('button', { name: /Controparte: Gialli Import/ })).toBeTruthy();
    fireEvent.click(screen.getByRole('button', { name: 'Azzera filtri' }));
    await waitFor(() => {
      expect(lists.at(-1)?.getAll('counterparty')).toEqual([]);
    });
  });

  it('counts the documents to review, and shows them on a click', async () => {
    await act(() => i18n.changeLanguage('it'));
    const lists = stubServer(7);
    renderDocuments();
    const counter = await screen.findByRole('button', { name: /Da rivedere\s*7/ });
    expect(counter.getAttribute('aria-pressed')).toBe('false');
    fireEvent.click(counter);
    await waitFor(() => {
      expect(lists.at(-1)?.get('needs_review')).toBe('true');
    });
    expect(counter.getAttribute('aria-pressed')).toBe('true');
  });
});
