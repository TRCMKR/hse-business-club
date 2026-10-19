// @vitest-environment node
import { beforeEach, describe, expect, it } from 'vitest';
import { DEMO_PASSWORD, handleMock, resetMockState } from '../server/mock';
import type { MockResponse } from '../server/mock';
import type { Application, Listing, ListingsPage, User } from '../api/types';

interface CallOptions {
  token?: string | null
  body?: unknown
  query?: Record<string, string | undefined>
}

function call(method: string, path: string, opts: CallOptions = {}): MockResponse {
  return handleMock({
    method,
    path,
    query: opts.query ?? {},
    token: opts.token ?? null,
    body: opts.body,
  });
}

function ok<T>(res: MockResponse): T {
  expect(res.status, JSON.stringify(res.body)).toBe(200);
  return res.body as T;
}

function login(username: string): string {
  return ok<{ token: string }>(call('POST', '/v1/auth/login', { body: { username, password: DEMO_PASSWORD } })).token;
}

function feed(token: string, query: Record<string, string> = {}): ListingsPage {
  return ok<ListingsPage>(call('GET', '/v1/listings', { token, query }));
}

function findByTitle(token: string, title: string): Listing {
  const mine = ok<{ items: Listing[] }>(call('GET', '/v1/me/listings', { token })).items;
  const found = [...mine, ...feed(token, { limit: '50' }).items].find(l => l.title === title);
  if (!found) {
    throw new Error(`listing not found: ${title}`);
  }
  return found;
}

const NEW_STARTUP = {
  kind: 'LISTING_KIND_STARTUP',
  title: '  StudyBuddy — поиск напарников по учёбе  ',
  description: 'Сервис, который подбирает студентам напарников для совместной подготовки к экзаменам.',
  lookingFor: 'Мобильный разработчик',
  tags: ['Flutter', ' Mobile ', 'flutter', ''],
  stage: 'STARTUP_STAGE_IDEA',
  compensation: '',
};

const LISTING_KEYS = [
  'author', 'closedAt', 'compensation', 'createdAt', 'description', 'id', 'isMine', 'kind',
  'lookingFor', 'myApplication', 'stage', 'stats', 'status', 'tags', 'title', 'updatedAt',
];
const USER_KEYS = ['about', 'avatarUrl', 'createdAt', 'displayName', 'headline', 'id', 'skills', 'telegram', 'username'];
const APPLICATION_KEYS = ['applicant', 'createdAt', 'decidedAt', 'id', 'listing', 'message', 'status', 'updatedAt'];
const RFC3339 = /^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}Z$/;
// 1×1 transparent PNG.
const PNG_BASE64 = 'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNkYPhfDwAChwGA60e6kgAAAABJRU5ErkJggg==';

