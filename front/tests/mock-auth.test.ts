// @vitest-environment node
import process from 'node:process';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { DEMO_PASSWORD, accessTtlMs, handleMock, issueMockSession, parseDuration, resetMockState } from '../server/mock';
import type { MockResponse } from '../server/mock';
import type { User } from '../api/types';

interface AuthBody {
  accessToken: string
  refreshToken: string
  user: User
  accessExpiresIn: number
}

const SESSION_EXPIRED = { status: 401, body: { code: 16, message: 'Сессия истекла', details: [] } };

function call(method: string, path: string, token: string | null = null, body?: unknown): MockResponse {
  return handleMock({ method, path, query: {}, token, body });
}

function ok<T>(res: MockResponse): T {
  expect(res.status, JSON.stringify(res.body)).toBe(200);
  return res.body as T;
}

const login = (username: string) =>
  ok<AuthBody>(call('POST', '/v1/auth/login', null, { username, password: DEMO_PASSWORD }));
const refresh = (refreshToken: unknown) => call('POST', '/v1/auth/refresh', null, { refreshToken });
const logout = (refreshToken: unknown) => call('POST', '/v1/auth/logout', null, { refreshToken });
const me = (token: string) => call('GET', '/v1/me', token);

describe('mock auth: tokens', () => {
  beforeEach(() => {
    resetMockState();
    delete process.env.NUXT_MOCK_ACCESS_TTL;
  });

  afterEach(() => {
    vi.useRealTimers();
    delete process.env.NUXT_MOCK_ACCESS_TTL;
  });

  it('login returns the V1AuthResponse shape', () => {
    const res = login('anna');
    expect(Object.keys(res).sort()).toEqual(['accessExpiresIn', 'accessToken', 'refreshToken', 'user']);
    expect(res).not.toHaveProperty('token');
    expect(res.accessExpiresIn).toBe(900);
    expect(res.user.username).toBe('anna');
    expect(res.accessToken).not.toBe('anna');
    expect(res.refreshToken).toMatch(/^[\w-]{43}$/);
    expect(ok<User>(me(res.accessToken)).username).toBe('anna');
  });

  it('register returns the same shape', () => {
    const res = ok<AuthBody>(call('POST', '/v1/auth/register', null, {
      username: 'newbie',
      password: 'secret123',
      displayName: 'Новичок',
    }));
    expect(Object.keys(res).sort()).toEqual(['accessExpiresIn', 'accessToken', 'refreshToken', 'user']);
    expect(res.accessExpiresIn).toBe(900);
    expect(ok<User>(me(res.accessToken)).username).toBe('newbie');
    expect(ok<AuthBody>(refresh(res.refreshToken)).user.username).toBe('newbie');
  });

  it('every login issues distinct tokens', () => {
    const a = login('anna');
    const b = login('anna');
    expect(a.accessToken).not.toBe(b.accessToken);
    expect(a.refreshToken).not.toBe(b.refreshToken);
  });

  it('unknown access token → 401', () => {
    expect(me('mock-access.nope').status).toBe(401);
    expect(me('anna').body).toMatchObject({ code: 16 });
  });

  it('expired access token → 401 (NUXT_MOCK_ACCESS_TTL)', () => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date('2026-10-20T10:00:00Z'));
    process.env.NUXT_MOCK_ACCESS_TTL = '1s';
    const res = login('anna');
    expect(res.accessExpiresIn).toBe(1);
    expect(me(res.accessToken).status).toBe(200);
    vi.setSystemTime(new Date('2026-10-20T10:00:00.999Z'));
    expect(me(res.accessToken).status).toBe(200);
    vi.setSystemTime(new Date('2026-10-20T10:00:01Z'));
    expect(me(res.accessToken).status).toBe(401);
    // The refresh token is still alive: a new access works.
    const next = ok<AuthBody>(refresh(res.refreshToken));
    expect(ok<User>(me(next.accessToken)).username).toBe('anna');
  });

  it('accessExpiresIn rounds up and is at least 1', () => {
    process.env.NUXT_MOCK_ACCESS_TTL = '500ms';
    expect(login('anna').accessExpiresIn).toBe(1);
    process.env.NUXT_MOCK_ACCESS_TTL = '2h';
    expect(login('anna').accessExpiresIn).toBe(7200);
    process.env.NUXT_MOCK_ACCESS_TTL = 'garbage';
    expect(login('anna').accessExpiresIn).toBe(900);
  });

  it('parseDuration / accessTtlMs', () => {
    expect(parseDuration('15m')).toBe(900_000);
    expect(parseDuration('1s')).toBe(1000);
    expect(parseDuration('500ms')).toBe(500);
    expect(parseDuration('2h')).toBe(7_200_000);
    expect(parseDuration('30')).toBe(30_000);
    expect(parseDuration(' 5m ')).toBe(300_000);
    for (const bad of ['', '   ', 'abc', '10d', '-1s', '0', '0s', '1 m', 'm']) {
      expect(parseDuration(bad), bad).toBeNull();
    }
    expect(parseDuration(undefined)).toBeNull();
    expect(accessTtlMs()).toBe(900_000);
    process.env.NUXT_MOCK_ACCESS_TTL = '1s';
    expect(accessTtlMs()).toBe(1000);
    process.env.NUXT_MOCK_ACCESS_TTL = 'nope';
    expect(accessTtlMs()).toBe(900_000);
  });
});

