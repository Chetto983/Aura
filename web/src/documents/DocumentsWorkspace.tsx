import { useCallback, useMemo, useState, type ReactNode } from 'react';
import { keepPreviousData, useQuery } from '@tanstack/react-query';
import { ListTree } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { Grid, Willow, WillowDark, type IApi, type IColumnConfig } from '@svar-ui/react-grid';
import { Locale, Pager } from '@svar-ui/react-core';
import '@/styles/svarWidgets';
import {
  EMPTY_FILTER,
  fetchDocuments,
  fetchReviewCount,
  withField,
  type DocumentRow,
  type DocumentsFilter,
  type Facets,
} from './documentsApi';
import { BulkBar } from './BulkBar';
import { BulkConfirmDialog } from './BulkConfirmDialog';
import { CustomCell, DateCell, FieldCell, NameCell, StatusCell, TagsCell } from './DocumentsCells';
import {
  DocumentsCellContext,
  SelectionContext,
  type FilterValueEvent,
  type OpenDocumentEvent,
} from './documentsCellContext';
import { DocumentDetail } from './DocumentDetail';
import { documentsWords } from './documentsLocale';
import { FilterBar } from './FilterBar';
import { ListsPage } from './ListsPage';
import { SelectCell, SelectPageCell } from './SelectionCells';
import { useBulkEdit } from './useBulkEdit';
import { useDocumentLists } from './useDocumentLists';
import { Button } from '@/components/ui/button';
import { useThemeMode } from '@/theme/useThemeMode';

const PAGE_SIZE = 50;
const SHOWN_FIELDS_KEY = 'aura.documents.fields';

/** The columns a header click sorts, each by the API's own sort key. */
const SORTABLE: Readonly<Record<string, string>> = {
  file_name: 'file_name',
  document_date: 'document_date',
};

function readShownFields(): readonly string[] {
  try {
    const parsed: unknown = JSON.parse(localStorage.getItem(SHOWN_FIELDS_KEY) ?? '[]');
    return Array.isArray(parsed) ? parsed.filter((v): v is string => typeof v === 'string') : [];
  } catch {
    return [];
  }
}

const NO_ROWS: readonly DocumentRow[] = [];
const NO_FACETS: Facets = {};

interface DocumentsWorkspaceProps {
  readonly mobileMenu?: ReactNode;
}

/**
 * The Documenti view's list (spec decision 4, mockup 1): every indexed document with the fields
 * the operator keeps on it, filtered by the bar above and paged on the server. A value's click
 * filters by it; the "Da rivedere" counter shows the documents a suggestion waits on.
 */
