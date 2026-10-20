import { beforeEach, describe, expect, it, vi } from 'vitest';
import { mountSuspended } from '@nuxt/test-utils/runtime';
import { flushPromises } from '@vue/test-utils';
import { StatusFilter } from '~/components/applications/status-filter';
import { parseStatusQuery, statusFilterLabel, statusFilterToEnum } from '~/components/applications/status';
import { useAuthStore } from '~/stores/auth';
import MyApplicationsPage from '~/pages/me/applications';
import { ApplicationStatus } from '~~/api/types';
import { makeApplication, makeUser } from './cards-fixtures';

const api = vi.hoisted(() => ({
  listSent: vi.fn(),
  listReceived: vi.fn(),
  accept: vi.fn(),
  decline: vi.fn(),
  withdraw: vi.fn(),
}));

vi.mock('~~/api/client', () => ({
  getApi: () => ({ applications: api }),
  setUnauthorizedHandler: () => {},
}));

beforeEach(() => {
  Object.values(api).forEach(fn => fn.mockReset());
  api.listSent.mockResolvedValue([]);
  api.listReceived.mockResolvedValue([]);
  useAuthStore().setUser(makeUser());
  clearNuxtData();
});

describe('applications/status filter helpers', () => {
  it('parses the `status` query and maps it to the API enum', () => {
    expect(parseStatusQuery('pending')).toBe('pending');
    expect(parseStatusQuery('constructor')).toBe('');
    expect(parseStatusQuery('__proto__')).toBe('');
    expect(parseStatusQuery('toString')).toBe('');
    expect(parseStatusQuery('ACCEPTED')).toBe('accepted');
    expect(parseStatusQuery(['withdrawn', 'pending'])).toBe('withdrawn');
    expect(parseStatusQuery('bogus')).toBe('');
    expect(parseStatusQuery(undefined)).toBe('');
    expect(statusFilterToEnum('declined')).toBe(ApplicationStatus.APPLICATION_STATUS_DECLINED);
    expect(statusFilterToEnum('')).toBeUndefined();
    expect(statusFilterLabel('accepted')).toBe('Принятые');
  });
});

describe('applications/StatusFilter', () => {
  it('marks the selected chip and emits a new value', async () => {
    const w = await mountSuspended(StatusFilter, { props: { modelValue: 'pending' } });
    expect(w.get('[data-testid="status-filter-pending"]').attributes('aria-pressed')).toBe('true');
    expect(w.get('[data-testid="status-filter-all"]').attributes('aria-pressed')).toBe('false');
    await w.get('[data-testid="status-filter-accepted"]').trigger('click');
    await w.get('[data-testid="status-filter-pending"]').trigger('click');
    expect(w.emitted('update:modelValue')).toEqual([['accepted']]);
  });
});

describe('pages/me/applications', () => {
  it('received tab + status from the query go to the API', async () => {
    api.listReceived.mockResolvedValue([makeApplication()]);
    const w = await mountSuspended(MyApplicationsPage, { route: '/me/applications?tab=received&status=pending' });
    await flushPromises();
    expect(api.listReceived).toHaveBeenCalledWith(ApplicationStatus.APPLICATION_STATUS_PENDING);
    expect(api.listSent).not.toHaveBeenCalled();
    expect(w.get('[role="tab"][aria-selected="true"]').text()).toContain('Полученные');
    expect(w.find('[data-testid="application-accept"]').exists()).toBe(true);
  });

  it('changing the filter updates the query and reloads', async () => {
    const w = await mountSuspended(MyApplicationsPage, { route: '/me/applications' });
    await flushPromises();
    expect(api.listSent).toHaveBeenLastCalledWith(undefined);
    expect(w.text()).toContain('Вы ещё не откликались');

    await w.get('[data-testid="status-filter-withdrawn"]').trigger('click');
    // router.replace goes through the async navigation guards.
    await vi.waitFor(() => expect(useRouter().currentRoute.value.query.status).toBe('withdrawn'));
    await flushPromises();
    expect(api.listSent).toHaveBeenLastCalledWith(ApplicationStatus.APPLICATION_STATUS_WITHDRAWN);
    expect(w.text()).toContain('Нет откликов со статусом «Отозванные»');
  });
});
