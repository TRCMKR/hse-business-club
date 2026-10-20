import { useAuthStore, useAuthUnavailable } from '~/stores/auth';
import { safeRedirect } from '~/utils/redirect';

/** Routes available without a session. */
const PUBLIC_ROUTES = new Set(['/login', '/register']);

export default defineNuxtRouteMiddleware((to) => {
  const auth = useAuthStore();
  const isPublic = PUBLIC_ROUTES.has(to.path);

  // Backend down while checking an existing session: an error page with «Повторить»
  // instead of a misleading login form.
  if (!auth.user && !isPublic && useAuthUnavailable().value) {
    return abortNavigation(createError({ statusCode: 503, statusMessage: 'Сервер временно недоступен', fatal: true }));
  }

  if (!auth.user && !isPublic) {
    return navigateTo({ path: '/login', query: to.fullPath === '/' ? {} : { redirect: to.fullPath } });
  }

  if (auth.user && isPublic) {
    return navigateTo(safeRedirect(to.query.redirect));
  }
});
