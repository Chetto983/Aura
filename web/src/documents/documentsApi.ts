import { getJSON } from '@/api/json';

/**
 * The Documenti view's client (internal/agui/documents_api.go): the library with the fields the
 * operator keeps on each document, and the lists those fields take their values from. Filters
 * carry label ids; the view resolves them to names through the lists.
 */
export const documentsBase = '/api/documents';

/** The three fields a filter picks values for. Its values sit on a document as "type",
 * "counterparty" and "tag". */
export type FilterField = 'type' | 'counterparty' | 'tags';

export const FILTER_FIELDS: readonly FilterField[] = ['type', 'counterparty', 'tags'];

/** One value on a document, with who set it (docfields.Value). */
export interface FieldValue {
  readonly field: string;
  readonly value_key: string;
  readonly value?: unknown;
  readonly set_by: string;
  readonly accepted: boolean;
  readonly evidence?: string;
  readonly set_at: string;
}

export interface DocumentRow {
  readonly id: string;
  readonly source_key: string;
  readonly file_name: string;
  readonly size_bytes: number;
  readonly indexed_at: string;
  readonly record_id?: string;
  readonly values: readonly FieldValue[];
  readonly reviewed_at?: string;
  readonly needs_review: boolean;
}

/** A count per value of each field in the current result, plus how many documents leave each
 * field unassigned. */
export type Facets = Readonly<Record<string, Readonly<Record<string, number>>>>;

export interface DocumentsPage {
  readonly rows: readonly DocumentRow[];
  readonly total: number;
  readonly facets: Facets;
}

export type LabelKind = 'type' | 'counterparty' | 'tag';

export interface Label {
  readonly id: string;
  readonly kind: LabelKind;
  readonly name: string;
  readonly match_algorithm: string;
  readonly match_pattern: string;
  readonly case_insensitive: boolean;
}

export interface CustomField {
  readonly id: string;
  readonly name: string;
  readonly data_type: 'date' | 'monetary' | 'integer' | 'text' | 'select';
  readonly options?: readonly string[];
}

type PerField = Readonly<Record<FilterField, readonly string[]>>;

/** The filter bar's state. include and exclude hold label ids per field; unassigned names the
 * fields that must be empty. */
export interface DocumentsFilter {
  readonly include: PerField;
  readonly exclude: PerField;
  readonly unassigned: readonly FilterField[];
  readonly tagsMode: 'any' | 'all';
  readonly dateFrom: string;
  readonly dateTo: string;
  readonly search: string;
  readonly needsReview: boolean;
  /** False keeps only the values the operator set or accepted. */
  readonly includeUnreviewed: boolean;
}

const NO_VALUES: PerField = { type: [], counterparty: [], tags: [] };

export const EMPTY_FILTER: DocumentsFilter = {
  include: NO_VALUES,
  exclude: NO_VALUES,
  unassigned: [],
  tagsMode: 'any',
  dateFrom: '',
  dateTo: '',
  search: '',
  needsReview: false,
  includeUnreviewed: true,
};

export interface DocumentsPageRequest {
  readonly sort: string;
  readonly offset: number;
  readonly limit: number;
}

/** documentsQuery spells a filter and a page the way GET /api/documents reads them. */
export function documentsQuery(filter: DocumentsFilter, page: DocumentsPageRequest): string {
  const params = new URLSearchParams();
  for (const field of FILTER_FIELDS) {
    for (const id of filter.include[field]) params.append(field, id);
    for (const id of filter.exclude[field]) params.append(`${field}_not`, id);
  }
  for (const field of filter.unassigned) params.append('unassigned', field);
  if (filter.tagsMode === 'all') params.set('tags_mode', 'all');
  if (filter.dateFrom !== '') params.set('date_from', filter.dateFrom);
  if (filter.dateTo !== '') params.set('date_to', filter.dateTo);
  if (filter.search.trim() !== '') params.set('q', filter.search.trim());
  if (filter.needsReview) params.set('needs_review', 'true');
  if (!filter.includeUnreviewed) params.set('include_unreviewed', 'false');
  if (page.sort !== '') params.set('sort', page.sort);
  params.set('offset', String(page.offset));
  params.set('limit', String(page.limit));
  return params.toString();
}

/** withField adds id to one side of a field, taking it out of the other, or takes it out when it
 * is already there: a value is included, excluded or neither. */
export function withField(
  filter: DocumentsFilter,
  side: 'include' | 'exclude',
  field: FilterField,
  id: string,
): DocumentsFilter {
  const other = side === 'include' ? 'exclude' : 'include';
  const present = filter[side][field].includes(id);
  return {
    ...filter,
    [side]: {
      ...filter[side],
      [field]: present ? filter[side][field].filter((v) => v !== id) : [...filter[side][field], id],
    },
    [other]: { ...filter[other], [field]: filter[other][field].filter((v) => v !== id) },
  };
}

export type ValueSource = 'rule' | 'extractor' | 'copied' | 'classifier' | 'llm';

/** sourceOf names who set a value the operator did not: the mark beside it in the list. */
export function sourceOf(setBy: string): ValueSource | null {
  if (setBy === 'operator') return null;
  if (setBy.startsWith('rule:')) return 'rule';
  if (setBy === 'extractor' || setBy === 'copied' || setBy === 'classifier' || setBy === 'llm') {
    return setBy;
  }
  return null;
}

export function fetchDocuments(
  filter: DocumentsFilter,
  page: DocumentsPageRequest,
  signal?: AbortSignal,
): Promise<DocumentsPage> {
  return getJSON<DocumentsPage>(`${documentsBase}?${documentsQuery(filter, page)}`, signal);
}

/** fetchReviewCount is how many documents of the whole library wait for review. */
export async function fetchReviewCount(signal?: AbortSignal): Promise<number> {
  const page = await getJSON<DocumentsPage>(`${documentsBase}?needs_review=true&limit=1`, signal);
  return page.total;
}

export async function fetchLabels(
  kind: LabelKind,
  signal?: AbortSignal,
): Promise<readonly Label[]> {
  const out = await getJSON<{ labels: readonly Label[] }>(
    `${documentsBase}/labels?kind=${encodeURIComponent(kind)}`,
    signal,
  );
  return out.labels;
}

export async function fetchCustomFields(signal?: AbortSignal): Promise<readonly CustomField[]> {
  const out = await getJSON<{ fields: readonly CustomField[] }>(
    `${documentsBase}/custom-fields`,
    signal,
  );
  return out.fields;
}
