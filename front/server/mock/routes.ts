/**
 * Endpoint handlers (SPEC §4.1) and a tiny router: method + path template with `:param`
 * segments. Business rules — SPEC §2.3.
 */
import { Buffer } from 'node:buffer';
import type { MockRequest, MockResponse } from './types';
import {
  alreadyExists,
  failedPrecondition,
  invalidArgument,
  methodNotAllowed,
  notFound,
  permissionDenied,
  unauthenticated,
} from './errors';
import type { ApplicationRecord, ApplicationStatus, ListingRecord, MockState, UserRecord } from './state';
import {
  byCreatedDesc,
  byPendingThenCreatedDesc,
  findUserByUsername,
  newId,
  nextSeq,
} from './state';
import { authenticateAccess, issueSession, revokeRefresh, rotateRefresh } from './sessions';
import { activeApplicationOf, toApplication, toListing, toUser } from './serialize';
import {
  AVATAR_MAX_BYTES,
  MESSAGES,
  asBody,
  detectImageType,
  parseApplicationStatus,
  parseListingStatus,
  readString,
  validateDisplayName,
  validateFeedParams,
  validateKind,
  validateListingFields,
  validateMessage,
  validatePassword,
  validateProfile,
  validateUsername,
} from './validation';

export interface Context {
  state: MockState
  req: MockRequest
  params: Record<string, string>
  /** Set for routes with `auth: true`. */
  viewer: UserRecord
}

/** Non-JSON response (avatar bytes) passed through as is. */
export class RawResponse {
  constructor(readonly response: MockResponse) {}
}

type Handler = (ctx: Context) => unknown;

interface Route {
  method: string
  pattern: string[]
  auth: boolean
  handler: Handler
}

const MSG = {
  wrongCredentials: 'Неверный логин или пароль',
  usernameTaken: 'Логин уже занят',
  userNotFound: 'Участник не найден',
  listingNotFound: 'Публикация не найдена',
  applicationNotFound: 'Отклик не найден',
  avatarNotFound: 'Фото не найдено',
  ownListing: 'Нельзя откликнуться на свою публикацию',
  listingClosed: 'Публикация закрыта',
  listingClosedEdit: 'Закрытую публикацию нельзя редактировать',
  alreadyApplied: 'Вы уже откликались на эту публикацию',
  alreadyDecided: 'Решение по отклику уже принято',
  editAuthorOnly: 'Редактировать публикацию может только автор',
  closeAuthorOnly: 'Закрыть публикацию может только автор',
  viewApplicationsAuthorOnly: 'Смотреть отклики может только автор публикации',
  decideAuthorOnly: 'Принять или отклонить отклик может только автор публикации',
  withdrawApplicantOnly: 'Отозвать отклик может только тот, кто его отправил',
} as const;

const now = () => new Date();

// ---------- lookups ----------

function getListing(ctx: Context, id: string): ListingRecord {
  const listing = ctx.state.listings.get(id);
  if (!listing) {
    throw notFound(MSG.listingNotFound);
  }
  return listing;
}

/** Applications are hidden (404, not 403) from anyone but the listing author and the applicant. */
function getVisibleApplication(ctx: Context, id: string): { application: ApplicationRecord, listing: ListingRecord } {
  const application = ctx.state.applications.get(id);
  const listing = application && ctx.state.listings.get(application.listingId);
  if (!application || !listing
    || (listing.authorId !== ctx.viewer.id && application.applicantId !== ctx.viewer.id)) {
    throw notFound(MSG.applicationNotFound);
  }
  return { application, listing };
}

const applicationList = (ctx: Context, items: ApplicationRecord[]) => ({
  items: items.map(a => toApplication(ctx.state, a)),
});

function filterByStatus(items: ApplicationRecord[], raw: string | undefined): ApplicationRecord[] {
  const status = parseApplicationStatus(raw);
  return status === 'APPLICATION_STATUS_UNSPECIFIED' ? items : items.filter(a => a.status === status);
}

// ---------- auth ----------

/** `V1AuthResponse`; a new token family. */
function authResponse(state: MockState, user: UserRecord) {
  return authBody(user, issueSession(state, user));
}

function authBody(user: UserRecord, pair: ReturnType<typeof issueSession>) {
  return {
    accessToken: pair.accessToken,
    refreshToken: pair.refreshToken,
    user: toUser(user),
    accessExpiresIn: pair.accessExpiresIn,
  };
}

