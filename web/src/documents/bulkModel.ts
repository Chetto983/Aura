import type { DocumentRow, Facets, FilterField } from './documentsApi';
import { valuesFor } from './documentsModel';

/** One value of a bulk diff, as the API's docfields.Diff carries it. */
export interface BulkValue {
  readonly field: string;
  readonly value_key: string;
}

export interface BulkDiff {
  readonly set: readonly BulkValue[];
  readonly remove: readonly BulkValue[];
}

/** What the operator touched in the edit bar. A single field holds the label id to set on every
 * document, or null to clear it; absent leaves it alone. Tags hold a change per tag. */
export interface BulkPending {
  readonly type?: string | null;
  readonly counterparty?: string | null;
  readonly tags: Readonly<Record<string, 'add' | 'remove'>>;
}

export const NO_PENDING: BulkPending = { tags: {} };

export type ValueState = 'all' | 'some' | 'none';

/** valueState is how many of the selected documents hold a value: the three states the edit
 * bar shows beside each one. */
export function valueState(
  rows: readonly DocumentRow[],
  field: FilterField,
  id: string,
): { readonly state: ValueState; readonly count: number } {
  const count = rows.filter((row) => valuesFor(row, field).some((v) => v.value_key === id)).length;
  const state: ValueState = count === 0 ? 'none' : count === rows.length ? 'all' : 'some';
  return { state, count };
}

const VALUE_FIELD: Readonly<Record<FilterField, string>> = {
  type: 'type',
  counterparty: 'counterparty',
  tags: 'tag',
};

/** The values a selection holds on a field, which clearing the field removes. */
export type HeldValues = (field: FilterField) => readonly string[];

/** heldInRows reads them from the selected documents, when they are all loaded. */
export function heldInRows(rows: readonly DocumentRow[]): HeldValues {
  return (field) => [
    ...new Set(rows.flatMap((row) => valuesFor(row, field).map((v) => v.value_key))),
  ];
}

/** heldInFacets reads them from the counts of a filter's whole result: a filter selection's
 * documents are not all loaded. */
export function heldInFacets(facets: Facets): HeldValues {
  return (field) => Object.keys(facets[field] ?? {}).filter((id) => (facets[field]?.[id] ?? 0) > 0);
}

/** bulkDiff is the difference the edit bar sends: only the fields the operator touched. Clearing
 * a single field removes every value the selection holds on it. */
export function bulkDiff(pending: BulkPending, held: HeldValues): BulkDiff {
  const set: BulkValue[] = [];
  const remove: BulkValue[] = [];
  for (const field of ['type', 'counterparty'] as const) {
    const target = pending[field];
    if (target === undefined) continue;
    if (target !== null) {
      set.push({ field: VALUE_FIELD[field], value_key: target });
      continue;
    }
    for (const key of [...held(field)].sort())
      remove.push({ field: VALUE_FIELD[field], value_key: key });
  }
  for (const [tag, change] of Object.entries(pending.tags)) {
    (change === 'add' ? set : remove).push({ field: VALUE_FIELD.tags, value_key: tag });
  }
  return { set, remove };
}

export function isEmptyDiff(diff: BulkDiff): boolean {
  return diff.set.length === 0 && diff.remove.length === 0;
}
