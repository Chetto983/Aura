import { describe, expect, it, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import '../../../i18n/i18n';
import { ZoneStep } from '../ZoneStep';
import { chooseOption, shownOption } from '../../../test/chooseOption';

describe('ZoneStep', () => {
  it('requires an explicit account selection when Cloudflare returns several accounts', async () => {
    render(
      <ZoneStep
        accounts={[
          { id: 'one', name: 'One' },
          { id: 'two', name: 'Two' },
        ]}
        generation={1}
        initialDomain="example.com"
        onSave={vi.fn(() => Promise.resolve())}
      />,
    );
    expect(shownOption('Cloudflare account')).toBe('Cloudflare account');
    expect(
      screen.getByRole('button', { name: 'Save and start setup' }).hasAttribute('disabled'),
    ).toBe(true);
    await chooseOption('Cloudflare account', 'Two');
    expect(
      screen.getByRole('button', { name: 'Save and start setup' }).hasAttribute('disabled'),
    ).toBe(false);
  });
});
