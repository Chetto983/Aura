import type { Locator } from '@playwright/test';

// The cockpit's selects are Radix menus, not native <select>s, so selectOption() and toHaveValue()
// cannot drive them. The trigger opens a listbox whose options carry their value in data-value,
// which keeps a spec independent of the cockpit's language, as selectOption() was.

/** Opens a cockpit select and picks the option whose value is `value`, as an operator does. */
export async function chooseValue(select: Locator, value: string): Promise<void> {
  await select.click();
  await select.page().locator(`[role="option"][data-value="${value}"]`).click();
}

/** Opens a cockpit select and returns the options of its menu; Escape closes it again. */
export async function openOptions(select: Locator): Promise<Locator> {
  await select.click();
  return select.page().getByRole('listbox').getByRole('option');
}
