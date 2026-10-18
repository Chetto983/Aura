import { useCallback, useMemo, useState, type ReactNode } from 'react';
import { keepPreviousData, useQuery } from '@tanstack/react-query';
import { ListTree } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { Grid, Willow, WillowDark, type IApi } from '@svar-ui/react-grid';
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
import {
  DocumentsCellContext,
  SelectionContext,
  type FilterValueEvent,
  type OpenDocumentEvent,
} from './documentsCellContext';
import { DocumentDetail } from './DocumentDetail';
import { documentColumns } from './documentsColumns';
import { documentsWords } from './documentsLocale';
import { FilterBar } from './FilterBar';
import { ListsPage } from './ListsPage';
import { useBulkEdit } from './useBulkEdit';
import { readShownFields, readWidths, saveShownFields, saveWidth } from './documentsPrefs';
import { useDocumentLists } from './useDocumentLists';
import { useWideView } from './useWideView';
import { Button } from '@/components/ui/button';
import { useThemeMode } from '@/theme/useThemeMode';

const PAGE_SIZE = 50;

/** The columns a header click sorts, each by the API's own sort key. */
const SORTABLE: Readonly<Record<string, string>> = {
  file_name: 'file_name',
  document_date: 'document_date',
};

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
  const [widths, setWidths] = useState(readWidths);
  const [openId, setOpenId] = useState<string | null>(null);
  const [listsOpen, setListsOpen] = useState(false);
  const wide = useWideView();

  const setFilter = useCallback((next: DocumentsFilter) => {
    setFilterState(next);
    setOffset(0);
  }, []);
  const setShownFields = useCallback((ids: readonly string[]) => {
    setShownFieldsState(ids);
    saveShownFields(ids);
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

  const columns = useMemo(
    () => documentColumns(t, customFields, shownFields, wide, widths),
    [t, customFields, shownFields, wide, widths],
  );

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
    // A column dragged to a new width keeps it, here and on the next visit; the drag itself
    // stays the grid's until the operator lets go.
    api.on('resize-column', (ev) => {
      const width = ev.width;
      if (ev.inProgress === true || width === undefined) return;
      setWidths((current) => saveWidth(current, String(ev.id), width));
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
        // The theme wrapper is a plain div with no height of its own: stretched here, the grid
        // gets the room left and scrolls inside it. Left to its content, the grid grew to all
        // fifty rows and the page cut off those below the fold and the pager (measured live
        // 2026-10-18, a 1,889 px grid in 438 px). FilesWorkspace stretches its manager the same way.
        <div className="min-h-0 flex-1 [&>*]:h-full">
          <Theme fonts={false}>
            <Locale words={documentsWords(i18n.language)}>
              <DocumentsCellContext.Provider value={names}>
                <SelectionContext.Provider value={bulk.selectionApi}>
                  <div className="flex h-full min-h-0 flex-col">
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
                  </div>
                </SelectionContext.Provider>
              </DocumentsCellContext.Provider>
            </Locale>
          </Theme>
        </div>
      )}
    </section>
  );
}
