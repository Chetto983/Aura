import { describe, expect, it } from 'vitest';
import { httpErrorFrom, isTrue } from './json';

describe('isTrue', () => {
  it('accepts only the JSON boolean true', () => {
    expect(isTrue(true)).toBe(true);
    expect(isTrue(false)).toBe(false);
    expect(isTrue('true')).toBe(false);
    expect(isTrue(1)).toBe(false);
    expect(isTrue(null)).toBe(false);
  });
});

describe('httpErrorFrom', () => {
  it("keeps the server's sentence beside its code", async () => {
    const res = new Response(
      JSON.stringify({ error: 'invalid', message: 'a regex rule needs a pattern' }),
      { status: 400 },
    );
    const err = await httpErrorFrom(res);
    expect(err.status).toBe(400);
    expect(err.reason).toBe('invalid');
    expect(err.detail).toBe('a regex rule needs a pattern');
  });

  it('carries no sentence when the body has none', async () => {
    const err = await httpErrorFrom(new Response('<html>bad gateway</html>', { status: 502 }));
    expect(err.reason).toBe('');
    expect(err.detail).toBe('');
  });
});