describe('mock API', () => {
  beforeEach(() => {
    resetMockState();
  });

  it('runs the main scenario of SPEC §1', () => {
    const anna = login('anna');
    const boris = login('boris');

    const created = ok<Listing>(call('POST', '/v1/listings', { token: anna, body: NEW_STARTUP }));
    expect(created.title).toBe('StudyBuddy — поиск напарников по учёбе');
    expect(created.tags).toEqual(['Flutter', 'Mobile']);
    expect(created.isMine).toBe(true);
    expect(created.stats).toEqual({ pending: 0, accepted: 0, declined: 0, total: 0 });

    const borisFeed = feed(boris);
    expect(borisFeed.items[0]!.id).toBe(created.id);
    expect(borisFeed.items[0]!.isMine).toBe(false);
    expect(borisFeed.items[0]!.stats).toBeNull();
    expect(borisFeed.total).toBe(6);

    const application = ok<Application>(call('POST', `/v1/listings/${created.id}/applications`, {
      token: boris,
      body: { message: 'Хочу помочь с мобильной разработкой' },
    }));
    expect(application.status).toBe('APPLICATION_STATUS_PENDING');
    expect(application.listing.id).toBe(created.id);
    expect(application.applicant.username).toBe('boris');

    const again = call('POST', `/v1/listings/${created.id}/applications`, { token: boris, body: { message: 'Ещё раз' } });
    expect(again).toEqual({
      status: 409,
      body: { code: 6, message: 'Вы уже откликались на эту публикацию', details: [] },
    });

    const viewed = ok<Listing>(call('GET', `/v1/listings/${created.id}`, { token: boris }));
    expect(viewed.myApplication?.id).toBe(application.id);

    const received = ok<{ items: Application[] }>(call('GET', '/v1/me/applications/received', { token: anna })).items;
    expect(received[0]!.id).toBe(application.id);
    expect(received[0]!.status).toBe('APPLICATION_STATUS_PENDING');

    const listingApps = ok<{ items: Application[] }>(
      call('GET', `/v1/listings/${created.id}/applications`, { token: anna }),
    ).items;
    expect(listingApps.map(a => a.id)).toEqual([application.id]);

    const accepted = ok<Application>(call('POST', `/v1/applications/${application.id}/accept`, { token: anna }));
    expect(accepted.status).toBe('APPLICATION_STATUS_ACCEPTED');
    expect(accepted.decidedAt).toMatch(RFC3339);
    // Idempotent repeat.
    expect(ok<Application>(call('POST', `/v1/applications/${application.id}/accept`, { token: anna })).status)
      .toBe('APPLICATION_STATUS_ACCEPTED');
    expect(call('POST', `/v1/applications/${application.id}/decline`, { token: anna }).body)
      .toMatchObject({ code: 9, message: 'Решение по отклику уже принято' });

    const sent = ok<{ items: Application[] }>(call('GET', '/v1/me/applications/sent', { token: boris })).items;
    expect(sent.find(a => a.id === application.id)?.status).toBe('APPLICATION_STATUS_ACCEPTED');

    const stats = ok<Listing>(call('GET', `/v1/listings/${created.id}`, { token: anna })).stats;
    expect(stats).toEqual({ pending: 0, accepted: 1, declined: 0, total: 1 });
  });

  it('serializes every field of Listing / User / Application', () => {
    const boris = login('boris');
    const listing = findByTitle(boris, 'FinTrack — учёт личных финансов для студентов');
    expect(Object.keys(listing).sort()).toEqual(LISTING_KEYS);
    expect(Object.keys(listing.author).sort()).toEqual(USER_KEYS);
    expect(listing.kind).toBe('LISTING_KIND_STARTUP');
    expect(listing.stage).toBe('STARTUP_STAGE_MVP');
    expect(listing.createdAt).toMatch(RFC3339);
    expect(listing.closedAt).toBeNull();
    expect(listing.author.avatarUrl).toBe('');
    expect(listing.myApplication).not.toBeNull();
    expect(Object.keys(listing.myApplication!).sort()).toEqual(APPLICATION_KEYS);
    expect(Object.keys(listing.myApplication!.listing).sort()).toEqual(['author', 'id', 'kind', 'status', 'title']);
    expect(listing.myApplication!.decidedAt).toBeNull();
  });

  it('feed: only active, newest first, filters and pagination', () => {
    const anna = login('anna');
    const all = feed(anna);
    expect(all.total).toBe(5);
    expect(all.items.every(l => l.status === 'LISTING_STATUS_ACTIVE')).toBe(true);
    const created = all.items.map(l => l.createdAt);
    expect([...created].sort().reverse()).toEqual(created);
    expect(feed(anna, { kind: 'LISTING_KIND_VACANCY' }).total).toBe(2);
    expect(feed(anna, { query: 'fintrack' }).total).toBe(2);
    const page2 = feed(anna, { page: '2', limit: '2' });
    expect(page2.items.map(l => l.id)).toEqual(all.items.slice(2, 4).map(l => l.id));
    expect(call('GET', '/v1/listings', { token: anna, query: { limit: '51' } }).status).toBe(400);
    expect(call('GET', '/v1/listings', { token: anna, query: { page: '-1' } }).status).toBe(400);
    expect(call('GET', '/v1/listings', { token: anna, query: { kind: 'NOPE' } }).status).toBe(400);
  });

  it('rejects applying to own listing', () => {
    const anna = login('anna');
    const own = findByTitle(anna, 'Backend-разработчик в FinTrack');
    const res = call('POST', `/v1/listings/${own.id}/applications`, { token: anna, body: { message: 'Привет' } });
    expect(res).toEqual({ status: 400, body: { code: 9, message: 'Нельзя откликнуться на свою публикацию', details: [] } });
  });

  it('rejects applying to a closed listing', () => {
    const dmitry = login('dmitry');
    const vera = login('vera');
    const closed = findByTitle(dmitry, 'Frontend-разработчик в Volunteer Hub');
    expect(closed.status).toBe('LISTING_STATUS_CLOSED');
    expect(closed.closedAt).toMatch(RFC3339);
    const res = call('POST', `/v1/listings/${closed.id}/applications`, { token: vera, body: { message: 'Привет' } });
    expect(res).toEqual({ status: 400, body: { code: 9, message: 'Публикация закрыта', details: [] } });
    // Still readable by a direct link.
    expect(call('GET', `/v1/listings/${closed.id}`, { token: vera }).status).toBe(200);
  });

  it('enforces permissions on applications (403 for participants, 404 for strangers)', () => {
    const anna = login('anna');
    const boris = login('boris');
    const dmitry = login('dmitry');
    const pending = ok<{ items: Application[] }>(call('GET', '/v1/me/applications/sent', { token: boris }))
      .items.find(a => a.status === 'APPLICATION_STATUS_PENDING')!;
    expect(pending.listing.author.username).toBe('anna');

    // Applicant cannot accept own application.
    expect(call('POST', `/v1/applications/${pending.id}/accept`, { token: boris }).status).toBe(403);
    // Author cannot withdraw.
    expect(call('POST', `/v1/applications/${pending.id}/withdraw`, { token: anna }).status).toBe(403);
    // Stranger does not learn the application exists.
    expect(call('POST', `/v1/applications/${pending.id}/accept`, { token: dmitry }).body)
      .toEqual({ code: 5, message: 'Отклик не найден', details: [] });
    // Listing applications — author only.
    const listingId = pending.listing.id;
    expect(call('GET', `/v1/listings/${listingId}/applications`, { token: boris }).status).toBe(403);
    // Edit/close — author only.
    expect(call('POST', `/v1/listings/${listingId}/close`, { token: boris }).status).toBe(403);
    expect(call('GET', '/v1/listings/00000000-0000-0000-0000-000000000000', { token: boris }).status).toBe(404);
  });

  it('allows re-applying after withdraw', () => {
    const vera = login('vera');
    const hub = findByTitle(vera, 'Volunteer Hub — платформа волонтёрских проектов ВШЭ');
    expect(hub.myApplication).toBeNull();
    const first = ok<Application>(call('POST', `/v1/listings/${hub.id}/applications`, { token: vera, body: { message: 'Снова я' } }));
    const withdrawn = ok<Application>(call('POST', `/v1/applications/${first.id}/withdraw`, { token: vera }));
    expect(withdrawn.status).toBe('APPLICATION_STATUS_WITHDRAWN');
    expect(withdrawn.decidedAt).toBeNull();
    expect(call('POST', `/v1/applications/${first.id}/withdraw`, { token: vera }).status).toBe(200);
    const second = call('POST', `/v1/listings/${hub.id}/applications`, { token: vera, body: { message: 'И ещё раз' } });
    expect(second.status).toBe(200);
  });

  it('uploads, serves and deletes an avatar', () => {
    const anna = login('anna');
    const user = ok<User>(call('POST', '/v1/me/avatar', { token: anna, body: { data: PNG_BASE64 } }));
    expect(user.avatarUrl).toMatch(/^\/v1\/avatars\/[0-9a-f-]{36}\.jpg$/);
    expect(ok<User>(call('GET', '/v1/me', { token: anna })).avatarUrl).toBe(user.avatarUrl);

    const file = call('GET', user.avatarUrl);
    expect(file.status).toBe(200);
    expect(file.headers?.['Content-Type']).toBe('image/png');
    expect(file.headers?.['Cache-Control']).toBe('public, max-age=31536000, immutable');
    expect(Buffer.from(file.body as Uint8Array).toString('base64')).toBe(PNG_BASE64);

    const replaced = ok<User>(call('POST', '/v1/me/avatar', { token: anna, body: { data: PNG_BASE64 } }));
    expect(replaced.avatarUrl).not.toBe(user.avatarUrl);
    expect(call('GET', user.avatarUrl).body).toEqual({ code: 5, message: 'Фото не найдено', details: [] });

    expect(ok<User>(call('DELETE', '/v1/me/avatar', { token: anna })).avatarUrl).toBe('');
    expect(ok<User>(call('DELETE', '/v1/me/avatar', { token: anna })).avatarUrl).toBe('');
  });

  it('rejects non-image avatar bytes', () => {
    const anna = login('anna');
    const res = call('POST', '/v1/me/avatar', { token: anna, body: { data: Buffer.from('hello world').toString('base64') } });
    expect(res).toEqual({ status: 400, body: { code: 3, message: 'Фото: поддерживаются JPEG, PNG и WebP', details: [] } });
  });

  it('auth: wrong password, missing token, register', () => {
    expect(call('POST', '/v1/auth/login', { body: { username: 'anna', password: 'wrong-pass' } })).toEqual({
      status: 401,
      body: { code: 16, message: 'Неверный логин или пароль', details: [] },
    });
    const noToken = call('GET', '/v1/me');
    expect(noToken.status).toBe(401);
    expect(noToken.body).toMatchObject({ code: 16 });
    expect(call('GET', '/v1/me', { token: 'ghost' }).status).toBe(401);
    expect(call('GET', '/v1/ping')).toEqual({ status: 200, body: { ok: true } });

    const reg = ok<{ token: string, user: User }>(call('POST', '/v1/auth/register', {
      body: { username: 'NewUser', password: 'secret123', displayName: 'Новичок' },
    }));
    expect(reg.user.username).toBe('newuser');
    expect(ok<User>(call('GET', '/v1/me', { token: reg.token })).displayName).toBe('Новичок');
    expect(call('POST', '/v1/auth/login', { body: { username: 'newuser', password: 'secret123' } }).status).toBe(200);
    expect(call('POST', '/v1/auth/register', { body: { username: 'anna', password: 'secret123', displayName: 'A' } }).body)
      .toEqual({ code: 6, message: 'Логин уже занят', details: [] });
    expect(call('POST', '/v1/auth/register', { body: { username: 'a', password: 'secret123', displayName: 'A' } }).body)
      .toEqual({ code: 3, message: 'Логин: от 3 до 32 символов', details: [] });
  });

  it('validates listings and profile', () => {
    const anna = login('anna');
    expect(call('POST', '/v1/listings', { token: anna, body: { ...NEW_STARTUP, title: 'ab' } }).body)
      .toEqual({ code: 3, message: 'Название: от 3 до 120 символов', details: [] });
    expect(call('POST', '/v1/listings', { token: anna, body: { ...NEW_STARTUP, kind: 'LISTING_KIND_VACANCY' } }).status)
      .toBe(400);
    expect(call('POST', '/v1/listings', { token: anna, body: { ...NEW_STARTUP, compensation: 'деньги' } }).status)
      .toBe(400);

    const me = ok<User>(call('PUT', '/v1/me', {
      token: anna,
      body: { displayName: ' Анна ', headline: '', about: '', skills: ['Go', 'go', ' '], telegram: '@anna' },
    }));
    expect(me).toMatchObject({ displayName: 'Анна', skills: ['Go'], telegram: 'anna' });
  });

  it('routes unknown paths and methods', () => {
    expect(call('GET', '/v1/nope')).toEqual({ status: 404, body: { code: 5, message: 'Не найдено', details: [] } });
    expect(call('DELETE', '/v1/listings').status).toBe(405);
  });
});
