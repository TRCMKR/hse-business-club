import { useAuthStore, useAuthUnavailable } from '~/stores/auth';
import { safeRedirect } from '~/utils/redirect';

/** Routes available without a session. */
const PUBLIC_ROUTES = new Set(['/login', '/register']);

export default defineNuxtRouteMiddleware((to) => {
  const auth = useAuthStore();
  const isPublic = PUBLIC_ROUTES.has(to.path);

  // Backend down while checking an existing session: an error page with «Повторить»
  // instead of a misleading login form.
  const unavailable = useAuthUnavailable();
  if (!auth.user && !isPublic && unavailable.value) {
    // On a client-side navigation (e.g. the bottom nav of the 503 page) the flag is a
    // leftover of the SSR request: re-check the session with a full load of the target.
    if (import.meta.client && !useNuxtApp().isHydrating) {
      unavailable.value = false;
      reloadNuxtApp({ path: to.fullPath, force: true });
      return abortNavigation();
    }
    return abortNavigation(createError({ statusCode: 503, statusMessage: 'Сервер временно недоступен', fatal: true }));
  }

  if (!auth.user && !isPublic) {
    return navigateTo({ path: '/login', query: to.fullPath === '/' ? {} : { redirect: to.fullPath } });
  }

  if (auth.user && isPublic) {
    return navigateTo(safeRedirect(to.query.redirect));
  }
});
