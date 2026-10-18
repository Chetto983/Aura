import { useId } from 'react';
import { useTranslation } from 'react-i18next';
import { sourceOf, type DocumentDetail, type HistoryEvent } from './documentsApi';
import type { DetailNames } from './detailNames';

interface DocumentHistoryProps {
  readonly doc: DocumentDetail;
  readonly names: DetailNames;
}

/**
 * A document's history (mockup 3, "Storico"): every value set, accepted, rejected or removed,
 * by whom and when, oldest first, then whether the operator has reviewed it.
 */
export function DocumentHistory({ doc, names }: DocumentHistoryProps) {
  const { t, i18n } = useTranslation();
  const headingId = useId();
  const when = (at: string) =>
    new Intl.DateTimeFormat(i18n.language, { dateStyle: 'short', timeStyle: 'short' }).format(
      new Date(at),
    );
  const actor = (event: HistoryEvent) => {
    const source = event.actor === 'operator' ? 'operator' : sourceOf(event.actor);
    return source === null ? event.actor : t(`documents.detail.actor.${source}`);
  };
  const line = (event: HistoryEvent) => {
    const action = t(`documents.detail.action.${event.action}`);
    if (event.action === 'reviewed') return `${action} ${actor(event)} · ${when(event.at)}`;
    const value = `${names.field(event.field)}: ${names.value(event.field, event.value_key)}`;
    return `${value} — ${action} ${actor(event)} · ${when(event.at)}`;
  };

  return (
    <section className="flex flex-col gap-2 border-t border-border pt-4">
      <h2
        id={headingId}
        className="text-xs font-semibold tracking-[0.08em] text-text-muted uppercase"
      >
        {t('documents.detail.history')}
      </h2>
      <ul aria-labelledby={headingId} className="flex flex-col gap-1 text-[13px] text-text">
        {doc.history.map((event) => (
          <li
            key={`${event.at}:${event.action}:${event.field}:${event.value_key}`}
            className="border-l-2 border-border-strong pl-2"
          >
            {line(event)}
          </li>
        ))}
      </ul>
      <p className="text-[13px] font-semibold text-text-muted">
        {doc.reviewed_at === undefined
          ? t('documents.detail.notReviewed')
          : t('documents.detail.reviewedAt', { when: when(doc.reviewed_at) })}
      </p>
    </section>
  );
}
