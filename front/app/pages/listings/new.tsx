import { defineComponent } from 'vue';
import { toast } from 'vue-sonner';
import { getApi } from '~~/api/client';
import { ListingForm } from '~/components/listing-form';
import type { ListingFormState } from '~/components/listing-form';
import { emptyListingForm, kindFromQuery, toCreateRequest } from '~/components/listing-form/mapping';
import { PageHeader } from '~/components/ui/page-header';
import styles from '~/components/listing-form/page.module.css';

export default defineComponent({
  name: 'ListingNewPage',
  setup() {
    useHead({ title: 'Новая публикация' });

    const api = getApi();
    const route = useRoute();
    const initialValues = emptyListingForm(kindFromQuery(route.query.kind));

    const submit = async (values: ListingFormState) => {
      const listing = await api.listings.create(toCreateRequest(values));
      toast.success('Публикация создана');
      await navigateTo(`/listings/${listing.id}`, { replace: true });
    };

    return () => (
      <div class={styles.page}>
        <PageHeader title={'Новая публикация'} subtitle={'Стартап или вакансия'} back={'/'} />
        <ListingForm mode={'create'} initialValues={initialValues} submit={submit} cancelTo={'/'} />
      </div>
    );
  },
});
