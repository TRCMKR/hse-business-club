import { beforeEach, describe, expect, it, vi } from 'vitest';
import { mountSuspended } from '@nuxt/test-utils/runtime';
import { flushPromises } from '@vue/test-utils';
import ListingApplicationsPage from '~/pages/listings/[id]/applications';
import { useAuthStore } from '~/stores/auth';
import { ApplicationStatus } from '~~/api/types';
import { makeApplication, makeListing, makeUser } from './cards-fixtures';

const api = vi.hoisted(() => ({
  applications: {
    listForListing: vi.fn(),
    listReceived: vi.fn(),
    accept: vi.fn(),
    decline: vi.fn(),
    withdraw: vi.fn(),
  },
  listings: { get: vi.fn() },
}));
const toast = vi.hoisted(() => ({ success: vi.fn(), error: vi.fn() }));

vi.mock('~~/api/client', () => ({
  getApi: () => api,
  setUnauthorizedHandler: () => {},
}));
vi.mock('vue-sonner', async importOriginal => ({
  ...(await importOriginal<typeof import('vue-sonner')>()),
  toast,
}));

const apiError = (status: number, message: string) =>
  Object.assign(new Error(message), { status, data: { code: 7, message } });

beforeEach(() => {
  Object.values(api.applications).forEach(fn => fn.mockReset());
  api.listings.get.mockReset();
  toast.error.mockReset();
  api.listings.get.mockResolvedValue(makeListing({ id: 'l-1', title: 'Сервис аренды самокатов' }));
  api.applications.listReceived.mockResolvedValue([]);
  useAuthStore().setUser(makeUser());
  clearNuxtData();
});

describe('pages/listings/[id]/applications', () => {
  it('author: incoming cards with actions, status filter from the query', async () => {
    api.applications.listForListing.mockResolvedValue([makeApplication()]);
    const w = await mountSuspended(ListingApplicationsPage, { route: '/listings/l-1/applications?status=pending' });
    await flushPromises();
    expect(api.applications.listForListing).toHaveBeenCalledWith('l-1', ApplicationStatus.APPLICATION_STATUS_PENDING);
    expect(w.text()).toContain('Сервис аренды самокатов');
    expect(w.find('[data-testid="application-accept"]').exists()).toBe(true);
    expect(w.get('[data-testid="status-filter-pending"]').attributes('aria-pressed')).toBe('true');
  });

  it('empty list → «Откликов пока нет»', async () => {
    api.applications.listForListing.mockResolvedValue([]);
    const w = await mountSuspended(ListingApplicationsPage, { route: '/listings/l-1/applications' });
    await flushPromises();
    expect(w.text()).toContain('Откликов пока нет');
  });

  it('not the author (403) → toast and redirect to the listing', async () => {
    api.applications.listForListing.mockRejectedValue(apiError(403, 'Недостаточно прав'));
    await mountSuspended(ListingApplicationsPage, { route: '/listings/l-1/applications' });
    await vi.waitFor(() => expect(useRouter().currentRoute.value.path).toBe('/listings/l-1'));
    expect(toast.error).toHaveBeenCalledWith('Отклики на публикацию видит только её автор');
  });
});
