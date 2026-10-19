import { defineStore } from 'pinia';
import { computed, ref } from 'vue';
import { getApi } from '~~/api/client';
import { getErrorStatus } from '~/utils/errors';
import type { LoginInput, RegisterInput, User } from '~~/api/types';

export const useAuthStore = defineStore('auth', () => {
  const user = ref<User | null>(null);

  const isAuthenticated = computed(() => user.value !== null);

  /** Replaces the cached current user (e.g. after PUT /v1/me or an avatar upload). */
  const setUser = (next: User | null) => {
    user.value = next;
  };

  /** Loads the current user. Resolves to null on 401 (no/expired session); rethrows other errors. */
  const fetchMe = async (): Promise<User | null> => {
    try {
      user.value = await getApi().users.me();
    } catch (error) {
      if (getErrorStatus(error) === 401) {
        user.value = null;
      } else {
        throw error;
      }
    }
    return user.value;
  };

  /** Client-only. Throws on wrong credentials (401) — show `getErrorMessage(error)`. */
  const login = async (input: LoginInput): Promise<User> => {
    user.value = await getApi().auth.login(input);
    return user.value;
  };

  /** Client-only. Throws on validation errors (400) / taken username (409). */
  const register = async (input: RegisterInput): Promise<User> => {
    user.value = await getApi().auth.register(input);
    return user.value;
  };

  /** Clears the httpOnly cookie on the Nitro side and the local state. */
  const logout = async () => {
    try {
      await getApi().auth.logout();
    } finally {
      user.value = null;
    }
  };

  return { user, isAuthenticated, setUser, fetchMe, login, register, logout };
});
