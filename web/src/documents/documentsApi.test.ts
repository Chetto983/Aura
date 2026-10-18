import { describe, expect, it } from 'vitest';
import {
  documentsQuery,
  EMPTY_FILTER,
  sourceOf,
  withField,
  type DocumentsFilter,
} from './documentsApi';

describe('documentsQuery', () => {
  it('sends only the page for an empty filter', () => {
    expect(documentsQuery(EMPTY_FILTER, { sort: '', offset: 0, limit: 50 })).toBe(
      'offset=0&limit=50',
    );
  });

  it('spells every filter the way the API reads it', () => {
    const filter: DocumentsFilter = {
      ...EMPTY_FILTER,
      include: { type: ['t1', 't2'], counterparty: [], tags: ['g1'] },
      exclude: { type: [], counterparty: ['c9'], tags: [] },
      unassigned: ['tags'],
      tagsMode: 'all',
      dateFrom: '2026-01-01',
      dateTo: '2026-03-31',
      search: ' fattura ',
      needsReview: true,
      includeUnreviewed: false,
    };
    const params = new URLSearchParams(
      documentsQuery(filter, { sort: '-indexed_at', offset: 50, limit: 50 }),
    );
    expect(params.getAll('type')).toEqual(['t1', 't2']);
    expect(params.getAll('tags')).toEqual(['g1']);
    expect(params.getAll('counterparty_not')).toEqual(['c9']);
    expect(params.getAll('unassigned')).toEqual(['tags']);
    expect(params.get('tags_mode')).toBe('all');
    expect(params.get('date_from')).toBe('2026-01-01');
    expect(params.get('date_to')).toBe('2026-03-31');
    expect(params.get('q')).toBe('fattura');
    expect(params.get('needs_review')).toBe('true');
    expect(params.get('include_unreviewed')).toBe('false');
    expect(params.get('sort')).toBe('-indexed_at');
    expect(params.get('offset')).toBe('50');
  });
});

describe('withField', () => {
  it('adds a value to a field, takes it out of the opposite list, and toggles it off', () => {
    const excluded: DocumentsFilter = {
      ...EMPTY_FILTER,
      exclude: { ...EMPTY_FILTER.exclude, type: ['t1'] },
    };
    const included = withField(excluded, 'include', 'type', 't1');
    expect(included.include.type).toEqual(['t1']);
    expect(included.exclude.type).toEqual([]);
    expect(withField(included, 'include', 'type', 't1').include.type).toEqual([]);
  });
});

describe('sourceOf', () => {
  it('names who set a value, and nothing for the operator', () => {
    expect(sourceOf('operator')).toBeNull();
    expect(sourceOf('rule:l-1')).toBe('rule');
    expect(sourceOf('extractor')).toBe('extractor');
    expect(sourceOf('copied')).toBe('copied');
    expect(sourceOf('classifier')).toBe('classifier');
    expect(sourceOf('llm')).toBe('llm');
  });
});
