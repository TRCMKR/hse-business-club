/**
 * Canonicalizes the backend path taken from `/api/<path>` before proxying.
 *
 * - splits on `/`, drops empty segments (collapses `//`, trailing `/`);
 * - percent-decodes each segment once and re-encodes it, so the backend sees exactly
 *   the segments we checked (`%6Cogin` → `login`, `%256C` stays encoded);
 * - rejects `.`/`..` segments, malformed percent-encoding and encoded `/`;
 * - only `/v1/**` is proxied, and the whole `/v1/auth/**` subtree is closed: login and
 *   register must go through `/api/auth/*` so the JWT never reaches browser JS.
 *
 * Returns the canonical path (`/v1/...`) or null if the request must be answered with 404.
 */
export const canonicalProxyPath = (rawPath: string): string | null => {
  const segments: string[] = [];
  for (const raw of rawPath.split('/')) {
    if (raw === '') {
      continue;
    }
    let decoded: string;
    try {
      decoded = decodeURIComponent(raw);
    } catch {
      return null;
    }
    if (decoded === '.' || decoded === '..' || decoded.includes('/') || decoded.includes('\\')) {
      return null;
    }
    segments.push(decoded);
  }

  if (segments[0] !== 'v1' || segments.length < 2) {
    return null;
  }
  if (segments[1]!.toLowerCase() === 'auth') {
    return null;
  }

  return `/${segments.map(encodeURIComponent).join('/')}`;
};
