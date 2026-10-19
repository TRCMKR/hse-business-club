/**
 * Records → API DTOs (SPEC §4.2): every field present, enums as string names,
 * RFC 3339 UTC timestamps without fractional seconds, `null` for unset nested
 * messages / timestamps.
 */
import type {
  Application,
  Listing,
  ListingStats,
  ListingSummary,
  User,
} from '../../api/types';
import type { ApplicationRecord, ListingRecord, MockState, UserRecord } from './state';
import { byCreatedDesc } from './state';

/** `2026-10-19T18:00:00.123Z` → `2026-10-19T18:00:00Z`. */
export function timestamp(date: Date): string {
  return date.toISOString().replace(/\.\d+Z$/, 'Z');
}

const nullableTimestamp = (date: Date | null): string | null => (date ? timestamp(date) : null);

export const avatarUrl = (key: string): string => (key ? `/v1/avatars/${key}` : '');

export function toUser(user: UserRecord): User {
  return {
    id: user.id,
    username: user.username,
    displayName: user.displayName,
    headline: user.headline,
    about: user.about,
    skills: [...user.skills],
    telegram: user.telegram,
    createdAt: timestamp(user.createdAt),
    avatarUrl: avatarUrl(user.avatarKey),
  };
}

function author(state: MockState, id: string): UserRecord {
  const user = state.users.get(id);
  if (!user) {
    throw new Error(`mock: dangling user id ${id}`);
  }
  return user;
}

export function toListingSummary(state: MockState, listing: ListingRecord): ListingSummary {
  return {
    id: listing.id,
    kind: listing.kind as ListingSummary['kind'],
    title: listing.title,
    status: listing.status as ListingSummary['status'],
    author: toUser(author(state, listing.authorId)),
  };
}

export function toApplication(state: MockState, application: ApplicationRecord): Application {
  const listing = state.listings.get(application.listingId);
  if (!listing) {
    throw new Error(`mock: dangling listing id ${application.listingId}`);
  }
  return {
    id: application.id,
    listing: toListingSummary(state, listing),
    applicant: toUser(author(state, application.applicantId)),
    message: application.message,
    status: application.status as Application['status'],
    createdAt: timestamp(application.createdAt),
    updatedAt: timestamp(application.updatedAt),
    decidedAt: nullableTimestamp(application.decidedAt),
  };
}

export function applicationsOf(state: MockState, listingId: string): ApplicationRecord[] {
  return [...state.applications.values()].filter(a => a.listingId === listingId);
}

/** Latest non-withdrawn application of the viewer on the listing (at most one by the rules). */
export function activeApplicationOf(
  state: MockState,
  listingId: string,
  applicantId: string,
): ApplicationRecord | undefined {
  return applicationsOf(state, listingId)
    .filter(a => a.applicantId === applicantId && a.status !== 'APPLICATION_STATUS_WITHDRAWN')
    .sort(byCreatedDesc)[0];
}

/**
 * Stats for the author. `total` counts every application that was not withdrawn
 * (= pending + accepted + declined): withdrawn ones are invisible noise for the author.
 */
export function listingStats(state: MockState, listingId: string): ListingStats {
  const stats = { pending: 0, accepted: 0, declined: 0, total: 0 };
  for (const a of applicationsOf(state, listingId)) {
    if (a.status === 'APPLICATION_STATUS_PENDING') {
      stats.pending += 1;
    } else if (a.status === 'APPLICATION_STATUS_ACCEPTED') {
      stats.accepted += 1;
    } else if (a.status === 'APPLICATION_STATUS_DECLINED') {
      stats.declined += 1;
    } else {
      continue;
    }
    stats.total += 1;
  }
  return stats;
}

/** Listing as seen by `viewer` (isMine / myApplication / stats depend on it). */
export function toListing(state: MockState, listing: ListingRecord, viewer: UserRecord): Listing {
  const isMine = listing.authorId === viewer.id;
  const mine = activeApplicationOf(state, listing.id, viewer.id);
  return {
    id: listing.id,
    kind: listing.kind as Listing['kind'],
    title: listing.title,
    description: listing.description,
    lookingFor: listing.lookingFor,
    tags: [...listing.tags],
    stage: listing.stage as Listing['stage'],
    compensation: listing.compensation,
    status: listing.status as Listing['status'],
    author: toUser(author(state, listing.authorId)),
    createdAt: timestamp(listing.createdAt),
    updatedAt: timestamp(listing.updatedAt),
    closedAt: nullableTimestamp(listing.closedAt),
    isMine,
    myApplication: mine ? toApplication(state, mine) : null,
    stats: isMine ? listingStats(state, listing.id) : null,
  };
}
