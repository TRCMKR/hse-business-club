import { afterEach, describe, expect, it, vi } from 'vitest';
import { mountSuspended } from '@nuxt/test-utils/runtime';
import { nextTick } from 'vue';
import { ListingBody } from '~/components/listing/listing-body';
import { ListingHeader } from '~/components/listing/listing-header';
import { ListingActionBar } from '~/components/listing/listing-action-bar';
import { MyApplicationBlock } from '~/components/listing/my-application-block';
import { OwnerPanel } from '~/components/listing/owner-panel';
import { ApplicationStatus, ListingStatus } from '~~/api/types';
import { makeApplication, makeListing, makeVacancy } from './cards-fixtures';

vi.mock('vue-sonner', () => ({ toast: { success: vi.fn(), error: vi.fn() } }));

const flush = async () => {
  for (let i = 0; i < 4; i++) {
    await nextTick();
  }
};

afterEach(() => {
  document.body.innerHTML = '';
  document.body.style.overflow = '';
});

describe('listing/ListingHeader', () => {
  it('kind, stage, title and author link', async () => {
    const w = await mountSuspended(ListingHeader, { props: { listing: makeListing() } });
    expect(w.text()).toContain('Стартап');
    expect(w.text()).toContain('MVP');
    expect(w.find('h1').text()).toBe('Сервис аренды самокатов');
    expect(w.find('a[href="/users/u-anna"]').exists()).toBe(true);
    expect(w.find('[data-testid="listing-closed"]').exists()).toBe(false);
  });

  it('closed → «Публикация закрыта»', async () => {
    const w = await mountSuspended(ListingHeader, {
      props: { listing: makeListing({ status: ListingStatus.LISTING_STATUS_CLOSED, closedAt: '2026-10-19T13:00:00Z' }) },
    });
    expect(w.find('[data-testid="listing-closed"]').text()).toContain('Публикация закрыта');
  });
});

describe('listing/ListingBody', () => {
  it('startup: description with line breaks, looking for, tags, stage', async () => {
    const w = await mountSuspended(ListingBody, {
      props: { listing: makeListing({ description: 'Строка 1\nСтрока 2' }) },
    });
    expect(w.find('p').element.textContent).toBe('Строка 1\nСтрока 2');
    expect(w.text()).toContain('Кого ищем');
    expect(w.text()).toContain('Backend-разработчика');
    expect(w.text()).toContain('mobility');
    expect(w.find('[data-testid="listing-stage"]').text()).toContain('MVP');
    expect(w.find('[data-testid="listing-compensation"]').exists()).toBe(false);
  });

  it('vacancy: compensation, no stage', async () => {
    const w = await mountSuspended(ListingBody, { props: { listing: makeVacancy() } });
    expect(w.find('[data-testid="listing-compensation"]').text()).toContain('от 120 000 ₽');
    expect(w.find('[data-testid="listing-stage"]').exists()).toBe(false);
  });
});

describe('listing/ListingActionBar', () => {
  it('enabled → emits apply', async () => {
    const w = await mountSuspended(ListingActionBar, { props: { apply: 'enabled' } });
    await w.find('[data-testid="apply"]').trigger('click');
    expect(w.emitted('apply')).toHaveLength(1);
  });

  it('disabled → disabled button with «Публикация закрыта»', async () => {
    const w = await mountSuspended(ListingActionBar, { props: { apply: 'disabled' } });
    const button = w.find('[data-testid="apply"]');
    expect((button.element as HTMLButtonElement).disabled).toBe(true);
    expect(w.text()).toContain('Публикация закрыта');
    await button.trigger('click');
    expect(w.emitted('apply')).toBeUndefined();
  });

  it('hidden → nothing', async () => {
    const w = await mountSuspended(ListingActionBar, { props: { apply: 'hidden' } });
    expect(w.find('[data-testid="listing-action-bar"]').exists()).toBe(false);
  });
});

describe('listing/MyApplicationBlock', () => {
  it('pending: «Вы откликнулись · Ожидает», message, withdraw after confirmation', async () => {
    const withdraw = vi.fn(() => Promise.resolve());
    const w = await mountSuspended(MyApplicationBlock, {
      props: { application: makeApplication(), canWithdraw: true, withdraw },
      attachTo: document.body,
    });
    expect(w.text()).toContain('Вы откликнулись');
    expect(w.find('[data-testid="my-application-status"]').text()).toBe('Ожидает');
    expect(w.text()).toContain('Хочу в команду.');

    await w.find('[data-testid="withdraw"]').trigger('click');
    await flush();
    const confirm = [...document.body.querySelectorAll<HTMLButtonElement>('[role="dialog"] button')]
      .find(b => b.textContent === 'Отозвать')!;
    confirm.click();
    await flush();
    expect(withdraw).toHaveBeenCalledTimes(1);
    expect(w.emitted('withdrawn')).toHaveLength(1);
    w.unmount();
  });

  it('accepted: no withdraw button', async () => {
    const w = await mountSuspended(MyApplicationBlock, {
      props: {
        application: makeApplication({ status: ApplicationStatus.APPLICATION_STATUS_ACCEPTED, decidedAt: '2026-10-19T13:00:00Z' }),
        canWithdraw: false,
        withdraw: vi.fn(),
      },
    });
    expect(w.find('[data-testid="my-application-status"]').text()).toBe('Принят');
    expect(w.find('[data-testid="withdraw"]').exists()).toBe(false);
  });
});

describe('listing/OwnerPanel', () => {
  it('stats 1 · 1 · 0, «Отклики (2)», edit and close for active', async () => {
    const w = await mountSuspended(OwnerPanel, {
      props: {
        listingId: 'l-1',
        stats: { total: 2, pending: 1, accepted: 1, declined: 0 },
        canEdit: true,
        canClose: true,
        close: vi.fn(),
      },
    });
    const values = w.findAll('[data-testid="owner-stats"] dd').map(d => d.text());
    expect(values).toEqual(['1', '1', '0']);
    expect(w.find('[data-testid="owner-applications"]').text()).toBe('Отклики (2)');
    expect(w.find('a[href="/listings/l-1/applications"]').exists()).toBe(true);
    expect(w.find('a[href="/listings/l-1/edit"]').exists()).toBe(true);
    expect(w.find('[data-testid="owner-close"]').exists()).toBe(true);
  });

  it('closed listing: no edit / close', async () => {
    const w = await mountSuspended(OwnerPanel, {
      props: { listingId: 'l-1', stats: { total: 0, pending: 0, accepted: 0, declined: 0 }, close: vi.fn() },
    });
    expect(w.find('a[href="/listings/l-1/edit"]').exists()).toBe(false);
    expect(w.find('[data-testid="owner-close"]').exists()).toBe(false);
  });
});
