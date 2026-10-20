import { AUTH_COOKIE } from '~~/api/constants';
import { useAuthStore, useAuthUnavailable } from '~/stores/auth';

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
    // Composables before the first `await`: the Nuxt context is gone after it.
    const unavailable = useAuthUnavailable();
    try {
      const user = await auth.fetchMe();
      if (!user) {
        // Expired / invalid token: drop the cookie so the next request is clean.
        cookie.value = null;
      }
    } catch (error) {
      // Backend down: the session may well be valid — do not send the user to /login
      // (the middleware shows a 503 error page with «Повторить» instead).
      console.error('[auth] fetchMe failed', error);
      unavailable.value = true;
    }
  },
});
