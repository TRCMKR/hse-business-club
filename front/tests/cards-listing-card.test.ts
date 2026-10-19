import { describe, expect, it } from 'vitest';
import { mountSuspended } from '@nuxt/test-utils/runtime';
import { ListingCard } from '~/components/cards/listing-card';
import { ApplicationStatus, ListingStatus } from '~~/api/types';
import { makeApplication, makeListing, makeVacancy } from './cards-fixtures';

describe('cards/ListingCard', () => {
  it('startup: kind and stage badges, lookingFor', async () => {
    const w = await mountSuspended(ListingCard, { props: { listing: makeListing() } });
    const text = w.text();
    expect(text).toContain('Стартап');
    expect(text).toContain('MVP');
    expect(text).toContain('Ищем:');
    expect(text).toContain('Backend-разработчика');
    expect(text).not.toContain('Ваша публикация');
    expect(text).not.toContain('Закрыта');
    expect(text).toContain('Анна Смирнова');
  });

  it('vacancy: kind badge and compensation, no stage', async () => {
    const w = await mountSuspended(ListingCard, { props: { listing: makeVacancy() } });
    expect(w.text()).toContain('Вакансия');
    expect(w.text()).toContain('от 120 000 ₽');
    expect(w.text()).not.toContain('Не указана');
  });

  it('isMine → «Ваша публикация»', async () => {
    const w = await mountSuspended(ListingCard, { props: { listing: makeListing({ isMine: true }) } });
    expect(w.text()).toContain('Ваша публикация');
  });

  it('my application status badges', async () => {
    const pending = await mountSuspended(ListingCard, {
      props: { listing: makeListing({ myApplication: makeApplication() }) },
    });
    expect(pending.text()).toContain('Вы откликнулись');

    const accepted = await mountSuspended(ListingCard, {
      props: {
        listing: makeListing({
          myApplication: makeApplication({ status: ApplicationStatus.APPLICATION_STATUS_ACCEPTED }),
        }),
      },
    });
    expect(accepted.text()).toContain('Вас приняли');
  });

  it('closed → «Закрыта»', async () => {
    const w = await mountSuspended(ListingCard, {
      props: { listing: makeListing({ status: ListingStatus.LISTING_STATUS_CLOSED }) },
    });
    expect(w.text()).toContain('Закрыта');
  });

  it('5 tags → 3 chips + «+2»', async () => {
    const w = await mountSuspended(ListingCard, {
      props: { listing: makeListing({ tags: ['a1', 'b2', 'c3', 'd4', 'e5'] }) },
    });
    const text = w.text();
    expect(text).toContain('a1');
    expect(text).toContain('c3');
    expect(text).not.toContain('d4');
    expect(text).not.toContain('e5');
    expect(text).toContain('+2');
  });

  it('stats only when not null', async () => {
    const without = await mountSuspended(ListingCard, { props: { listing: makeListing() } });
    expect(without.find('[data-testid="listing-stats"]').exists()).toBe(false);

    const w = await mountSuspended(ListingCard, {
      props: { listing: makeListing({ isMine: true, stats: { total: 5, pending: 2, accepted: 2, declined: 1 } }) },
    });
    const stats = w.find('[data-testid="listing-stats"]');
    expect(stats.exists()).toBe(true);
    expect(stats.text()).toContain('5 откликов');
    expect(stats.text()).toContain('2 новых');
  });

  it('renders the actions slot outside the link', async () => {
    const w = await mountSuspended(ListingCard, {
      props: { listing: makeListing() },
      slots: { actions: () => 'Закрыть публикацию' },
    });
    expect(w.text()).toContain('Закрыть публикацию');
    expect(w.find('a').text()).not.toContain('Закрыть публикацию');
  });

  it('links to /listings/<id> by default and to `to` when set', async () => {
    const w = await mountSuspended(ListingCard, { props: { listing: makeListing({ id: 'l-42' }) } });
    expect(w.find('a').attributes('href')).toBe('/listings/l-42');

    const custom = await mountSuspended(ListingCard, { props: { listing: makeListing(), to: '/me/listings' } });
    expect(custom.find('a').attributes('href')).toBe('/me/listings');
  });
});