export default function DocumentsWorkspace({ mobileMenu }: DocumentsWorkspaceProps) {
  const { t, i18n } = useTranslation();
  const [filter, setFilterState] = useState<DocumentsFilter>(EMPTY_FILTER);
  const [sort, setSort] = useState('');
  const [offset, setOffset] = useState(0);
  const [shownFields, setShownFieldsState] = useState(readShownFields);
  const [openId, setOpenId] = useState<string | null>(null);
  const [listsOpen, setListsOpen] = useState(false);

  const setFilter = useCallback((next: DocumentsFilter) => {
    setFilterState(next);
    setOffset(0);
  }, []);
  const setShownFields = useCallback((ids: readonly string[]) => {
    setShownFieldsState(ids);
    try {
      localStorage.setItem(SHOWN_FIELDS_KEY, JSON.stringify(ids));
    } catch {
      // Storage is optional; the columns still change.
    }
  }, []);

  const { lists, names, customFields } = useDocumentLists();
  const reviewCount = useQuery({
    queryKey: ['documents-review'],
    queryFn: ({ signal }) => fetchReviewCount(signal),
  });
  const page = useQuery({
    queryKey: ['documents', filter, sort, offset],
    queryFn: ({ signal }) => fetchDocuments(filter, { sort, offset, limit: PAGE_SIZE }, signal),
    placeholderData: keepPreviousData,
  });

  const total = page.data?.total ?? 0;
  const rows = page.data?.rows ?? NO_ROWS;
  const facets = page.data?.facets ?? NO_FACETS;
  const bulk = useBulkEdit({ rows, total, filter, facets });
  // A selection belongs to the result it was made in: a new filter drops it.
  const changeFilter = (next: DocumentsFilter) => {
    bulk.clear();
    setFilter(next);
  };

  const columns = useMemo<IColumnConfig[]>(() => {
    const custom = customFields
      .filter((cf) => shownFields.includes(cf.id))
      .map((cf) => ({ id: `custom:${cf.id}`, header: cf.name, width: 150, cell: CustomCell }));
    return [
      { id: 'select', header: [{ cell: SelectPageCell }], width: 52, cell: SelectCell },
      {
        id: 'file_name',
        header: t('documents.columns.name'),
        flexgrow: 2,
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
  }, [t, customFields, shownFields]);

  // The library is sorted on the server, a page at a time: a header click becomes the query's
  // sort, and the grid's own sort is stopped (an intercept that answers false ends the action).
  const init = useCallback((api: IApi) => {
    api.intercept('sort-rows', (ev) => {
      const key = SORTABLE[String(ev.key)];
      if (key !== undefined) {
        setSort((current) => (current === key ? `-${key}` : key));
        setOffset(0);
      }
      return false;
    });
  }, []);
  const sortMarks = useMemo(() => {
    const key = sort.replace(/^-/, '');
    const column = Object.keys(SORTABLE).find((id) => SORTABLE[id] === key);
    return column === undefined
      ? {}
      : { [column]: { order: sort.startsWith('-') ? ('desc' as const) : ('asc' as const) } };
  }, [sort]);

  const Theme = useThemeMode() === 'light' ? Willow : WillowDark;
  const review = reviewCount.data ?? 0;

  if (openId !== null) {
    return (
      <DocumentDetail
        id={openId}
        onClose={() => {
          setOpenId(null);
        }}
        onOpen={setOpenId}
      />
    );
  }
  if (listsOpen) {
    return (
      <ListsPage
        onClose={() => {
          setListsOpen(false);
        }}
        onFilter={(field, id) => {
          setListsOpen(false);
          changeFilter(withField(EMPTY_FILTER, 'include', field, id));
        }}
      />
    );
  }
  return (
    <section
      aria-label={t('documents.title')}
      className="relative flex h-full min-h-0 min-w-0 flex-col bg-bg"
    >
      <header className="flex flex-wrap items-center gap-2 border-b border-border px-3 py-2">
        <span className="md:hidden">{mobileMenu}</span>
        <h1 className="text-[15px] font-semibold text-text">{t('documents.title')}</h1>
        <span className="text-sm text-text-muted">{t('documents.total', { count: total })}</span>
        <Button
          type="button"
          size="sm"
          variant="ghost"
          className="ml-auto gap-2"
          onClick={() => {
            setListsOpen(true);
          }}
        >
          <ListTree aria-hidden="true" />
          {t('documents.lists.open')}
        </Button>
        <Button
          type="button"
          size="sm"
          variant={filter.needsReview ? 'default' : 'outline'}
          aria-pressed={filter.needsReview}
          className="gap-2"
          onClick={() => {
            changeFilter({ ...filter, needsReview: !filter.needsReview });
          }}
        >
          {t('documents.review')}
          <span className="rounded-full bg-warning px-2 text-xs font-bold text-bg">{review}</span>
        </Button>
      </header>
      {bulk.active ? (
        <BulkBar {...bulk.bar} lists={lists} />
      ) : (
        <FilterBar
          filter={filter}
          onChange={changeFilter}
          facets={facets}
          lists={lists}
          names={names}
          customFields={customFields}
          shownFields={shownFields}
          onShownFieldsChange={setShownFields}
        />
      )}
      <BulkConfirmDialog {...bulk.confirm} names={names} />
      {page.isError && (
        <p role="alert" className="px-3 py-2 text-sm text-danger">
          {t('documents.loadFailed')}
        </p>
      )}
      {page.isPending ? (
        <p role="status" className="grid flex-1 place-items-center text-sm text-text-muted">
          {t('documents.loading')}
        </p>
      ) : (
        <div className="flex min-h-0 flex-1 flex-col">
          <Theme fonts={false}>
            <Locale words={documentsWords(i18n.language)}>
              <DocumentsCellContext.Provider value={names}>
                <SelectionContext.Provider value={bulk.selectionApi}>
                  <div className="min-h-0 flex-1 [&>*]:h-full">
                    <Grid
                      data={rows.map((row) => ({ ...row }))}
                      columns={columns}
                      sortMarks={sortMarks}
                      init={init}
                      select={false}
                      onFilterValue={(ev: FilterValueEvent) => {
                        changeFilter(withField(filter, 'include', ev.field, ev.id));
                      }}
                      onOpenDocument={(ev: OpenDocumentEvent) => {
                        setOpenId(ev.id);
                      }}
                    />
                  </div>
                  {total > PAGE_SIZE && (
                    <Pager
                      total={total}
                      pageSize={PAGE_SIZE}
                      value={Math.floor(offset / PAGE_SIZE) + 1}
                      onChange={(ev) => {
                        setOffset(ev.from);
                      }}
                    />
                  )}
                </SelectionContext.Provider>
              </DocumentsCellContext.Provider>
            </Locale>
          </Theme>
        </div>
      )}
    </section>
  );
}
