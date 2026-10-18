import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { renderAsync } from 'docx-preview';
import '../../../i18n/i18n'; // side-effect: initialise i18next so preview t() keys resolve
import ImagePreview from './ImagePreview';
import PdfPreview from './PdfPreview';
import TextPreview from './TextPreview';
import HtmlPreview from './HtmlPreview';
import { AssetSourceContext, type AssetSource } from './assetSourceContext';
import DocxPreview from './DocxPreview';
import XlsxPreview from './XlsxPreview';

// The lazy per-MIME renderers (D-07/D-08/D-09). docx-preview is the heavy dep confined to its own
// chunk — here it is MOCKED (vi.mock, hoisted above the imports) so the suite asserts the wrapper
// calls it with the right args without loading the real library (Pitfall 4, which would drag the
// 85% aggregate). The security-critical surface is pinned directly: HtmlPreview's null-origin
// sandbox, and XlsxPreview's GridCraft frame, which hears only its own frame on this origin.
vi.mock('docx-preview', () => ({
  renderAsync: vi.fn(() => Promise.resolve({})),
}));
interface FetchShape {
  readonly ok?: boolean;
  readonly status?: number;
  readonly text?: string;
  readonly blob?: Blob;
  readonly buffer?: ArrayBuffer;
  readonly pending?: boolean;
}

function stubFetch(shape: FetchShape): void {
  const res = {
    ok: shape.ok ?? true,
    status: shape.status ?? 200,
    text: () => Promise.resolve(shape.text ?? ''),
    blob: () => Promise.resolve(shape.blob ?? new Blob(['docx-bytes'])),
    arrayBuffer: () => Promise.resolve(shape.buffer ?? new ArrayBuffer(8)),
  } as unknown as Response;
  vi.stubGlobal(
    'fetch',
    vi.fn(() =>
      shape.pending === true ? new Promise<Response>(() => undefined) : Promise.resolve(res),
    ),
  );
}

function iframeOf(container: HTMLElement): HTMLIFrameElement {
  const frame = container.querySelector('iframe');
  if (frame === null) throw new Error('expected an <iframe>');
  return frame;
}

const props = { assetId: 'asset-1', mimeType: 'application/octet-stream', fileName: 'report' };

/** A source like the file manager's: the same routes, and a save the test provides. */
const FILE_SOURCE: AssetSource = {
  assetUrl: (id) => `/files/${id}`,
  streamUrl: (id) => `/files/${id}`,
  credentials: 'same-origin',
};

beforeEach(() => {
  URL.createObjectURL = vi.fn(() => 'blob:mock');
  URL.revokeObjectURL = vi.fn();
  vi.mocked(renderAsync).mockClear();
});

afterEach(() => {
  vi.unstubAllGlobals();
});

describe('ImagePreview', () => {
  it('renders an <img> from the relabelled object URL', async () => {
    stubFetch({ blob: new Blob(['img']) });
    const { container } = render(
      <ImagePreview {...props} mimeType="image/png" fileName="cat.png" />,
    );

    await waitFor(() => {
      expect(container.querySelector('img')).not.toBeNull();
    });
    const img = container.querySelector('img');
    expect(img?.getAttribute('src')).toBe('blob:mock');
    expect(img?.getAttribute('alt')).toBe('cat.png');
  });

  it('shows the loading state while the fetch is in flight', () => {
    stubFetch({ pending: true });
    render(<ImagePreview {...props} mimeType="image/png" />);
    expect(screen.getByRole('status')).toBeTruthy();
  });

  it('shows the error state on a non-ok response', async () => {
    stubFetch({ ok: false, status: 404 });
    render(<ImagePreview {...props} mimeType="image/png" />);
    await waitFor(() => {
      expect(screen.getByRole('alert')).toBeTruthy();
    });
  });
});

describe('PdfPreview', () => {
  it('renders a native <iframe src> from the object URL', async () => {
    stubFetch({ blob: new Blob(['pdf']) });
    const { container } = render(<PdfPreview {...props} mimeType="application/pdf" />);

    await waitFor(() => {
      expect(container.querySelector('iframe')).not.toBeNull();
    });
    expect(iframeOf(container).getAttribute('src')).toBe('blob:mock');
  });

  it('shows the error state on a non-ok response', async () => {
    stubFetch({ ok: false, status: 404 });
    render(<PdfPreview {...props} mimeType="application/pdf" />);
    await waitFor(() => {
      expect(screen.getByRole('alert')).toBeTruthy();
    });
  });
});

