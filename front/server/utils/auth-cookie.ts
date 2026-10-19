import type { H3Event } from 'h3';
import { deleteCookie, getCookie, setCookie } from 'h3';
import { AUTH_COOKIE, AUTH_COOKIE_MAX_AGE } from '~~/api/constants';

export const readAuthToken = (event: H3Event): string | null => getCookie(event, AUTH_COOKIE) || null;

export const writeAuthToken = (event: H3Event, token: string) => {
  setCookie(event, AUTH_COOKIE, token, {
    httpOnly: true,
    sameSite: 'lax',
    path: '/',
    maxAge: AUTH_COOKIE_MAX_AGE,
    // Off by default: compose serves the app over plain http://localhost:3000 and Safari
    // drops Secure cookies there. Set NUXT_COOKIE_SECURE=true behind an HTTPS proxy.
    secure: process.env.NUXT_COOKIE_SECURE === 'true',
  });
};

export const clearAuthToken = (event: H3Event) => {
  deleteCookie(event, AUTH_COOKIE, { path: '/' });
};
