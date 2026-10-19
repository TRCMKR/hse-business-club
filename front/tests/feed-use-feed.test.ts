import { beforeEach, describe, expect, it, vi } from 'vitest';
import { defineComponent, h, nextTick, ref } from 'vue';
import { mountSuspended } from '@nuxt/test-utils/runtime';
import { useFeed } from '~/composables/useFeed';
import type { FeedFetcher, FeedState } from '~/composables/useFeed';
import { EMPTY_FEED_FILTERS } from '~/components/feed/query';
import type { FeedFilters } from '~/components/feed/query';
import { ListingKind } from '~~/api/types';
import type { Listing } from '~~/api/types';
import { makeListing, makeVacancy } from './cards-fixtures';

/** 5 startups + 2 vacancies, newest first. */
const DATA: Listing[] = [
  ...Array.from({ length: 5 }, (_, i) => makeListing({ id: `s${i}`, title: `Startup ${i}` })),
  ...Array.from({ length: 2 }, (_, i) => makeVacancy({ id: `v${i}`, title: `Vacancy ${i}` })),
];

/** In-memory `listings.list` with the same filtering/paging rules as the API. */
const makeFetcher = (data = DATA) => {
  const fn = vi.fn<FeedFetcher>(async ({ kind, query = '', page = 1, limit = 20 }) => {
    const found = data.filter(l =>
      (!kind || kind === ListingKind.LISTING_KIND_UNSPECIFIED || l.kind === kind)
      && l.title.toLowerCase().includes(query.toLowerCase()));
    return { items: found.slice((page - 1) * limit, page * limit), total: found.length, page, limit };
  });
  return fn;
};

const mountFeed = async (fetcher: FeedFetcher, initial: FeedFilters = EMPTY_FEED_FILTERS) => {
  const filters = ref<FeedFilters>({ ...initial });
  let feed!: FeedState;
  const Host = defineComponent({
    setup() {
      feed = useFeed(filters, { fetcher, limit: 2 });
      return () => h('div');
    },
  });
  await mountSuspended(Host);
  await vi.waitFor(() => expect(feed.loading.value).toBe(false));
  return { feed, filters };
};

const ids = (feed: FeedState) => feed.items.value.map(l => l.id);

