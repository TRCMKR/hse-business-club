import { computed, defineComponent, ref } from 'vue';
import { toast } from 'vue-sonner';
import { getApi } from '~~/api/client';
import type { Listing } from '~~/api/types';
import { MyListingItem } from '~/components/my-listings/my-listing-item';
import { MyListingsSkeleton } from '~/components/my-listings/my-listings-skeleton';
import {
  buildMyListingsTabs,
  listingsForTab,
  myListingsTabQuery,
  parseMyListingsTab,
  replaceListing,
} from '~/components/my-listings/tabs';
import type { MyListingsTab } from '~/components/my-listings/tabs';
import { EmptyState } from '~/components/states/empty-state';
import { ErrorState } from '~/components/states/error-state';
import { Button } from '~/components/ui/button';
import { ConfirmDialog } from '~/components/ui/confirm-dialog';
import { PageHeader } from '~/components/ui/page-header';
import { Tabs } from '~/components/ui/tabs';
import { getErrorMessage } from '~/utils/errors';
import styles from '~/components/my-listings/page.module.css';

const TAB_EMPTY: Record<MyListingsTab, { title: string, description: string }> = {
  active: {
    title: 'Нет активных публикаций',
    description: 'Опубликуйте стартап или вакансию — участники клуба смогут откликнуться.',
  },
  closed: {
    title: 'Нет закрытых публикаций',
    description: 'Сюда попадут публикации, которые вы закроете.',
  },
};

export default defineComponent({
  name: 'MyListingsPage',
  setup() {
    useHead({ title: 'Мои публикации' });

    const api = getApi();
    const route = useRoute();

    // One request for both tabs: counters stay correct and a closed listing just moves.
    const { data, error, status, refresh } = useAsyncData('my-listings', () => api.listings.listMine());

    const tab = computed(() => parseMyListingsTab(route.query.status));
    const listings = computed(() => data.value ?? []);
    const tabs = computed(() => buildMyListingsTabs(listings.value));
    const visible = computed(() => listingsForTab(listings.value, tab.value));

    const setTab = (next: string) => {
      const status = myListingsTabQuery(next as MyListingsTab);
      void navigateTo({ query: { ...route.query, status } }, { replace: true });
    };

    // Close with confirmation.
    const toClose = ref<Listing | null>(null);
    const closing = ref(false);
    const confirmClose = async () => {
      const target = toClose.value;
      if (!target || closing.value) {
        return;
      }
      closing.value = true;
      try {
        const updated = await api.listings.close(target.id);
        data.value = replaceListing(listings.value, updated);
        toClose.value = null;
        toast.success('Публикация закрыта', { description: 'Она во вкладке «Закрытые».' });
      } catch (e) {
        toast.error(getErrorMessage(e));
      } finally {
        closing.value = false;
      }
    };

    const createButton = () => <Button to={'/listings/new'} icon={'plus'}>{'Создать публикацию'}</Button>;

    const renderBody = () => {
      if (error.value) {
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
        return <MyListingsSkeleton />;
      }
      if (listings.value.length === 0) {
        return (
          <EmptyState
            title={'У вас нет публикаций'}
            description={'Расскажите о стартапе или вакансии — участники клуба смогут откликнуться.'}
            data-testid={'my-listings-empty'}
          >
            {{ actions: createButton }}
          </EmptyState>
        );
      }
      return (
        <>
          <Tabs
            modelValue={tab.value}
            tabs={tabs.value}
            idPrefix={'my-listings'}
            ariaLabel={'Статус публикаций'}
            onUpdate:modelValue={setTab}
          />
          <div
            id={`my-listings-panel-${tab.value}`}
            role={'tabpanel'}
            aria-labelledby={`my-listings-tab-${tab.value}`}
            class={styles.panel}
          >
            {visible.value.length === 0
              ? (
                <EmptyState title={TAB_EMPTY[tab.value].title} description={TAB_EMPTY[tab.value].description}>
                  {tab.value === 'active' ? { actions: createButton } : {}}
                </EmptyState>
              )
              : visible.value.map(l => (
                <MyListingItem
                  key={l.id}
                  listing={l}
                  closing={closing.value && toClose.value?.id === l.id}
                  onClose={(listing: Listing) => (toClose.value = listing)}
                />
              ))}
          </div>
        </>
      );
    };

    return () => (
      <div class={styles.page}>
        <PageHeader title={'Мои публикации'} subtitle={'Активные и закрытые'} />
        {renderBody()}
        <ConfirmDialog
          open={toClose.value !== null}
          title={'Закрыть публикацию?'}
          message={toClose.value
            ? `«${toClose.value.title}» пропадёт из ленты и перестанет принимать отклики. Открыть её снова или отредактировать будет нельзя.`
            : undefined}
          confirmText={'Закрыть'}
          tone={'danger'}
          loading={closing.value}
          onConfirm={confirmClose}
          onCancel={() => (toClose.value = null)}
        />
      </div>
    );
  },
});
