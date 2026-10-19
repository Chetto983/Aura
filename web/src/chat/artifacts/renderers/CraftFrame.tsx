import { useEffect, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { useAssetSource } from './assetSourceContext';
import { useAssetContent } from './useAssetContent';
import { PreviewError, PreviewErrorWithDownload, type RendererProps } from './PreviewStatus';

// A Craft app (docs/superpowers/specs/2026-10-19-craft-web-apps-design.md) aura serves at
// /apps/<app>/ from the image. Framed in host mode it announces <app>:ready, and the file goes to
// it as bytes, read through the asset source so the share page's token-scoped route works
// unchanged. The frame is same-origin because the app answers only its own origin's parent, so
// only messages from THIS frame on THIS origin are heard. A save comes back as <app>:save: where
// the source can write (the file manager's), it goes over the file under its own name, or beside
// it under a new one; elsewhere the frame says that nothing made in it is stored. A frame that has
// not said ready after a while (a 404 outside the image, a wasm the browser refuses) gets the
// original file offered over it; the frame stays, so an app that starts late on a slow link
// still takes over.

export type CraftApp =
  'gridcraft' | 'pdfcraft' | 'wordcraft' | 'deckcraft' | 'photocraft' | 'filmcraft';

const READY_WITHIN_MS = 15_000;

interface CraftMessage {
  readonly type?: string;
  readonly error?: string;
  readonly dirty?: boolean;
  readonly name?: string;
  readonly bytes?: ArrayBuffer;
}

type SaveState =
  | { readonly kind: 'none' }
  | { readonly kind: 'unsaved' }
  | { readonly kind: 'saved' }
  | { readonly kind: 'failed'; readonly message: string };

interface CraftFrameProps extends RendererProps {
  readonly app: CraftApp;
}

export default function CraftFrame({ app, assetId, fileName }: CraftFrameProps) {
  const { t } = useTranslation();
  const frame = useRef<HTMLIFrameElement>(null);
  const { data, error } = useAssetContent(assetId, 'arrayBuffer');
  const [ready, setReady] = useState(false);
  const [late, setLate] = useState(false);
  const [failed, setFailed] = useState<string>();
  const [saving, setSaving] = useState<SaveState>({ kind: 'none' });
  const { save } = useAssetSource();

  useEffect(() => {
    const onMessage = (event: MessageEvent<CraftMessage | null>) => {
      if (
        event.source !== frame.current?.contentWindow ||
        event.origin !== window.location.origin
      ) {
        return;
      }
      const message = event.data ?? {};
      if (message.type === `${app}:ready`) setReady(true);
      else if (message.type === `${app}:failed`) setFailed(message.error ?? '');
      else if (message.type === `${app}:save` && save !== undefined && message.bytes) {
        const saveAs = message.name === fileName ? undefined : message.name;
        save(assetId, message.bytes, saveAs).then(
          () => {
            setSaving({ kind: 'saved' });
          },
          (err: unknown) => {
            setSaving({
              kind: 'failed',
              message: err instanceof Error ? err.message : String(err),
            });
          },
        );
      } else if (save === undefined && (message.type === `${app}:save` || message.dirty === true)) {
        setSaving({ kind: 'unsaved' });
      }
    };
    window.addEventListener('message', onMessage);
    return () => {
      window.removeEventListener('message', onMessage);
    };
  }, [app, assetId, fileName, save]);

  useEffect(() => {
    if (ready) return;
    const timer = window.setTimeout(() => {
      setLate(true);
    }, READY_WITHIN_MS);
    return () => {
      window.clearTimeout(timer);
    };
  }, [ready]);

  useEffect(() => {
    const target = frame.current?.contentWindow;
    if (!ready || data === undefined || target === null || target === undefined) return;
    // The bytes are transferred, so the frame gets a copy and the fetched buffer stays whole.
    const bytes = data.slice(0);
    target.postMessage({ type: `${app}:open`, name: fileName, bytes }, window.location.origin, [
      bytes,
    ]);
  }, [app, ready, data, fileName]);

  const problem = error ?? failed;
  if (problem !== undefined) return <PreviewError detail={problem} />;
  return (
    <div className="relative h-full w-full">
      <iframe
        ref={frame}
        src={`/apps/${app}/?host=parent`}
        title={fileName}
        allow="clipboard-read; clipboard-write; fullscreen"
        className="h-full w-full border-0 bg-white"
      />
      {late && !ready ? (
        <div className="absolute inset-0 flex items-center justify-center bg-bg/90">
          <PreviewErrorWithDownload
            assetId={assetId}
            fileName={fileName}
            detail={t('artifacts.preview.notStarted')}
          />
        </div>
      ) : null}
      {saving.kind === 'failed' ? (
        <p
          role="alert"
          className="absolute right-3 bottom-3 rounded-md bg-danger px-3 py-1.5 text-xs font-semibold text-bg shadow-lg"
        >
          {t('artifacts.preview.saveFailed', { message: saving.message })}
        </p>
      ) : saving.kind !== 'none' ? (
        <p
          role="status"
          className="absolute right-3 bottom-3 rounded-md bg-warning px-3 py-1.5 text-xs font-semibold text-bg shadow-lg"
        >
          {t(saving.kind === 'saved' ? 'artifacts.preview.saved' : 'artifacts.preview.notSaved')}
        </p>
      ) : null}
    </div>
  );
}
