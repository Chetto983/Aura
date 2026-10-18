import type {
  DocumentRow,
  DocumentsFilter,
  FieldValue,
  FilterField,
  Label,
  LabelKind,
} from './documentsApi';
import { FILTER_FIELDS } from './documentsApi';

/** A label id read from a proposed counterparty: a name the extractor found on no list. */
export const PROPOSED_PREFIX = 'new:';

/** The label kind a filter field takes its values from. */
export const KIND_OF: Readonly<Record<FilterField, LabelKind>> = {
  type: 'type',
  counterparty: 'counterparty',
  tags: 'tag',
};

/** The value field a filter field reads on a document. */
export const VALUE_FIELD: Readonly<Record<FilterField, string>> = {
  type: 'type',
  counterparty: 'counterparty',
  tags: 'tag',
};

/** The filter field, and so the list, behind a value field that holds label ids. */
export const LIST_OF: Readonly<Partial<Record<string, FilterField>>> = {
  type: 'type',
  counterparty: 'counterparty',
  tag: 'tags',
};

export type LabelNames = Readonly<Record<FilterField, ReadonlyMap<string, string>>>;

export function labelNames(lists: Readonly<Record<FilterField, readonly Label[]>>): LabelNames {
  const names = (labels: readonly Label[]) =>
    new Map(labels.map((label) => [label.id, label.name]));
  return {
    type: names(lists.type),
    counterparty: names(lists.counterparty),
    tags: names(lists.tags),
  };
}

/** valuesFor is a document's values on one column: a filter field, the date or a custom field. */
export function valuesFor(row: DocumentRow, column: string): readonly FieldValue[] {
  const field = column in VALUE_FIELD ? VALUE_FIELD[column as FilterField] : column;
  return row.values.filter((value) => value.field === field);
}

/** displayDate renders an ISO day in the cockpit language, independent of the time zone. */
export function displayDate(iso: string, language: string): string {
  const day = new Date(`${iso}T00:00:00Z`);
  if (Number.isNaN(day.getTime())) return iso;
  return new Intl.DateTimeFormat(language, {
    day: '2-digit',
    month: '2-digit',
    year: 'numeric',
    timeZone: 'UTC',
  }).format(day);
}

/** One active filter, as a chip the operator removes with a click. */
export interface FilterChip {
  readonly key: string;
  readonly field: FilterField | 'date';
  readonly label: string;
  readonly excluded: boolean;
  readonly remove: (filter: DocumentsFilter) => DocumentsFilter;
}

/** filterChips lists every active filter but the search box and the review switch, which show
 * themselves. */
export function filterChips(
  filter: DocumentsFilter,
  names: LabelNames,
  unassignedLabel: string,
  dateLabel: (from: string, to: string) => string,
): readonly FilterChip[] {
  const chips: FilterChip[] = [];
  for (const field of FILTER_FIELDS) {
    for (const side of ['include', 'exclude'] as const) {
      for (const id of filter[side][field]) {
        chips.push({
          key: `${side}:${field}:${id}`,
          field,
          label: names[field].get(id) ?? id,
          excluded: side === 'exclude',
          remove: (f) => ({
            ...f,
            [side]: { ...f[side], [field]: f[side][field].filter((v) => v !== id) },
          }),
        });
      }
    }
    if (filter.unassigned.includes(field)) {
      chips.push({
        key: `unassigned:${field}`,
        field,
        label: unassignedLabel,
        excluded: false,
        remove: (f) => ({ ...f, unassigned: f.unassigned.filter((v) => v !== field) }),
      });
    }
  }
  if (filter.dateFrom !== '' || filter.dateTo !== '') {
    chips.push({
      key: 'date',
      field: 'date',
      label: dateLabel(filter.dateFrom, filter.dateTo),
      excluded: false,
      remove: (f) => ({ ...f, dateFrom: '', dateTo: '' }),
    });
  }
  return chips;
}
