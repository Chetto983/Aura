import { lazy, type ReactNode } from 'react';
import type { PreviewKind } from '../artifactMeta';
import type { RendererProps } from './PreviewStatus';
import EditInCraft from './EditInCraft';

// previewDispatch — the one place a PreviewKind becomes a renderer. The cockpit modal and the
// share page previewed the same assets through two identical switches; a seventh kind (video)
// made them long enough for the duplication gate to fail, which is the gate doing its job:
// a kind added to one and forgotten in the other is a silent blank frame on that surface.
//
// Each renderer keeps its own lazy boundary (D-08): the dynamic import() holds its bytes — and
// docx's heavy parser — out of the importing chunk until an asset of that kind is opened.
// Spreadsheets and decks open in their Craft app; documents, images and video keep a light
// viewer, and their Craft app edits them on request (EditInCraft). Callers differ only in what
// "no safe renderer" looks like, so that arm is a prop. This is a component rather than a plain
// function because the file defines the lazy renderers, and a module that exports a
// non-component alongside them loses fast refresh (only-export-components, the same rule
// assetSourceContext.ts documents).

const ImagePreview = lazy(() => import('./ImagePreview'));
const PdfPreview = lazy(() => import('./PdfPreview'));
const TextPreview = lazy(() => import('./TextPreview'));
const HtmlPreview = lazy(() => import('./HtmlPreview'));
const DocxPreview = lazy(() => import('./DocxPreview'));
const CraftFrame = lazy(() => import('./CraftFrame'));
const VideoPreview = lazy(() => import('./VideoPreview'));
const AudioPreview = lazy(() => import('./AudioPreview'));

export interface PreviewByKindProps {
  readonly kind: PreviewKind;
  readonly asset: RendererProps;
  /** What to show for a kind no renderer may open — SVG chief among them. */
  readonly downloadFallback: ReactNode;
}

export function PreviewByKind({ kind, asset, downloadFallback }: PreviewByKindProps): ReactNode {
  switch (kind) {
    case 'image':
      return (
        <EditInCraft app="photocraft" asset={asset}>
          <ImagePreview {...asset} />
        </EditInCraft>
      );
    case 'pdf':
      return (
        <EditInCraft app="pdfcraft" asset={asset}>
          <PdfPreview {...asset} />
        </EditInCraft>
      );
    case 'text':
      return <TextPreview {...asset} />;
    case 'html':
      return <HtmlPreview {...asset} />;
    case 'docx':
      return (
        <EditInCraft app="wordcraft" asset={asset}>
          <DocxPreview {...asset} />
        </EditInCraft>
      );
    case 'xlsx':
      return <CraftFrame app="gridcraft" {...asset} />;
    case 'pptx':
      return <CraftFrame app="deckcraft" {...asset} />;
    case 'video':
      return (
        <EditInCraft app="filmcraft" asset={asset}>
          <VideoPreview {...asset} />
        </EditInCraft>
      );
    case 'audio':
      return <AudioPreview {...asset} />;
    case 'download':
      return downloadFallback;
  }
}
