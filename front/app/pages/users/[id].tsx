import { computed, defineComponent } from 'vue';
import { getApi } from '~~/api/client';
import { PageHeader } from '~/components/ui/page-header';
import { Button } from '~/components/ui/button';
import { Card } from '~/components/ui/card';
import { Icon } from '~/components/ui/icon';
import { Skeleton } from '~/components/ui/skeleton';
import { EmptyState } from '~/components/states/empty-state';
import { ErrorState } from '~/components/states/error-state';
import { ProfileCard } from '~/components/profile/profile-card';
import { getErrorMessage, getErrorStatus } from '~/utils/errors';
import styles from './[id].module.css';

const UUID_PATTERN = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;

/** Public profile of a club member (GET /v1/users/{id}), read-only. */
export default defineComponent({
  name: 'UserPage',
  setup() {
    const api = getApi();
    const route = useRoute();
    const id = computed(() => String(route.params.id));
    const validId = computed(() => UUID_PATTERN.test(id.value));

    const { data, error, status, refresh } = useAsyncData(
      () => `user:${id.value}`,
      // A malformed id can't exist — answer «not found» without a request.
      () => (validId.value ? api.users.get(id.value) : Promise.resolve(null)),
    );

    const notFound = computed(() => !validId.value || getErrorStatus(error.value) === 404);

    useHead({
      title: computed(() => data.value?.displayName ?? (notFound.value ? 'Участник не найден' : 'Профиль участника')),
    });

    return () => (
      <div>
        <PageHeader title={'Профиль участника'} back={true} />
        {notFound.value
          ? (
            <EmptyState title={'Участник не найден'} description={'Возможно, ссылка устарела или в ней опечатка.'}>
              {{
                icon: () => <Icon name={'users'} size={32} />,
                actions: () => <Button variant={'secondary'} to={'/'}>{'В ленту'}</Button>,
              }}
            </EmptyState>
          )
          : error.value
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
