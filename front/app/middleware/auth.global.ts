import { useAuthStore } from '~/stores/auth';
import { safeRedirect } from '~/utils/redirect';

/** Routes available without a session. */
const PUBLIC_ROUTES = new Set(['/login', '/register']);

export default defineNuxtRouteMiddleware((to) => {
  const auth = useAuthStore();
  const isPublic = PUBLIC_ROUTES.has(to.path);

  if (!auth.user && !isPublic) {
    return navigateTo({ path: '/login', query: to.fullPath === '/' ? {} : { redirect: to.fullPath } });
  }

  if (auth.user && isPublic) {
    return navigateTo(safeRedirect(to.query.redirect));
  }
});
