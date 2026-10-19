import { computed, defineComponent, ref } from 'vue';
import { useRouter } from 'vue-router';
import { Button } from '~/components/ui/button';
import { Icon } from '~/components/ui/icon';
import { ErrorState } from '~/components/states/error-state';
import { ApplySheet } from '~/components/listing/apply-sheet';
import { ListingActionBar } from '~/components/listing/listing-action-bar';
import { ListingBody } from '~/components/listing/listing-body';
import { ListingHeader } from '~/components/listing/listing-header';
import { ListingSkeleton } from '~/components/listing/listing-skeleton';
import { MyApplicationBlock } from '~/components/listing/my-application-block';
import { OwnerPanel } from '~/components/listing/owner-panel';
import { resolveActions } from '~/components/listing/resolve-actions';
import { getErrorMessage, getErrorStatus } from '~/utils/errors';
import { getApi } from '~~/api/client';
import styles from './index.module.css';

/** Listing details (SPEC §7): content, apply sheet, my application, author panel. */
export default defineComponent({
  name: 'ListingPage',
  setup() {
    definePageMeta({ hideCreateFab: true });

    const api = getApi();
    const route = useRoute();
    const router = useRouter();
    const event = import.meta.server ? useRequestEvent() : undefined;
    const id = computed(() => String(route.params.id));

    const { data, error, status, refresh } = useAsyncData(
      () => `listing:${id.value}`,
      () => api.listings.get(id.value),
    );

    const notFound = computed(() => getErrorStatus(error.value) === 404);
    const actions = computed(() => (data.value ? resolveActions(data.value) : null));
    const applyOpen = ref(false);

    useHead({
      title: computed(() => data.value?.title ?? (notFound.value ? 'Публикация не найдена' : 'Публикация')),
    });

    const goBack = () => {
      if (typeof window !== 'undefined' && window.history.state?.back) {
        router.back();
      } else {
        router.push('/');
      }
    };

    // State changed under us (closed meanwhile, already applied in another tab, decided
    // by the author): reload so the page shows the actual actions.
    const refreshAfterFailure = (e: unknown) => {
      const code = getErrorStatus(e);
      if (code !== undefined && code >= 400 && code < 500) {
        refresh();
      }
    };

    const submitApplication = (message: string) => api.applications.create(id.value, message);
    const closeListing = () => api.listings.close(id.value);

    const renderContent = () => {
      const listing = data.value!;
      const a = actions.value!;
      return (
        <div class={styles.layout}>
          <div class={styles.main}>
            <div class={styles.header}><ListingHeader listing={listing} /></div>
            <div class={styles.body}><ListingBody listing={listing} /></div>
          </div>
          <aside class={styles.aside} aria-label={'Действия'}>
            {a.ownerPanel && (
              <div class={styles.panel}>
                <OwnerPanel
                  listingId={listing.id}
                  stats={listing.stats}
                  canEdit={a.canEdit}
                  canClose={a.canClose}
                  close={closeListing}
                  onClosed={() => refresh()}
                  onFailed={refreshAfterFailure}
                />
              </div>
            )}
            {a.myApplication && listing.myApplication && (
              <div class={styles.panel}>
                <MyApplicationBlock
                  application={listing.myApplication}
                  author={listing.author}
                  canWithdraw={a.canWithdraw}
                  withdraw={() => api.applications.withdraw(listing.myApplication!.id)}
                  onWithdrawn={() => refresh()}
                  onFailed={refreshAfterFailure}
                />
              </div>
            )}
            <div class={styles.actionBar}>
              <ListingActionBar apply={a.apply} onApply={() => (applyOpen.value = true)} />
            </div>
          </aside>
          <ApplySheet
            open={applyOpen.value}
            listingTitle={listing.title}
            submit={submitApplication}
            onUpdate:open={(v: boolean) => (applyOpen.value = v)}
            onSuccess={() => refresh()}
            onFailed={refreshAfterFailure}
          />
        </div>
      );
    };

    const renderState = () => {
      if (data.value) {
        return renderContent();
      }
      if (notFound.value) {
        if (event) {
          setResponseStatus(event, 404);
        }
        return (
          <ErrorState title={'Публикация не найдена'} message={'Возможно, ссылка устарела или публикацию удалили.'}>
            {{ actions: () => <Button to={'/'} variant={'secondary'}>{'В ленту'}</Button> }}
          </ErrorState>
        );
      }
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
      return <ListingSkeleton />;
    };

    return () => (
      <div class={styles.page}>
        <button type={'button'} class={styles.back} onClick={goBack}>
          <Icon name={'chevron-left'} size={20} />
          <span>{'Назад'}</span>
        </button>
        {renderState()}
      </div>
    );
  },
});
