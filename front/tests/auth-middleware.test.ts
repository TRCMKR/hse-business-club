import { afterEach, describe, expect, it } from 'vitest';
import type { RouteLocationNormalized } from 'vue-router';
import authMiddleware from '~/middleware/auth.global';
import { useAuthStore, useAuthUnavailable } from '~/stores/auth';
import { makeUser } from './cards-fixtures';

const route = (fullPath: string) => {
  const [path, search = ''] = fullPath.split('?');
  return { path, fullPath, query: Object.fromEntries(new URLSearchParams(search)) } as unknown as RouteLocationNormalized;
};

const run = (to: string) => authMiddleware(route(to), route('/'));

afterEach(() => {
  useAuthStore().setUser(null);
  useAuthUnavailable().value = false;
});

describe('middleware/auth.global', () => {
  it('backend unreachable during SSR → 503 error instead of /login', () => {
    useAuthUnavailable().value = true;
    let thrown: { statusCode?: number, statusMessage?: string } | undefined;
    try {
      run('/me/listings');
    } catch (error) {
      thrown = error as typeof thrown;
    }
    expect(thrown?.statusCode).toBe(503);
    expect(thrown?.statusMessage).toBe('Сервер временно недоступен');
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
