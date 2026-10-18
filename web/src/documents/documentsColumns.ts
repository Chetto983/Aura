import type { TFunction } from 'i18next';
import type { IColumnConfig } from '@svar-ui/react-grid';
import type { CustomField } from './documentsApi';
import { CustomCell, DateCell, FieldCell, NameCell, StatusCell, TagsCell } from './DocumentsCells';
import { SelectCell, SelectPageCell } from './SelectionCells';

/**
 * The list's columns (mockup 1): the selection, the name, the fields, the custom fields the
 * operator switched on, the tags and the review state. On a wide view the name takes the room the
 * others leave; on a narrow one the others alone overflow it, so the name holds a width of its own
 * and the row scrolls sideways (measured live 2026-10-18: a 17 px name on a 390 px phone).
 */
export function documentColumns(
  t: TFunction,
  customFields: readonly CustomField[],
  shownFields: readonly string[],
  wide: boolean,
): IColumnConfig[] {
  const custom = customFields
    .filter((cf) => shownFields.includes(cf.id))
    .map((cf) => ({ id: `custom:${cf.id}`, header: cf.name, width: 150, cell: CustomCell }));
  return [
    { id: 'select', header: [{ cell: SelectPageCell }], width: 52, cell: SelectCell },
    {
      id: 'file_name',
      header: t('documents.columns.name'),
      ...(wide ? { flexgrow: 2 } : { width: 200 }),
      sort: true,
      cell: NameCell,
    },
    { id: 'type', header: t('documents.columns.type'), width: 170, cell: FieldCell },
    {
      id: 'counterparty',
      header: t('documents.columns.counterparty'),
      width: 200,
      cell: FieldCell,
    },
    {
      id: 'document_date',
      header: t('documents.columns.date'),
      width: 130,
      sort: true,
      cell: DateCell,
    },
    ...custom,
    { id: 'tags', header: t('documents.columns.tags'), width: 200, cell: TagsCell },
    { id: 'status', header: t('documents.columns.status'), width: 140, cell: StatusCell },
  ];
}