describe('TextPreview', () => {
  it('renders fetched text React-escaped inside a <pre> (no HTML interpretation)', async () => {
    stubFetch({ text: 'line1\n<script>alert(1)</script>' });
    const { container } = render(<TextPreview {...props} />);

    await waitFor(() => {
      expect(container.querySelector('pre')).not.toBeNull();
    });
    const pre = container.querySelector('pre');
    // React escapes the bytes: the literal text is present, no real <script> element exists.
    expect(pre?.textContent).toContain('<script>alert(1)</script>');
    expect(container.querySelector('script')).toBeNull();
  });

  it('shows the error state on a non-ok response', async () => {
    stubFetch({ ok: false, status: 500 });
    render(<TextPreview {...props} />);
    await waitFor(() => {
      expect(screen.getByRole('alert')).toBeTruthy();
    });
  });

  it('highlights a file by its extension, still as escaped text', async () => {
    stubFetch({ text: 'import os\nprint("<script>alert(1)</script>")' });
    const { container } = render(<TextPreview {...props} fileName="tenant_checks.py" />);

    await waitFor(
      () => {
        expect(container.querySelector('pre.shiki')).not.toBeNull();
      },
      { timeout: 5000 },
    );
    expect(container.textContent).toContain('print("<script>alert(1)</script>")');
    expect(container.querySelector('script')).toBeNull();
  });

  it('keeps a file whose extension has no grammar as plain text', async () => {
    stubFetch({ text: 'a,b\n1,2' });
    const { container } = render(<TextPreview {...props} fileName="table.csv" />);

    await waitFor(() => {
      expect(container.querySelector('pre')?.textContent).toBe('a,b\n1,2');
    });
    await new Promise((resolve) => setTimeout(resolve, 0));
    expect(container.querySelector('.shiki')).toBeNull();
  });
});

describe('HtmlPreview (WEBART-07 / T-37B-08)', () => {
  // These assertions were rewritten when the rendered lane moved from srcdoc to the sealed
  // src= document: the OLD test pinned `srcdoc == the fetched bytes`, which is exactly the
  // property the change removes (a srcdoc document inherits the embedder's CSP and can carry
  // no policy of its own). The null-origin invariant it also pinned is preserved verbatim
  // below, and the srcdoc lane still has a test of its own — on the tier that still uses it.
  it('rendered lane frames the SEALED document: src=/render, never srcdoc, null origin', async () => {
    stubFetch({ text: '<h1>hello</h1>' });
    const { container } = render(<HtmlPreview {...props} fileName="page.html" />);

    await waitFor(() => {
      expect(container.querySelector('iframe')).not.toBeNull();
    });
    const frame = iframeOf(container);
    expect(frame.getAttribute('sandbox')).toBe('allow-scripts');
    expect(frame.getAttribute('sandbox')).not.toContain('allow-same-origin');
    expect(frame.getAttribute('src')).toBe('/api/assets/asset-1/render');
    expect(frame.getAttribute('srcdoc')).toBeNull();
  });

  it('does not fetch the body for the rendered lane — the frame does', () => {
    stubFetch({ text: '<h1>hello</h1>' });
    render(<HtmlPreview {...props} fileName="page.html" />);
    expect(vi.mocked(fetch)).not.toHaveBeenCalled();
  });

  it('percent-encodes the asset id in the render URL', async () => {
    stubFetch({ text: 'x' });
    const { container } = render(<HtmlPreview {...props} assetId="a/b?c" fileName="page.html" />);
    await waitFor(() => {
      expect(container.querySelector('iframe')).not.toBeNull();
    });
    expect(iframeOf(container).getAttribute('src')).toBe('/api/assets/a%2Fb%3Fc/render');
  });

  it('source tab shows the markup ESCAPED, never parsed', async () => {
    stubFetch({ text: '<script>alert(1)</script>' });
    const { container } = render(<HtmlPreview {...props} fileName="page.html" />);
    fireEvent.click(screen.getByRole('tab', { name: /source|sorgente/i }));

    await waitFor(() => {
      expect(container.querySelector('pre')).not.toBeNull();
    });
    const pre = container.querySelector('pre');
    expect(pre?.textContent).toBe('<script>alert(1)</script>');
    // Escaped, so the markup never became a node of ours.
    expect(pre?.querySelector('script')).toBeNull();
  });

  it('shows the error state when the SOURCE fetch fails', async () => {
    stubFetch({ ok: false, status: 500 });
    render(<HtmlPreview {...props} fileName="page.html" />);
    fireEvent.click(screen.getByRole('tab', { name: /source|sorgente/i }));
    await waitFor(() => {
      expect(screen.getByRole('alert')).toBeTruthy();
    });
  });

  it('falls back to the null-origin srcdoc lane when the tier has no render route', async () => {
    stubFetch({ text: '<h1>shared</h1>' });
    const shareTier: AssetSource = {
      assetUrl: (id) => `/s/tok/asset/${id}`,
      streamUrl: (id) => `/s/tok/asset/${id}/stream`,
      credentials: 'omit',
    };
    const { container } = render(
      <AssetSourceContext.Provider value={shareTier}>
        <HtmlPreview {...props} fileName="page.html" />
      </AssetSourceContext.Provider>,
    );

    await waitFor(() => {
      expect(container.querySelector('iframe')).not.toBeNull();
    });
    const frame = iframeOf(container);
    expect(frame.getAttribute('srcdoc')).toBe('<h1>shared</h1>');
    expect(frame.getAttribute('src')).toBeNull();
    expect(frame.getAttribute('sandbox')).toBe('allow-scripts');
  });
});

