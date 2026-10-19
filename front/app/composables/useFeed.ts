import { computed, ref, shallowRef, toValue, watch } from 'vue';
import type { MaybeRefOrGetter } from 'vue';
import { getApi } from '~~/api/client';
import type { Listing, ListingsPage } from '~~/api/types';
import type { ListListingsParams } from '~~/api/services/listings';
import { isFeedFiltered, normalizeFeedFilters } from '~/components/feed/query';
import type { FeedFilters } from '~/components/feed/query';

/** Page size of the feed (SPEC §4.1 default). */
export const FEED_PAGE_SIZE = 20;

export type FeedFetcher = (params: ListListingsParams) => Promise<ListingsPage>;

export interface UseFeedOptions {
  /** Replaces `getApi().listings.list` (tests). */
  fetcher?: FeedFetcher
  limit?: number
}

/**
 * Feed state for the given filters.
 *
 * The first page goes through `useAsyncData` keyed by the filters (SSR + hydration
 * without a second request; a new key refetches). Further pages («Показать ещё») are
 * client-only and appended to the first one. Changing filters drops the extra pages and
 * starts over from page 1; responses that arrive for stale filters are ignored.
 *
 * Call synchronously at the start of `setup()` (uses `getApi()` and `useAsyncData`).
 */
export const useFeed = (filters: MaybeRefOrGetter<FeedFilters>, options: UseFeedOptions = {}) => {
  const limit = options.limit ?? FEED_PAGE_SIZE;
  const fetcher: FeedFetcher = options.fetcher ?? (() => {
    const api = getApi();
    return (params: ListListingsParams) => api.listings.list(params);
  })();

  const current = computed(() => normalizeFeedFilters(toValue(filters)));
  const key = computed(() => `feed:${current.value.kind}:${limit}:${current.value.q}`);

  const params = (page: number): ListListingsParams => ({
    kind: current.value.kind,
    query: current.value.q,
    page,
    limit,
  });

  const first = useAsyncData(key, () => fetcher(params(1)));

  /** Items of pages 2..N. */
  const extra = shallowRef<Listing[]>([]);
  /** Last loaded page number. */
  const page = ref(1);
  /** `total` from the latest «load more» response (fresher than the first page's). */
  const moreTotal = ref<number | null>(null);
  /** The server returned an empty page before `total` was reached. */
  const exhausted = ref(false);
  const loadingMore = ref(false);
  const loadMoreError = shallowRef<unknown>(null);
  /** Bumped on every reset so in-flight «load more» responses can be dropped. */
  let generation = 0;

  const reset = () => {
    generation++;
    extra.value = [];
    page.value = 1;
    moreTotal.value = null;
    exhausted.value = false;
    loadingMore.value = false;
    loadMoreError.value = null;
  };

  watch(key, reset);

  const items = computed<Listing[]>(() => {
    const head = first.data.value?.items ?? [];
    if (!extra.value.length) {
      return head;
    }
    // New listings shift pages while the user scrolls, so a page may repeat items.
    const seen = new Set(head.map(l => l.id));
    return [...head, ...extra.value.filter(l => !seen.has(l.id) && seen.add(l.id))];
  });

  const total = computed(() => moreTotal.value ?? first.data.value?.total ?? 0);
  const hasMore = computed(() => !!first.data.value && !exhausted.value && items.value.length < total.value);
  /** First page is loading (no data for the current filters yet). */
  const loading = computed(() => !first.data.value && (first.status.value === 'pending' || first.status.value === 'idle'));

  const loadMore = async (): Promise<void> => {
    if (loadingMore.value || !hasMore.value) {
      return;
    }
    const gen = generation;
    const next = page.value + 1;
    loadingMore.value = true;
    loadMoreError.value = null;
    try {
      const res = await fetcher(params(next));
      if (gen !== generation) {
        return;
      }
      extra.value = [...extra.value, ...res.items];
      page.value = next;
      moreTotal.value = res.total;
      if (!res.items.length) {
        exhausted.value = true;
      }
    } catch (error) {
      if (gen === generation) {
        loadMoreError.value = error;
      }
    } finally {
      if (gen === generation) {
        loadingMore.value = false;
      }
    }
  };

  /** Reloads from page 1 (retry after an error). */
  const refresh = async (): Promise<void> => {
    reset();
    await first.refresh();
  };

  return {
    filters: current,
    filtered: computed(() => isFeedFiltered(current.value)),
    items,
    total,
    page,
    hasMore,
    loading,
    status: first.status,
    error: first.error,
    loadingMore,
    loadMoreError,
    loadMore,
    refresh,
  };
};

export type FeedState = ReturnType<typeof useFeed>;
