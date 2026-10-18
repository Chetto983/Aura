import { useContext } from 'react';
import { useTranslation } from 'react-i18next';
import type { ICellProps } from '@svar-ui/react-grid';
import type { DocumentRow } from './documentsApi';
import { SelectionContext } from './documentsCellContext';
import { Checkbox } from '@/components/ui/checkbox';

// The grid selects nothing on a row click (select={false}): the checkboxes are the selection, and
// data-action="ignore-click" keeps a click on one from reaching the row (SVAR, "Selecting rows").

export function SelectCell({ row }: ICellProps) {
  const { t } = useTranslation();
  const selection = useContext(SelectionContext);
  const doc = row as unknown as DocumentRow;
  if (selection === null) return null;
  return (
    <div data-action="ignore-click" className="grid h-full place-items-center">
      <Checkbox
        aria-label={t('documents.bulk.selectRow', { name: doc.file_name })}
        checked={selection.isSelected(doc.id)}
        onCheckedChange={() => {
          selection.toggle(doc);
        }}
      />
    </div>
  );
}

export function SelectPageCell() {
  const { t } = useTranslation();
  const selection = useContext(SelectionContext);
  if (selection === null) return null;
  return (
    <div data-action="ignore-click" className="grid h-full place-items-center">
      <Checkbox
        aria-label={t('documents.bulk.selectPage')}
        checked={
          selection.pageState === 'all'
            ? true
            : selection.pageState === 'some'
              ? 'indeterminate'
              : false
        }
        onCheckedChange={selection.togglePage}
      />
    </div>
  );
}
