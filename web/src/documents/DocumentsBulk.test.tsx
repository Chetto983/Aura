import { act, fireEvent, screen, waitFor, within } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { renderDocuments, stubServer } from './documentsTestServer';
import i18n from '@/i18n/i18n';

afterEach(() => {
  vi.unstubAllGlobals();
});

async function selectBoth() {
  fireEvent.click(await screen.findByRole('checkbox', { name: 'Seleziona fattura_2026_031.pdf' }));
  fireEvent.click(screen.getByRole('checkbox', { name: 'Seleziona fattura_2026_044.pdf' }));
  return screen.getByRole('toolbar', { name: 'Modifica in blocco' });
}

describe('bulk edit', () => {
  it('shows each value on all, some or none of the selected documents', async () => {
    await act(() => i18n.changeLanguage('it'));
    stubServer();
    renderDocuments();
    const bar = await selectBoth();
    expect(within(bar).getByText('2 selezionati')).toBeTruthy();
    fireEvent.click(within(bar).getByRole('button', { name: 'Controparte' }));
    const menu = await screen.findByRole('dialog', { name: 'Controparte, sui 2 selezionati' });
    expect(within(menu).getByRole('button', { name: /Gialli Import.*su 1 di 2/ })).toBeTruthy();
    expect(within(menu).getByRole('button', { name: /Rossi SRL.*su nessuno/ })).toBeTruthy();
    fireEvent.click(within(bar).getByRole('button', { name: 'Tipo' }));
    const types = await screen.findByRole('dialog', { name: 'Tipo, sui 2 selezionati' });
    expect(within(types).getByRole('button', { name: /Fattura.*su tutti/ })).toBeTruthy();
  });

  it('applies only the difference, after a confirmation naming it and the count', async () => {
    await act(() => i18n.changeLanguage('it'));
    const { posts } = stubServer();
    renderDocuments();
    const bar = await selectBoth();
    fireEvent.click(within(bar).getByRole('button', { name: 'Controparte' }));
    const menu = await screen.findByRole('dialog', { name: 'Controparte, sui 2 selezionati' });
    fireEvent.click(within(menu).getByRole('button', { name: /Gialli Import/ }));
    expect(within(menu).getByRole('button', { name: /Gialli Import.*→ tutti/ })).toBeTruthy();
    fireEvent.click(within(bar).getByRole('button', { name: 'Applica' }));
    const confirm = await screen.findByRole('dialog', { name: 'Confermi la modifica?' });
    expect(confirm.textContent).toContain('Gialli Import');
    expect(confirm.textContent).toContain('2 documenti');
    fireEvent.click(within(confirm).getByRole('button', { name: 'Conferma' }));
    await waitFor(() => {
      expect(posts).toHaveLength(1);
    });
    expect(posts[0]).toEqual({
      selection: { ids: ['d1', 'd2'] },
      diff: { set: [{ field: 'counterparty', value_key: 'c-gialli' }], remove: [] },
    });
    await waitFor(() => {
      expect(screen.queryByRole('toolbar', { name: 'Modifica in blocco' })).toBeNull();
    });
  });

  it('applies to every document of the filter, and asks again when the count changed', async () => {
    await act(() => i18n.changeLanguage('it'));
    const { posts } = stubServer(7, [
      { status: 409, body: { error: 'count_changed', count: 25 } },
      { status: 200, body: { applied: 25 } },
    ]);
    renderDocuments();
    const bar = await selectBoth();
    fireEvent.click(within(bar).getByRole('button', { name: 'seleziona tutti i 24 del filtro' }));
    expect(within(bar).getByText('24 selezionati')).toBeTruthy();
    fireEvent.click(within(bar).getByRole('button', { name: 'Tag' }));
    const menu = await screen.findByRole('dialog', { name: 'Tag, sui 24 selezionati' });
    fireEvent.click(within(menu).getByRole('button', { name: /ricambi/ }));
    fireEvent.click(within(bar).getByRole('button', { name: 'Applica' }));
    let confirm = await screen.findByRole('dialog', { name: 'Confermi la modifica?' });
    expect(confirm.textContent).toContain('24 documenti');
    fireEvent.click(within(confirm).getByRole('button', { name: 'Conferma' }));
    await waitFor(() => {
      expect(screen.getByRole('dialog', { name: 'Confermi la modifica?' }).textContent).toContain(
        '25 documenti',
      );
    });
    confirm = screen.getByRole('dialog', { name: 'Confermi la modifica?' });
    expect(confirm.textContent).toContain('Il filtro ora contiene 25 documenti');
    fireEvent.click(within(confirm).getByRole('button', { name: 'Conferma' }));
    await waitFor(() => {
      expect(posts).toHaveLength(2);
    });
    interface Sent {
      selection: { expected_count: number; exclude: string[] };
    }
    const [first, second] = posts as [Sent, Sent];
    expect(first.selection.expected_count).toBe(24);
    expect(first.selection.exclude).toEqual([]);
    expect(second.selection.expected_count).toBe(25);
  });
});
