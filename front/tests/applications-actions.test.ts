import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { mountSuspended } from '@nuxt/test-utils/runtime';
import { flushPromises } from '@vue/test-utils';
import { nextTick } from 'vue';
import { ApplicationActions } from '~/components/applications/application-actions';
import { ApplicationList } from '~/components/applications/application-list';
import { useAuthStore } from '~/stores/auth';
import { ApplicationStatus } from '~~/api/types';
import { makeApplication, makeUser } from './cards-fixtures';

const api = vi.hoisted(() => ({
  accept: vi.fn(),
  decline: vi.fn(),
  withdraw: vi.fn(),
  listReceived: vi.fn(),
}));
const toast = vi.hoisted(() => ({ success: vi.fn(), error: vi.fn() }));

vi.mock('~~/api/client', () => ({
  getApi: () => ({ applications: api }),
  setUnauthorizedHandler: () => {},
}));
vi.mock('vue-sonner', async importOriginal => ({
  ...(await importOriginal<typeof import('vue-sonner')>()),
  toast,
}));

const apiError = (status: number, message: string) =>
  Object.assign(new Error(message), { status, data: { code: 9, message } });

const confirmButton = () => [...document.body.querySelectorAll<HTMLButtonElement>('[role="dialog"] button')]
  .find(b => b.textContent?.trim() !== 'Отмена');

const deferred = <T>() => {
  let resolve!: (v: T) => void;
  const promise = new Promise<T>((r) => {
    resolve = r;
  });
  return { promise, resolve };
};

beforeEach(() => {
  Object.values(api).forEach(fn => fn.mockReset());
  Object.values(toast).forEach(fn => fn.mockReset());
  api.listReceived.mockResolvedValue([]);
  useAuthStore().setUser(makeUser());
});

afterEach(() => {
  document.body.innerHTML = '';
  useAuthStore().setUser(null);
});

describe('applications/ApplicationActions', () => {
  it('accept: one request despite a double click, success toast, `updated`, badge reload', async () => {
    const d = deferred<unknown>();
    const accepted = makeApplication({ status: ApplicationStatus.APPLICATION_STATUS_ACCEPTED });
    api.accept.mockReturnValue(d.promise);
    const w = await mountSuspended(ApplicationActions, { props: { application: makeApplication(), mode: 'incoming' } });

    await w.get('[data-testid="application-accept"]').trigger('click');
    await w.get('[data-testid="application-accept"]').trigger('click');
    expect(api.accept).toHaveBeenCalledTimes(1);
    expect(api.accept).toHaveBeenCalledWith('a-1');
    expect(w.get('[data-testid="application-accept"]').attributes('disabled')).toBeDefined();
    expect(w.get('[data-testid="application-decline"]').attributes('disabled')).toBeDefined();

    d.resolve(accepted);
    await flushPromises();
    expect(w.emitted('updated')?.[0]).toEqual([accepted]);
    expect(w.emitted('stale')).toBeUndefined();
    expect(toast.success).toHaveBeenCalledWith('Отклик принят');
    expect(api.listReceived).toHaveBeenCalledWith(ApplicationStatus.APPLICATION_STATUS_PENDING);
  });

  it('400 «Решение по отклику уже принято» → error toast and `stale`', async () => {
    api.accept.mockRejectedValue(apiError(400, 'Решение по отклику уже принято'));
    const w = await mountSuspended(ApplicationActions, { props: { application: makeApplication(), mode: 'incoming' } });
    await w.get('[data-testid="application-accept"]').trigger('click');
    await flushPromises();
    expect(toast.error).toHaveBeenCalledWith('Решение по отклику уже принято');
    expect(w.emitted('stale')?.[0]).toEqual(['a-1']);
    expect(w.emitted('updated')).toBeUndefined();
    // Buttons are unlocked again.
    expect(w.get('[data-testid="application-accept"]').attributes('disabled')).toBeUndefined();
  });

  it('404 → error toast and `stale`; network error → toast only', async () => {
    api.accept.mockRejectedValueOnce(apiError(404, 'Отклик не найден'));
    const w = await mountSuspended(ApplicationActions, { props: { application: makeApplication(), mode: 'incoming' } });
    await w.get('[data-testid="application-accept"]').trigger('click');
    await flushPromises();
    expect(toast.error).toHaveBeenLastCalledWith('Отклик не найден');
    expect(w.emitted('stale')).toHaveLength(1);

    api.accept.mockRejectedValueOnce(Object.assign(new TypeError('Failed to fetch')));
    await w.get('[data-testid="application-accept"]').trigger('click');
    await flushPromises();
    expect(toast.error).toHaveBeenCalledTimes(2);
    expect(w.emitted('stale')).toHaveLength(1);
  });

  it('decline asks for confirmation first', async () => {
    const declined = makeApplication({ status: ApplicationStatus.APPLICATION_STATUS_DECLINED });
    api.decline.mockResolvedValue(declined);
    const w = await mountSuspended(ApplicationActions, {
      props: { application: makeApplication(), mode: 'incoming' },
      attachTo: document.body,
    });
    await w.get('[data-testid="application-decline"]').trigger('click');
    await nextTick();
    expect(api.decline).not.toHaveBeenCalled();
    const confirm = confirmButton();
    expect(confirm?.textContent).toContain('Отклонить');
    confirm!.click();
    await flushPromises();
    expect(api.decline).toHaveBeenCalledWith('a-1');
    expect(w.emitted('updated')?.[0]).toEqual([declined]);
    expect(toast.success).toHaveBeenCalledWith('Отклик отклонён');
    w.unmount();
  });

  it('outgoing: «Отозвать» with confirmation, no accept/decline', async () => {
    const withdrawn = makeApplication({ status: ApplicationStatus.APPLICATION_STATUS_WITHDRAWN });
    api.withdraw.mockResolvedValue(withdrawn);
    const w = await mountSuspended(ApplicationActions, {
      props: { application: makeApplication(), mode: 'outgoing' },
      attachTo: document.body,
    });
    expect(w.find('[data-testid="application-accept"]').exists()).toBe(false);
    await w.get('[data-testid="application-withdraw"]').trigger('click');
    await nextTick();
    expect(api.withdraw).not.toHaveBeenCalled();
    confirmButton()!.click();
    await flushPromises();
    expect(api.withdraw).toHaveBeenCalledWith('a-1');
    expect(w.emitted('updated')?.[0]).toEqual([withdrawn]);
    w.unmount();
  });

  it('renders nothing for a final status', async () => {
    const w = await mountSuspended(ApplicationActions, {
      props: { application: makeApplication({ status: ApplicationStatus.APPLICATION_STATUS_WITHDRAWN }), mode: 'outgoing' },
    });
    expect(w.find('button').exists()).toBe(false);
  });
});

describe('applications/ApplicationList', () => {
  it('shows actions only for pending applications', async () => {
    const w = await mountSuspended(ApplicationList, {
      props: {
        mode: 'outgoing',
        items: [
          makeApplication({ id: 'a-1' }),
          makeApplication({ id: 'a-2', status: ApplicationStatus.APPLICATION_STATUS_WITHDRAWN }),
        ],
      },
    });
    const cards = w.findAll('[data-testid="application-card"]');
    expect(cards).toHaveLength(2);
    expect(cards[0]!.find('[data-testid="application-withdraw"]').exists()).toBe(true);
    expect(cards[1]!.find('button').exists()).toBe(false);
    expect(cards[1]!.text()).toContain('Отозван');
  });
});
