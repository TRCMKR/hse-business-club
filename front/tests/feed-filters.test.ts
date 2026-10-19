import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { nextTick } from 'vue';
import { mountSuspended } from '@nuxt/test-utils/runtime';
import { FEED_SEARCH_DEBOUNCE_MS, FeedFilters } from '~/components/feed/feed-filters';
import { useAuthStore } from '~/stores/auth';
import { makeUser } from './cards-fixtures';

const query = () => useRouter().currentRoute.value.query;

const mountFilters = (route: string) => mountSuspended(FeedFilters, { route });

describe('feed/FeedFilters', () => {
  // The global auth middleware sends anonymous visitors of `/` to /login.
  beforeEach(() => {
    useAuthStore().setUser(makeUser());
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it('reads kind and q from the URL', async () => {
    const w = await mountFilters('/?kind=LISTING_KIND_VACANCY&q=Go');
    expect((w.find('input').element as HTMLInputElement).value).toBe('Go');
    const checked = w.find('[role="radio"][aria-checked="true"]');
    expect(checked.text()).toBe('Вакансии');
  });

  it('kind switch writes the URL immediately and keeps q', async () => {
    const w = await mountFilters('/?q=fin');
    const startups = w.findAll('[role="radio"]').find(b => b.text() === 'Стартапы')!;
    await startups.trigger('click');
    await vi.waitFor(() => expect(query()).toEqual({ kind: 'LISTING_KIND_STARTUP', q: 'fin' }));

    const all = w.findAll('[role="radio"]').find(b => b.text() === 'Все')!;
    await all.trigger('click');
    await vi.waitFor(() => expect(query()).toEqual({ q: 'fin' }));
  });

  it('search is debounced by 300ms and trimmed', async () => {
    const w = await mountFilters('/?kind=LISTING_KIND_STARTUP');
    vi.useFakeTimers({ toFake: ['setTimeout', 'clearTimeout'] });
    const input = w.find('input');

    await input.setValue('Fin');
    vi.advanceTimersByTime(100);
    await input.setValue('FinTrack ');
    vi.advanceTimersByTime(FEED_SEARCH_DEBOUNCE_MS - 1);
    await nextTick();
    expect(query()).toEqual({ kind: 'LISTING_KIND_STARTUP' });

    vi.advanceTimersByTime(1);
    await vi.waitFor(() => expect(query()).toEqual({ kind: 'LISTING_KIND_STARTUP', q: 'FinTrack' }));
    // The trailing space the user is typing is not wiped by the URL sync.
    expect((input.element as HTMLInputElement).value).toBe('FinTrack ');
  });

  it('Enter applies the search without waiting', async () => {
    const w = await mountFilters('/');
    const input = w.find('input');
    await input.setValue('Go');
    await input.trigger('keydown', { key: 'Enter' });
    await vi.waitFor(() => expect(query()).toEqual({ q: 'Go' }));
  });

  it('clear button empties the input and drops q at once', async () => {
    const w = await mountFilters('/?q=xyz&kind=LISTING_KIND_VACANCY');
    await w.find('[data-testid="feed-search-clear"]').trigger('click');
    await vi.waitFor(() => expect(query()).toEqual({ kind: 'LISTING_KIND_VACANCY' }));
    expect((w.find('input').element as HTMLInputElement).value).toBe('');
    expect(w.find('[data-testid="feed-search-clear"]').exists()).toBe(false);
  });

  it('follows external URL changes (reset filters, back)', async () => {
    const w = await mountFilters('/?q=xyz');
    await useRouter().replace({ query: {} });
    await vi.waitFor(() => expect((w.find('input').element as HTMLInputElement).value).toBe(''));
  });

  it('clearing the input by typing drops q after the debounce', async () => {
    const w = await mountFilters('/?q=Go');
    await w.find('input').setValue('  ');
    await vi.waitFor(() => expect(query()).toEqual({}), { timeout: 1000 });
  });
});
