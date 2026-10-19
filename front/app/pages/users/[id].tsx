import { computed, defineComponent } from 'vue';
import { getApi } from '~~/api/client';
import { PageHeader } from '~/components/ui/page-header';
import { Card } from '~/components/ui/card';
import { Skeleton } from '~/components/ui/skeleton';
import { ErrorState } from '~/components/states/error-state';
import { ProfileCard } from '~/components/profile/profile-card';
import { getErrorMessage, getErrorStatus } from '~/utils/errors';
import styles from './[id].module.css';

const UUID_PATTERN = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;

/** Public profile of a club member (GET /v1/users/{id}), read-only. */
export default defineComponent({
  name: 'UserPage',
  async setup() {
    const api = getApi();
    const route = useRoute();
    const id = computed(() => String(route.params.id));
    const validId = computed(() => UUID_PATTERN.test(id.value));

    const asyncData = useAsyncData(
      () => `user:${id.value}`,
      // A malformed id can't exist — answer «not found» without a request.
      () => (validId.value ? api.users.get(id.value) : Promise.resolve(null)),
    );

    const { data, error, status, refresh } = asyncData;

    useHead({
      title: computed(() => data.value?.displayName ?? 'Профиль участника'),
    });

    // Every composable is registered above: after `await` the Nuxt context is gone.
    // 404 → error page with a 404 HTTP status on SSR (not a 200 with an empty state).
    await asyncData;
    if (!validId.value || getErrorStatus(error.value) === 404) {
      throw createError({ statusCode: 404, statusMessage: 'Участник не найден', fatal: true });
    }

    return () => (
      <div>
        <PageHeader title={'Профиль участника'} back={true} />
        {error.value
          ? (
            <ErrorState
              message={getErrorMessage(error.value)}
              retry
              retrying={status.value === 'pending'}
              onRetry={() => refresh()}
            />
          )
          : data.value
            ? <ProfileCard user={data.value} />
            : (
              <Card>
                <div class={styles.skeleton} aria-busy={'true'}>
                  <Skeleton circle width={112} height={112} />
                  <div class={styles.skeletonText}><Skeleton lines={3} /></div>
                </div>
              </Card>
            )}
      </div>
    );
  },
});
