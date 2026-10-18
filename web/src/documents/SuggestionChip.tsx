import { X } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { sourceOf, type FieldValue } from './documentsApi';

interface SuggestionChipProps {
  readonly value: FieldValue;
  /** The value as the operator reads it: a label's name, a date in their language. */
  readonly name: string;
  readonly disabled: boolean;
  readonly onUse: () => void;
  readonly onReject: () => void;
}

/**
 * A suggestion under its field (mockup 3): the value, which a click puts in the form without
 * saving it; who proposed it, with the line or the rule it rests on; and × to reject it.
 */
export function SuggestionChip({ value, name, disabled, onUse, onReject }: SuggestionChipProps) {
  const { t } = useTranslation();
  const source = sourceOf(value.set_by);
  const evidence = value.evidence ?? '';
  return (
    <div className="flex flex-wrap items-center gap-x-2 gap-y-1 text-xs">
      <span className="text-text-muted">{t('documents.detail.suggested')}</span>
      <button
        type="button"
        aria-label={t('documents.detail.use', { name })}
        disabled={disabled}
        onClick={onUse}
        className="rounded-full border border-dashed border-warning/70 bg-warning/10 px-3 py-1 font-semibold text-text transition-colors hover:bg-warning/25 focus-visible:outline-2 focus-visible:outline-ring disabled:opacity-60"
      >
        {name}
      </button>
      {source !== null && (
        <span className="min-w-0 text-text-muted">
          {t(`documents.source.${source}`)}
          {evidence !== '' && <span className="font-mono text-[11px]"> · «{evidence}»</span>}
        </span>
      )}
      <button
        type="button"
        aria-label={t('documents.detail.reject', { name })}
        disabled={disabled}
        onClick={onReject}
        className="grid size-7 place-items-center rounded-full text-text-muted transition-colors hover:bg-danger/15 hover:text-danger focus-visible:outline-2 focus-visible:outline-ring disabled:opacity-60"
      >
        <X aria-hidden="true" className="size-3.5" />
      </button>
    </div>
  );
}
