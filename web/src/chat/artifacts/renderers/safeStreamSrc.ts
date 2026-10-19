// Media components only receive a stream sibling from the active Aura asset tier.
// This rejects provider, cross-origin and ambiguous query/fragment URLs.
export function safeStreamSrc(raw: string): string | null {
  if (!raw.startsWith('/') || raw.startsWith('//')) return null;
  try {
    const url = new URL(raw, location.origin);
    if (url.origin !== location.origin || url.hash) return null;
    const path = url.pathname;
    if (url.search) return isFileManagerObject(url) ? url.href : null;
    if (
      !/^\/api\/assets\/[^/]+\/stream$/.test(path) &&
      !/^\/s\/[^/]+\/asset\/[^/]+\/stream$/.test(path) &&
      !/^\/api\/shares\/[^/]+\/asset\/[^/]+\/stream$/.test(path)
    )
      return null;
    return url.href;
  } catch {
    return null;
  }
}

// The file manager names its object in the query (FILE_MANAGER_SOURCE), and its route answers
// Range like the asset tier's: one id and nothing else, so no second parameter changes what is
// served.
function isFileManagerObject(url: URL): boolean {
  const keys = [...url.searchParams.keys()];
  return (
    url.pathname === '/api/filemanager/direct' &&
    keys.length === 1 &&
    keys[0] === 'id' &&
    url.searchParams.get('id') !== ''
  );
}
