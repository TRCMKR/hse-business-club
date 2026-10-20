import type { BackendContext } from '~~/api/backend-context';
import { BACKEND_CONTEXT_KEY } from '~~/api/backend-context';
import { useAuthStore, useAuthUnavailable } from '~/stores/auth';

/**
 * SSR: resolve the current user once per request if the request carries a session
 * (access and/or refresh cookie). An expired access token is refreshed transparently by
 * the backend caller; if the refresh fails it clears the cookies itself.
 */
export default defineNuxtPlugin({
  name: 'auth',
  dependsOn: ['pinia'],
  async setup() {
    const backend = useRequestEvent()?.context[BACKEND_CONTEXT_KEY] as BackendContext | undefined;
    if (!backend?.hasSession()) {
      return;
    }
    const auth = useAuthStore();
    // Composables before the first `await`: the Nuxt context is gone after it.
    const unavailable = useAuthUnavailable();
    try {
      await auth.fetchMe();
    } catch (error) {
      // Backend down: the session may well be valid — do not send the user to /login
      // (the middleware shows a 503 error page with «Повторить» instead).
      console.error('[auth] fetchMe failed', error);
      unavailable.value = true;
    }
  },
});
