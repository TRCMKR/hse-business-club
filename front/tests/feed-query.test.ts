// @vitest-environment node
import { describe, expect, it } from 'vitest';
import {
  EMPTY_FEED_FILTERS,
  FEED_QUERY_MAX_LENGTH,
  buildFeedQuery,
  isFeedFiltered,
  parseFeedQuery,
} from '~/components/feed/query';
import { ListingKind } from '~~/api/types';

describe('feed/query', () => {
  it('parses kind and q from the URL', () => {
    expect(parseFeedQuery({ kind: 'LISTING_KIND_VACANCY', q: ' Go ' })).toEqual({
      kind: ListingKind.LISTING_KIND_VACANCY,
      q: 'Go',
    });
  });

  it('falls back to defaults on garbage', () => {
    expect(parseFeedQuery({})).toEqual(EMPTY_FEED_FILTERS);
    expect(parseFeedQuery({ kind: 'nope' }).kind).toBe(ListingKind.LISTING_KIND_UNSPECIFIED);
    expect(parseFeedQuery({ kind: 'LISTING_KIND_UNSPECIFIED' }).kind).toBe(ListingKind.LISTING_KIND_UNSPECIFIED);
    expect(parseFeedQuery({ kind: ['LISTING_KIND_STARTUP', 'x'], q: [null] })).toEqual({
      kind: ListingKind.LISTING_KIND_STARTUP,
      q: '',
    });
  });

  it('clips the query to the API limit', () => {
    expect(parseFeedQuery({ q: 'я'.repeat(150) }).q).toHaveLength(FEED_QUERY_MAX_LENGTH);
  });

  it('builds a clean query and keeps unrelated params', () => {
    expect(buildFeedQuery(EMPTY_FEED_FILTERS)).toEqual({});
    expect(buildFeedQuery({ kind: ListingKind.LISTING_KIND_STARTUP, q: 'fin' }, { kind: 'x', q: 'y', utm: 'tg' }))
      .toEqual({ kind: 'LISTING_KIND_STARTUP', q: 'fin', utm: 'tg' });
  });

  it('isFeedFiltered', () => {
    expect(isFeedFiltered(EMPTY_FEED_FILTERS)).toBe(false);
    expect(isFeedFiltered({ ...EMPTY_FEED_FILTERS, q: 'a' })).toBe(true);
    expect(isFeedFiltered({ ...EMPTY_FEED_FILTERS, kind: ListingKind.LISTING_KIND_VACANCY })).toBe(true);
  });
});
