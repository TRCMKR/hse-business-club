import { defineComponent, ref } from 'vue';
import { PageHeader } from '~/components/ui/page-header';
import { Button } from '~/components/ui/button';
import { AvatarEditor } from '~/components/profile/avatar-editor';
import { ProfileCard } from '~/components/profile/profile-card';
import { ProfileForm } from '~/components/profile/profile-form';
import { useAuthStore } from '~/stores/auth';

/** Own profile: view, inline edit (PUT /v1/me), photo upload/delete, logout. */
export default defineComponent({
  name: 'MePage',
  setup() {
    useHead({ title: 'Профиль' });
    const auth = useAuthStore();
    const editing = ref(false);
    const loggingOut = ref(false);

    const onLogout = async () => {
      loggingOut.value = true;
      try {
        await auth.logout();
        await navigateTo('/login');
      } finally {
        loggingOut.value = false;
      }
    };

    return () => {
      const user = auth.user;
      return (
        <div>
          <PageHeader title={'Профиль'} subtitle={'Ваши данные в клубе'}>
            {{
              actions: () => (
                <Button variant={'secondary'} size={'sm'} icon={'logout'} loading={loggingOut.value} onClick={onLogout}>
                  {'Выйти'}
                </Button>
              ),
            }}
          </PageHeader>
          {user && (editing.value
            ? (
              <ProfileForm
                user={user}
                onSaved={() => (editing.value = false)}
                onCancel={() => (editing.value = false)}
              />
            )
            : (
              <ProfileCard user={user}>
                {{
                  avatar: () => <AvatarEditor user={user} />,
                  actions: () => (
                    <Button variant={'secondary'} size={'sm'} icon={'edit'} onClick={() => (editing.value = true)}>
                      {'Редактировать'}
                    </Button>
                  ),
                }}
              </ProfileCard>
            ))}
        </div>
      );
    };
  },
});
