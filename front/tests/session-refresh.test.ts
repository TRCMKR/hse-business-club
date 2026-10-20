// @vitest-environment node
import { createApp, eventHandler, toWebHandler } from 'h3';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { resetMockState } from '../server/mock';
import type { BackendConfig } from '../server/utils/backend';
import { logoutSession, proxyAuth } from '../server/utils/backend-auth';
import { forwardToBackend } from '../server/utils/proxy';
import type { RefreshFn, RefreshOutcome } from '../server/utils/session-refresh';
import { REFRESH_DEDUP_MS, refreshOnce, resetRefreshFlights, withRefresh } from '../server/utils/session-refresh';

const MOCK: BackendConfig = { apiBaseUrl: 'http://127.0.0.1:9', mockApi: true };

/** h3 app with the real Nitro handlers (minus auto-imports), served in-process. */
const makeApp = (config: BackendConfig, extra?: (app: ReturnType<typeof createApp>) => void) => {
  const app = createApp();
  app.use('/api/auth/login', eventHandler(e => proxyAuth(e, config, '/v1/auth/login')));
  app.use('/api/auth/register', eventHandler(e => proxyAuth(e, config, '/v1/auth/register')));
  app.use('/api/auth/logout', eventHandler(e => logoutSession(e, config)));
  extra?.(app);
  app.use('/api', eventHandler(e => forwardToBackend(e, config)));
  return toWebHandler(app);
};

interface SetCookie {
  name: string
  value: string
  attrs: string
}

const parseSetCookies = (res: Response): SetCookie[] =>
  res.headers.getSetCookie().map((line) => {
    const [pair = '', ...attrs] = line.split(';');
    const eq = pair.indexOf('=');
    return { name: pair.slice(0, eq).trim(), value: decodeURIComponent(pair.slice(eq + 1)), attrs: attrs.join(';') };
  });

/** Minimal cookie jar: remembers what the server sets, drops Max-Age=0. */
class Browser {
  jar = new Map<string, string>();
  constructor(private readonly handler: (req: Request) => Promise<Response>) {}

  cookieHeader() {
    return [...this.jar].map(([k, v]) => `${k}=${encodeURIComponent(v)}`).join('; ');
  }

  async fetch(path: string, init: RequestInit = {}) {
    const headers = new Headers(init.headers);
    if (this.jar.size) headers.set('cookie', this.cookieHeader());
    if (init.body) headers.set('content-type', 'application/json');
    const res = await this.handler(new Request(`http://front.test${path}`, { ...init, headers }));
    for (const c of parseSetCookies(res)) {
      if (/max-age=0/i.test(c.attrs) || c.value === '') this.jar.delete(c.name);
      else this.jar.set(c.name, c.value);
    }
    return res;
  }

  login(username = 'anna') {
    return this.fetch('/api/auth/login', { method: 'POST', body: JSON.stringify({ username, password: 'demo1234' }) });
  }
}

let now = Date.now();
const advance = (ms: number) => {
  now += ms;
};

let infoSpy: ReturnType<typeof vi.spyOn>;
const refreshLogs = () => infoSpy.mock.calls.filter(args => args[0] === '[auth] session refreshed').length;

beforeEach(() => {
  now = Date.now();
  vi.spyOn(Date, 'now').mockImplementation(() => now);
  infoSpy = vi.spyOn(console, 'info').mockImplementation(() => {});
  vi.spyOn(console, 'error').mockImplementation(() => {});
  process.env.NUXT_MOCK_ACCESS_TTL = '1s';
  resetMockState();
  resetRefreshFlights();
});

afterEach(() => {
  vi.restoreAllMocks();
  delete process.env.NUXT_MOCK_ACCESS_TTL;
  delete process.env.NUXT_COOKIE_SECURE;
});

