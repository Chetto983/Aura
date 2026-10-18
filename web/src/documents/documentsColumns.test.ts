import type { TFunction } from 'i18next';
import { describe, expect, it } from 'vitest';
import { documentColumns } from './documentsColumns';

const t = ((key: string) => key) as unknown as TFunction;

describe('the list columns', () => {
  it('lets the name take the room left on a wide view', () => {
    const name = documentColumns(t, [], [], true, {}).find((c) => c.id === 'file_name');
    expect(name).toMatchObject({ flexgrow: 2 });
    expect(name?.width).toBeUndefined();
  });

  it('gives the name a width of its own on a phone, where the other columns overflow it', () => {
    const name = documentColumns(t, [], [], false, {}).find((c) => c.id === 'file_name');
    expect(name).toMatchObject({ width: 200 });
    expect(name?.flexgrow).toBeUndefined();
  });

  it('lets the operator resize every column but the selection', () => {
    const columns = documentColumns(t, [], [], true, {});
    expect(columns.filter((c) => c.resize === true).map((c) => c.id)).toEqual([
      'file_name',
      'type',
      'counterparty',
      'document_date',
      'tags',
      'status',
    ]);
  });

  it('keeps the widths the operator chose, the name included', () => {
    const columns = documentColumns(t, [], [], true, { file_name: 420, counterparty: 260 });
    expect(columns.find((c) => c.id === 'file_name')).toMatchObject({ width: 420 });
    expect(columns.find((c) => c.id === 'file_name')?.flexgrow).toBeUndefined();
    expect(columns.find((c) => c.id === 'counterparty')).toMatchObject({ width: 260 });
    expect(columns.find((c) => c.id === 'type')).toMatchObject({ width: 170 });
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
      {},
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
