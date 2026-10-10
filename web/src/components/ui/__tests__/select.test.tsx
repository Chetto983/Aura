import { describe, expect, it, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import { chooseOption } from '../../../test/chooseOption';
import { Select } from '../select';

const OPTIONS = [
  { value: '', label: 'Every action' },
  { value: 'list', label: 'list' },
  { value: 'delete', label: 'delete' },
];

describe('Select', () => {
  it('shows the chosen option and reports a new choice by its value', async () => {
    const onValueChange = vi.fn();
    render(
      <Select aria-label="Action" value="list" onValueChange={onValueChange} options={OPTIONS} />,
    );
    expect(screen.getByRole('combobox', { name: 'Action' }).textContent).toContain('list');
    await chooseOption('Action', 'delete');
    expect(onValueChange).toHaveBeenCalledWith('delete');
  });

  it('keeps an empty value empty, though Radix reserves it', async () => {
    const onValueChange = vi.fn();
    render(<Select aria-label="Action" value="" onValueChange={onValueChange} options={OPTIONS} />);
    expect(screen.getByRole('combobox', { name: 'Action' }).textContent).toContain('Every action');
    await chooseOption('Action', 'list');
    expect(onValueChange).toHaveBeenLastCalledWith('list');
  });

  it('reports the empty option as an empty value', async () => {
    const onValueChange = vi.fn();
    render(
      <Select aria-label="Action" value="list" onValueChange={onValueChange} options={OPTIONS} />,
    );
    await chooseOption('Action', 'Every action');
    expect(onValueChange).toHaveBeenCalledWith('');
  });

  it('shows its placeholder until something is chosen, and can be disabled', () => {
    render(
      <Select
        aria-label="Account"
        value=""
        onValueChange={vi.fn()}
        options={[{ value: 'a', label: 'Account A' }]}
        placeholder="Choose an account"
        disabled
      />,
    );
    const trigger = screen.getByRole('combobox', { name: 'Account' }) as HTMLButtonElement;
    expect(trigger.textContent).toContain('Choose an account');
    expect(trigger.disabled).toBe(true);
  });
});
