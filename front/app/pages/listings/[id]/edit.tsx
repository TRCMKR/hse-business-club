import { defineComponent } from 'vue';
import { PageHeader } from '~/components/ui/page-header';
import { EmptyState } from '~/components/states/empty-state';

/** Stub (F0). Implemented in F3. */
export default defineComponent({
  name: 'ListingEditPage',
  setup() {
    useHead({ title: 'Редактирование' });

    return () => (
      <div>
        <PageHeader title={'Редактирование'} subtitle={'Изменение публикации'} back={true} />
        <EmptyState title={'Скоро здесь будет экран'} description={'Экран в разработке (F3).'} />
      </div>
    );
  },
});
