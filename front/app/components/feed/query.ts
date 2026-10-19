import type { LocationQuery, LocationQueryRaw } from 'vue-router';
import { ListingKind } from '~~/api/types';
import { LIMITS } from '~/utils/validation';

/** Feed filters as they live in the URL: `/?kind=LISTING_KIND_STARTUP&q=FinTrack`. */
export interface FeedFilters {
  /** `LISTING_KIND_UNSPECIFIED` = all kinds. */
  kind: ListingKind
  /** Trimmed search query, '' = no search. */
  q: string
}

export const EMPTY_FEED_FILTERS: FeedFilters = {
  kind: ListingKind.LISTING_KIND_UNSPECIFIED,
  q: '',
};

/** Max search length accepted by the API (SPEC §2.4), in characters. */
export const FEED_QUERY_MAX_LENGTH = LIMITS.query.max;

const KINDS = new Set<string>([ListingKind.LISTING_KIND_STARTUP, ListingKind.LISTING_KIND_VACANCY]);

const first = (v: LocationQuery[string] | undefined): string => {
  const value = Array.isArray(v) ? v[0] : v;
  return typeof value === 'string' ? value : '';
};

/** Normalizes filters: unknown kind → all, query trimmed and clipped. */
export const normalizeFeedFilters = (f: Partial<FeedFilters>): FeedFilters => ({
  kind: f.kind && KINDS.has(f.kind) ? f.kind : ListingKind.LISTING_KIND_UNSPECIFIED,
  q: Array.from((f.q ?? '').trim()).slice(0, FEED_QUERY_MAX_LENGTH).join('').trim(),
});

/** Reads feed filters from `route.query`; garbage values fall back to defaults. */
export const parseFeedQuery = (query: LocationQuery): FeedFilters => normalizeFeedFilters({
  kind: first(query.kind) as ListingKind,
  q: first(query.q),
});

/**
 * Builds `route.query` for the given filters, keeping unrelated params.
 * Default values are omitted so `/` stays clean.
 */
export const buildFeedQuery = (filters: FeedFilters, current: LocationQuery = {}): LocationQueryRaw => {
  const { kind: _kind, q: _q, ...rest } = current;
  const f = normalizeFeedFilters(filters);
  return {
    ...rest,
    ...(f.kind !== ListingKind.LISTING_KIND_UNSPECIFIED ? { kind: f.kind } : {}),
    ...(f.q ? { q: f.q } : {}),
  };
};

export const isFeedFiltered = (f: FeedFilters): boolean =>
  f.kind !== ListingKind.LISTING_KIND_UNSPECIFIED || f.q !== '';

export const sameFeedFilters = (a: FeedFilters, b: FeedFilters): boolean => a.kind === b.kind && a.q === b.q;