function register(ctx: Context) {
  const body = asBody(ctx.req.body);
  const username = validateUsername(readString(body, 'username'));
  const password = validatePassword(readString(body, 'password'));
  const displayName = validateDisplayName(readString(body, 'displayName'));
  if (findUserByUsername(ctx.state, username)) {
    throw alreadyExists(MSG.usernameTaken);
  }
  const at = now();
  const user: UserRecord = {
    id: newId(),
    username,
    password,
    displayName,
    headline: '',
    about: '',
    skills: [],
    telegram: '',
    avatarKey: '',
    createdAt: at,
    updatedAt: at,
  };
  ctx.state.users.set(user.id, user);
  return authResponse(ctx.state, user);
}

function login(ctx: Context) {
  const body = asBody(ctx.req.body);
  const user = findUserByUsername(ctx.state, readString(body, 'username').trim().toLowerCase());
  if (!user || user.password !== readString(body, 'password')) {
    throw unauthenticated(MSG.wrongCredentials);
  }
  return authResponse(ctx.state, user);
}

function refresh(ctx: Context) {
  const { user, pair } = rotateRefresh(ctx.state, readString(asBody(ctx.req.body), 'refreshToken'));
  return authBody(user, pair);
}

function logout(ctx: Context) {
  revokeRefresh(ctx.state, readString(asBody(ctx.req.body), 'refreshToken'));
  return {};
}

/** Issues a session without a password check (test helper). */
export function issueSessionFor(state: MockState, username: string) {
  const user = findUserByUsername(state, username);
  if (!user) {
    throw new Error(`mock user not found: ${username}`);
  }
  return issueSession(state, user);
}

// ---------- users ----------

function updateMe(ctx: Context) {
  const fields = validateProfile(asBody(ctx.req.body));
  Object.assign(ctx.viewer, fields, { updatedAt: now() });
  return toUser(ctx.viewer);
}

function getUser(ctx: Context) {
  const user = ctx.state.users.get(ctx.params.id!);
  if (!user) {
    throw notFound(MSG.userNotFound);
  }
  return toUser(user);
}

function uploadAvatar(ctx: Context) {
  const data = readString(asBody(ctx.req.body), 'data').replace(/\s+/g, '');
  if (!data || !/^[\w+/-]+={0,2}$/.test(data)) {
    throw invalidArgument(MESSAGES.avatarType);
  }
  const bytes = new Uint8Array(Buffer.from(data, 'base64'));
  if (bytes.length > AVATAR_MAX_BYTES) {
    throw invalidArgument(MESSAGES.avatarSize);
  }
  const contentType = detectImageType(bytes);
  if (!contentType) {
    throw invalidArgument(MESSAGES.avatarType);
  }
  // No resize/re-encode in the mock: bytes are stored as is with the source content type.
  const key = `${newId()}.jpg`;
  ctx.state.avatars.set(key, { bytes, contentType });
  if (ctx.viewer.avatarKey) {
    ctx.state.avatars.delete(ctx.viewer.avatarKey);
  }
  ctx.viewer.avatarKey = key;
  ctx.viewer.updatedAt = now();
  return toUser(ctx.viewer);
}

function deleteAvatar(ctx: Context) {
  if (ctx.viewer.avatarKey) {
    ctx.state.avatars.delete(ctx.viewer.avatarKey);
    ctx.viewer.avatarKey = '';
    ctx.viewer.updatedAt = now();
  }
  return toUser(ctx.viewer);
}

function getAvatar(ctx: Context): RawResponse {
  const file = ctx.state.avatars.get(ctx.params.file!);
  if (!file) {
    throw notFound(MSG.avatarNotFound);
  }
  return new RawResponse({
    status: 200,
    body: new Uint8Array(file.bytes),
    headers: {
      'Content-Type': file.contentType,
      'Cache-Control': 'public, max-age=31536000, immutable',
    },
  });
}

// ---------- listings ----------

function listListings(ctx: Context) {
  const { kind, query, page, limit } = validateFeedParams(ctx.req.query);
  const needle = query.toLowerCase();
  const matched = [...ctx.state.listings.values()]
    .filter(l => l.status === 'LISTING_STATUS_ACTIVE')
    .filter(l => kind === 'LISTING_KIND_UNSPECIFIED' || l.kind === kind)
    .filter(l => !needle || l.title.toLowerCase().includes(needle) || l.description.toLowerCase().includes(needle))
    .sort(byCreatedDesc);
  const items = matched.slice((page - 1) * limit, page * limit);
  return {
    items: items.map(l => toListing(ctx.state, l, ctx.viewer)),
    total: matched.length,
    page,
    limit,
  };
}

