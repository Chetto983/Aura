import type { TFunction } from 'i18next';
import { describe, expect, it } from 'vitest';
import { documentColumns } from './documentsColumns';

const t = ((key: string) => key) as unknown as TFunction;

describe('the list columns', () => {
  it('lets the name take the room left on a wide view', () => {
    const name = documentColumns(t, [], [], true).find((c) => c.id === 'file_name');
    expect(name).toMatchObject({ flexgrow: 2 });
    expect(name?.width).toBeUndefined();
  });

  it('gives the name a width of its own on a phone, where the other columns overflow it', () => {
    const name = documentColumns(t, [], [], false).find((c) => c.id === 'file_name');
    expect(name).toMatchObject({ width: 200 });
    expect(name?.flexgrow).toBeUndefined();
  });

  it('adds the custom fields the operator switched on, before the tags', () => {
    const ids = documentColumns(
      t,
      [
        { id: 'cf-a', name: 'Numero', data_type: 'text' },
        { id: 'cf-b', name: 'Stato', data_type: 'text' },
      ],
      ['cf-b'],
      true,
    ).map((c) => c.id);
    expect(ids).toEqual([
      'select',
      'file_name',
      'type',
      'counterparty',
      'document_date',
      'custom:cf-b',
      'tags',
      'status',
    ]);
  });
});
