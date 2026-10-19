import type { H3Event } from 'h3';
import {
  getQuery,
  getRequestURL,
  proxyRequest,
  readRawBody,
  send,
  setResponseHeaders,
  setResponseStatus,
} from 'h3';
import { handleMock } from '../mock';

const notFound = (event: H3Event) => {
  setResponseStatus(event, 404);
  return { code: 5, message: 'Не найдено', details: [] };
};

/** Login/register must go through /api/auth/* so the JWT stays in the httpOnly cookie. */
const BLOCKED = new Set(['/v1/auth/login', '/v1/auth/register']);

const handleWithMock = async (event: H3Event, path: string, token: string | null) => {
  const raw = event.method === 'GET' || event.method === 'HEAD' ? undefined : await readRawBody(event, 'utf8');
  let body: unknown;
  if (raw) {
    try {
      body = JSON.parse(raw);
    } catch {
      setResponseStatus(event, 400);
      return { code: 3, message: 'Некорректный JSON', details: [] };
    }
  }

  const query = Object.fromEntries(
    Object.entries(getQuery(event)).map(([key, value]) => [key, Array.isArray(value) ? String(value[0]) : value === undefined ? undefined : String(value)]),
  );

  const res = handleMock({ method: event.method, path, query, token, body });
  setResponseStatus(event, res.status);
  if (res.headers) {
    setResponseHeaders(event, res.headers);
  }
  if (res.body instanceof Uint8Array) {
    return send(event, res.body);
  }
  return res.body;
};

/**
 * BFF proxy: /api/v1/** → ${apiBaseUrl}/v1/** with `Authorization: Bearer <cookie>`.
 * Streams bodies both ways, so binary responses (avatars) and their Content-Type /
 * Cache-Control pass through unchanged; no request body limit is imposed here
 * (avatar uploads are ~7 MB of base64 JSON).
 */
export default defineEventHandler(async (event) => {
  const url = getRequestURL(event);
  const path = url.pathname.replace(/^\/api/, '');

  if (!path.startsWith('/v1/') || BLOCKED.has(path)) {
    return notFound(event);
  }

  const token = readAuthToken(event);
  const config = useRuntimeConfig(event);

  if (config.mockApi) {
    return handleWithMock(event, path, token);
  }

  const headers: Record<string, string> = { cookie: '' };
  if (token) {
    headers.authorization = `Bearer ${token}`;
  }

  try {
    return await proxyRequest(event, `${config.apiBaseUrl}${path}${url.search}`, {
      headers,
      fetchOptions: { redirect: 'manual' },
    });
  } catch (error) {
    console.error('[proxy] backend unreachable', error);
    setResponseStatus(event, 502);
    return { code: 14, message: 'Сервер недоступен, попробуйте позже', details: [] };
  }
});
