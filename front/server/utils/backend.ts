import type { ApiErrorBody } from '~~/api/types';
import { handleMock } from '../mock';

/** Subset of `runtimeConfig` the backend helpers need (passed explicitly for testability). */
export interface BackendConfig {
  apiBaseUrl: string
  mockApi: boolean
}

export interface BackendRequest {
  /** Upper-case HTTP method. */
  method: string
  /** Canonical backend path: `/v1/listings/abc`. */
  path: string
  /** Query string with the leading `?`, or empty. */
  search?: string
  /** Request headers to forward (without `authorization` / `cookie`). */
  headers?: Record<string, string>
  /** Buffered body: kept in memory so the request can be replayed after a refresh. */
  body?: Uint8Array | string
  /** Abort the backend call after this many milliseconds (none by default: uploads). */
  timeoutMs?: number
}

const PAYLOAD_METHODS = new Set(['POST', 'PUT', 'PATCH', 'DELETE']);

const jsonResponse = (status: number, body: unknown, headers?: Record<string, string>) =>
  new Response(JSON.stringify(body), {
    status,
    headers: { 'content-type': 'application/json', ...headers },
  });

const callMock = (req: BackendRequest, access: string | null): Response => {
  let body: unknown;
  if (PAYLOAD_METHODS.has(req.method) && req.body !== undefined) {
    const raw = typeof req.body === 'string' ? req.body : new TextDecoder().decode(req.body);
    if (raw.trim()) {
      try {
        body = JSON.parse(raw);
      } catch {
        return jsonResponse(400, { code: 3, message: 'Некорректный JSON', details: [] });
      }
    }
  }

  const query: Record<string, string | undefined> = {};
  for (const [key, value] of new URLSearchParams(req.search ?? '')) {
    if (!(key in query)) {
      query[key] = value;
    }
  }

  const res = handleMock({ method: req.method, path: req.path, query, token: access, body });
  if (res.body instanceof Uint8Array) {
    return new Response(res.body as BodyInit, { status: res.status, headers: res.headers });
  }
  return jsonResponse(res.status, res.body, res.headers);
};

/**
 * One call to the backend (or to the in-memory mock when `mockApi` is on) with the given
 * access token. Resolves with the raw `Response` for any HTTP status; rejects only when
 * the backend is unreachable or the call times out.
 */
export const backendFetch = async (config: BackendConfig, req: BackendRequest, access: string | null): Promise<Response> => {
  if (config.mockApi) {
    return callMock(req, access);
  }

  const headers: Record<string, string> = { ...req.headers };
  if (access) {
    headers.authorization = `Bearer ${access}`;
  }
  return fetch(`${config.apiBaseUrl}${req.path}${req.search ?? ''}`, {
    method: req.method,
    headers,
    // Node's Buffer is a Uint8Array<ArrayBufferLike>; fetch accepts it at runtime.
    body: PAYLOAD_METHODS.has(req.method) ? req.body as BodyInit | undefined : undefined,
    redirect: 'manual',
    signal: req.timeoutMs ? AbortSignal.timeout(req.timeoutMs) : undefined,
  });
};

/** `{code, message, details}` of a grpc-gateway error, or a generic one if the body is not JSON. */
export const readErrorBody = async (res: Response): Promise<ApiErrorBody> => {
  const data = await res.json().catch(() => undefined) as Partial<ApiErrorBody> | undefined;
  if (data && typeof data.message === 'string') {
    return { code: data.code ?? 2, message: data.message, details: data.details ?? [] };
  }
  return { code: 13, message: 'Внутренняя ошибка', details: [] };
};

export const unavailableBody = (): ApiErrorBody => ({ code: 14, message: 'Сервер недоступен, попробуйте позже', details: [] });
