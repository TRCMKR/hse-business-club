/**
 * Mock session tokens (SPEC §2.3, §4.3): opaque access tokens with a short TTL and
 * rotating refresh tokens grouped into families. Stored as is — this is a mock
 * (the real backend issues a JWT access token and keeps only sha256 of refresh tokens).
 */
import process from 'node:process';
import { randomBytes, randomUUID } from 'node:crypto';
import { unauthenticated } from './errors';
import type { MockState, UserRecord } from './state';

export const DEFAULT_ACCESS_TTL_MS = 15 * 60 * 1000;
export const REFRESH_TTL_MS = 30 * 24 * 60 * 60 * 1000;
export const SESSION_EXPIRED = 'Сессия истекла';

export interface AccessTokenRecord {
  userId: string
  expiresAt: number
}

export interface RefreshTokenRecord {
  userId: string
  familyId: string
  expiresAt: number
  revokedAt: number | null
  /** True when revoked by rotation (a replacement was issued). */
  rotated: boolean
}

export interface SessionPair {
  accessToken: string
  refreshToken: string
  accessExpiresIn: number
}

const UNIT_MS: Record<string, number> = { ms: 1, s: 1000, m: 60_000, h: 3_600_000 };

/**
 * Parses `500ms`, `1s`, `15m`, `2h` or a bare number (seconds) into milliseconds.
 * Returns null for empty, malformed or non-positive values.
 */
export function parseDuration(raw: string | undefined | null): number | null {
  const match = /^(\d+(?:\.\d+)?)(ms|[hms])?$/.exec((raw ?? '').trim());
  if (!match) {
    return null;
  }
  const ms = Number(match[1]) * UNIT_MS[match[2] ?? 's']!;
  return Number.isFinite(ms) && ms > 0 ? ms : null;
}

/** Access TTL from `NUXT_MOCK_ACCESS_TTL`, read on every issue; invalid/empty → 15m. */
export function accessTtlMs(): number {
  return parseDuration(process.env.NUXT_MOCK_ACCESS_TTL) ?? DEFAULT_ACCESS_TTL_MS;
}

const opaque = (prefix: string, bytes: number) => prefix + randomBytes(bytes).toString('base64url');

/** Issues an access + refresh pair; a new family unless `familyId` is given. */
export function issueSession(state: MockState, user: UserRecord, familyId: string = randomUUID()): SessionPair {
  const now = Date.now();
  const ttl = accessTtlMs();
  const accessToken = opaque('mock-access.', 24);
  state.accessTokens.set(accessToken, { userId: user.id, expiresAt: now + ttl });
  const refreshToken = opaque('', 32);
  state.refreshTokens.set(refreshToken, {
    userId: user.id,
    familyId,
    expiresAt: now + REFRESH_TTL_MS,
    revokedAt: null,
    rotated: false,
  });
  return { accessToken, refreshToken, accessExpiresIn: Math.max(1, Math.ceil(ttl / 1000)) };
}

/** Resolves a live access token to its user; unknown/expired → 401. */
export function authenticateAccess(state: MockState, token: string | null): UserRecord {
  const record = token ? state.accessTokens.get(token) : undefined;
  if (!record || record.expiresAt <= Date.now()) {
    if (token && record) {
      state.accessTokens.delete(token);
    }
    throw unauthenticated();
  }
  const user = state.users.get(record.userId);
  if (!user) {
    throw unauthenticated();
  }
  return user;
}

function revokeFamily(state: MockState, familyId: string, at: number): void {
  for (const record of state.refreshTokens.values()) {
    if (record.familyId === familyId && record.revokedAt === null) {
      record.revokedAt = at;
    }
  }
}

/**
 * Rotates a refresh token: unknown/expired → 401; already revoked (reuse) → the whole
 * family is revoked and 401; otherwise the token is revoked and a new pair is issued
 * in the same family.
 */
export function rotateRefresh(state: MockState, token: string): { user: UserRecord, pair: SessionPair } {
  const now = Date.now();
  const record = token ? state.refreshTokens.get(token) : undefined;
  if (!record || record.expiresAt <= now) {
    throw unauthenticated(SESSION_EXPIRED);
  }
  if (record.revokedAt !== null) {
    revokeFamily(state, record.familyId, now);
    throw unauthenticated(SESSION_EXPIRED);
  }
  const user = state.users.get(record.userId);
  if (!user) {
    throw unauthenticated(SESSION_EXPIRED);
  }
  record.revokedAt = now;
  record.rotated = true;
  return { user, pair: issueSession(state, user, record.familyId) };
}

/** Revokes a refresh token if it is known; idempotent. Access tokens stay valid until expiry. */
export function revokeRefresh(state: MockState, token: string): void {
  const record = token ? state.refreshTokens.get(token) : undefined;
  if (record && record.revokedAt === null) {
    record.revokedAt = Date.now();
  }
}
