import { toast } from 'vue-sonner';
import { setUnauthorizedHandler } from '~~/api/client';
import { useAuthStore } from '~/stores/auth';

/** Client: any 401 from the API (outside of /auth/*) means the session has expired. */
export default defineNuxtPlugin({
  name: 'api-unauthorized',
  dependsOn: ['pinia'],
  setup(nuxtApp) {
    const router = useRouter();
    setUnauthorizedHandler(() => {
      const auth = useAuthStore(nuxtApp.$pinia as never);
      if (!auth.user) {
        return;
      }
      auth.setUser(null);
      toast.error('Сессия истекла, войдите снова');
      const redirect = router.currentRoute.value.fullPath;
      nuxtApp.runWithContext(() => navigateTo({ path: '/login', query: { redirect } }));
    });
  },
});
