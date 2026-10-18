import { useTranslation } from 'react-i18next';
import { Download } from 'lucide-react';
import { useAssetSource } from './assetSourceContext';

// Shared chrome + prop contract for the six lazy preview renderers (D-09). Extracted
// so the per-MIME chunks don't each duplicate the identical loading/error markup — the
// web jscpd gate fails on ANY ≥100-token clone (threshold 0) and CLAUDE.md forbids
// duplication. Carries no heavy deps, so importing it into every renderer chunk is free.

/** The uniform contract PreviewModal hands every renderer: the asset id (for the
 *  authenticated fetch), its SSE mime_type (relabel for the object-URL kinds), and the
 *  file name (alt text / iframe title). A renderer destructures only what it needs. */
export interface RendererProps {
  readonly assetId: string;
  readonly mimeType: string;
  readonly fileName: string;
}

/** The pending state shown while an asset's bytes are in flight. */
export function PreviewLoading() {
  const { t } = useTranslation();
  return (
    <div
      role="status"
      className="flex h-full items-center justify-center p-8 text-sm text-text-muted"
    >
      {t('artifacts.preview.loading')}
    </div>
  );
}

/** The failure state shown when the authenticated fetch or a client parser rejected. */
export function PreviewError({ detail }: { detail?: string }) {
  const { t } = useTranslation();
  return (
    <div
      role="alert"
      className="flex h-full flex-col items-center justify-center gap-2 p-8 text-center"
    >
      <p className="text-sm font-medium text-danger">{t('artifacts.preview.error')}</p>
      {detail !== undefined ? <p className="text-xs text-text-faint">{detail}</p> : null}
    </div>
  );
}

/** The failure state with the way out: the original file, from the active tier's asset URL, so
 *  a share page stays token scoped. */
export function PreviewErrorWithDownload({
  assetId,
  fileName,
  detail,
}: {
  readonly assetId: string;
  readonly fileName: string;
  readonly detail?: string;
}) {
  const { t } = useTranslation();
  const { assetUrl } = useAssetSource();
  return (
    <div className="flex flex-col gap-2">
      <PreviewError {...(detail === undefined ? {} : { detail })} />
      <a
        href={assetUrl(assetId)}
        download={fileName}
        data-required-touch-target
        className="inline-flex min-h-11 min-w-11 w-fit items-center gap-2 rounded-[var(--radius-sm)] border border-border px-3 text-sm text-accent-text focus-visible:outline-2 focus-visible:outline-accent"
        aria-label={t('artifacts.preview.download', { name: fileName })}
      >
        <Download aria-hidden className="size-4" />
        {t('display.artifact.download')}
      </a>
    </div>
  );
}