function createListing(ctx: Context) {
  const body = asBody(ctx.req.body);
  const kind = validateKind(body.kind);
  const fields = validateListingFields(body, kind);
  const at = now();
  const listing: ListingRecord = {
    id: newId(),
    seq: nextSeq(ctx.state),
    kind,
    ...fields,
    status: 'LISTING_STATUS_ACTIVE',
    authorId: ctx.viewer.id,
    createdAt: at,
    updatedAt: at,
    closedAt: null,
  };
  ctx.state.listings.set(listing.id, listing);
  return toListing(ctx.state, listing, ctx.viewer);
}

const getListingHandler = (ctx: Context) => toListing(ctx.state, getListing(ctx, ctx.params.id!), ctx.viewer);

function updateListing(ctx: Context) {
  const listing = getListing(ctx, ctx.params.id!);
  if (listing.authorId !== ctx.viewer.id) {
    throw permissionDenied(MSG.editAuthorOnly);
  }
  if (listing.status !== 'LISTING_STATUS_ACTIVE') {
    throw failedPrecondition(MSG.listingClosedEdit);
  }
  // `kind` is immutable; a `kind` in the body is ignored (not part of V1UpdateListingRequest).
  Object.assign(listing, validateListingFields(asBody(ctx.req.body), listing.kind), { updatedAt: now() });
  return toListing(ctx.state, listing, ctx.viewer);
}

function closeListing(ctx: Context) {
  const listing = getListing(ctx, ctx.params.id!);
  if (listing.authorId !== ctx.viewer.id) {
    throw permissionDenied(MSG.closeAuthorOnly);
  }
  if (listing.status !== 'LISTING_STATUS_CLOSED') {
    const at = now();
    listing.status = 'LISTING_STATUS_CLOSED';
    listing.closedAt = at;
    listing.updatedAt = at;
  }
  return toListing(ctx.state, listing, ctx.viewer);
}

function listMyListings(ctx: Context) {
  const status = parseListingStatus(ctx.req.query.status);
  const items = [...ctx.state.listings.values()]
    .filter(l => l.authorId === ctx.viewer.id)
    .filter(l => status === 'LISTING_STATUS_UNSPECIFIED' || l.status === status)
    .sort(byCreatedDesc);
  return { items: items.map(l => toListing(ctx.state, l, ctx.viewer)) };
}

// ---------- applications ----------

function createApplication(ctx: Context) {
  const message = validateMessage(asBody(ctx.req.body));
  const listing = getListing(ctx, ctx.params.listingId!);
  if (listing.authorId === ctx.viewer.id) {
    throw failedPrecondition(MSG.ownListing);
  }
  if (listing.status !== 'LISTING_STATUS_ACTIVE') {
    throw failedPrecondition(MSG.listingClosed);
  }
  if (activeApplicationOf(ctx.state, listing.id, ctx.viewer.id)) {
    throw alreadyExists(MSG.alreadyApplied);
  }
  const at = now();
  const application: ApplicationRecord = {
    id: newId(),
    seq: nextSeq(ctx.state),
    listingId: listing.id,
    applicantId: ctx.viewer.id,
    message,
    status: 'APPLICATION_STATUS_PENDING',
    createdAt: at,
    updatedAt: at,
    decidedAt: null,
  };
  ctx.state.applications.set(application.id, application);
  return toApplication(ctx.state, application);
}

function listListingApplications(ctx: Context) {
  const listing = getListing(ctx, ctx.params.listingId!);
  if (listing.authorId !== ctx.viewer.id) {
    throw permissionDenied(MSG.viewApplicationsAuthorOnly);
  }
  const items = [...ctx.state.applications.values()].filter(a => a.listingId === listing.id);
  return applicationList(ctx, filterByStatus(items, ctx.req.query.status).sort(byPendingThenCreatedDesc));
}

/**
 * pending → target; target → target is idempotent (200, current state);
 * any other final status → 400.
 */