describe('useFeed', () => {
  // useAsyncData entries are shared by key across mounts.
  beforeEach(() => clearNuxtData());

  it('loads the first page with the filters', async () => {
    const fetcher = makeFetcher();
    const { feed } = await mountFeed(fetcher, { kind: ListingKind.LISTING_KIND_STARTUP, q: ' Startup ' });
    expect(fetcher).toHaveBeenCalledWith({ kind: ListingKind.LISTING_KIND_STARTUP, query: 'Startup', page: 1, limit: 2 });
    expect(ids(feed)).toEqual(['s0', 's1']);
    expect(feed.total.value).toBe(5);
    expect(feed.hasMore.value).toBe(true);
    expect(feed.filtered.value).toBe(true);
  });

  it('accumulates pages until total, then hasMore = false', async () => {
    const fetcher = makeFetcher();
    const { feed } = await mountFeed(fetcher);
    expect(feed.total.value).toBe(7);

    await feed.loadMore();
    expect(feed.page.value).toBe(2);
    expect(ids(feed)).toEqual(['s0', 's1', 's2', 's3']);
    expect(fetcher).toHaveBeenLastCalledWith(expect.objectContaining({ page: 2 }));

    await feed.loadMore();
    await feed.loadMore();
    expect(ids(feed)).toHaveLength(7);
    expect(feed.page.value).toBe(4);
    expect(feed.hasMore.value).toBe(false);

    // No requests past the end.
    const calls = fetcher.mock.calls.length;
    await feed.loadMore();
    expect(fetcher.mock.calls.length).toBe(calls);
  });

  it('a filter change resets to page 1', async () => {
    const fetcher = makeFetcher();
    const { feed, filters } = await mountFeed(fetcher);
    await feed.loadMore();
    expect(feed.page.value).toBe(2);

    filters.value = { kind: ListingKind.LISTING_KIND_VACANCY, q: '' };
    await nextTick();
    expect(feed.page.value).toBe(1);
    await vi.waitFor(() => expect(ids(feed)).toEqual(['v0', 'v1']));
    expect(fetcher).toHaveBeenLastCalledWith({ kind: ListingKind.LISTING_KIND_VACANCY, query: '', page: 1, limit: 2 });
    expect(feed.total.value).toBe(2);
    expect(feed.hasMore.value).toBe(false);
  });

  it('drops a «load more» response that arrives after the filters changed', async () => {
    const base = makeFetcher();
    let release!: () => void;
    const fetcher = vi.fn<FeedFetcher>(async (p) => {
      if (p.page === 2) {
        await new Promise<void>(r => (release = r));
      }
      return base(p);
    });
    const { feed, filters } = await mountFeed(fetcher);

    const pending = feed.loadMore();
    expect(feed.loadingMore.value).toBe(true);
    filters.value = { kind: ListingKind.LISTING_KIND_VACANCY, q: '' };
    await nextTick();
    expect(feed.loadingMore.value).toBe(false);
    await vi.waitFor(() => expect(ids(feed)).toEqual(['v0', 'v1']));

    release();
    await pending;
    expect(ids(feed)).toEqual(['v0', 'v1']);
    expect(feed.page.value).toBe(1);
  });

  it('skips duplicates when pages shift', async () => {
    const fetcher = vi.fn<FeedFetcher>(async ({ page = 1 }) => page === 1
      ? { items: [DATA[0]!, DATA[1]!], total: 3, page, limit: 2 }
      : { items: [DATA[1]!, DATA[2]!], total: 3, page, limit: 2 });
    const { feed } = await mountFeed(fetcher);
    await feed.loadMore();
    expect(ids(feed)).toEqual(['s0', 's1', 's2']);
    expect(feed.hasMore.value).toBe(false);
  });

  it('stops on an empty page even if total says more', async () => {
    const fetcher = vi.fn<FeedFetcher>(async ({ page = 1 }) => page === 1
      ? { items: [DATA[0]!], total: 10, page, limit: 2 }
      : { items: [], total: 10, page, limit: 2 });
    const { feed } = await mountFeed(fetcher);
    expect(feed.hasMore.value).toBe(true);
    await feed.loadMore();
    expect(feed.hasMore.value).toBe(false);
  });

  it('keeps loaded items and exposes the error when «load more» fails', async () => {
    const base = makeFetcher();
    const fetcher = vi.fn<FeedFetcher>(async p => (p.page === 2 ? Promise.reject(new Error('boom')) : base(p)));
    const { feed } = await mountFeed(fetcher);
    await feed.loadMore();
    expect(feed.loadMoreError.value).toBeInstanceOf(Error);
    expect(feed.loadingMore.value).toBe(false);
    expect(ids(feed)).toEqual(['s0', 's1']);
    expect(feed.page.value).toBe(1);
    expect(feed.hasMore.value).toBe(true);
  });

  it('first page error → error, refresh retries', async () => {
    const base = makeFetcher();
    let fail = true;
    const fetcher = vi.fn<FeedFetcher>(async p => (fail ? Promise.reject(new Error('down')) : base(p)));
    const filters = ref<FeedFilters>({ ...EMPTY_FEED_FILTERS, q: 'err-case' });
    let feed!: FeedState;
    await mountSuspended(defineComponent({
      setup() {
        feed = useFeed(filters, { fetcher, limit: 2 });
        return () => h('div');
      },
    }));
    await vi.waitFor(() => expect(feed.error.value).toBeTruthy());
    expect(feed.items.value).toEqual([]);

    fail = false;
    filters.value = { ...EMPTY_FEED_FILTERS };
    await feed.refresh();
    await vi.waitFor(() => expect(ids(feed)).toEqual(['s0', 's1']));
    expect(feed.error.value).toBeFalsy();
  });
});
