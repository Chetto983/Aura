import { lazy, Suspense, useState, type ReactNode } from 'react';
import { useTranslation } from 'react-i18next';
import { Pencil } from 'lucide-react';
import { useAssetSource } from './assetSourceContext';
import { PreviewLoading, type RendererProps } from './PreviewStatus';
import type { CraftApp } from './CraftFrame';
import { Button } from '@/components/ui/button';

const CraftFrame = lazy(() => import('./CraftFrame'));

// A file a light viewer shows and a Craft app edits (docs/superpowers/specs/2026-10-19-craft-web-apps-design.md):
// the viewer stays the preview, so reading costs no app download, and Edit swaps the app in. It
// is offered only where the source can store what the app saves (the file manager's tier); an
// edit made anywhere else could only be lost.
export default function EditInCraft({
  app,
  asset,
  children,
}: {
  readonly app: CraftApp;
  readonly asset: RendererProps;
  readonly children: ReactNode;
}) {
  const { t } = useTranslation();
  const { save } = useAssetSource();
  const [editing, setEditing] = useState(false);
  if (editing) {
    return (
      <Suspense fallback={<PreviewLoading />}>
        <CraftFrame app={app} {...asset} />
      </Suspense>
    );
  }
  if (save === undefined) return children;
  return (
    <div className="relative h-full w-full">
      {children}
      {/* Solid, not outline: it floats over the viewer, and over a white PDF page a transparent
          button could not be seen. */}
      <Button
        type="button"
        variant="default"
        className="absolute top-3 right-3 shadow-lg"
        onClick={() => {
          setEditing(true);
        }}
      >
        <Pencil data-icon aria-hidden="true" className="size-4" />
        {t('artifacts.preview.edit')}
      </Button>
    </div>
  );
}
