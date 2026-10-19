import { describe, expect, it } from 'vitest';
import { mountSuspended } from '@nuxt/test-utils/runtime';
import { FEED_SKELETON_COUNT, FeedList } from '~/components/feed/feed-list';
import { makeListing, makeVacancy } from './cards-fixtures';

const items = [makeListing({ id: 'a' }), makeVacancy({ id: 'b' })];

describe('feed/FeedList', () => {
  it('skeletons while the first page loads', async () => {
    const w = await mountSuspended(FeedList, { props: { items: [], loading: true } });
    expect(w.findAll('[data-testid="feed-skeleton"]')).toHaveLength(FEED_SKELETON_COUNT);
  });

  it('cards link to the listing; «Показать ещё» while hasMore', async () => {
    const w = await mountSuspended(FeedList, { props: { items, hasMore: true } });
    expect(w.findAll('[data-testid="listing-card"]')).toHaveLength(2);
    expect(w.find('a[href="/listings/a"]').exists()).toBe(true);
    const more = w.find('[data-testid="feed-load-more"]');
    expect(more.text()).toContain('Показать ещё');
    await more.trigger('click');
    expect(w.emitted('loadMore')).toHaveLength(1);
    expect(w.find('[data-testid="feed-end"]').exists()).toBe(false);
  });

  it('loadingMore → button busy, no double emit', async () => {
    const w = await mountSuspended(FeedList, { props: { items, hasMore: true, loadingMore: true } });
    const more = w.find('[data-testid="feed-load-more"]');
    expect(more.attributes('aria-busy')).toBe('true');
    await more.trigger('click');
    expect(w.emitted('loadMore')).toBeUndefined();
  });

  it('«Это всё» when everything is loaded', async () => {
    const w = await mountSuspended(FeedList, { props: { items, hasMore: false } });
    expect(w.find('[data-testid="feed-end"]').text()).toBe('Это всё');
  });

  it('empty without filters → CTA to create', async () => {
    const w = await mountSuspended(FeedList, { props: { items: [] } });
    expect(w.text()).toContain('Пока нет предложений');
    expect(w.find('a[href="/listings/new"]').text()).toContain('Создать публикацию');
  });

  it('empty with filters → reset', async () => {
    const w = await mountSuspended(FeedList, { props: { items: [], filtered: true } });
    expect(w.text()).toContain('Ничего не нашлось');
    const reset = w.findAll('button').find(b => b.text() === 'Сбросить фильтры')!;
    await reset.trigger('click');
    expect(w.emitted('reset')).toHaveLength(1);
  });

  it('error → error state with retry', async () => {
    const w = await mountSuspended(FeedList, { props: { items: [], error: new Error('x') } });
    expect(w.find('[role="alert"]').exists()).toBe(true);
    const retry = w.findAll('button').find(b => b.text() === 'Повторить')!;
    await retry.trigger('click');
    expect(w.emitted('retry')).toHaveLength(1);
  });
});
