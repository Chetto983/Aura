import { afterEach, describe, expect, it, vi } from 'vitest';
import { act, render, screen, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import i18n from '../../i18n/i18n';
import { StandingApprovalsPanel } from '../StandingApprovalsPanel';

// StandingApprovalsPanel holds the operator's grants and, below them, the tool policies. Under
// the dev and local_trusted profiles the gateway applies neither, and this suite pins the one
// sentence that says so: present when the server reports policies_enforced false, and absent
// in every other case (true, the field missing, /api/me failing or still loading), so a strict
// deployment never sees it flash.

const NOT_APPLIED = /saved but not applied/;
const ME = { identity_id: 'local', capabilities: [] };

function urlOf(input: RequestInfo | URL): string {
  if (typeof input === 'string') return input;
  if (input instanceof URL) return input.href;
  return input.url;
}

function answer(body: object, status = 200) {
  return () => new Response(JSON.stringify(body), { status });
}

// stubFetch answers the panel's reads: /api/me with `me`, or never when `me` is undefined, and
// an empty list for the grants, the policies and the policy form's tools.
function stubFetch(me?: () => Response) {
  vi.stubGlobal(
    'fetch',
    vi.fn((input: RequestInfo | URL) => {
      if (urlOf(input) === '/api/me') {
        return me === undefined ? new Promise<Response>(() => undefined) : Promise.resolve(me());
      }
      return Promise.resolve(new Response('[]', { status: 200 }));
    }),
  );
}

function renderPanel() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  render(
    <QueryClientProvider client={client}>
      <StandingApprovalsPanel />
    </QueryClientProvider>,
  );
  return client;
}

afterEach(async () => {
  vi.unstubAllGlobals();
  await act(() => i18n.changeLanguage('en'));
});

describe('StandingApprovalsPanel', () => {
  it('says the policies and approvals are saved but not applied when the profile skips them', async () => {
    stubFetch(answer({ ...ME, policies_enforced: false }));
    renderPanel();
    const notice = await screen.findByText(NOT_APPLIED);
    expect(notice.closest('[role="status"]')).not.toBeNull();
  });

  it('says it in Italian too', async () => {
    await act(() => i18n.changeLanguage('it'));
    stubFetch(answer({ ...ME, policies_enforced: false }));
    renderPanel();
    expect(await screen.findByText(/salvate ma non applicate/)).toBeTruthy();
  });

  it.each([
    ['reports policies_enforced true', answer({ ...ME, policies_enforced: true })],
    ['omits the field, as an older daemon does', answer(ME)],
    ['fails', answer({ error: 'boom' }, 500)],
  ])('stays quiet when /api/me %s', async (_case, me) => {
    stubFetch(me);
    const client = renderPanel();
    await waitFor(() => {
      expect(client.getQueryState(['me'])?.status).not.toBe('pending');
    });
    await screen.findByText(/No standing approvals/);
    expect(screen.queryByText(NOT_APPLIED)).toBeNull();
  });

  it('does not show the notice while /api/me has not answered', async () => {
    stubFetch();
    renderPanel();
    await screen.findByText(/No standing approvals/);
    expect(screen.queryByText(NOT_APPLIED)).toBeNull();
  });
});
