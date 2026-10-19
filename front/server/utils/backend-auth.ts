import type { H3Event } from 'h3';
import { readBody, setResponseStatus } from 'h3';
import { FetchError } from 'ofetch';
import type { ApiErrorBody, User } from '~~/api/types';
import { handleMock } from '../mock';

interface AuthResult {
  token: string
  user: User
}

type Outcome
  = | { ok: true, data: AuthResult }
    | { ok: false, status: number, error: ApiErrorBody };

const internalError = (): ApiErrorBody => ({ code: 13, message: 'Внутренняя ошибка', details: [] });

const callBackend = async (event: H3Event, path: string, body: unknown): Promise<Outcome> => {
  const config = useRuntimeConfig(event);

  if (config.mockApi) {
    const res = handleMock({ method: 'POST', path, query: {}, token: null, body });
    if (res.status >= 200 && res.status < 300) {
      return { ok: true, data: res.body as AuthResult };
    }
    return { ok: false, status: res.status, error: res.body as ApiErrorBody };
  }

  try {
    const data = await $fetch<AuthResult>(`${config.apiBaseUrl}${path}`, { method: 'POST', body: body as Record<string, unknown> });
    return { ok: true, data };
  } catch (error) {
    if (error instanceof FetchError && error.response) {
      const data = error.data as Partial<ApiErrorBody> | undefined;
      if (data && typeof data.message === 'string') {
        return {
          ok: false,
          status: error.response.status,
          error: { code: data.code ?? 2, message: data.message, details: data.details ?? [] },
        };
      }
      return { ok: false, status: error.response.status, error: internalError() };
    }
    console.error('[auth] backend unreachable', error);
    return { ok: false, status: 502, error: { code: 14, message: 'Сервер недоступен, попробуйте позже', details: [] } };
  }
};

/**
 * Shared handler for /api/auth/login and /api/auth/register: forwards the body to the
 * backend (or the mock), stores the JWT in the httpOnly cookie and returns only
 * `{ user }` — the token never reaches browser JS. Errors keep the backend status and
 * the `{code, message, details}` body.
 */
export const proxyAuth = async (event: H3Event, path: '/v1/auth/login' | '/v1/auth/register') => {
  const body = await readBody(event).catch(() => undefined);
  const outcome = await callBackend(event, path, body ?? {});

  if (!outcome.ok) {
    setResponseStatus(event, outcome.status);
    return outcome.error;
  }

  writeAuthToken(event, outcome.data.token);
  return { user: outcome.data.user };
};
