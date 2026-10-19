import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import {
  APPLICATIONS_BADGE_STATE_KEY,
  APPLICATIONS_BADGE_TTL_MS,
  useApplicationsBadgeControl,
} from '~/composables/useApplicationsBadge';
import type { ApplicationsBadgeState } from '~/composables/useApplicationsBadge';
import { useAuthStore } from '~/stores/auth';
import { ApplicationStatus } from '~~/api/types';
import { boris, makeApplication, makeUser } from './cards-fixtures';

const { listReceived } = vi.hoisted(() => ({ listReceived: vi.fn() }));

vi.mock('~~/api/client', () => ({
  getApi: () => ({ applications: { listReceived } }),
  setUnauthorizedHandler: () => {},
}));

const pending = (n: number) => Array.from({ length: n }, (_, i) => makeApplication({ id: `a-${i}` }));

const deferred = <T>() => {
  let resolve!: (v: T) => void;
  const promise = new Promise<T>((r) => {
    resolve = r;
  });
  return { promise, resolve };
};

let now = 1_000_000;

beforeEach(() => {
  now += 10 * APPLICATIONS_BADGE_TTL_MS;
  vi.spyOn(Date, 'now').mockImplementation(() => now);
  listReceived.mockReset();
  useState<ApplicationsBadgeState>(APPLICATIONS_BADGE_STATE_KEY).value = { userId: null, count: 0, fetchedAt: 0 };
  useAuthStore().setUser(makeUser());
});

afterEach(() => {
  vi.restoreAllMocks();
  useAuthStore().setUser(null);
});

describe('useApplicationsBadge', () => {
  it('loads the number of pending received applications', async () => {
    listReceived.mockResolvedValue(pending(2));
    const badge = useApplicationsBadgeControl();
    await badge.refresh();
    expect(listReceived).toHaveBeenCalledWith(ApplicationStatus.APPLICATION_STATUS_PENDING);
    expect(badge.count.value).toBe(2);
  });

  it('throttles non-forced refreshes to once per 30s, force bypasses it', async () => {
    listReceived.mockResolvedValue(pending(1));
    const badge = useApplicationsBadgeControl();
    await badge.refresh();
    expect(listReceived).toHaveBeenCalledTimes(1);

    now += APPLICATIONS_BADGE_TTL_MS - 1;
    await badge.refresh();
    expect(listReceived).toHaveBeenCalledTimes(1);

    listReceived.mockResolvedValue([]);
    await badge.refresh({ force: true });
    expect(listReceived).toHaveBeenCalledTimes(2);
    expect(badge.count.value).toBe(0);

    now += APPLICATIONS_BADGE_TTL_MS;
    await badge.refresh();
    expect(listReceived).toHaveBeenCalledTimes(3);
  });

  it('shares state between instances and dedupes concurrent requests', async () => {
    const d = deferred<ReturnType<typeof pending>>();
    listReceived.mockReturnValue(d.promise);
    const header = useApplicationsBadgeControl();
    const tabBar = useApplicationsBadgeControl();
    const p1 = header.refresh();
    const p2 = tabBar.refresh();
    d.resolve(pending(3));
    await Promise.all([p1, p2]);
    expect(listReceived).toHaveBeenCalledTimes(1);
    expect(header.count.value).toBe(3);
    expect(tabBar.count.value).toBe(3);
  });

  it('a forced refresh wins over an older in-flight request', async () => {
    const slow = deferred<ReturnType<typeof pending>>();
    listReceived.mockReturnValueOnce(slow.promise).mockResolvedValueOnce(pending(0));
    const badge = useApplicationsBadgeControl();
    const first = badge.refresh();
    await badge.refresh({ force: true });
    slow.resolve(pending(5));
    await first;
    expect(badge.count.value).toBe(0);
  });

  it('resets and reloads after a user switch; logout gives 0', async () => {
    listReceived.mockResolvedValue(pending(1));
    const badge = useApplicationsBadgeControl();
    await badge.refresh();
    expect(badge.count.value).toBe(1);

    const auth = useAuthStore();
    auth.setUser(boris());
    // The previous user's number is never shown for the new one.
    expect(badge.count.value).toBe(0);
    listReceived.mockResolvedValue(pending(4));
    await badge.refresh(); // not forced and within the TTL — still reloads for the new user
    expect(listReceived).toHaveBeenCalledTimes(2);
    expect(badge.count.value).toBe(4);

    auth.setUser(null);
    await badge.refresh();
    expect(badge.count.value).toBe(0);
    expect(listReceived).toHaveBeenCalledTimes(2);
  });

  it('keeps the previous number when the request fails', async () => {
    listReceived.mockResolvedValueOnce(pending(2)).mockRejectedValueOnce(new Error('network'));
    const badge = useApplicationsBadgeControl();
    await badge.refresh();
    await expect(badge.refresh({ force: true })).resolves.toBeUndefined();
    expect(badge.count.value).toBe(2);
  });
});
