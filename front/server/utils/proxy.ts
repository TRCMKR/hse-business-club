import type { H3Event } from 'h3';
import { getRequestHeaders, readRawBody, sendStream, setResponseStatus } from 'h3';
import type { BackendConfig, BackendRequest } from './backend';
import { backendFetch, unavailableBody } from './backend';
import { canonicalProxyPath } from './proxy-path';
import { withRefresh } from './session-refresh';

const PAYLOAD_METHODS = new Set(['POST', 'PUT', 'PATCH', 'DELETE']);

/** Request headers never forwarded to the backend. */
const DROPPED_REQUEST_HEADERS = new Set([
  'accept-encoding',
  'authorization',
  'connection',
  'content-length',
  'cookie',
  'expect',
  'host',
  'keep-alive',
  'transfer-encoding',
  'upgrade',
]);

/** Response headers never copied back (fetch already decoded the body; cookies are ours). */
const DROPPED_RESPONSE_HEADERS = new Set([
  'connection',
  'content-encoding',
  'content-length',
  'keep-alive',
  'set-cookie',
  'transfer-encoding',
]);

const notFound = (event: H3Event) => {
  setResponseStatus(event, 404);
  return { code: 5, message: 'Не найдено', details: [] };
};

/**
 * Proxies the current request to the backend (or the mock):
 *
 * - the path comes from the raw `event.path` and is canonicalized; only `/v1/**` passes and
 *   the whole `/v1/auth/**` subtree is 404 (tokens must never reach browser JS);
 * - the request body is buffered (≤ ~7 MB of base64 JSON for photos) so the call can be
 *   replayed after a refresh;
 * - `Authorization` comes from the `hbc_access` cookie, browser cookies are not forwarded;
 * - a 401 triggers one `V1Refresh` + replay (`withRefresh`);
 * - the response streams back with its status, `Content-Type`, `Cache-Control`, …
 */
export const forwardToBackend = async (event: H3Event, config: BackendConfig) => {
  // Raw request path (before URL normalization), without the `/api` mount prefix.
  const [rawPathWithPrefix = '', ...rest] = event.path.split('?');
  const rawPath = rawPathWithPrefix.replace(/^\/api(?=\/|$)/, '');
  const path = canonicalProxyPath(rawPath);
  if (!path) {
    return notFound(event);
  }

  const headers: Record<string, string> = {};
  for (const [name, value] of Object.entries(getRequestHeaders(event))) {
    if (value !== undefined && !DROPPED_REQUEST_HEADERS.has(name.toLowerCase())) {
      headers[name] = value;
    }
  }

  const request: BackendRequest = {
    method: event.method,
    path,
    search: rest.length ? `?${rest.join('?')}` : '',
    headers,
    body: PAYLOAD_METHODS.has(event.method) ? await readRawBody(event, false) : undefined,
  };

  let res: Response;
  try {
    res = await withRefresh(event, config, access => backendFetch(config, request, access));
  } catch (error) {
    console.error('[proxy] backend unreachable', error);
    setResponseStatus(event, 502);
    return unavailableBody();
  }

  setResponseStatus(event, res.status, res.statusText || undefined);
  for (const [name, value] of res.headers) {
    if (!DROPPED_RESPONSE_HEADERS.has(name)) {
      event.node.res.setHeader(name, value);
    }
  }
  if (!res.body) {
    event.node.res.end();
    return;
  }
  return sendStream(event, res.body);
};
