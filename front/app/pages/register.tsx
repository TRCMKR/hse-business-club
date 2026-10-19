import { defineComponent, ref } from 'vue';
import { useForm } from 'vee-validate';
import { toTypedSchema } from '@vee-validate/yup';
import { NuxtLink } from '#components';
import { Button } from '~/components/ui/button';
import { Input } from '~/components/ui/input';
import { useAuthStore } from '~/stores/auth';
import { getErrorMessage } from '~/utils/errors';
import { safeRedirect } from '~/utils/redirect';
import { registerFormSchema } from '~/utils/validation';
import styles from './auth.module.css';

export default defineComponent({
  name: 'RegisterPage',
  setup() {
    definePageMeta({ layout: 'auth' });
    useHead({ title: 'Регистрация' });

    const auth = useAuthStore();
    const route = useRoute();
    const formError = ref('');

    const { defineField, errors, handleSubmit, isSubmitting } = useForm({
      validationSchema: toTypedSchema(registerFormSchema),
      initialValues: { username: '', password: '', passwordConfirm: '', displayName: '' },
    });
    const [username] = defineField('username');
    const [displayName] = defineField('displayName');
    const [password] = defineField('password');
    const [passwordConfirm] = defineField('passwordConfirm');

    const onSubmit = handleSubmit(async (values) => {
      formError.value = '';
      try {
        await auth.register({ username: values.username, password: values.password, displayName: values.displayName });
        await navigateTo(safeRedirect(route.query.redirect));
      } catch (error) {
        formError.value = getErrorMessage(error);
      }
    });

    // Double click / Enter while the request is in flight is ignored.
    const onFormSubmit = (e: Event) => {
      e.preventDefault();
      if (isSubmitting.value) {
        return;
      }
      void onSubmit(e);
    };

    return () => (
      <div class={styles.page}>
        <h1 class={styles.title}>{'Регистрация'}</h1>
        <p class={styles.lead}>{'Создайте аккаунт участника клуба.'}</p>

        <form class={styles.form} onSubmit={onFormSubmit} novalidate>
          <Input
            label={'Логин'}
            name={'username'}
            autocomplete={'username'}
            hint={'3–32 символа: латиница в нижнем регистре, цифры, _'}
            modelValue={username.value ?? ''}
            onUpdate:modelValue={(value: string) => (username.value = value)}
            error={errors.value.username}
          />
          <Input
            label={'Имя'}
            name={'displayName'}
            autocomplete={'name'}
            modelValue={displayName.value ?? ''}
            onUpdate:modelValue={(value: string) => (displayName.value = value)}
            error={errors.value.displayName}
          />
          <Input
            label={'Пароль'}
            name={'password'}
            type={'password'}
            autocomplete={'new-password'}
            hint={'От 8 до 72 символов'}
            modelValue={password.value ?? ''}
            onUpdate:modelValue={(value: string) => (password.value = value)}
            error={errors.value.password}
          />
          <Input
            label={'Повторите пароль'}
            name={'passwordConfirm'}
            type={'password'}
            autocomplete={'new-password'}
            modelValue={passwordConfirm.value ?? ''}
            onUpdate:modelValue={(value: string) => (passwordConfirm.value = value)}
            error={errors.value.passwordConfirm}
          />
          {formError.value && <p class={styles.error} role={'alert'}>{formError.value}</p>}
          <Button type={'submit'} block loading={isSubmitting.value}>{'Зарегистрироваться'}</Button>
        </form>

        <p class={styles.footer}>
          {'Уже есть аккаунт? '}
          <NuxtLink to={{ path: '/login', query: route.query.redirect ? { redirect: route.query.redirect } : {} }}>
            {'Войти'}
          </NuxtLink>
        </p>
      </div>
    );
  },
});
