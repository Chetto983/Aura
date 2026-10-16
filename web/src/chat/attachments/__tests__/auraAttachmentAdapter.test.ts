import { beforeEach, describe, expect, it, vi } from 'vitest';
import type { PendingAttachment } from '@assistant-ui/core';
import { createAuraAttachmentAdapter } from '../auraAttachmentAdapter';
import type { Asset } from '../types';

// The adapter is the whole upload lifecycle now, so the identity it publishes is the thing
// worth pinning: the runtime indexes attachments by id, and this is where a wandering id
// turns one file into two chips.

const { presignAsset, finalizeAsset, deleteAsset, putWithProgress, pollUntilReady } = vi.hoisted(
  () => ({
    presignAsset: vi.fn(),
    finalizeAsset: vi.fn(),
    deleteAsset: vi.fn(),
    putWithProgress: vi.fn(),
    pollUntilReady: vi.fn(),
  }),
);

vi.mock('../api', () => ({ presignAsset, finalizeAsset, deleteAsset, getAsset: vi.fn() }));
vi.mock('../upload', async (importOriginal) => {
  const actual = await importOriginal<typeof import('../upload')>();
  return { ...actual, putWithProgress, pollUntilReady };
});

function asset(overrides: Partial<Asset> = {}): Asset {
  return {
    id: 'asset-1',
    status: 'complete',
    modality: 'document',
    file_name: 'note.txt',
    mime_type: 'text/plain',
    declared_size_bytes: 4,
    size_bytes: 4,
    ...overrides,
  };
}

async function drain(gen: AsyncGenerator<PendingAttachment, void>): Promise<PendingAttachment[]> {
  const out: PendingAttachment[] = [];
  for await (const value of gen) out.push(value);
  return out;
}

function file(name = 'note.txt', type = 'text/plain'): File {
  return new File(['data'], name, { type });
}

beforeEach(() => {
  vi.clearAllMocks();
  presignAsset.mockResolvedValue({
    asset: asset({ status: 'presigned' }),
    upload: { upload_url: 'https://store.invalid/put', required_headers: {} },
  });
  putWithProgress.mockResolvedValue(undefined);
  finalizeAsset.mockResolvedValue(asset({ status: 'processing' }));
  pollUntilReady.mockResolvedValue(asset());
});

