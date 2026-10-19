import { defineComponent } from 'vue';
import { PageHeader } from '~/components/ui/page-header';
import { EmptyState } from '~/components/states/empty-state';

/** Stub (F0). Implemented in F5. */
export default defineComponent({
  name: 'UserPage',
  setup() {
    useHead({ title: 'Профиль участника' });

    return () => (
      <div>
        <PageHeader title={'Профиль участника'} back={true} />
        <EmptyState title={'Скоро здесь будет экран'} description={'Экран в разработке (F5).'} />
      </div>
    );
  },
});
