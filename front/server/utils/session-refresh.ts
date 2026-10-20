import type { H3Event } from 'h3';
import type { AuthResponse } from '~~/api/types';
import { clearAuthSession, readAuthSession, writeAuthSession } from './auth-cookie';
import type { BackendConfig } from './backend';
import { backendFetch } from './backend';

export type RefreshOutcome
  = | { ok: true, auth: AuthResponse }
    | { ok: false, reason: 'unauthenticated' | 'unavailable' };

export type RefreshFn = (config: BackendConfig, refreshToken: string) => Promise<RefreshOutcome>;

/** `POST /v1/auth/refresh` (backend or mock). Never throws. */
export const callRefresh: RefreshFn = async (config, refreshToken) => {
  try {
    const res = await backendFetch(config, {
      method: 'POST',
      path: '/v1/auth/refresh',
      headers: { 'content-type': 'application/json', 'accept': 'application/json' },
      body: JSON.stringify({ refreshToken }),
      timeoutMs: 10_000,
    }, null);
    if (res.ok) {
      const auth = await res.json() as AuthResponse;
      if (auth.accessToken && auth.refreshToken) {
        console.info('[auth] session refreshed');
        return { ok: true, auth };
      }
      console.error('[auth] refresh: malformed response');
      return { ok: false, reason: 'unavailable' };
    }
    await res.body?.cancel();
    // 401 (expired / revoked / reused) and any other 4xx: the refresh token is no good.
    if (res.status < 500) {
      console.info(`[auth] refresh rejected: ${res.status}`);
      return { ok: false, reason: 'unauthenticated' };
    }
    console.error(`[auth] refresh failed: ${res.status}`);
    return { ok: false, reason: 'unavailable' };
  } catch (error) {
    console.error('[auth] refresh: backend unreachable', error);
    return { ok: false, reason: 'unavailable' };
  }
};

/**
 * How long a settled refresh is remembered by the old refresh token. Parallel requests of
 * one page all carry the same (old) cookie; the backend rotates the refresh token on every
 * use and treats a second use as theft (revokes the whole family). So every request that
 * shows up with the same old token within this window gets the result of the first call.
 */
export const REFRESH_DEDUP_MS = 5_000;

interface Flight {
  promise: Promise<RefreshOutcome>
  timer?: ReturnType<typeof setTimeout>
}

const flights = new Map<string, Flight>();

/** Single-flight `refresh(refreshToken)` per process, keyed by the refresh token value. */
export const refreshOnce = (config: BackendConfig, refreshToken: string, refresh: RefreshFn = callRefresh): Promise<RefreshOutcome> => {
  const existing = flights.get(refreshToken);
  if (existing) {
    return existing.promise;
  }
  const flight: Flight = { promise: refresh(config, refreshToken) };
  flights.set(refreshToken, flight);
  flight.promise.then((outcome) => {
    // An unreachable backend is not a verdict on the token: allow an immediate retry.
    if (!outcome.ok && outcome.reason === 'unavailable') {
      flights.delete(refreshToken);
      return;
    }
    flight.timer = setTimeout(() => flights.delete(refreshToken), REFRESH_DEDUP_MS);
    flight.timer.unref?.();
  }, () => flights.delete(refreshToken));
  return flight.promise;
};

/** Test helper: forget all remembered refreshes. */
export const resetRefreshFlights = () => {
  for (const flight of flights.values()) {
    clearTimeout(flight.timer);
  }
  flights.clear();
};

const errorResponse = (status: number, body: unknown) =>
  new Response(JSON.stringify(body), { status, headers: { 'content-type': 'application/json' } });

export const sessionExpiredResponse = () =>
  errorResponse(401, { code: 16, message: 'Сессия истекла', details: [] });

/**
 * Runs a backend call with the access token from the cookies and, if the backend answers
 * 401 while a refresh cookie is present, refreshes the session once (single-flight, see
 * `refreshOnce`), writes both new cookies and replays the call once with the new access
 * token. `call` must be replayable (buffered body).
 *
 * - no access cookie but a refresh cookie → refresh first, no wasted 401;
 * - refresh rejected → both cookies are cleared and the client gets 401 «Сессия истекла»;
 * - refresh impossible (backend down) → cookies kept, 502;
 * - 401 without a refresh cookie → cookies cleared, the original 401 is returned.
 *
 * Errors thrown by `call` (backend unreachable) propagate to the caller.
 */
export const withRefresh = async (
  event: H3Event,
  config: BackendConfig,
  call: (access: string | null) => Promise<Response>,
  refresh: RefreshFn = callRefresh,
): Promise<Response> => {
  const { access, refresh: refreshToken } = readAuthSession(event);

  if (access || !refreshToken) {
    const res = await call(access);
    if (res.status !== 401) {
      return res;
    }
    if (!refreshToken) {
      if (access) {
        clearAuthSession(event);
      }
      return res;
    }
    await res.body?.cancel();
  }

  const outcome = await refreshOnce(config, refreshToken, refresh);
  if (!outcome.ok) {
    if (outcome.reason === 'unavailable') {
      return errorResponse(502, { code: 14, message: 'Сервер недоступен, попробуйте позже', details: [] });
    }
    clearAuthSession(event);
    return sessionExpiredResponse();
  }

  writeAuthSession(event, outcome.auth);
  const retried = await call(outcome.auth.accessToken);
  if (retried.status === 401) {
    // A brand-new access token rejected: nothing more to try.
    clearAuthSession(event);
  }
  return retried;
};
