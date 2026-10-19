import { afterEach, describe, expect, it, vi } from 'vitest';
import { fireEvent, render, screen } from '@testing-library/react';
import { useState, type ReactNode } from 'react';
import '../../../i18n/i18n'; // side-effect: initialise i18next so preview t() keys resolve
import { AssetSourceContext, IDENTITY_SCOPED, type AssetSource } from './assetSourceContext';
import EditInCraft from './EditInCraft';
import { PreviewActionsContext } from './previewActionsContext';

// EditInCraft: the light viewer stays the preview, and the Craft app is offered only where what
// it saves can be stored.

const asset = { assetId: 'a-1', mimeType: 'image/png', fileName: 'photo.png' };

afterEach(() => {
  vi.unstubAllGlobals();
});

function viewer() {
  return <p data-testid="viewer">viewer</p>;
}

/** A host as PreviewModal is one: a header that holds the preview's actions, and the preview. */
function Host({ children }: { readonly children: ReactNode }) {
  const [actions, setActions] = useState<HTMLElement | null>(null);
  return (
    <>
      <header data-testid="header">
        <span ref={setActions} />
      </header>
      <PreviewActionsContext.Provider value={actions}>
        <main data-testid="body">{children}</main>
      </PreviewActionsContext.Provider>
    </>
  );
}

describe('EditInCraft', () => {
  it('shows only the viewer where a save could not be stored', () => {
    render(
      <AssetSourceContext.Provider value={IDENTITY_SCOPED}>
        <EditInCraft app="photocraft" asset={asset}>
          {viewer()}
        </EditInCraft>
      </AssetSourceContext.Provider>,
    );
    expect(screen.getByTestId('viewer')).toBeTruthy();
    expect(screen.queryByRole('button')).toBeNull();
  });

  it('swaps the app in for the viewer when Edit is pressed', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(() => new Promise<Response>(() => undefined)),
    );
    const source: AssetSource = { ...IDENTITY_SCOPED, save: vi.fn(() => Promise.resolve()) };
    const { container } = render(
      <AssetSourceContext.Provider value={source}>
        <Host>
          <EditInCraft app="photocraft" asset={asset}>
            {viewer()}
          </EditInCraft>
        </Host>
      </AssetSourceContext.Provider>,
    );
    expect(screen.getByTestId('viewer')).toBeTruthy();
    const edit = await screen.findByRole('button', { name: /Edit|Modifica/ });
    expect(screen.getByTestId('header').contains(edit)).toBe(true);
    expect(screen.getByTestId('body').contains(edit)).toBe(false);
    fireEvent.click(edit);
    expect(await screen.findByTitle('photo.png')).toBeTruthy();
    expect(container.querySelector('iframe')?.getAttribute('src')).toBe(
      '/apps/photocraft/?host=parent',
    );
    expect(screen.queryByTestId('viewer')).toBeNull();
  });
});
