import { defineComponent } from 'vue';
import { PageHeader } from '~/components/ui/page-header';
import { EmptyState } from '~/components/states/empty-state';

/** Stub (F0). Implemented in F2. */
export default defineComponent({
  name: 'ListingPage',
  setup() {
    useHead({ title: 'Публикация' });

    return () => (
      <div>
        <PageHeader title={'Публикация'} subtitle={'Подробности'} back={true} />
        <EmptyState title={'Скоро здесь будет экран'} description={'Экран в разработке (F2).'} />
      </div>
    );
  },
});