describe('createAuraAttachmentAdapter', () => {
  // Measured live 2026-08-17: the first yield carried the file name as its id and later ones
  // the asset id, so the runtime kept BOTH — one chip frozen at 0%, one at 100%, for a single
  // file. The id must not move once the runtime has seen it.
  // The presigner owns the allowlist (internal/assets documentExts, derived from the ingest's),
  // and a copy kept here drifted: a .go or a .tsx the ingest reads was not even selectable.
  it('leaves the choice of file to the presigner', () => {
    expect(createAuraAttachmentAdapter({ threadId: 't' }).accept).toBe('*');
  });

  it("shows a document chip once the server classified the file, even a source file's", async () => {
    presignAsset.mockResolvedValue({
      asset: asset({ status: 'presigned', modality: 'document', file_name: 'nastro.go' }),
      upload: { upload_url: 'https://store.invalid/put', required_headers: {} },
    });
    const adapter = createAuraAttachmentAdapter({ threadId: 'conv-1' });

    const yields = await drain(
      adapter.add({ file: file('nastro.go', '') }) as AsyncGenerator<PendingAttachment, void>,
    );

    expect(yields[0]?.type).toBe('file');
    expect(yields[yields.length - 1]?.type).toBe('document');
  });

  // The server classifies by extension before MIME type (internal/assets InferModality), and a
  // hint would win over both: browsers label a PL/SQL .pls "audio/x-scpls".
  it('sends no modality hint of its own, so the server classifies a source file by its name', async () => {
    const adapter = createAuraAttachmentAdapter({ threadId: 'conv-1' });

    await drain(
      adapter.add({ file: file('pkg_ordini.pls', 'audio/x-scpls') }) as AsyncGenerator<
        PendingAttachment,
        void
      >,
    );

    expect(presignAsset).toHaveBeenCalledWith(
      expect.objectContaining({ file_name: 'pkg_ordini.pls', modality_hint: 'unknown' }),
    );
  });

  it("keeps the server's type on the chip when a later step fails", async () => {
    presignAsset.mockResolvedValue({
      asset: asset({ status: 'presigned', modality: 'document', file_name: 'nastro.go' }),
      upload: { upload_url: 'https://store.invalid/put', required_headers: {} },
    });
    putWithProgress.mockRejectedValue(new Error('upload failed: HTTP 403'));
    const adapter = createAuraAttachmentAdapter({ threadId: 'conv-1' });

    const yields = await drain(
      adapter.add({ file: file('nastro.go', '') }) as AsyncGenerator<PendingAttachment, void>,
    );
    const last = yields[yields.length - 1];

    expect(last?.status).toMatchObject({ type: 'incomplete', message: 'upload failed: HTTP 403' });
    expect(last?.type).toBe('document');
  });

  it('keeps one identity for the whole upload', async () => {
    const adapter = createAuraAttachmentAdapter({ threadId: 'conv-1' });

    const yields = await drain(
      adapter.add({ file: file() }) as AsyncGenerator<PendingAttachment, void>,
    );

    expect(yields.length).toBeGreaterThan(1);
    const ids = new Set(yields.map((a) => a.id));
    expect(ids.size).toBe(1);
  });

  it('ends ready-for-send and hands the asset back under that same id', async () => {
    const adapter = createAuraAttachmentAdapter({ threadId: 'conv-1' });

    const yields = await drain(
      adapter.add({ file: file() }) as AsyncGenerator<PendingAttachment, void>,
    );
    const last = yields[yields.length - 1];

    expect(last?.status).toEqual({ type: 'requires-action', reason: 'composer-send' });
    // The client id is NOT the asset id; the map is what bridges them for the run envelope.
    expect(last?.id).not.toBe('asset-1');
    expect(adapter.assetFor(last?.id ?? '')?.id).toBe('asset-1');
  });

  it('reports a refused asset as an error the operator can read, not as ready', async () => {
    pollUntilReady.mockResolvedValue(asset({ status: 'refused', error_message: 'troppo grande' }));
    const adapter = createAuraAttachmentAdapter({ threadId: 'conv-1' });

    const yields = await drain(
      adapter.add({ file: file() }) as AsyncGenerator<PendingAttachment, void>,
    );
    const last = yields[yields.length - 1];

    expect(last?.status).toEqual({
      type: 'incomplete',
      reason: 'error',
      message: 'troppo grande',
    });
    expect(adapter.assetFor(last?.id ?? '')).toBeUndefined();
  });

  it('surfaces a presign failure on the same attachment rather than dropping it', async () => {
    presignAsset.mockRejectedValue(new Error('presign refused'));
    const adapter = createAuraAttachmentAdapter({ threadId: 'conv-1' });

    const yields = await drain(
      adapter.add({ file: file() }) as AsyncGenerator<PendingAttachment, void>,
    );

    expect(new Set(yields.map((a) => a.id)).size).toBe(1);
    expect(yields[yields.length - 1]?.status).toEqual({
      type: 'incomplete',
      reason: 'error',
      message: 'presign refused',
    });
  });

  it('deletes the asset on remove and forgets it', async () => {
    deleteAsset.mockResolvedValue(asset({ status: 'deleted' }));
    const adapter = createAuraAttachmentAdapter({ threadId: 'conv-1' });
    const yields = await drain(
      adapter.add({ file: file() }) as AsyncGenerator<PendingAttachment, void>,
    );
    const attachment = yields[yields.length - 1];
    if (attachment === undefined) throw new Error('expected an attachment');

    await adapter.remove(attachment);

    expect(deleteAsset).toHaveBeenCalledWith('asset-1');
    expect(adapter.assetFor(attachment.id)).toBeUndefined();
  });

  it('completes without inlining content — Aura references the asset by id', async () => {
    const adapter = createAuraAttachmentAdapter({ threadId: 'conv-1' });
    const yields = await drain(
      adapter.add({ file: file() }) as AsyncGenerator<PendingAttachment, void>,
    );
    const attachment = yields[yields.length - 1];
    if (attachment === undefined) throw new Error('expected an attachment');

    const complete = await adapter.send(attachment);

    expect(complete.status).toEqual({ type: 'complete' });
    expect(complete.content).toEqual([]);
    expect(complete.id).toBe(attachment.id);
  });
});
