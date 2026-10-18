import { afterEach, describe, expect, it, vi } from 'vitest';
import { readShownFields, readWidths, saveShownFields, saveWidth } from './documentsPrefs';

afterEach(() => {
  localStorage.clear();
  vi.restoreAllMocks();
});

describe('the list preferences', () => {
  it('remembers the width of each column the operator dragged', () => {
    saveWidth({}, 'file_name', 420);
    saveWidth(readWidths(), 'counterparty', 260);
    expect(readWidths()).toEqual({ file_name: 420, counterparty: 260 });
  });

  it('reads nothing from a value it did not write', () => {
    localStorage.setItem('aura.documents.widths', '{"file_name":"wide","type":-3,"tags":180}');
    expect(readWidths()).toEqual({ tags: 180 });
    localStorage.setItem('aura.documents.widths', 'not json');
    expect(readWidths()).toEqual({});
    localStorage.setItem('aura.documents.fields', '["cf-a", 3]');
    expect(readShownFields()).toEqual(['cf-a']);
  });

  it('keeps working when storage refuses', () => {
    vi.spyOn(Storage.prototype, 'setItem').mockImplementation(() => {
      throw new Error('quota');
    });
    expect(saveWidth({}, 'type', 200)).toEqual({ type: 200 });
    expect(() => {
      saveShownFields(['cf-a']);
    }).not.toThrow();
  });
});
