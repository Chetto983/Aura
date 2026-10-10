import { fireEvent, screen } from '@testing-library/react';

/** Opens the select named `label` and picks the option named `option`, as an operator does. */
export async function chooseOption(label: string | RegExp, option: string | RegExp): Promise<void> {
  fireEvent.click(await screen.findByRole('combobox', { name: label }));
  fireEvent.click(await screen.findByRole('option', { name: option }));
}

/** Opens the select named `label` and reads the names of its options, in order. */
export async function optionNames(label: string | RegExp): Promise<string[]> {
  fireEvent.click(await screen.findByRole('combobox', { name: label }));
  return (await screen.findAllByRole('option')).map((option) => option.textContent);
}

/** The option a select named `label` shows as chosen, as the operator reads it. */
export function shownOption(label: string | RegExp): string {
  return screen.getByRole('combobox', { name: label }).textContent;
}
