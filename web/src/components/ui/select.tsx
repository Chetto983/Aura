import { type ReactNode } from 'react';
import { Check, ChevronDown } from 'lucide-react';
import { Select as SelectPrimitive } from 'radix-ui';
import { selectItemClass, selectMenuClass, selectTriggerClass } from './select-classes';
import { cn } from '@/lib/utils';

// The cockpit's one select, drawn after the work board's filters (2026-10-10): a rounded
// trigger with its chevron, and a menu of rounded rows where the chosen one turns accent and
// carries a check. A native <select> opens the operating system's own list instead, which
// looked different on every machine and could not be themed.

// Radix reserves the empty string for "nothing chosen". A select whose first option means
// "none" or "every" keeps "" at its call site; this stands in for it inside.
const EMPTY_VALUE = '\u0000empty';

export interface SelectOption {
  readonly value: string;
  readonly label: ReactNode;
  readonly disabled?: boolean;
}

export interface SelectProps {
  readonly value: string;
  readonly onValueChange: (value: string) => void;
  readonly options: readonly SelectOption[];
  readonly id?: string;
  readonly disabled?: boolean;
  readonly placeholder?: ReactNode;
  readonly size?: 'sm' | 'default';
  readonly className?: string;
  readonly 'aria-label'?: string | undefined;
  readonly 'aria-invalid'?: boolean | undefined;
  readonly 'aria-describedby'?: string | undefined;
}

export function Select({
  value,
  onValueChange,
  options,
  id,
  disabled,
  placeholder,
  size = 'default',
  className,
  ...aria
}: SelectProps) {
  // "" is an option only when one is listed; otherwise it is Radix's "nothing chosen", which
  // shows the placeholder.
  const emptyIsOption = options.some((option) => option.value === '');
  return (
    <SelectPrimitive.Root
      value={value === '' && emptyIsOption ? EMPTY_VALUE : value}
      onValueChange={(next) => {
        onValueChange(next === EMPTY_VALUE ? '' : next);
      }}
      {...(disabled === undefined ? {} : { disabled })}
    >
      <SelectPrimitive.Trigger
        id={id}
        data-slot="select-trigger"
        className={cn(selectTriggerClass, size === 'sm' && 'min-h-8 px-2 py-1', className)}
        {...aria}
      >
        <span className="min-w-0 truncate">
          <SelectPrimitive.Value placeholder={placeholder} />
        </span>
        <SelectPrimitive.Icon asChild>
          <ChevronDown aria-hidden="true" className="size-4 shrink-0 text-text-muted" />
        </SelectPrimitive.Icon>
      </SelectPrimitive.Trigger>
      <SelectPrimitive.Portal>
        <SelectPrimitive.Content
          position="popper"
          sideOffset={6}
          className={cn(
            selectMenuClass,
            'max-h-(--radix-select-content-available-height) min-w-(--radix-select-trigger-width)',
          )}
        >
          <SelectPrimitive.Viewport>
            {options.map((option) => (
              <SelectPrimitive.Item
                key={option.value}
                value={option.value === '' ? EMPTY_VALUE : option.value}
                data-value={option.value}
                {...(option.disabled === true ? { disabled: true } : {})}
                className={selectItemClass}
              >
                <SelectPrimitive.ItemText>{option.label}</SelectPrimitive.ItemText>
                <SelectPrimitive.ItemIndicator className="absolute right-2 inline-flex size-4 items-center justify-center">
                  <Check aria-hidden="true" className="size-3.5" />
                </SelectPrimitive.ItemIndicator>
              </SelectPrimitive.Item>
            ))}
          </SelectPrimitive.Viewport>
        </SelectPrimitive.Content>
      </SelectPrimitive.Portal>
    </SelectPrimitive.Root>
  );
}
