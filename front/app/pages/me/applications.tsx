import { computed, defineComponent } from 'vue';
import { ApplicationList, ApplicationListSkeleton, patchApplication } from '~/components/applications/application-list';
import { StatusFilter } from '~/components/applications/status-filter';
import { parseStatusQuery, statusFilterLabel, statusFilterToEnum } from '~/components/applications/status';
import type { StatusFilterValue } from '~/components/applications/status';
import { EmptyState } from '~/components/states/empty-state';
import { ErrorState } from '~/components/states/error-state';
import { Button } from '~/components/ui/button';
import { PageHeader } from '~/components/ui/page-header';
import { Tabs } from '~/components/ui/tabs';
import { useApplicationsBadgeControl } from '~/composables/useApplicationsBadge';
import { getErrorMessage } from '~/utils/errors';
import { getApi } from '~~/api/client';
import type { Application } from '~~/api/types';
import styles from '~/components/applications/page.module.css';

type MyApplicationsTab = 'sent' | 'received';

const TAB_PREFIX = 'my-applications';

const parseTab = (raw: unknown): MyApplicationsTab => {
  const value = Array.isArray(raw) ? raw[0] : raw;
  return value === 'received' ? 'received' : 'sent';
};

/** `/me/applications` — sent applications (withdraw) and received ones (accept / decline). */
export default defineComponent({
  name: 'MyApplicationsPage',
  setup() {
    useHead({ title: 'Отклики' });

    const api = getApi();
    const route = useRoute();
    const router = useRouter();
    const badge = useApplicationsBadgeControl();

    const tab = computed(() => parseTab(route.query.tab));
    const filter = computed(() => parseStatusQuery(route.query.status));

    const { data, error, status, refresh } = useAsyncData(
      () => `my-applications:${tab.value}:${filter.value || 'all'}`,
      () => {
        const statusEnum = statusFilterToEnum(filter.value);
        return tab.value === 'received'
          ? api.applications.listReceived(statusEnum)
          : api.applications.listSent(statusEnum);
      },
    );

    const tabs = computed(() => [
      { value: 'sent', label: 'Отправленные' },
      { value: 'received', label: 'Полученные', count: badge.count.value > 0 ? badge.count.value : undefined },
    ]);

    const setTab = (value: string) => {
      // The status filter is per-tab: switching tabs resets it.
      void router.replace({ query: { tab: value === 'received' ? 'received' : undefined } });
    };

    const setFilter = (value: StatusFilterValue) => {
      void router.replace({ query: { ...route.query, status: value || undefined } });
    };

    const onUpdated = (next: Application) => {
      data.value = patchApplication(data.value, next);
      void refresh();
    };

    const renderEmpty = () => {
      if (filter.value) {
        return (
          <EmptyState title={`Нет откликов со статусом «${statusFilterLabel(filter.value)}»`}>
            {{ actions: () => <Button variant={'secondary'} onClick={() => setFilter('')}>{'Показать все'}</Button> }}
          </EmptyState>
        );
      }
      if (tab.value === 'received') {
        return (
          <EmptyState
            title={'Откликов пока нет'}
            description={'Здесь появятся отклики на ваши публикации.'}
          >
            {{ actions: () => <Button variant={'secondary'} to={'/me/listings'}>{'Мои публикации'}</Button> }}
          </EmptyState>
        );
      }
      return (
        <EmptyState
          title={'Вы ещё не откликались'}
          description={'Найдите стартап или вакансию в ленте и откликнитесь.'}
        >
          {{ actions: () => <Button variant={'secondary'} to={'/'}>{'Перейти в ленту'}</Button> }}
        </EmptyState>
      );
    };

    const renderBody = () => {
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
        return renderEmpty();
      }
      return (
        <ApplicationList
          items={data.value}
          mode={tab.value === 'received' ? 'incoming' : 'outgoing'}
          onUpdated={onUpdated}
          onStale={() => refresh()}
        />
      );
    };

    return () => (
      <div class={styles.page}>
        <PageHeader title={'Отклики'} subtitle={'Отправленные и полученные'} />
        <Tabs
          modelValue={tab.value}
          tabs={tabs.value}
          idPrefix={TAB_PREFIX}
          ariaLabel={'Отклики'}
          onUpdate:modelValue={setTab}
        />
        <div
          class={styles.panel}
          role={'tabpanel'}
          id={`${TAB_PREFIX}-panel-${tab.value}`}
          aria-labelledby={`${TAB_PREFIX}-tab-${tab.value}`}
        >
          <StatusFilter modelValue={filter.value} onUpdate:modelValue={setFilter} />
          {renderBody()}
        </div>
      </div>
    );
  },
});