function transition(ctx: Context, target: ApplicationStatus) {
  const { application, listing } = getVisibleApplication(ctx, ctx.params.id!);
  if (target === 'APPLICATION_STATUS_WITHDRAWN') {
    if (application.applicantId !== ctx.viewer.id) {
      throw permissionDenied(MSG.withdrawApplicantOnly);
    }
  } else if (listing.authorId !== ctx.viewer.id) {
    throw permissionDenied(MSG.decideAuthorOnly);
  }
  if (application.status !== target) {
    if (application.status !== 'APPLICATION_STATUS_PENDING') {
      throw failedPrecondition(MSG.alreadyDecided);
    }
    const at = now();
    application.status = target;
    application.updatedAt = at;
    if (target !== 'APPLICATION_STATUS_WITHDRAWN') {
      application.decidedAt = at;
    }
  }
  return toApplication(ctx.state, application);
}

function listSent(ctx: Context) {
  const items = [...ctx.state.applications.values()].filter(a => a.applicantId === ctx.viewer.id);
  return applicationList(ctx, filterByStatus(items, ctx.req.query.status).sort(byCreatedDesc));
}

function listReceived(ctx: Context) {
  const items = [...ctx.state.applications.values()].filter(
    a => ctx.state.listings.get(a.listingId)?.authorId === ctx.viewer.id,
  );
  return applicationList(ctx, filterByStatus(items, ctx.req.query.status).sort(byPendingThenCreatedDesc));
}

// ---------- router ----------

const route = (method: string, path: string, auth: boolean, handler: Handler): Route => ({
  method,
  pattern: path.split('/').filter(Boolean),
  auth,
  handler,
});

export const ROUTES: Route[] = [
  route('GET', '/v1/ping', false, () => ({ ok: true })),
  route('POST', '/v1/auth/register', false, register),
  route('POST', '/v1/auth/login', false, login),
  route('POST', '/v1/auth/refresh', false, refresh),
  route('POST', '/v1/auth/logout', false, logout),
  route('GET', '/v1/me', true, ctx => toUser(ctx.viewer)),
  route('PUT', '/v1/me', true, updateMe),
  route('GET', '/v1/users/:id', true, getUser),
  route('POST', '/v1/me/avatar', true, uploadAvatar),
  route('DELETE', '/v1/me/avatar', true, deleteAvatar),
  route('GET', '/v1/avatars/:file', false, getAvatar),
  route('GET', '/v1/listings', true, listListings),
  route('POST', '/v1/listings', true, createListing),
  route('GET', '/v1/listings/:id', true, getListingHandler),
  route('PUT', '/v1/listings/:id', true, updateListing),
  route('POST', '/v1/listings/:id/close', true, closeListing),
  route('GET', '/v1/me/listings', true, listMyListings),
  route('POST', '/v1/listings/:listingId/applications', true, createApplication),
  route('GET', '/v1/listings/:listingId/applications', true, listListingApplications),
  route('POST', '/v1/applications/:id/accept', true, ctx => transition(ctx, 'APPLICATION_STATUS_ACCEPTED')),
  route('POST', '/v1/applications/:id/decline', true, ctx => transition(ctx, 'APPLICATION_STATUS_DECLINED')),
  route('POST', '/v1/applications/:id/withdraw', true, ctx => transition(ctx, 'APPLICATION_STATUS_WITHDRAWN')),
  route('GET', '/v1/me/applications/sent', true, listSent),
  route('GET', '/v1/me/applications/received', true, listReceived),
];

function matchPattern(pattern: string[], segments: string[]): Record<string, string> | null {
  if (pattern.length !== segments.length) {
    return null;
  }
  const params: Record<string, string> = {};
  for (let i = 0; i < pattern.length; i++) {
    const p = pattern[i]!;
    const s = segments[i]!;
    if (s === '') {
      return null;
    }
    if (p.startsWith(':')) {
      try {
        params[p.slice(1)] = decodeURIComponent(s);
      } catch {
        return null;
      }
    } else if (p !== s) {
      return null;
    }
  }
  return params;
}

/** Finds the route; unknown path → 404, known path with another method → 405. */
export function resolve(method: string, path: string): { route: Route, params: Record<string, string> } {
  // Strict matching: no collapsing of `//` and no trailing-slash tolerance.
  const clean = path.split('?')[0]!;
  if (!clean.startsWith('/')) {
    throw notFound();
  }
  const segments = clean.slice(1).split('/');
  let pathMatched = false;
  for (const r of ROUTES) {
    const params = matchPattern(r.pattern, segments);
    if (!params) {
      continue;
    }
    if (r.method === method.toUpperCase()) {
      return { route: r, params };
    }
    pathMatched = true;
  }
  throw pathMatched ? methodNotAllowed() : notFound();
}

export function authenticate(state: MockState, token: string | null): UserRecord {
  return authenticateAccess(state, token);
}
