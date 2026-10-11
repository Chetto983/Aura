import { useEffect, useState } from 'react';
import { highlightCode } from '../../displays/shiki';
import { useAssetContent } from './useAssetContent';
import { PreviewError, PreviewLoading } from './PreviewStatus';
import { useThemeMode } from '@/theme/useThemeMode';
import { cn } from '@/lib/utils';

// An asset's text, shown React-escaped in a <pre> at once and upgraded to Shiki's escaped-span
// HTML when its lang is in the allow-list (displays/shiki.ts): Shiki tokenizes as text and never
// emits live markup from the source, so the bytes are shown, never parsed or run (T-37B-10). An
// unknown lang keeps the plain <pre>. Both the text preview (lang from the file extension) and an
// HTML artifact's source tab use it.

interface HighlightedSourceProps {
  readonly assetId: string;
  /** A Shiki lang or file extension; resolveLang decides whether it is highlighted. */
  readonly lang: string;
  /** Wrap long lines: a viewed file reads as a document, a page's markup as code. */
  readonly wrap?: boolean;
}

export function HighlightedSource({ assetId, lang, wrap = false }: HighlightedSourceProps) {
  const { data, error } = useAssetContent(assetId, 'text');
  const dark = useThemeMode() === 'dark';
  const [highlighted, setHighlighted] = useState<{ source: string; html: string } | null>(null);
  useEffect(() => {
    if (data === undefined) return;
    let cancelled = false;
    void highlightCode(data, lang, dark).then(
      (html) => {
        if (!cancelled && html !== null) setHighlighted({ source: data, html });
      },
      () => undefined,
    );
    return () => {
      cancelled = true;
    };
  }, [data, lang, dark]);
  if (error !== undefined) return <PreviewError detail={error} />;
  if (data === undefined) return <PreviewLoading />;
  const layout = cn(
    'h-full overflow-auto p-4 font-mono',
    wrap
      ? 'whitespace-pre-wrap break-words text-sm [&_pre]:whitespace-pre-wrap [&_pre]:break-words'
      : 'text-xs leading-relaxed',
  );
  if (highlighted?.source === data)
    return (
      <div
        className={cn(layout, '[&_pre]:m-0 [&_pre]:!bg-transparent')}
        dangerouslySetInnerHTML={{ __html: highlighted.html }}
      />
    );
  return <pre className={cn(layout, 'text-text')}>{data}</pre>;
}
