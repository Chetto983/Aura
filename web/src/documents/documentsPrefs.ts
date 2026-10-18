import type { ColumnWidths } from './documentsColumns';

// The list's preferences each viewer keeps in this browser: the custom fields shown as columns
// and the widths the columns were dragged to. Storage is optional, so a refusal changes nothing
// but what the next visit remembers, and a value this code did not write reads as nothing.

const SHOWN_FIELDS_KEY = 'aura.documents.fields';
const WIDTHS_KEY = 'aura.documents.widths';

function read(key: string, fallback: string): unknown {
  try {
    return JSON.parse(localStorage.getItem(key) ?? fallback) as unknown;
  } catch {
    return undefined;
  }
}

function write(key: string, value: unknown): void {
  try {
    localStorage.setItem(key, JSON.stringify(value));
  } catch {
    // Storage is optional; the view still changes.
  }
}

export function readShownFields(): readonly string[] {
  const parsed = read(SHOWN_FIELDS_KEY, '[]');
  return Array.isArray(parsed) ? parsed.filter((v): v is string => typeof v === 'string') : [];
}

export function saveShownFields(ids: readonly string[]): void {
  write(SHOWN_FIELDS_KEY, ids);
}

export function readWidths(): ColumnWidths {
  const parsed = read(WIDTHS_KEY, '{}');
  if (parsed === null || typeof parsed !== 'object' || Array.isArray(parsed)) return {};
  return Object.fromEntries(
    Object.entries(parsed).filter(
      (entry): entry is [string, number] => typeof entry[1] === 'number' && entry[1] > 0,
    ),
  );
}

/** saveWidth records one column's width and returns the widths with it. */
export function saveWidth(widths: ColumnWidths, id: string, width: number): ColumnWidths {
  const next = { ...widths, [id]: Math.round(width) };
  write(WIDTHS_KEY, next);
  return next;
}
