import { createContext } from 'react';
import type { FilterField } from './documentsApi';
import type { LabelNames } from './documentsModel';

/** What the cells read beside their row: the names behind label ids. */
export const DocumentsCellContext = createContext<LabelNames | null>(null);

/** The grid action a value's click sends; the grid hands it to onFilterValue. */
export const FILTER_VALUE_ACTION = 'filter-value';

export interface FilterValueEvent {
  readonly field: FilterField;
  readonly id: string;
}
