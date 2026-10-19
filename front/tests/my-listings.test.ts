import { beforeEach, describe, expect, it, vi } from 'vitest';
import { flushPromises } from '@vue/test-utils';
import { mountSuspended } from '@nuxt/test-utils/runtime';
import {
  buildMyListingsTabs,
  listingsForTab,
  myListingsTabQuery,
  parseMyListingsTab,
  replaceListing,
} from '~/components/my-listings/tabs';
import { ListingStatus } from '~~/api/types';
import type { Listing } from '~~/api/types';
import { useAuthStore } from '~/stores/auth';
import { makeListing, makeUser, makeVacancy } from './cards-fixtures';

const stats = { pending: 1, accepted: 1, declined: 0, total: 2 };
const active = makeListing({ id: 'a', title: 'Активный стартап', isMine: true, stats });
const activeVacancy = makeVacancy({ id: 'b', title: 'Активная вакансия', isMine: true, stats: { ...stats, pending: 0, total: 1 } });
const closed = makeListing({
  id: 'c',
  title: 'Закрытый стартап',
  isMine: true,
  status: ListingStatus.LISTING_STATUS_CLOSED,
  closedAt: '2026-10-15T18:00:00Z',
  stats,
});

describe('my-listings tabs helpers', () => {
  it('parses the status query, active by default', () => {
    expect(parseMyListingsTab(undefined)).toBe('active');
    expect(parseMyListingsTab('active')).toBe('active');
    expect(parseMyListingsTab('closed')).toBe('closed');
    expect(parseMyListingsTab(ListingStatus.LISTING_STATUS_CLOSED)).toBe('closed');
    expect(parseMyListingsTab(['closed'])).toBe('closed');
    expect(parseMyListingsTab('nope')).toBe('active');
    expect(myListingsTabQuery('active')).toBeUndefined();
    expect(myListingsTabQuery('closed')).toBe('closed');
  });

  it('splits listings by status and counts them', () => {
    const all = [active, closed, activeVacancy];
    expect(listingsForTab(all, 'active').map(l => l.id)).toEqual(['a', 'b']);
    expect(listingsForTab(all, 'closed').map(l => l.id)).toEqual(['c']);
    expect(buildMyListingsTabs(all)).toEqual([
      { value: 'active', label: 'Активные', count: 2 },
      { value: 'closed', label: 'Закрытые', count: 1 },
    ]);
  });

  it('replaceListing moves a closed listing to the closed tab and keeps stats', () => {
    const all = [active, activeVacancy];
    const updated: Listing = { ...active, status: ListingStatus.LISTING_STATUS_CLOSED, stats: null };
    const next = replaceListing(all, updated);
    expect(listingsForTab(next, 'active').map(l => l.id)).toEqual(['b']);
    expect(listingsForTab(next, 'closed')).toEqual([{ ...updated, stats }]);
    expect(all[0]!.status).toBe(ListingStatus.LISTING_STATUS_ACTIVE);
  });
});

const api = {
  listings: {
    listMine: vi.fn<() => Promise<Listing[]>>(),
    close: vi.fn<(id: string) => Promise<Listing>>(),
  },
};

vi.mock('~~/api/client', () => ({ getApi: () => api }));

const { default: MyListingsPage } = await import('~/pages/me/listings');

async function settle() {
  for (let i = 0; i < 3; i++) {
    await flushPromises();
    await new Promise(resolve => setTimeout(resolve, 0));
  }
}

const titles = (w: { findAll: (selector: string) => { text: () => string }[] }) =>
  w.findAll('[data-testid="listing-card"] h3').map(h => h.text());

describe('pages/me/listings', () => {
  beforeEach(() => {
    // The global auth middleware would redirect an anonymous visitor to /login.
    useAuthStore().setUser(makeUser());
    clearNuxtData();
    api.listings.listMine.mockReset();
    api.listings.close.mockReset();
  });

  it('shows active listings with stats and tab counters by default', async () => {
    api.listings.listMine.mockResolvedValue([active, closed, activeVacancy]);
    const w = await mountSuspended(MyListingsPage, { route: '/me/listings' });
    await settle();
    expect(titles(w)).toEqual(['Активный стартап', 'Активная вакансия']);
    expect(w.findAll('[role="tab"]').map(t => t.text())).toEqual(['Активные2', 'Закрытые1']);
    expect(w.find('[data-testid="listing-stats"]').text()).toContain('2 отклика');

    w.unmount();
  });

  it('opens the closed tab from ?status=closed', async () => {
    api.listings.listMine.mockResolvedValue([active, closed, activeVacancy]);
    const w = await mountSuspended(MyListingsPage, { route: '/me/listings?status=closed' });
    await settle();
    expect(titles(w)).toEqual(['Закрытый стартап']);
    expect(w.find('[data-testid="my-listing-close"]').exists()).toBe(false);
    w.unmount();
  });

  it('switching tabs updates ?status', async () => {
    api.listings.listMine.mockResolvedValue([active, closed, activeVacancy]);
    const w = await mountSuspended(MyListingsPage, { route: '/me/listings' });
    await settle();
    await w.findAll('[role="tab"]')[1]!.trigger('click');
    await vi.waitFor(() => expect(useRouter().currentRoute.value.query.status).toBe('closed'), { timeout: 3000 });
    expect(titles(w)).toEqual(['Закрытый стартап']);
    await w.findAll('[role="tab"]')[0]!.trigger('click');
    await vi.waitFor(() => expect(useRouter().currentRoute.value.query.status).toBeUndefined(), { timeout: 3000 });
    expect(titles(w)).toEqual(['Активный стартап', 'Активная вакансия']);
    w.unmount();
  });

  it('empty state with a CTA', async () => {
    api.listings.listMine.mockResolvedValue([]);
    const w = await mountSuspended(MyListingsPage, { route: '/me/listings' });
    await settle();
    expect(w.text()).toContain('У вас нет публикаций');
    expect(w.find('a[href="/listings/new"]').exists()).toBe(true);
  });

  it('error state with retry', async () => {
    api.listings.listMine.mockRejectedValue({ status: 500, data: { code: 13, message: 'Внутренняя ошибка' } });
    const w = await mountSuspended(MyListingsPage, { route: '/me/listings' });
    await settle();
    expect(w.text()).toContain('Не удалось загрузить');
    expect(w.text()).toContain('Внутренняя ошибка');
  });

  it('close: confirm, then the card leaves the active tab', async () => {
    api.listings.listMine.mockResolvedValue([active, activeVacancy]);
    api.listings.close.mockImplementation(async id => ({
      ...active,
      id,
      status: ListingStatus.LISTING_STATUS_CLOSED,
      closedAt: '2026-10-20T10:00:00Z',
    }));
    const w = await mountSuspended(MyListingsPage, { route: '/me/listings' });
    await settle();

    await w.find('[data-testid="my-listing-close"]').trigger('click');
    await settle();
    expect(api.listings.close).not.toHaveBeenCalled();
    const confirm = [...document.querySelectorAll<HTMLButtonElement>('[role="dialog"] button')]
      .find(b => b.textContent === 'Закрыть');
    expect(confirm).toBeTruthy();
    confirm!.click();
    await settle();

    expect(api.listings.close).toHaveBeenCalledWith('a');
    expect(titles(w)).toEqual(['Активная вакансия']);
    expect(w.findAll('[role="tab"]').map(t => t.text())).toEqual(['Активные1', 'Закрытые1']);
    w.unmount();
  });
});
