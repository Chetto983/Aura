import { act, fireEvent, render, screen } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import DocumentsSurface from './DocumentsSurface';
import i18n from '@/i18n/i18n';

vi.mock('@/files/FilesWorkspace', () => ({ default: () => <p>file manager</p> }));
vi.mock('./DocumentsWorkspace', () => ({ default: () => <p>documents list</p> }));

afterEach(() => {
  localStorage.clear();
});

describe('DocumentsSurface', () => {
  it('opens on the folders, switches to the list, and remembers the choice', async () => {
    await act(() => i18n.changeLanguage('it'));
    const first = render(<DocumentsSurface />);
    expect(screen.getByText('file manager')).toBeTruthy();
    fireEvent.click(screen.getByRole('radio', { name: 'Documenti' }));
    expect(screen.getByText('documents list')).toBeTruthy();
    first.unmount();
    render(<DocumentsSurface />);
    expect(screen.getByText('documents list')).toBeTruthy();
    expect(screen.getByRole('radio', { name: 'Documenti' }).getAttribute('aria-checked')).toBe(
      'true',
    );
  });
});
