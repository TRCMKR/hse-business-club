import { defineComponent } from 'vue';
import { PageHeader } from '~/components/ui/page-header';
import { EmptyState } from '~/components/states/empty-state';

/** Stub (F0). Implemented in F1. */
export default defineComponent({
  name: 'FeedPage',
  setup() {
    useHead({ title: 'Лента' });

    return () => (
      <div>
        <PageHeader title={'Лента'} subtitle={'Стартапы и вакансии участников клуба'} />
        <EmptyState title={'Скоро здесь будет экран'} description={'Экран в разработке (F1).'} />
      </div>
    );
  },
});
