/**
 * «Мои публикации» tabs: active / closed, synced with `?status=` (SPEC §7).
 * Pure helpers — covered by `tests/my-listings.test.ts`.
 */
import { ListingStatus } from '~~/api/types';
import type { Listing } from '~~/api/types';
import type { TabItem } from '~/components/ui/tabs';

export type MyListingsTab = 'active' | 'closed';

export const DEFAULT_MY_LISTINGS_TAB: MyListingsTab = 'active';

const TAB_STATUS: Record<MyListingsTab, ListingStatus> = {
  active: ListingStatus.LISTING_STATUS_ACTIVE,
  closed: ListingStatus.LISTING_STATUS_CLOSED,
};

const TAB_LABEL: Record<MyListingsTab, string> = {
  active: 'Активные',
  closed: 'Закрытые',
};

/** Tab from the `status` query value; unknown values fall back to «Активные». */
export function parseMyListingsTab(value: unknown): MyListingsTab {
  const raw = Array.isArray(value) ? value[0] : value;
  return raw === 'closed' || raw === ListingStatus.LISTING_STATUS_CLOSED ? 'closed' : DEFAULT_MY_LISTINGS_TAB;
}

/** `status` query value for a tab; the default tab has none. */
export function myListingsTabQuery(tab: MyListingsTab): string | undefined {
  return tab === DEFAULT_MY_LISTINGS_TAB ? undefined : tab;
}

/** Listings of one tab, order preserved (the API returns newest first). */
export function listingsForTab(listings: readonly Listing[], tab: MyListingsTab): Listing[] {
  return listings.filter(l => l.status === TAB_STATUS[tab]);
}

/** Tabs with counters for the `Tabs` component. */
export function buildMyListingsTabs(listings: readonly Listing[]): TabItem[] {
  return (['active', 'closed'] as const).map(tab => ({
    value: tab,
    label: TAB_LABEL[tab],
    count: listingsForTab(listings, tab).length,
  }));
}

/**
 * Replaces a listing with the server's response (e.g. after close), so it moves to the
 * «Закрытые» tab without a refetch. `stats` is kept if the response has none.
 */
export function replaceListing(listings: readonly Listing[], updated: Listing): Listing[] {
  return listings.map(l => (l.id === updated.id ? { ...updated, stats: updated.stats ?? l.stats } : l));
}
