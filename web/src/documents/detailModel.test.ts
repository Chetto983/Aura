import { describe, expect, it } from 'vitest';
import type { DocumentRow, FieldValue } from './documentsApi';
import { formDiff, formOf, suggestionsOf } from './detailModel';

const value = (field: string, key: string, setBy: string, accepted = false): FieldValue => ({
  field,
  value_key: key,
  set_by: setBy,
  accepted,
  set_at: '2026-10-18T10:00:00Z',
});

const doc: DocumentRow = {
  id: 'd1',
  source_key: 'chat/a.pdf',
  file_name: 'a.pdf',
  size_bytes: 1,
  indexed_at: '2026-10-18 10:00:00.000',
  needs_review: true,
  values: [
    value('type', 't-fattura', 'rule:t-fattura'),
    value('counterparty', 'c-gialli', 'extractor', true),
    value('custom:cf-num', 'A-17', 'operator'),
    value('tag', 'g-ricambi', 'operator'),
    value('tag', 'g-urgente', 'rule:g-urgente'),
  ],
};

describe('detail form', () => {
  it('holds what the operator set or accepted, and leaves suggestions out of it', () => {
    expect(formOf(doc)).toEqual({
      single: { counterparty: 'c-gialli', 'custom:cf-num': 'A-17' },
      tags: ['g-ricambi'],
    });
    expect(suggestionsOf(doc, 'type').map((v) => v.value_key)).toEqual(['t-fattura']);
    expect(suggestionsOf(doc, 'tag').map((v) => v.value_key)).toEqual(['g-urgente']);
    expect(suggestionsOf(doc, 'counterparty')).toEqual([]);
  });

  it('sends nothing for a form left as it was', () => {
    expect(formDiff(doc, formOf(doc))).toEqual({ set: [], remove: [] });
  });

  it('removes a cleared value, and adds and removes tags', () => {
    const form = formOf(doc);
    const changed = {
      single: { ...form.single, counterparty: '', type: 't-contratto' },
      tags: ['g-urgente'],
    };
    expect(formDiff(doc, changed)).toEqual({
      set: [
        { field: 'type', value_key: 't-contratto' },
        { field: 'tag', value_key: 'g-urgente' },
      ],
      remove: [
        { field: 'counterparty', value_key: 'c-gialli' },
        { field: 'tag', value_key: 'g-ricambi' },
      ],
    });
  });
});