describe('DocxPreview (D-08 / T-37B-12)', () => {
  it('calls docx-preview.renderAsync with the fetched Blob and the aura-docx className', async () => {
    stubFetch({ blob: new Blob(['docx']) });
    render(<DocxPreview {...props} fileName="memo.docx" />);

    await waitFor(() => {
      expect(vi.mocked(renderAsync)).toHaveBeenCalledTimes(1);
    });
    expect(vi.mocked(renderAsync)).toHaveBeenCalledWith(
      expect.any(Blob),
      expect.any(HTMLElement),
      expect.anything(),
      expect.objectContaining({
        className: 'aura-docx',
        inWrapper: true,
        ignoreLastRenderedPageBreak: true,
      }),
    );
  });

  it('shows the error state when renderAsync rejects', async () => {
    stubFetch({ blob: new Blob(['docx']) });
    vi.mocked(renderAsync).mockRejectedValueOnce(new Error('corrupt docx'));
    render(<DocxPreview {...props} fileName="memo.docx" />);
    await waitFor(() => {
      expect(screen.getByRole('alert')).toBeTruthy();
    });
  });

  it('shows the error state when the fetch fails', async () => {
    stubFetch({ ok: false, status: 404 });
    render(<DocxPreview {...props} fileName="memo.docx" />);
    await waitFor(() => {
      expect(screen.getByRole('alert')).toBeTruthy();
    });
    expect(vi.mocked(renderAsync)).not.toHaveBeenCalled();
  });
});

