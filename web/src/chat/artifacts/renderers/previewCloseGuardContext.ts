import { createContext } from 'react';

// An edit a Craft app has not saved lives only in the app, and the host's Esc, click outside or ✕
// would drop it without a word. The host keeps the flag: a renderer reports its unsaved changes,
// the host holds the first close they meet, and the renderer reads `held` to say the next close
// discards them.
export interface PreviewCloseGuard {
  /** Reports whether the preview holds changes a close would drop. */
  readonly setUnsaved: (unsaved: boolean) => void;
  /** A close was held for those changes: the next one closes. */
  readonly held: boolean;
}

/** The host's guard; null where the host has none. */
export const PreviewCloseGuardContext = createContext<PreviewCloseGuard | null>(null);