describe('mock auth: refresh rotation', () => {
  beforeEach(() => {
    resetMockState();
  });

  it('refresh issues a new pair; both access tokens work', () => {
    const first = login('anna');
    const second = ok<AuthBody>(refresh(first.refreshToken));
    expect(Object.keys(second).sort()).toEqual(['accessExpiresIn', 'accessToken', 'refreshToken', 'user']);
    expect(second.user.username).toBe('anna');
    expect(second.refreshToken).not.toBe(first.refreshToken);
    expect(second.accessToken).not.toBe(first.accessToken);
    expect(me(first.accessToken).status).toBe(200);
    expect(me(second.accessToken).status).toBe(200);
  });

  it('refresh returns the current user', () => {
    const first = login('anna');
    ok<User>(call('PUT', '/v1/me', first.accessToken, {
      displayName: 'Анна Обновлённая',
      headline: '',
      about: '',
      skills: [],
      telegram: '',
    }));
    expect(ok<AuthBody>(refresh(first.refreshToken)).user.displayName).toBe('Анна Обновлённая');
  });

  it('reuse of a rotated refresh revokes the whole family, other families survive', () => {
    const first = login('anna');
    const other = login('anna');
    const second = ok<AuthBody>(refresh(first.refreshToken));

    expect(refresh(first.refreshToken)).toEqual(SESSION_EXPIRED);
    // The family is revoked: the newest token of this login is dead too.
    expect(refresh(second.refreshToken)).toEqual(SESSION_EXPIRED);

    const otherNext = ok<AuthBody>(refresh(other.refreshToken));
    expect(ok<AuthBody>(refresh(otherNext.refreshToken)).user.username).toBe('anna');
  });

  it('unknown / empty / malformed refresh → 401', () => {
    expect(refresh('nope')).toEqual(SESSION_EXPIRED);
    expect(refresh('')).toEqual(SESSION_EXPIRED);
    expect(call('POST', '/v1/auth/refresh', null, undefined)).toEqual(SESSION_EXPIRED);
    expect(call('POST', '/v1/auth/refresh', null, {})).toEqual(SESSION_EXPIRED);
    // An access token is not a refresh token.
    expect(refresh(login('anna').accessToken)).toEqual(SESSION_EXPIRED);
  });

  it('expired refresh → 401', () => {
    vi.useFakeTimers();
    try {
      vi.setSystemTime(new Date('2026-10-20T10:00:00Z'));
      const session = issueMockSession('boris');
      vi.setSystemTime(new Date('2026-11-19T10:00:00Z'));
      expect(refresh(session.refreshToken)).toEqual(SESSION_EXPIRED);
    } finally {
      vi.useRealTimers();
    }
  });

  it('refresh and logout are public; GET → 405', () => {
    const session = issueMockSession('vera');
    expect(refresh(session.refreshToken).status).toBe(200);
    expect(call('GET', '/v1/auth/refresh').status).toBe(405);
    expect(call('GET', '/v1/auth/logout').status).toBe(405);
  });

  it('resetMockState drops all tokens', () => {
    const session = login('anna');
    resetMockState();
    expect(me(session.accessToken).status).toBe(401);
    expect(refresh(session.refreshToken)).toEqual(SESSION_EXPIRED);
  });
});

describe('mock auth: logout', () => {
  beforeEach(() => {
    resetMockState();
  });

  it('logout revokes the refresh token, idempotently', () => {
    const session = login('anna');
    expect(logout(session.refreshToken)).toEqual({ status: 200, body: {} });
    expect(refresh(session.refreshToken)).toEqual(SESSION_EXPIRED);
    expect(logout(session.refreshToken)).toEqual({ status: 200, body: {} });
  });

  it('logout with unknown / empty token → 200', () => {
    expect(logout('nope')).toEqual({ status: 200, body: {} });
    expect(logout('')).toEqual({ status: 200, body: {} });
    expect(call('POST', '/v1/auth/logout', null, undefined)).toEqual({ status: 200, body: {} });
  });

  it('logout does not affect other sessions', () => {
    const a = login('anna');
    const b = issueMockSession('anna');
    expect(logout(a.refreshToken).status).toBe(200);
    expect(refresh(b.refreshToken).status).toBe(200);
  });
});
