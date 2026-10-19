import { describe, expect, it } from 'vitest';
import { mountSuspended } from '@nuxt/test-utils/runtime';
import { ApplicationCard } from '~/components/cards/application-card';
import { UserSummary } from '~/components/cards/user-summary';
import { ApplicationStatus, ListingStatus } from '~~/api/types';
import { makeApplication, makeSummary, makeUser } from './cards-fixtures';

describe('cards/ApplicationCard', () => {
  it('incoming: applicant name, headline, message and status', async () => {
    const w = await mountSuspended(ApplicationCard, {
      props: { application: makeApplication(), mode: 'incoming' },
    });
    const text = w.text();
    expect(text).toContain('Борис Иванов');
    expect(text).toContain('Backend-разработчик');
    expect(text).toContain('Хочу в команду.');
    expect(text).toContain('На рассмотрении');
    const hrefs = w.findAll('a').map(a => a.attributes('href'));
    expect(hrefs).toContain('/users/u-boris');
    expect(hrefs).toContain('/listings/l-1');
  });

  it('incoming: shows decision time', async () => {
    const w = await mountSuspended(ApplicationCard, {
      props: {
        application: makeApplication({
          status: ApplicationStatus.APPLICATION_STATUS_ACCEPTED,
          decidedAt: '2026-10-19T13:00:00Z',
        }),
        mode: 'incoming',
      },
    });
    expect(w.text()).toContain('Принят');
    expect(w.text()).toContain('решение');
  });

  it('outgoing: listing title and author', async () => {
    const w = await mountSuspended(ApplicationCard, {
      props: { application: makeApplication(), mode: 'outgoing' },
    });
    const text = w.text();
    expect(text).toContain('Сервис аренды самокатов');
    expect(text).toContain('Анна Смирнова');
    expect(text).toContain('Стартап');
    expect(text).toContain('Хочу в команду.');
    expect(text).not.toContain('Публикация закрыта');
    expect(w.find('a').attributes('href')).toBe('/listings/l-1');
  });

  it('outgoing: closed listing → «Публикация закрыта»', async () => {
    const w = await mountSuspended(ApplicationCard, {
      props: {
        application: makeApplication({ listing: makeSummary({ status: ListingStatus.LISTING_STATUS_CLOSED }) }),
        mode: 'outgoing',
      },
    });
    expect(w.text()).toContain('Публикация закрыта');
  });

  it('renders the actions slot', async () => {
    for (const mode of ['incoming', 'outgoing'] as const) {
      const w = await mountSuspended(ApplicationCard, {
        props: { application: makeApplication(), mode },
        slots: { actions: () => 'Принять' },
      });
      expect(w.text()).toContain('Принять');
    }
  });
});

describe('cards/UserSummary', () => {
  it('lg shows telegram link and skills', async () => {
    const w = await mountSuspended(UserSummary, { props: { user: makeUser(), size: 'lg' } });
    expect(w.text()).toContain('Анна Смирнова');
    expect(w.find('a[href="https://t.me/anna_s"]').text()).toBe('@anna_s');
    expect(w.text()).toContain('SQL');
  });

  it('md hides telegram and skills; `to` makes the name a link', async () => {
    const w = await mountSuspended(UserSummary, { props: { user: makeUser(), to: '/users/u-anna' } });
    expect(w.text()).not.toContain('@anna_s');
    expect(w.text()).not.toContain('SQL');
    expect(w.find('a').attributes('href')).toBe('/users/u-anna');
  });
});
