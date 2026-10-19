import { defineComponent } from 'vue';
import { PageHeader } from '~/components/ui/page-header';
import { EmptyState } from '~/components/states/empty-state';

/** Stub (F0). Implemented in F3. */
export default defineComponent({
  name: 'ListingNewPage',
  setup() {
    useHead({ title: 'Новая публикация' });

    return () => (
      <div>
        <PageHeader title={'Новая публикация'} subtitle={'Стартап или вакансия'} back={'/'} />
        <EmptyState title={'Скоро здесь будет экран'} description={'Экран в разработке (F3).'} />
      </div>
    );
  },
});
