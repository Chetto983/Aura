import { useSyncExternalStore } from 'react';

// Below this width the list's fixed columns alone overflow the view, and a name column left to
// take the room left gets none (measured live 2026-10-18: 17 px on a 390 px phone).
const NARROW = '(max-width: 767px)';

function subscribe(onChange: () => void): () => void {
  if (typeof window.matchMedia !== 'function') return () => undefined;
  const query = window.matchMedia(NARROW);
  query.addEventListener('change', onChange);
  return () => {
    query.removeEventListener('change', onChange);
  };
}

function isWide(): boolean {
  return typeof window.matchMedia !== 'function' || !window.matchMedia(NARROW).matches;
}

/** useWideView reports a view wide enough for the list's name column to take the room left. */
export function useWideView(): boolean {
  return useSyncExternalStore(subscribe, isWide);
}
