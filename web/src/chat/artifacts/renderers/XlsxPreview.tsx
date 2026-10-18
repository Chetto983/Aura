import { useEffect, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { useAssetContent } from './useAssetContent';
import { PreviewError, type RendererProps } from './PreviewStatus';

// .xlsx: GridCraft (docs/superpowers/specs/2026-10-18-xlsx-viewer-gridcraft-design.md), an
// Excel-style spreadsheet aura serves at /apps/gridcraft/ from the image. Framed in host mode it
// announces gridcraft:ready, and the workbook goes to it as bytes, read through the asset source
// so the share page's token-scoped route works unchanged. The frame is same-origin because the
// app answers only its own origin's parent, so only messages from THIS frame on THIS origin are
// heard. What the operator changes inside is not stored: this is a preview.

const GRIDCRAFT = '/apps/gridcraft/?host=parent';

interface GridcraftMessage {
  readonly type?: string;
  readonly error?: string;
  readonly dirty?: boolean;
}

export default function XlsxPreview({ assetId, fileName }: RendererProps) {
  const { t } = useTranslation();
  const frame = useRef<HTMLIFrameElement>(null);
  const { data, error } = useAssetContent(assetId, 'arrayBuffer');
  const [ready, setReady] = useState(false);
  const [failed, setFailed] = useState<string>();
  const [edited, setEdited] = useState(false);

  useEffect(() => {
    const onMessage = (event: MessageEvent<GridcraftMessage | null>) => {
      if (
        event.source !== frame.current?.contentWindow ||
        event.origin !== window.location.origin
      ) {
        return;
      }
      const message = event.data ?? {};
      if (message.type === 'gridcraft:ready') setReady(true);
      else if (message.type === 'gridcraft:failed') setFailed(message.error ?? '');
      else if (message.type === 'gridcraft:save' || message.dirty === true) setEdited(true);
    };
    window.addEventListener('message', onMessage);
    return () => {
      window.removeEventListener('message', onMessage);
    };
  }, []);

  useEffect(() => {
    const target = frame.current?.contentWindow;
    if (!ready || data === undefined || target === null || target === undefined) return;
    // The bytes are transferred, so the frame gets a copy and the fetched buffer stays whole.
    const bytes = data.slice(0);
    target.postMessage({ type: 'gridcraft:open', name: fileName, bytes }, window.location.origin, [
      bytes,
    ]);
  }, [ready, data, fileName]);

  const problem = error ?? failed;
  if (problem !== undefined) return <PreviewError detail={problem} />;
  return (
    <div className="relative h-full w-full">
      <iframe
        ref={frame}
        src={GRIDCRAFT}
        title={fileName}
        allow="clipboard-read; clipboard-write; fullscreen"
        className="h-full w-full border-0 bg-white"
      />
      {edited && (
        <p
          role="status"
          className="absolute right-3 bottom-3 rounded-md bg-warning px-3 py-1.5 text-xs font-semibold text-bg shadow-lg"
        >
          {t('artifacts.preview.notSaved')}
        </p>
      )}
    </div>
  );
}
