import { AUTH_COOKIE } from '~~/api/constants';
import { useAuthStore } from '~/stores/auth';

/** SSR: resolve the current user once per request if the auth cookie is present. */
export default defineNuxtPlugin({
  name: 'auth',
  dependsOn: ['pinia'],
  async setup() {
    const cookie = useCookie(AUTH_COOKIE);
    if (!cookie.value) {
      return;
    }
    const auth = useAuthStore();
    try {
      const user = await auth.fetchMe();
      if (!user) {
        // Expired / invalid token: drop the cookie so the next request is clean.
        cookie.value = null;
      }
    } catch (error) {
      // Backend down: render as anonymous; the middleware sends the user to /login.
      console.error('[auth] fetchMe failed', error);
    }
  },
});
