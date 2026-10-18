import { useState, type ReactNode } from 'react';
import { useTranslation } from 'react-i18next';
import DocumentsWorkspace from './DocumentsWorkspace';
import FilesWorkspace from '@/files/FilesWorkspace';
import { ToggleGroup, ToggleGroupItem } from '@/components/ui/toggle-group';

type DocumentsView = 'folders' | 'documents';

const VIEW_KEY = 'aura.documents.view';

function readView(): DocumentsView {
  try {
    return localStorage.getItem(VIEW_KEY) === 'documents' ? 'documents' : 'folders';
  } catch {
    return 'folders';
  }
}

/**
 * The documents surface (spec decision 4): the file manager, unchanged, and the Documenti list,
 * one at a time. The choice is remembered like the surface itself. No field editor goes inside the
 * file manager, whose root keydown handler would turn typing into file actions.
 */
export default function DocumentsSurface({ mobileMenu }: { readonly mobileMenu?: ReactNode }) {
  const { t } = useTranslation();
  const [view, setViewState] = useState(readView);

  function setView(next: DocumentsView) {
    setViewState(next);
    try {
      localStorage.setItem(VIEW_KEY, next);
    } catch {
      // Storage is optional; the visible view still changes.
    }
  }

  return (
    <div className="flex h-full min-h-0 flex-col">
      <div className="flex justify-center border-b border-border bg-bg px-3 py-1.5">
        <ToggleGroup
          type="single"
          variant="outline"
          size="sm"
          aria-label={t('documents.views.label')}
          value={view}
          onValueChange={(next) => {
            if (next === 'folders' || next === 'documents') setView(next);
          }}
        >
          <ToggleGroupItem value="folders">{t('documents.views.folders')}</ToggleGroupItem>
          <ToggleGroupItem value="documents">{t('documents.views.documents')}</ToggleGroupItem>
        </ToggleGroup>
      </div>
      <div className="min-h-0 flex-1">
        {view === 'folders' ? (
          <FilesWorkspace mobileMenu={mobileMenu} />
        ) : (
          <DocumentsWorkspace mobileMenu={mobileMenu} />
        )}
      </div>
    </div>
  );
}
