import { describe, expect, it } from 'vitest';
import { fireEvent, render, screen } from '@testing-library/react';
import { Command, CommandItem, CommandList } from '../command';
import { Select } from '../select';
import { selectItemClass } from '../select-classes';

// jsdom applies no CSS, so a test click reaches a row that a browser leaves dead under
// pointer-events-none. These read the row's disabling variants as the selectors Tailwind
// compiles them to, and ask the DOM which rows they match. cmdk writes
// data-disabled="false" on every enabled row, which a bare data-[disabled] matches: the tool
// picker shipped greyed out and unclickable that way (2026-10-10).
const DISABLING = new Set(['pointer-events-none', 'opacity-50']);

function disablingSelectors(classes: string): string[] {
  return classes.split(/\s+/).flatMap((cls) => {
    const split = cls.lastIndexOf(':');
    if (split < 0 || !DISABLING.has(cls.slice(split + 1))) return [];
    const variant = cls.slice(0, split);
    if (variant === 'aria-disabled') return ['[aria-disabled="true"]'];
    const data = /^data-\[([\w-]+)(?:=([\w-]+))?\]$/.exec(variant);
    if (data === null) throw new Error(`no selector known for the variant ${variant}`);
    const [, name = '', value] = data;
    return [value === undefined ? `[data-${name}]` : `[data-${name}="${value}"]`];
  });
}

function matchedBy(row: HTMLElement): string[] {
  return disablingSelectors(selectItemClass).filter((selector) => row.matches(selector));
}

describe('selectItemClass', () => {
  it('disables rows through at least one variant', () => {
    expect(disablingSelectors(selectItemClass)).not.toEqual([]);
  });

  it('leaves an enabled cmdk row clickable and greys a disabled one', () => {
    render(
      <Command>
        <CommandList>
          <CommandItem value="board" className={selectItemClass}>
            board
          </CommandItem>
          <CommandItem value="shell_exec" disabled className={selectItemClass}>
            shell_exec
          </CommandItem>
        </CommandList>
      </Command>,
    );
    expect(matchedBy(screen.getByRole('option', { name: 'board' }))).toEqual([]);
    expect(matchedBy(screen.getByRole('option', { name: 'shell_exec' }))).not.toEqual([]);
  });

  it('leaves an enabled Radix row clickable and greys a disabled one', () => {
    render(
      <Select
        aria-label="Action"
        value=""
        onValueChange={() => undefined}
        options={[
          { value: 'list', label: 'list' },
          { value: 'delete', label: 'delete', disabled: true },
        ]}
      />,
    );
    fireEvent.click(screen.getByRole('combobox', { name: 'Action' }));
    expect(matchedBy(screen.getByRole('option', { name: 'list' }))).toEqual([]);
    expect(matchedBy(screen.getByRole('option', { name: 'delete' }))).not.toEqual([]);
  });
});
