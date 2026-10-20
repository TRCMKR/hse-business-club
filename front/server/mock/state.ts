/**
 * In-memory storage records. Shapes mirror the DB model (SPEC §3), not the API DTOs —
 * see serialize.ts for the conversion.
 */
import { randomUUID } from 'node:crypto';
import type { AccessTokenRecord, RefreshTokenRecord } from './sessions';

export type ListingKind = 'LISTING_KIND_UNSPECIFIED' | 'LISTING_KIND_STARTUP' | 'LISTING_KIND_VACANCY';
export type ListingStatus = 'LISTING_STATUS_UNSPECIFIED' | 'LISTING_STATUS_ACTIVE' | 'LISTING_STATUS_CLOSED';
export type StartupStage
  = | 'STARTUP_STAGE_UNSPECIFIED'
    | 'STARTUP_STAGE_IDEA'
    | 'STARTUP_STAGE_PROTOTYPE'
    | 'STARTUP_STAGE_MVP'
    | 'STARTUP_STAGE_LAUNCHED';
export type ApplicationStatus
  = | 'APPLICATION_STATUS_UNSPECIFIED'
    | 'APPLICATION_STATUS_PENDING'
    | 'APPLICATION_STATUS_ACCEPTED'
    | 'APPLICATION_STATUS_DECLINED'
    | 'APPLICATION_STATUS_WITHDRAWN';

export const LISTING_KINDS: readonly ListingKind[] = [
  'LISTING_KIND_UNSPECIFIED',
  'LISTING_KIND_STARTUP',
  'LISTING_KIND_VACANCY',
];
export const LISTING_STATUSES: readonly ListingStatus[] = [
  'LISTING_STATUS_UNSPECIFIED',
  'LISTING_STATUS_ACTIVE',
  'LISTING_STATUS_CLOSED',
];
export const STARTUP_STAGES: readonly StartupStage[] = [
  'STARTUP_STAGE_UNSPECIFIED',
  'STARTUP_STAGE_IDEA',
  'STARTUP_STAGE_PROTOTYPE',
  'STARTUP_STAGE_MVP',
  'STARTUP_STAGE_LAUNCHED',
];
export const APPLICATION_STATUSES: readonly ApplicationStatus[] = [
  'APPLICATION_STATUS_UNSPECIFIED',
  'APPLICATION_STATUS_PENDING',
  'APPLICATION_STATUS_ACCEPTED',
  'APPLICATION_STATUS_DECLINED',
  'APPLICATION_STATUS_WITHDRAWN',
];

export interface UserRecord {
  id: string
  username: string
  /** Plain text: this is a mock. */
  password: string
  displayName: string
  headline: string
  about: string
  skills: string[]
  telegram: string
  /** File name in the avatar store (`<uuid>.jpg`), '' — no photo. */
  avatarKey: string
  createdAt: Date
  updatedAt: Date
}

export interface ListingRecord {
  id: string
  /** Insertion order, tie-breaker for equal timestamps. */
  seq: number
  kind: ListingKind
  title: string
  description: string
  lookingFor: string
  tags: string[]
  stage: StartupStage
  compensation: string
  status: ListingStatus
  authorId: string
  createdAt: Date
  updatedAt: Date
  closedAt: Date | null
}

export interface ApplicationRecord {
  id: string
  seq: number
  listingId: string
  applicantId: string
  message: string
  status: ApplicationStatus
  createdAt: Date
  updatedAt: Date
  decidedAt: Date | null
}

export interface AvatarFile {
  bytes: Uint8Array
  contentType: string
}

export interface MockState {
  users: Map<string, UserRecord>
  listings: Map<string, ListingRecord>
  applications: Map<string, ApplicationRecord>
  avatars: Map<string, AvatarFile>
  /** Access token → record (SPEC §4.3). */
  accessTokens: Map<string, AccessTokenRecord>
  /** Refresh token → record. */
  refreshTokens: Map<string, RefreshTokenRecord>
  seq: number
}

export const newId = (): string => randomUUID();

export function createEmptyState(): MockState {
  return {
    users: new Map(),
    listings: new Map(),
    applications: new Map(),
    avatars: new Map(),
    accessTokens: new Map(),
    refreshTokens: new Map(),
    seq: 0,
  };
}

export function nextSeq(state: MockState): number {
  state.seq += 1;
  return state.seq;
}

export function findUserByUsername(state: MockState, username: string): UserRecord | undefined {
  for (const user of state.users.values()) {
    if (user.username === username) {
      return user;
    }
  }
  return undefined;
}

/** Newest first; equal timestamps — later insertion first. */
export function byCreatedDesc<T extends { createdAt: Date, seq: number }>(a: T, b: T): number {
  return b.createdAt.getTime() - a.createdAt.getTime() || b.seq - a.seq;
}

/** Pending first, then newest first (SPEC §4.1). */
export function byPendingThenCreatedDesc(a: ApplicationRecord, b: ApplicationRecord): number {
  const pa = a.status === 'APPLICATION_STATUS_PENDING' ? 0 : 1;
  const pb = b.status === 'APPLICATION_STATUS_PENDING' ? 0 : 1;
  return pa - pb || byCreatedDesc(a, b);
}
