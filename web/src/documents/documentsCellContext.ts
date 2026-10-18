import { createContext } from 'react';
import type { DocumentRow, FilterField } from './documentsApi';
import type { LabelNames } from './documentsModel';

/** What the cells read beside their row: the names behind label ids. */
export const DocumentsCellContext = createContext<LabelNames | null>(null);

/** The grid action a value's click sends; the grid hands it to onFilterValue. */
export const FILTER_VALUE_ACTION = 'filter-value';

export interface FilterValueEvent {
  readonly field: FilterField;
  readonly id: string;
}

/** The selection, as the checkbox cells read and change it. */
export interface SelectionApi {
  readonly isSelected: (id: string) => boolean;
  readonly toggle: (row: DocumentRow) => void;
  /** How much of the current page is selected, for the header's checkbox. */
  readonly pageState: 'all' | 'some' | 'none';
  readonly togglePage: () => void;
}

export const SelectionContext = createContext<SelectionApi | null>(null);
