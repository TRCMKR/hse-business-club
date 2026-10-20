import { afterEach, describe, expect, it, vi } from 'vitest';
import { mockNuxtImport } from '@nuxt/test-utils/runtime';
import type { RouteLocationNormalized } from 'vue-router';
import authMiddleware from '~/middleware/auth.global';
import { useAuthStore, useAuthUnavailable } from '~/stores/auth';
import { makeUser } from './cards-fixtures';

const route = (fullPath: string) => {
  const [path, search = ''] = fullPath.split('?');
  return { path, fullPath, query: Object.fromEntries(new URLSearchParams(search)) } as unknown as RouteLocationNormalized;
};

const run = (to: string) => authMiddleware(route(to), route('/'));

const { reloadNuxtAppMock } = vi.hoisted(() => ({ reloadNuxtAppMock: vi.fn() }));
mockNuxtImport('reloadNuxtApp', () => reloadNuxtAppMock);

afterEach(() => {
  useAuthStore().setUser(null);
  useAuthUnavailable().value = false;
  useNuxtApp().isHydrating = false;
  reloadNuxtAppMock.mockClear();
});

describe('middleware/auth.global', () => {
  it('backend unreachable during SSR → 503 error instead of /login', () => {
    useAuthUnavailable().value = true;
    useNuxtApp().isHydrating = true;
    let thrown: { statusCode?: number, statusMessage?: string } | undefined;
    try {
      run('/me/listings');
    } catch (error) {
      thrown = error as typeof thrown;
    }
    expect(thrown?.statusCode).toBe(503);
    expect(thrown?.statusMessage).toBe('Сервер временно недоступен');
  });

  it('client navigation after the SSR 503 → full reload of the target, flag reset', () => {
    useAuthUnavailable().value = true;
    run('/me/listings');
    expect(reloadNuxtAppMock).toHaveBeenCalledWith({ path: '/me/listings', force: true });
    expect(useAuthUnavailable().value).toBe(false);
  });

  it('public routes stay available while the backend is down', () => {
    useAuthUnavailable().value = true;
    expect(run('/login')).toBeUndefined();
  });

  it('signed-in user: private routes pass', () => {
    useAuthStore().setUser(makeUser());
    useAuthUnavailable().value = true;
    expect(run('/me/listings')).toBeUndefined();
  });
});
