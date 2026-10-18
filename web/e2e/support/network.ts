import { expect, type Page } from '@playwright/test';

export interface NetworkLog {
  /** Every http(s) origin touched so far, in order of first sight. */
  origins(): readonly string[];
  /** A point in that list, so a later call can ask what a single gesture added. */
  mark(): number;
  since(mark: number): readonly string[];
  /** The object-store origins the appliance's OWN presign answers named, once they have been
   *  read. An empty set means the session never asked to upload anything. */
  store(): Promise<readonly string[]>;
}

/**
 * Every http(s) origin the session touched, from before the first navigation. Both the page and
 * its context are listened to: a dedicated worker's requests reach the context even where they do
 * not surface on the page.
 *
 * The second half of this is what makes the claim portable without weakening it. An upload goes
 * to a presigned URL, and which origin that is belongs to the DEPLOYMENT: on the appliance Caddy
 * fronts the store on the browsing origin itself, while CI presigns to garage on another port. So
 * the allowed set is not a list written here — it is exactly what the cockpit's own
 * `/api/assets/presign` answer named, and nothing else is tolerated.
 */
export function watchNetwork(page: Page): NetworkLog {
  const seen: string[] = [];
  const presigned = new Set<string>();
  const reading: Promise<void>[] = [];
  const record = (url: string) => {
    if (!url.startsWith('http:') && !url.startsWith('https:')) return;
    const origin = new URL(url).origin;
    if (!seen.includes(origin)) seen.push(origin);
  };
  page.on('request', (request) => {
    record(request.url());
  });
  page.context().on('request', (request) => {
    record(request.url());
  });
  page.on('response', (response) => {
    if (!response.url().includes('/api/assets/presign')) return;
    reading.push(
      response
        .json()
        .then((body: { upload?: { upload_url?: unknown } }) => {
          const url = body.upload?.upload_url;
          if (typeof url === 'string') presigned.add(new URL(url).origin);
        })
        .catch(() => undefined),
    );
  });
  return {
    origins: () => [...seen],
    mark: () => seen.length,
    since: (mark) => seen.slice(mark),
    store: async () => {
      await Promise.all(reading);
      return [...presigned].sort();
    },
  };
}

/**
 * Nothing left the appliance. Every origin is either the cockpit the operator is looking at or
 * the object store the cockpit itself presigned an upload to — no font CDN, no telemetry, no
 * third party of any kind.
 */
export async function expectNothingLeftTheAppliance(page: Page, network: NetworkLog) {
  const here = new URL(page.url()).origin;
  const store = await network.store();
  const strangers = network
    .origins()
    .filter((origin) => origin !== here && !store.includes(origin));
  expect(strangers, `off-origin requests (store origins: ${store.join(', ') || 'none'})`).toEqual(
    [],
  );
}
