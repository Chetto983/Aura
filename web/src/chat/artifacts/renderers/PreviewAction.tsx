import { useContext, type ReactNode } from 'react';
import { createPortal } from 'react-dom';
import { PreviewActionsContext } from './previewActionsContext';

/** Renders its children in the host's header, or nowhere when the host gives no place. */
export function PreviewAction({ children }: { readonly children: ReactNode }) {
  const slot = useContext(PreviewActionsContext);
  return slot === null ? null : createPortal(children, slot);
}
