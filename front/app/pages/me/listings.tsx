import { defineComponent } from 'vue';
import { PageHeader } from '~/components/ui/page-header';
import { EmptyState } from '~/components/states/empty-state';

/** Stub (F0). Implemented in F3. */
export default defineComponent({
  name: 'MyListingsPage',
  setup() {
    useHead({ title: 'Мои публикации' });

    return () => (
      <div>
        <PageHeader title={'Мои публикации'} subtitle={'Активные и закрытые'} />
        <EmptyState title={'Скоро здесь будет экран'} description={'Экран в разработке (F3).'} />
      </div>
    );
  },
});
