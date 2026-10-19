import { defineComponent } from 'vue';
import { PageHeader } from '~/components/ui/page-header';
import { Button } from '~/components/ui/button';
import { Card } from '~/components/ui/card';
import { UserSummary } from '~/components/cards/user-summary';
import { EmptyState } from '~/components/states/empty-state';
import { useAuthStore } from '~/stores/auth';
import styles from './index.module.css';

/** Stub (F0): shows the current user and «Выйти». Profile view/edit and photo — F5. */
export default defineComponent({
  name: 'MePage',
  setup() {
    useHead({ title: 'Профиль' });
    const auth = useAuthStore();

    const onLogout = async () => {
      await auth.logout();
      await navigateTo('/login');
    };

    return () => (
      <div>
        <PageHeader title={'Профиль'} subtitle={'Ваши данные в клубе'}>
          {{
            actions: () => (
              <Button variant={'secondary'} size={'sm'} onClick={onLogout}>{'Выйти'}</Button>
            ),
          }}
        </PageHeader>
        {auth.user && (
          <Card class={styles.card}>
            <UserSummary user={auth.user} size={'lg'} />
          </Card>
        )}
        <EmptyState title={'Скоро здесь будет экран'} description={'Редактирование профиля и фото — в разработке (F5).'} />
      </div>
    );
  },
});
