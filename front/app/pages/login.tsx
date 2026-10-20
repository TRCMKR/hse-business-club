import { computed, defineComponent, ref } from 'vue';
import { useForm } from 'vee-validate';
import { toTypedSchema } from '@vee-validate/yup';
import { NuxtLink } from '#components';
import { Button } from '~/components/ui/button';
import { Input } from '~/components/ui/input';
import { useAuthStore } from '~/stores/auth';
import { getErrorMessage } from '~/utils/errors';
import { safeRedirect } from '~/utils/redirect';
import { loginFormSchema } from '~/utils/validation';
import styles from './auth.module.css';

/** Seed accounts (SPEC §6); password is the same for all. */
const DEMO_ACCOUNTS = [
  { username: 'anna', name: 'Анна' },
  { username: 'boris', name: 'Борис' },
  { username: 'vera', name: 'Вера' },
  { username: 'dmitry', name: 'Дмитрий' },
] as const;

const DEMO_PASSWORD = 'demo1234';

export default defineComponent({
  name: 'LoginPage',
  setup() {
    definePageMeta({ layout: 'auth' });
    useHead({ title: 'Вход' });

    const auth = useAuthStore();
    const route = useRoute();
    const formError = ref('');
    const demoLoading = ref<string | null>(null);

    const { defineField, errors, handleSubmit, isSubmitting } = useForm({
      validationSchema: toTypedSchema(loginFormSchema),
      initialValues: { username: '', password: '' },
    });
    const [username] = defineField('username');
    const [password] = defineField('password');

    const busy = computed(() => isSubmitting.value || demoLoading.value !== null);

    const signIn = async (input: { username: string, password: string }) => {
      formError.value = '';
      try {
        await auth.login(input);
        await navigateTo(safeRedirect(route.query.redirect));
      } catch (error) {
        formError.value = getErrorMessage(error);
      }
    };

    const onSubmit = handleSubmit(values => signIn(values));

    // Double click / Enter while a sign-in is in flight is ignored.
    const onFormSubmit = (e: Event) => {
      e.preventDefault();
      if (busy.value) {
        return;
      }
      void onSubmit(e);
    };

    const loginAs = async (login: string) => {
      if (busy.value) {
        return;
      }
      demoLoading.value = login;
      try {
        await signIn({ username: login, password: DEMO_PASSWORD });
      } finally {
        demoLoading.value = null;
      }
    };

    return () => (
      <div class={styles.page}>
        <h1 class={styles.title}>{'Вход'}</h1>
        <p class={styles.lead}>{'Публикуйте стартапы и вакансии, находите команду в клубе.'}</p>

        <form class={styles.form} onSubmit={onFormSubmit} novalidate>
          <Input
            label={'Логин'}
            name={'username'}
            autocomplete={'username'}
            modelValue={username.value ?? ''}
            onUpdate:modelValue={(value: string) => (username.value = value)}
            error={errors.value.username}
          />
          <Input
            label={'Пароль'}
            name={'password'}
            type={'password'}
            autocomplete={'current-password'}
            modelValue={password.value ?? ''}
            onUpdate:modelValue={(value: string) => (password.value = value)}
            error={errors.value.password}
          />
          {formError.value && <p class={styles.error} role={'alert'}>{formError.value}</p>}
          <Button type={'submit'} block loading={isSubmitting.value} disabled={busy.value}>{'Войти'}</Button>
        </form>

        <div class={styles.divider}><span>{'или демо-аккаунт'}</span></div>

        <div class={styles.demo}>
          {DEMO_ACCOUNTS.map(account => (
            <Button
              key={account.username}
              variant={'secondary'}
              size={'sm'}
              loading={demoLoading.value === account.username}
              disabled={busy.value}
              onClick={() => loginAs(account.username)}
            >
              {`Войти как ${account.name}`}
            </Button>
          ))}
        </div>

        <p class={styles.footer}>
          {'Нет аккаунта? '}
          <NuxtLink to={{ path: '/register', query: route.query.redirect ? { redirect: route.query.redirect } : {} }}>
            {'Зарегистрироваться'}
          </NuxtLink>
        </p>
      </div>
    );
  },
});
