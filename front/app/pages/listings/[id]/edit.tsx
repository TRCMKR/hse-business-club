import { computed, defineComponent, onMounted, watch } from 'vue';
import { toast } from 'vue-sonner';
import { getApi } from '~~/api/client';
import { ListingStatus } from '~~/api/types';
import type { Listing } from '~~/api/types';
import { ListingForm } from '~/components/listing-form';
import type { ListingFormState } from '~/components/listing-form';
import { fromListing, toUpdateRequest } from '~/components/listing-form/mapping';
import { ErrorState } from '~/components/states/error-state';
import { PageHeader } from '~/components/ui/page-header';
import { Skeleton } from '~/components/ui/skeleton';
import { getErrorMessage, getErrorStatus } from '~/utils/errors';
import styles from '~/components/listing-form/page.module.css';

/** Only the author can edit, and only while the listing is active (SPEC §2.3). */
const canEdit = (listing: Listing) => listing.isMine && listing.status === ListingStatus.LISTING_STATUS_ACTIVE;

export default defineComponent({
  name: 'ListingEditPage',
  setup() {
    useHead({ title: 'Редактирование' });

    const api = getApi();
    const route = useRoute();
    const id = computed(() => String(route.params.id));
    const listingUrl = computed(() => `/listings/${id.value}`);

    const { data, error, status, refresh } = useAsyncData(
      () => `listing:${id.value}`,
      () => api.listings.get(id.value),
    );

    // 404 from the API → error page (same as the listing page).
    watch(error, (e) => {
      if (getErrorStatus(e) === 404) {
        showError({ statusCode: 404, statusMessage: 'Публикация не найдена', fatal: true });
      }
    }, { immediate: true });

    // Not the author or already closed → back to the listing with a toast. Done on the
    // client so the toast is visible; SSR renders the loading state meanwhile.
    onMounted(() => {
      watch(data, (listing) => {
        if (listing && !canEdit(listing)) {
          toast.error('Редактирование недоступно');
          void navigateTo(listingUrl.value, { replace: true });
        }
      }, { immediate: true });
    });

    const submit = async (values: ListingFormState) => {
      const updated = await api.listings.update(id.value, toUpdateRequest(values));
      data.value = updated;
      toast.success('Изменения сохранены');
      await navigateTo(listingUrl.value);
    };

    const renderBody = () => {
      if (error.value && getErrorStatus(error.value) !== 404) {
        return (
          <ErrorState
            message={getErrorMessage(error.value)}
            retry
            retrying={status.value === 'pending'}
            onRetry={() => refresh()}
          />
        );
      }
      const listing = data.value;
      if (!listing || !canEdit(listing)) {
        return (
          <div class={styles.loading} aria-busy={'true'} data-testid={'listing-edit-loading'}>
            <Skeleton height={40} />
            <Skeleton height={44} />
            <Skeleton height={140} />
            <Skeleton height={72} />
          </div>
        );
      }
      return (
        <ListingForm
          key={listing.id}
          mode={'edit'}
          initialValues={fromListing(listing)}
          submit={submit}
          cancelTo={listingUrl.value}
        />
      );
    };

    return () => (
      <div class={styles.page}>
        <PageHeader title={'Редактирование'} subtitle={data.value?.title ?? 'Изменение публикации'} back={listingUrl.value} />
        {renderBody()}
      </div>
    );
  },
});
