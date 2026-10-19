import { createContext } from 'react';

// A preview's own actions (a Craft app's Edit, its Full screen) belong in the host's header,
// beside its download and drawn the same way: floated over the viewer, Edit covered the PDF
// viewer's own toolbar. The host gives a place there (an element it keeps in state through a
// callback ref) and a renderer puts its actions in it with PreviewAction.

/** The header element a host holds a preview's actions in; null while it has none. */
export const PreviewActionsContext = createContext<HTMLElement | null>(null);

/** The header's pill: the download link and every preview action share it. */
export const previewActionClass =
  'inline-flex h-9 items-center gap-2 rounded-full border border-border bg-surface px-3.5 text-[13px] font-medium text-accent-text transition-colors hover:border-accent hover:bg-surface-2 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent';
