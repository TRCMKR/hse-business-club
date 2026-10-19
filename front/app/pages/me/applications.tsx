import { defineComponent } from 'vue';
import { PageHeader } from '~/components/ui/page-header';
import { EmptyState } from '~/components/states/empty-state';

/** Stub (F0). Implemented in F4. */
export default defineComponent({
  name: 'MyApplicationsPage',
  setup() {
    useHead({ title: 'Отклики' });

    return () => (
      <div>
        <PageHeader title={'Отклики'} subtitle={'Отправленные и полученные'} />
        <EmptyState title={'Скоро здесь будет экран'} description={'Экран в разработке (F4).'} />
      </div>
    );
  },
});
