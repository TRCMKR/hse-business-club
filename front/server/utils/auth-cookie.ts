import type { H3Event } from 'h3';
import { deleteCookie, getCookie, setCookie } from 'h3';

/** httpOnly cookie with the short-lived access JWT. */
export const ACCESS_COOKIE = 'hbc_access';
/** httpOnly cookie with the long-lived opaque refresh token. */
export const REFRESH_COOKIE = 'hbc_refresh';
/**
 * 30 days for both cookies (= REFRESH_TTL). The access cookie outlives the JWT on purpose:
 * the backend decides when access has expired (401), and the proxy then refreshes it.
 */
export const SESSION_COOKIE_MAX_AGE = 60 * 60 * 24 * 30;

export interface SessionTokens {
  access: string | null
  refresh: string | null
}

export interface TokenPair {
  accessToken: string
  refreshToken: string
}

/**
 * Tokens written during the current request (after a refresh or a logout) win over the
 * incoming cookies, so later backend calls of the same SSR render use the new pair.
 */
const CONTEXT_KEY = 'hbcSession';

const cookieOptions = () => ({
  httpOnly: true,
  sameSite: 'lax' as const,
  path: '/',
  maxAge: SESSION_COOKIE_MAX_AGE,
  // Off by default: compose serves the app over plain http://localhost:3000 and Safari
  // drops Secure cookies there. Set NUXT_COOKIE_SECURE=true behind an HTTPS proxy.
  secure: process.env.NUXT_COOKIE_SECURE === 'true',
});

export const readAuthSession = (event: H3Event): SessionTokens => {
  const override = event.context[CONTEXT_KEY] as SessionTokens | undefined;
  if (override) {
    return override;
  }
  return {
    access: getCookie(event, ACCESS_COOKIE) || null,
    refresh: getCookie(event, REFRESH_COOKIE) || null,
  };
};

export const hasAuthSession = (event: H3Event): boolean => {
  const { access, refresh } = readAuthSession(event);
  return Boolean(access || refresh);
};

export const writeAuthSession = (event: H3Event, pair: TokenPair) => {
  setCookie(event, ACCESS_COOKIE, pair.accessToken, cookieOptions());
  setCookie(event, REFRESH_COOKIE, pair.refreshToken, cookieOptions());
  event.context[CONTEXT_KEY] = { access: pair.accessToken, refresh: pair.refreshToken } satisfies SessionTokens;
};

export const clearAuthSession = (event: H3Event) => {
  const { secure } = cookieOptions();
  deleteCookie(event, ACCESS_COOKIE, { path: '/', httpOnly: true, sameSite: 'lax', secure });
  deleteCookie(event, REFRESH_COOKIE, { path: '/', httpOnly: true, sameSite: 'lax', secure });
  event.context[CONTEXT_KEY] = { access: null, refresh: null } satisfies SessionTokens;
};
