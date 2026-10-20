import type { H3Event } from 'h3';
import { readRawBody, setResponseStatus } from 'h3';
import type { ApiErrorBody, AuthResponse } from '~~/api/types';
import { clearAuthSession, readAuthSession, writeAuthSession } from './auth-cookie';
import type { BackendConfig } from './backend';
import { backendFetch, readErrorBody, unavailableBody } from './backend';

type Outcome
  = | { ok: true, data: AuthResponse }
    | { ok: false, status: number, error: ApiErrorBody };

const JSON_HEADERS = { 'content-type': 'application/json', 'accept': 'application/json' };

const postAuth = async (config: BackendConfig, path: string, body: string): Promise<Outcome> => {
  let res: Response;
  try {
    res = await backendFetch(config, { method: 'POST', path, headers: JSON_HEADERS, body, timeoutMs: 15_000 }, null);
  } catch (error) {
    console.error('[auth] backend unreachable', error);
    return { ok: false, status: 502, error: unavailableBody() };
  }
  if (res.ok) {
    return { ok: true, data: await res.json() as AuthResponse };
  }
  return { ok: false, status: res.status, error: await readErrorBody(res) };
};

/**
 * Shared handler for /api/auth/login and /api/auth/register: forwards the body to the
 * backend (or the mock), stores the access and refresh tokens in two httpOnly cookies and
 * returns only `{ user }` — tokens never reach browser JS. Errors keep the backend status
 * and the `{code, message, details}` body.
 */
export const proxyAuth = async (event: H3Event, config: BackendConfig, path: '/v1/auth/login' | '/v1/auth/register') => {
  const raw = await readRawBody(event, 'utf8').catch(() => undefined);
  const outcome = await postAuth(config, path, raw?.trim() ? raw : '{}');

  if (!outcome.ok) {
    setResponseStatus(event, outcome.status);
    return outcome.error;
  }

  writeAuthSession(event, outcome.data);
  return { user: outcome.data.user };
};

/**
 * /api/auth/logout: revokes the refresh token on the backend (`V1Logout`; any failure is
 * swallowed — the user is logging out anyway) and clears both cookies.
 */
export const logoutSession = async (event: H3Event, config: BackendConfig) => {
  const { refresh } = readAuthSession(event);
  clearAuthSession(event);
  if (refresh) {
    try {
      const res = await backendFetch(config, {
        method: 'POST',
        path: '/v1/auth/logout',
        headers: JSON_HEADERS,
        body: JSON.stringify({ refreshToken: refresh }),
        timeoutMs: 10_000,
      }, null);
      await res.body?.cancel();
      if (!res.ok) {
        console.error(`[auth] logout: backend answered ${res.status}`);
      }
    } catch (error) {
      console.error('[auth] logout: backend unreachable', error);
    }
  }
  return {};
};