describe('auth routes', () => {
  it('login sets two httpOnly cookies and returns only {user}', async () => {
    const browser = new Browser(makeApp(MOCK));
    const res = await browser.login();
    expect(res.status).toBe(200);
    const body = await res.json();
    expect(Object.keys(body)).toEqual(['user']);
    expect(body.user.username).toBe('anna');

    const cookies = parseSetCookies(res);
    expect(cookies.map(c => c.name).sort()).toEqual(['hbc_access', 'hbc_refresh']);
    for (const c of cookies) {
      expect(c.value).not.toBe('');
      expect(c.attrs).toMatch(/HttpOnly/);
      expect(c.attrs).toMatch(/SameSite=Lax/);
      expect(c.attrs).toMatch(/Path=\//);
      expect(c.attrs).toMatch(/Max-Age=2592000/);
      expect(c.attrs).not.toMatch(/Secure/);
    }
    expect(JSON.stringify(body)).not.toContain(browser.jar.get('hbc_access'));
    expect(JSON.stringify(body)).not.toContain(browser.jar.get('hbc_refresh'));
  });

  it('NUXT_COOKIE_SECURE=true adds Secure', async () => {
    process.env.NUXT_COOKIE_SECURE = 'true';
    const res = await new Browser(makeApp(MOCK)).login();
    for (const c of parseSetCookies(res)) {
      expect(c.attrs).toMatch(/Secure/);
    }
  });

  it('register sets both cookies too', async () => {
    const browser = new Browser(makeApp(MOCK));
    const res = await browser.fetch('/api/auth/register', {
      method: 'POST',
      body: JSON.stringify({ username: 'newbie_f9', password: 'secret123', displayName: 'Новичок' }),
    });
    expect(res.status).toBe(200);
    expect([...browser.jar.keys()].sort()).toEqual(['hbc_access', 'hbc_refresh']);
  });

  it('wrong password keeps the backend status and body, no cookies', async () => {
    const browser = new Browser(makeApp(MOCK));
    const res = await browser.fetch('/api/auth/login', { method: 'POST', body: JSON.stringify({ username: 'anna', password: 'nope1234' }) });
    expect(res.status).toBe(401);
    expect((await res.json()).message).toBe('Неверный логин или пароль');
    expect(browser.jar.size).toBe(0);
  });

  it('logout revokes the refresh token and clears both cookies', async () => {
    const browser = new Browser(makeApp(MOCK));
    await browser.login();
    const stale = browser.cookieHeader();

    const res = await browser.fetch('/api/auth/logout', { method: 'POST' });
    expect(res.status).toBe(200);
    expect(await res.json()).toEqual({});
    expect(parseSetCookies(res).map(c => c.name).sort()).toEqual(['hbc_access', 'hbc_refresh']);
    expect(browser.jar.size).toBe(0);

    // The old cookies do not bring the session back once access has expired.
    advance(1500);
    const handler = makeApp(MOCK);
    const replay = await handler(new Request('http://front.test/api/v1/me', { headers: { cookie: stale } }));
    expect(replay.status).toBe(401);
    expect((await replay.json()).message).toBe('Сессия истекла');
  });

  it('logout swallows backend errors', async () => {
    const handler = makeApp({ apiBaseUrl: 'http://127.0.0.1:9', mockApi: false });
    const res = await handler(new Request('http://front.test/api/auth/logout', {
      method: 'POST',
      headers: { cookie: 'hbc_access=a; hbc_refresh=r' },
    }));
    expect(res.status).toBe(200);
    expect(parseSetCookies(res).every(c => c.value === '')).toBe(true);
  });
});

describe('proxy with refresh', () => {
  it('passes a valid access token without refreshing', async () => {
    const browser = new Browser(makeApp(MOCK));
    await browser.login();
    const res = await browser.fetch('/api/v1/me');
    expect(res.status).toBe(200);
    expect((await res.json()).username).toBe('anna');
    expect(parseSetCookies(res)).toEqual([]);
    expect(refreshLogs()).toBe(0);
  });

  it('expired access → one refresh, new cookies, request replayed', async () => {
    const browser = new Browser(makeApp(MOCK));
    await browser.login();
    const before = new Map(browser.jar);

    advance(1500);
    const res = await browser.fetch('/api/v1/me');
    expect(res.status).toBe(200);
    expect((await res.json()).username).toBe('anna');
    expect(refreshLogs()).toBe(1);
    expect(browser.jar.get('hbc_access')).not.toBe(before.get('hbc_access'));
    expect(browser.jar.get('hbc_refresh')).not.toBe(before.get('hbc_refresh'));

    // The new pair works without another refresh.
    expect((await browser.fetch('/api/v1/me')).status).toBe(200);
    expect(refreshLogs()).toBe(1);
  });

  it('replays the buffered request body', async () => {
    const browser = new Browser(makeApp(MOCK));
    await browser.login();
    advance(1500);
    const res = await browser.fetch('/api/v1/listings', {
      method: 'POST',
      body: JSON.stringify({
        kind: 'LISTING_KIND_STARTUP',
        title: 'Replay Startup',
        description: 'Стартап, созданный повтором запроса',
        lookingFor: 'CTO',
        tags: ['Go'],
        stage: 'STARTUP_STAGE_IDEA',
        compensation: '',
      }),
    });
    expect(res.status).toBe(200);
    expect((await res.json()).title).toBe('Replay Startup');
    expect(refreshLogs()).toBe(1);
  });

  it('parallel requests with the same expired session share one refresh', async () => {
    const browser = new Browser(makeApp(MOCK));
    await browser.login();
    advance(1500);

    const responses = await Promise.all([
      browser.fetch('/api/v1/me'),
      browser.fetch('/api/v1/listings'),
      browser.fetch('/api/v1/me/listings'),
      browser.fetch('/api/v1/me/applications/sent'),
      browser.fetch('/api/v1/me/applications/received'),
    ]);
    expect(responses.map(r => r.status)).toEqual([200, 200, 200, 200, 200]);
    expect(refreshLogs()).toBe(1);
    // All of them handed out the same new pair, and the family is still alive.
    const access = new Set(responses.map(r => parseSetCookies(r).find(c => c.name === 'hbc_access')?.value));
    expect(access.size).toBe(1);
    advance(1500);
    expect((await browser.fetch('/api/v1/me')).status).toBe(200);
    expect(refreshLogs()).toBe(2);
  });

  it('reusing a rotated refresh token outside the dedup window → 401 «Сессия истекла», cookies cleared, family revoked', async () => {
    const handler = makeApp(MOCK);
    const browser = new Browser(handler);
    await browser.login();
    const stale = browser.cookieHeader();
    advance(1500);
    expect((await browser.fetch('/api/v1/me')).status).toBe(200);

    resetRefreshFlights(); // > REFRESH_DEDUP_MS later
    const reuse = await handler(new Request('http://front.test/api/v1/me', { headers: { cookie: stale } }));
    expect(reuse.status).toBe(401);
    expect(await reuse.json()).toEqual({ code: 16, message: 'Сессия истекла', details: [] });
    const cleared = parseSetCookies(reuse);
    expect(cleared.map(c => c.name).sort()).toEqual(['hbc_access', 'hbc_refresh']);
    expect(cleared.every(c => c.value === '' && /Max-Age=0/i.test(c.attrs))).toBe(true);

    // Theft detection revoked the whole family: the legit pair dies on its next refresh.
    advance(1500);
    const legit = await browser.fetch('/api/v1/me');
    expect(legit.status).toBe(401);
    expect(browser.jar.size).toBe(0);
  });

  it('no cookies → 401 without a refresh attempt', async () => {
    const browser = new Browser(makeApp(MOCK));
    const res = await browser.fetch('/api/v1/me');
    expect(res.status).toBe(401);
    expect(refreshLogs()).toBe(0);
  });

  it('only a refresh cookie → refresh first, then the call', async () => {
    const browser = new Browser(makeApp(MOCK));
    await browser.login();
    browser.jar.delete('hbc_access');
    const res = await browser.fetch('/api/v1/me');
    expect(res.status).toBe(200);
    expect(refreshLogs()).toBe(1);
    expect(browser.jar.has('hbc_access')).toBe(true);
  });

  it('401 with only an access cookie clears it', async () => {
    const handler = makeApp(MOCK);
    const res = await handler(new Request('http://front.test/api/v1/me', { headers: { cookie: 'hbc_access=garbage' } }));
    expect(res.status).toBe(401);
    expect(parseSetCookies(res).map(c => c.name)).toEqual(['hbc_access', 'hbc_refresh']);
  });

  it.each([
    '/api/v1/auth/refresh',
    '/api/v1/auth/logout',
    '/api/v1/auth/login',
    '/api/v1/%61uth/refresh',
  ])('%s stays closed in the proxy', async (path) => {
    const browser = new Browser(makeApp(MOCK));
    await browser.login();
    const res = await browser.fetch(path, { method: 'POST', body: JSON.stringify({ refreshToken: browser.jar.get('hbc_refresh') }) });
    expect(res.status).toBe(404);
  });

  it('does not forward browser cookies or a client Authorization header', async () => {
    const seen: Headers[] = [];
    vi.stubGlobal('fetch', vi.fn(async (_url: string, init: RequestInit) => {
      seen.push(new Headers(init.headers));
      return new Response('{}', { status: 200, headers: { 'content-type': 'application/json' } });
    }));
    const handler = makeApp({ apiBaseUrl: 'http://backend.test', mockApi: false });
    await handler(new Request('http://front.test/api/v1/me', {
      headers: { cookie: 'hbc_access=A; hbc_refresh=R; other=1', authorization: 'Bearer forged' },
    }));
    vi.unstubAllGlobals();
    expect(seen[0]?.get('authorization')).toBe('Bearer A');
    expect(seen[0]?.get('cookie')).toBeNull();
  });
});

describe('withRefresh', () => {
  const run = (cookie: string, call: (access: string | null) => Promise<Response>, refresh: RefreshFn) =>
    makeApp(MOCK, app => app.use('/probe', eventHandler(e => withRefresh(e, MOCK, call, refresh))))(
      new Request('http://front.test/probe', { headers: { cookie } }),
    );

  const ok = (body: unknown = {}) => new Response(JSON.stringify(body), { status: 200 });
  const unauthorized = () => new Response('{"code":16,"message":"Требуется вход","details":[]}', { status: 401 });

  it('retries once with the new access token', async () => {
    const call = vi.fn(async (access: string | null) => (access === 'new-a' ? ok({ fine: true }) : unauthorized()));
    const refresh = vi.fn<RefreshFn>(async () => ({ ok: true, auth: { accessToken: 'new-a', refreshToken: 'new-r', accessExpiresIn: 900, user: {} as never } }));
    const res = await run('hbc_access=old-a; hbc_refresh=old-r', call, refresh);
    expect(res.status).toBe(200);
    expect(call.mock.calls.map(c => c[0])).toEqual(['old-a', 'new-a']);
    expect(refresh).toHaveBeenCalledTimes(1);
    expect(refresh.mock.calls[0]?.[1]).toBe('old-r');
    expect(parseSetCookies(res).map(c => `${c.name}=${c.value}`).sort()).toEqual(['hbc_access=new-a', 'hbc_refresh=new-r']);
  });

  it('refresh backend unavailable → 502, cookies kept', async () => {
    const res = await run('hbc_access=a; hbc_refresh=r', async () => unauthorized(), async () => ({ ok: false, reason: 'unavailable' }));
    expect(res.status).toBe(502);
    expect(parseSetCookies(res)).toEqual([]);
  });

  it('a non-401 error is returned as is', async () => {
    const refresh = vi.fn<RefreshFn>();
    const res = await run('hbc_access=a; hbc_refresh=r', async () => new Response('{}', { status: 403 }), refresh);
    expect(res.status).toBe(403);
    expect(refresh).not.toHaveBeenCalled();
  });
});

describe('refreshOnce', () => {
  const success: RefreshOutcome = { ok: true, auth: { accessToken: 'a', refreshToken: 'r2', accessExpiresIn: 900, user: {} as never } };

  it('deduplicates concurrent and recent calls with the same token', async () => {
    let resolve!: (o: RefreshOutcome) => void;
    const refresh = vi.fn<RefreshFn>(() => new Promise((r) => {
      resolve = r;
    }));
    const p1 = refreshOnce(MOCK, 'r1', refresh);
    const p2 = refreshOnce(MOCK, 'r1', refresh);
    resolve(success);
    expect(await p1).toBe(await p2);
    expect(await refreshOnce(MOCK, 'r1', refresh)).toBe(success);
    expect(refresh).toHaveBeenCalledTimes(1);
    const other = refreshOnce(MOCK, 'other', refresh);
    expect(refresh).toHaveBeenCalledTimes(2);
    resolve(success);
    await other;
  });

  it('forgets a result after the dedup window', async () => {
    vi.useFakeTimers({ toFake: ['setTimeout', 'clearTimeout'] });
    try {
      const refresh = vi.fn<RefreshFn>(async () => success);
      await refreshOnce(MOCK, 'r1', refresh);
      await vi.advanceTimersByTimeAsync(REFRESH_DEDUP_MS + 1);
      await refreshOnce(MOCK, 'r1', refresh);
      expect(refresh).toHaveBeenCalledTimes(2);
    } finally {
      vi.useRealTimers();
    }
  });

  it('does not remember an unreachable backend', async () => {
    const refresh = vi.fn<RefreshFn>(async () => ({ ok: false, reason: 'unavailable' }));
    await refreshOnce(MOCK, 'r1', refresh);
    await Promise.resolve();
    await refreshOnce(MOCK, 'r1', refresh);
    expect(refresh).toHaveBeenCalledTimes(2);
  });
});