describe('XlsxPreview', () => {
  /** gridcraft sends a message as the framed GridCraft does, from its window on this origin. */
  function gridcraft(frame: HTMLIFrameElement, data: unknown, source = frame.contentWindow): void {
    window.dispatchEvent(
      new MessageEvent('message', { data, origin: window.location.origin, source }),
    );
  }

  it('frames GridCraft in host mode, and opens the workbook in it once it is ready', async () => {
    const workbook = new ArrayBuffer(16);
    stubFetch({ buffer: workbook });
    const { container } = render(<XlsxPreview {...props} fileName="book.xlsx" />);
    const frame = iframeOf(container);
    expect(frame.getAttribute('src')).toBe('/apps/gridcraft/?host=parent');
    expect(frame.getAttribute('title')).toBe('book.xlsx');
    const target = frame.contentWindow;
    if (target === null) throw new Error('the frame has no window');
    const post = vi.spyOn(target, 'postMessage');
    gridcraft(frame, { type: 'gridcraft:ready', version: '0.8.0' });
    await waitFor(() => {
      expect(post).toHaveBeenCalledTimes(1);
    });
    const [message, origin] = post.mock.calls[0] as [
      { type: string; name: string; bytes: ArrayBuffer },
      string,
    ];
    expect(message.type).toBe('gridcraft:open');
    expect(message.name).toBe('book.xlsx');
    expect(message.bytes.byteLength).toBe(16);
    expect(origin).toBe(window.location.origin);
  });

  it('hears nothing from a window that is not its frame', async () => {
    stubFetch({ buffer: new ArrayBuffer(16) });
    const { container } = render(<XlsxPreview {...props} fileName="book.xlsx" />);
    const frame = iframeOf(container);
    const target = frame.contentWindow;
    if (target === null) throw new Error('the frame has no window');
    const post = vi.spyOn(target, 'postMessage');
    gridcraft(frame, { type: 'gridcraft:ready' }, window);
    gridcraft(frame, { type: 'gridcraft:failed', error: 'forged' }, window);
    await new Promise((resolve) => setTimeout(resolve, 20));
    expect(post).not.toHaveBeenCalled();
    expect(screen.queryByRole('alert')).toBeNull();
  });

  it('shows the error when GridCraft cannot start', async () => {
    stubFetch({ buffer: new ArrayBuffer(16) });
    const { container } = render(<XlsxPreview {...props} fileName="book.xlsx" />);
    gridcraft(iframeOf(container), { type: 'gridcraft:failed', error: 'no WebGL' });
    expect((await screen.findByRole('alert')).textContent).toContain('no WebGL');
  });

  it('saves over the file when GridCraft saves it under its own name', async () => {
    stubFetch({ buffer: new ArrayBuffer(16) });
    const save = vi.fn(() => Promise.resolve());
    const source: AssetSource = { ...FILE_SOURCE, save };
    const { container } = render(
      <AssetSourceContext.Provider value={source}>
        <XlsxPreview {...props} assetId="/chat/c5d7.xlsx" fileName="book.xlsx" />
      </AssetSourceContext.Provider>,
    );
    const frame = iframeOf(container);
    const saved = new ArrayBuffer(32);
    gridcraft(frame, { type: 'gridcraft:save', name: 'book.xlsx', bytes: saved });
    expect((await screen.findByRole('status')).textContent).toMatch(/Saved|Salvato/);
    expect(save).toHaveBeenCalledWith('/chat/c5d7.xlsx', saved, undefined);
    gridcraft(frame, { type: 'gridcraft:save', name: 'book.pdf', bytes: saved });
    await waitFor(() => {
      expect(save).toHaveBeenLastCalledWith('/chat/c5d7.xlsx', saved, 'book.pdf');
    });
  });

  it('shows the error when a save is refused', async () => {
    stubFetch({ buffer: new ArrayBuffer(16) });
    const save = vi.fn(() => Promise.reject(new Error('HTTP 502')));
    const { container } = render(
      <AssetSourceContext.Provider value={{ ...FILE_SOURCE, save }}>
        <XlsxPreview {...props} fileName="book.xlsx" />
      </AssetSourceContext.Provider>,
    );
    gridcraft(iframeOf(container), {
      type: 'gridcraft:save',
      name: 'book.xlsx',
      bytes: new ArrayBuffer(8),
    });
    expect((await screen.findByRole('alert')).textContent).toContain('HTTP 502');
  });

  it('says that changes made in the preview are not stored', async () => {
    stubFetch({ buffer: new ArrayBuffer(16) });
    const { container } = render(<XlsxPreview {...props} fileName="book.xlsx" />);
    gridcraft(iframeOf(container), { type: 'gridcraft:dirty', dirty: true });
    expect((await screen.findByRole('status')).textContent).toMatch(
      /not saved|non vengono salvate/,
    );
  });

  it('shows the error when the workbook cannot be read', async () => {
    stubFetch({ ok: false, status: 404 });
    render(<XlsxPreview {...props} fileName="book.xlsx" />);
    expect(await screen.findByRole('alert')).toBeTruthy();
  });
});
