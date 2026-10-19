import { computed, defineComponent, onMounted, watch } from 'vue';
import { toast } from 'vue-sonner';
import { ApplicationList, ApplicationListSkeleton, patchApplication } from '~/components/applications/application-list';
import { StatusFilter } from '~/components/applications/status-filter';
import { parseStatusQuery, statusFilterLabel, statusFilterToEnum } from '~/components/applications/status';
import type { StatusFilterValue } from '~/components/applications/status';
import { EmptyState } from '~/components/states/empty-state';
import { ErrorState } from '~/components/states/error-state';
import { Button } from '~/components/ui/button';
import { PageHeader } from '~/components/ui/page-header';
import { getErrorMessage, getErrorStatus } from '~/utils/errors';
import { getApi } from '~~/api/client';
import type { Application } from '~~/api/types';
import styles from '~/components/applications/page.module.css';

const NOT_AUTHOR_MESSAGE = 'Отклики на публикацию видит только её автор';

/** `/listings/:id/applications` — the author reviews applications to one listing (SPEC §7). */
export default defineComponent({
  name: 'ListingApplicationsPage',
  async setup() {
    const api = getApi();
    const route = useRoute();
    const router = useRouter();

    const id = computed(() => String(route.params.id));
    const filter = computed(() => parseStatusQuery(route.query.status));

    const listing = useAsyncData(
      () => `listing-applications-head:${id.value}`,
      () => api.listings.get(id.value),
    );
    const applications = useAsyncData(
      () => `listing-applications:${id.value}:${filter.value || 'all'}`,
      () => api.applications.listForListing(id.value, statusFilterToEnum(filter.value)),
    );
    await Promise.all([listing, applications]);

    const { data, error, status, refresh } = applications;

    if (getErrorStatus(listing.error.value) === 404 || getErrorStatus(error.value) === 404) {
      throw createError({ statusCode: 404, statusMessage: 'Публикация не найдена', fatal: true });
    }

    const forbidden = computed(() => getErrorStatus(error.value) === 403);

    useHead(() => ({
      title: listing.data.value ? `Отклики · ${listing.data.value.title}` : 'Отклики на публикацию',
    }));

    // Not the author → back to the listing. Done on the client so the toast is visible
    // (an SSR redirect would lose it); SSR renders the «no rights» state meanwhile.
    onMounted(() => {
      watch(forbidden, (isForbidden) => {
        if (isForbidden) {
          toast.error(NOT_AUTHOR_MESSAGE);
          void navigateTo(`/listings/${id.value}`, { replace: true });
        }
      }, { immediate: true });
    });

    const setFilter = (value: StatusFilterValue) => {
      void router.replace({ query: { ...route.query, status: value || undefined } });
    };

    const onUpdated = (next: Application) => {
      data.value = patchApplication(data.value, next);
      void refresh();
    };

    const renderBody = () => {
      if (forbidden.value) {
        return <ErrorState title={'Недостаточно прав'} message={NOT_AUTHOR_MESSAGE} />;
      }
      if (error.value && !data.value) {
        return (
          <ErrorState
            message={getErrorMessage(error.value)}
            retry
            retrying={status.value === 'pending'}
            onRetry={() => refresh()}
          />
        );
      }
      if (!data.value) {
        return <ApplicationListSkeleton />;
      }
      if (data.value.length === 0) {
        return filter.value
          ? (
            <EmptyState title={`Нет откликов со статусом «${statusFilterLabel(filter.value)}»`}>
              {{ actions: () => <Button variant={'secondary'} onClick={() => setFilter('')}>{'Показать все'}</Button> }}
            </EmptyState>
          )
          : (
            <EmptyState
              title={'Откликов пока нет'}
              description={'Когда кто-нибудь откликнется на публикацию, отклик появится здесь.'}
            />
          );
      }
      return (
        <ApplicationList
          items={data.value}
          mode={'incoming'}
          onUpdated={onUpdated}
          onStale={() => refresh()}
        />
      );
    };

    return () => (
      <div class={styles.page}>
        <PageHeader
          title={'Отклики на публикацию'}
          subtitle={listing.data.value?.title}
          back={`/listings/${id.value}`}
          backLabel={'К публикации'}
        />
        {!forbidden.value && (
          <StatusFilter modelValue={filter.value} onUpdate:modelValue={setFilter} />
        )}
        {renderBody()}
      </div>
    );
  },
});
