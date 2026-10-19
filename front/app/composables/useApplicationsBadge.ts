import { computed, onMounted, watch } from 'vue';
import type { ComputedRef } from 'vue';
import { getApi } from '~~/api/client';
import { ApplicationStatus } from '~~/api/types';
import { useAuthStore } from '~/stores/auth';

/** Minimum interval between two non-forced badge requests. */
export const APPLICATIONS_BADGE_TTL_MS = 30_000;

export const APPLICATIONS_BADGE_STATE_KEY = 'applications-badge';

/**
 * Serializable badge state (shared through `useState`). `userId` ties the count to the
 * account it was loaded for: after a user switch the count is reset and reloaded, so
 * `clearNuxtData()` on login/logout does not have to know about it.
 */
export interface ApplicationsBadgeState {
  userId: string | null
  count: number
  /** `Date.now()` of the last request start; 0 = never loaded. */
  fetchedAt: number
}

/** Non-serializable per-app runtime: the in-flight request and a sequence number. */
interface BadgeRuntime {
  inflight: Promise<void> | null
  seq: number
}

const runtimes = new WeakMap<object, BadgeRuntime>();

export interface RefreshBadgeOptions {
  /** Ignore the 30s throttle (after accept/decline/withdraw). */
  force?: boolean
}

export interface ApplicationsBadgeControl {
  /** Pending received applications of the current user. */
  count: ComputedRef<number>
  /**
   * Reloads the count. Without `force` it is a no-op if the last request started less
   * than 30s ago (or is still running). Never throws; client-only (no-op in SSR).
   */
  refresh: (options?: RefreshBadgeOptions) => Promise<void>
}

const emptyState = (): ApplicationsBadgeState => ({ userId: null, count: 0, fetchedAt: 0 });

/**
 * Badge controller: call synchronously in `setup()`, then `refresh({ force: true })`
 * after actions that change the number of pending received applications.
 */
export const useApplicationsBadgeControl = (): ApplicationsBadgeControl => {
  const nuxtApp = useNuxtApp();
  const api = getApi();
  const auth = useAuthStore();
  const state = useState<ApplicationsBadgeState>(APPLICATIONS_BADGE_STATE_KEY, emptyState);

  let runtime = runtimes.get(nuxtApp);
  if (!runtime) {
    runtime = { inflight: null, seq: 0 };
    runtimes.set(nuxtApp, runtime);
  }
  const rt = runtime;

  const refresh = (options: RefreshBadgeOptions = {}): Promise<void> => {
    if (import.meta.server) {
      return Promise.resolve();
    }
    const userId = auth.user?.id ?? null;
    if (!userId) {
      rt.seq += 1; // drop the result of a request made for the previous user
      rt.inflight = null;
      state.value = emptyState();
      return Promise.resolve();
    }

    let force = options.force ?? false;
    if (state.value.userId !== userId) {
      rt.seq += 1;
      rt.inflight = null;
      state.value = { userId, count: 0, fetchedAt: 0 };
      force = true;
    }
    if (!force) {
      if (rt.inflight) {
        return rt.inflight;
      }
      if (Date.now() - state.value.fetchedAt < APPLICATIONS_BADGE_TTL_MS) {
        return Promise.resolve();
      }
    }

    const seq = ++rt.seq;
    state.value = { ...state.value, fetchedAt: Date.now() };
    const request: Promise<void> = api.applications
      .listReceived(ApplicationStatus.APPLICATION_STATUS_PENDING)
      .then((items) => {
        // A newer request (forced refresh / user switch) wins.
        if (seq === rt.seq && state.value.userId === userId) {
          state.value = { ...state.value, count: items.length };
        }
      })
      .catch(() => {
        // Keep the previous number; the next navigation after the TTL retries.
      })
      .finally(() => {
        if (rt.inflight === request) {
          rt.inflight = null;
        }
      });
    rt.inflight = request;
    return request;
  };

  return {
    count: computed(() => (state.value.userId !== null && state.value.userId === auth.user?.id
      ? state.value.count
      : 0)),
    refresh,
  };
};

/**
 * Number shown on the «Отклики» nav item: pending applications to the current user's
 * listings (`GET /v1/me/applications/received?status=APPLICATION_STATUS_PENDING`).
 *
 * Used by the nav components (header and tab bar): loads on mount (client only), after
 * a user switch (login / logout) and on navigation — at most once per 30s. Actions force
 * a reload with `useApplicationsBadgeControl().refresh({ force: true })`.
 */
export const useApplicationsBadge = (): ComputedRef<number> => {
  const badge = useApplicationsBadgeControl();
  const auth = useAuthStore();
  const route = useRoute();

  onMounted(() => {
    void badge.refresh();
    watch(() => auth.user?.id, () => void badge.refresh());
    watch(() => route.path, () => void badge.refresh());
  });

  return badge.count;
};
