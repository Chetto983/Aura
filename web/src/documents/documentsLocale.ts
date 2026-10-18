import { en as coreEn, it as coreIt } from '@svar-ui/core-locales';

// SVAR ships Italian for the core words only; the grid's own words come in English and Chinese
// (@svar-ui/grid-locales/locales/en.js), so the Italian ones are written here. The keys ARE the
// English strings, the component's lookup contract: a key that drifts from upstream falls back to
// English rather than failing.
const gridIt = {
  'Add before': 'Aggiungi prima',
  'Add after': 'Aggiungi dopo',
  Copy: 'Copia',
  Cut: 'Taglia',
  Paste: 'Incolla',
  Delete: 'Elimina',
  'New row': 'Nuova riga',
  'Move up': 'Sposta su',
  'Move down': 'Sposta giù',
  Undo: 'Annulla',
  Redo: 'Ripeti',
  selected: 'selezionati',
};

/** The word pack for one cockpit language. English is the component's own default. */
export function documentsWords(language: string): Record<string, unknown> {
  if (!language.toLowerCase().startsWith('it')) return { ...coreEn };
  return { ...coreIt, grid: gridIt };
}
