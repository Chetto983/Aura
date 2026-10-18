import { type ReactNode } from 'react';
import { Pencil, Trash2 } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { Button } from '@/components/ui/button';
import { ConfirmDialog } from '@/components/ui/confirm-dialog';
import { Input } from '@/components/ui/input';
import { ToggleGroup, ToggleGroupItem } from '@/components/ui/toggle-group';

// The parts the list pages share: an entry with its actions, the editor's frame and its choice
// of kind, and the confirmation before a delete.

const captionClass = 'text-xs font-semibold tracking-[0.08em] text-text-muted uppercase';

interface AdminEntryProps {
  readonly name: string;
  readonly lines: readonly string[];
  readonly actions?: ReactNode;
  readonly onEdit: () => void;
  readonly onDelete: () => void;
}

/** One value of a list: its name, what it says about itself, and its actions. */
export function AdminEntry({ name, lines, actions, onEdit, onDelete }: AdminEntryProps) {
  const { t } = useTranslation();
  return (
    <li className="flex flex-wrap items-center gap-x-4 gap-y-2 border-b border-border py-3">
      <div className="flex min-w-0 flex-1 basis-56 flex-col gap-0.5">
        <h3 className="truncate text-[15px] font-semibold text-text">{name}</h3>
        {lines.map((line) => (
          <p key={line} className="text-[13px] text-text-muted">
            {line}
          </p>
        ))}
      </div>
      <div className="flex flex-wrap items-center gap-1">
        {actions}
        <Button
          type="button"
          variant="ghost"
          size="icon"
          aria-label={t('documents.lists.edit', { name })}
          onClick={onEdit}
        >
          <Pencil aria-hidden="true" />
        </Button>
        <Button
          type="button"
          variant="ghost"
          size="icon"
          aria-label={t('documents.lists.delete', { name })}
          onClick={onDelete}
          className="hover:text-danger"
        >
          <Trash2 aria-hidden="true" />
        </Button>
      </div>
    </li>
  );
}

interface EditorFrameProps {
  readonly title: string;
  readonly name: string;
  readonly onNameChange: (name: string) => void;
  readonly busy: boolean;
  readonly error: string;
  readonly onSave: () => void;
  readonly onCancel: () => void;
  readonly children: ReactNode;
}

/** The editor of one value: its name, what the page adds below it, and Save. */
export function EditorFrame(props: EditorFrameProps) {
  const { title, name, onNameChange, busy, error, onSave, onCancel, children } = props;
  const { t } = useTranslation();
  return (
    <form
      aria-label={title}
      onSubmit={(event) => {
        event.preventDefault();
        onSave();
      }}
      className="flex flex-col gap-4 rounded-lg border border-border-strong bg-surface-2 p-4"
    >
      <h3 className="font-display text-base font-semibold text-text">{title}</h3>
      <label className="flex flex-col gap-1.5">
        <span className={captionClass}>{t('documents.lists.name')}</span>
        <Input
          value={name}
          disabled={busy}
          onChange={(event) => {
            onNameChange(event.target.value);
          }}
        />
      </label>
      {children}
      {error !== '' && (
        <p role="alert" className="text-sm text-danger">
          {error}
        </p>
      )}
      <div className="flex flex-wrap gap-2">
        <Button type="submit" disabled={busy || name.trim() === ''}>
          {t('documents.lists.save')}
        </Button>
        <Button type="button" variant="ghost" disabled={busy} onClick={onCancel}>
          {t('documents.lists.cancel')}
        </Button>
      </div>
    </form>
  );
}

interface KindChoiceProps<K extends string> {
  readonly label: string;
  readonly kinds: readonly K[];
  readonly value: K;
  readonly name: (kind: K) => string;
  readonly onChange: (kind: K) => void;
  readonly disabled: boolean;
}

/** One choice among a few kinds, as radios: a rule's algorithm, a field's type of value. */
export function KindChoice<K extends string>(props: KindChoiceProps<K>) {
  const { label, kinds, value, name, onChange, disabled } = props;
  return (
    <div className="flex flex-col gap-1.5">
      <span className={captionClass}>{label}</span>
      <ToggleGroup
        type="single"
        variant="outline"
        size="sm"
        aria-label={label}
        className="flex-wrap"
        value={value}
        disabled={disabled}
        onValueChange={(next) => {
          const kind = kinds.find((k) => k === next);
          if (kind !== undefined) onChange(kind);
        }}
      >
        {kinds.map((kind) => (
          <ToggleGroupItem key={kind} value={kind}>
            {name(kind)}
          </ToggleGroupItem>
        ))}
      </ToggleGroup>
    </div>
  );
}

/** A labelled box below the name: the rule's text, a choice's options. */
export function EditorField({
  label,
  children,
}: {
  readonly label: string;
  readonly children: ReactNode;
}) {
  return (
    <label className="flex flex-col gap-1.5">
      <span className={captionClass}>{label}</span>
      {children}
    </label>
  );
}

interface DeleteConfirmProps {
  /** The name of the value to delete, or null when nothing is being deleted. */
  readonly name: string | null;
  readonly body: string;
  readonly onConfirm: () => void;
  readonly onCancel: () => void;
}

export function DeleteConfirm({ name, body, onConfirm, onCancel }: DeleteConfirmProps) {
  const { t } = useTranslation();
  return (
    <ConfirmDialog
      open={name !== null}
      onOpenChange={(open) => {
        if (!open) onCancel();
      }}
      title={t('documents.lists.deleteTitle', { name: name ?? '' })}
      description={body}
      cancelLabel={t('documents.lists.cancel')}
      confirmLabel={t('documents.lists.deleteConfirm')}
      onConfirm={onConfirm}
    />
  );
}
