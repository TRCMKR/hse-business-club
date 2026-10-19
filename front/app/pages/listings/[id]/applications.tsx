import { defineComponent } from 'vue';
import { PageHeader } from '~/components/ui/page-header';
import { EmptyState } from '~/components/states/empty-state';

/** Stub (F0). Implemented in F4. */
export default defineComponent({
  name: 'ListingApplicationsPage',
  setup() {
    useHead({ title: 'Отклики на публикацию' });

    return () => (
      <div>
        <PageHeader title={'Отклики на публикацию'} subtitle={'Принять или отклонить'} back={true} />
        <EmptyState title={'Скоро здесь будет экран'} description={'Экран в разработке (F4).'} />
      </div>
    );
  },
});
