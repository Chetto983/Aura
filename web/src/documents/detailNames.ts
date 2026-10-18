import type { TFunction } from 'i18next';
import type { CustomField } from './documentsApi';
import { displayDate, LIST_OF, PROPOSED_PREFIX, type LabelNames } from './documentsModel';

const CUSTOM_PREFIX = 'custom:';

/** How the detail names a field and a value to the operator. */
export interface DetailNames {
  readonly field: (field: string) => string;
  readonly value: (field: string, key: string) => string;
}

export function detailNames(
  names: LabelNames,
  customFields: readonly CustomField[],
  t: TFunction,
  language: string,
): DetailNames {
  const custom = (field: string) => customFields.find((cf) => `${CUSTOM_PREFIX}${cf.id}` === field);
  const fixed: Readonly<Record<string, string>> = {
    type: t('documents.columns.type'),
    counterparty: t('documents.columns.counterparty'),
    document_date: t('documents.detail.date'),
    tag: t('documents.detail.tags'),
  };
  return {
    field: (field) => fixed[field] ?? custom(field)?.name ?? field,
    value: (field, key) => {
      const list = LIST_OF[field];
      if (list !== undefined) {
        if (key.startsWith(PROPOSED_PREFIX)) {
          return t('documents.detail.proposed', { name: key.slice(PROPOSED_PREFIX.length) });
        }
        return names[list].get(key) ?? t('documents.unknownLabel');
      }
      if (field === 'document_date' || custom(field)?.data_type === 'date') {
        return displayDate(key, language);
      }
      return key;
    },
  };
}
