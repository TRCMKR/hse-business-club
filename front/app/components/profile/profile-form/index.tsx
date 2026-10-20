import { defineComponent, ref } from 'vue';
import { useForm } from 'vee-validate';
import { useEventListener } from '@vueuse/core';
import { toTypedSchema } from '@vee-validate/yup';
import { toast } from 'vue-sonner';
import { prop } from '@fe/prop-types';
import type { ExtractProps } from '@fe/prop-types';
import { getApi } from '~~/api/client';
import type { User } from '~~/api/types';
import { Button } from '~/components/ui/button';
import { Card } from '~/components/ui/card';
import { ChipsInput } from '~/components/ui/chips-input';
import { ConfirmDialog } from '~/components/ui/confirm-dialog';
import { Input } from '~/components/ui/input';
import { Textarea } from '~/components/ui/textarea';
import { useAuthStore } from '~/stores/auth';
import { getErrorMessage } from '~/utils/errors';
import { LIMITS, profileFormSchema } from '~/utils/validation';
import styles from './index.module.css';

const props = {
  /** Current values; the form is initialised from them once. */
  user: prop<User>().required(),
};

export type ProfileFormProps = ExtractProps<typeof props>;

/**
 * Inline profile editor (PUT /v1/me — full replacement of editable fields).
 * On success writes the returned user to `auth.setUser` and emits `saved(user)`;
 * «Отмена» emits `cancel`. Server errors (400) are shown above the buttons.
 */
export const ProfileForm = defineComponent((props: ProfileFormProps, { emit }) => {
  const api = getApi();
  const auth = useAuthStore();
  const formError = ref('');

  const { defineField, errors, handleSubmit, isSubmitting, meta } = useForm({
    validationSchema: toTypedSchema(profileFormSchema),
    initialValues: {
      displayName: props.user.displayName,
      headline: props.user.headline,
      about: props.user.about,
      skills: [...props.user.skills],
      telegram: props.user.telegram,
    },
  });
  const [displayName] = defineField('displayName');
  const [headline] = defineField('headline');
  const [about] = defineField('about');
  const [skills] = defineField('skills');
  const [telegram] = defineField('telegram');

  const onSubmit = handleSubmit(async (values) => {
    formError.value = '';
    try {
      const user = await api.users.updateMe({
        displayName: values.displayName,
        headline: values.headline,
        about: values.about,
        skills: values.skills,
        telegram: values.telegram,
      });
      saved.value = true;
      auth.setUser(user);
      toast.success('Профиль сохранён');
      emit('saved', user);
    } catch (error) {
      formError.value = getErrorMessage(error);
    }
  });

  // Unsaved changes guard (same as the listing form).
  const saved = ref(false);
  const hasUnsaved = () => meta.value.dirty && !saved.value;
  const leaveOpen = ref(false);
  let resolveLeave: ((ok: boolean) => void) | null = null;
  const answerLeave = (ok: boolean) => {
    leaveOpen.value = false;
    resolveLeave?.(ok);
    resolveLeave = null;
  };

  onBeforeRouteLeave(() => {
    if (!hasUnsaved()) {
      return true;
    }
    resolveLeave?.(false);
    leaveOpen.value = true;
    return new Promise<boolean>((resolve) => {
      resolveLeave = resolve;
    });
  });

  if (import.meta.client) {
    useEventListener(window, 'beforeunload', (e: BeforeUnloadEvent) => {
      if (hasUnsaved()) {
        e.preventDefault();
        e.returnValue = '';
      }
    });
  }

  // Double click / Enter while the request is in flight is ignored.
  const onFormSubmit = (e: Event) => {
    e.preventDefault();
    if (isSubmitting.value) {
      return;
    }
    void onSubmit(e);
  };

  return () => (
    <Card>
      <form class={styles.form} onSubmit={onFormSubmit} novalidate data-testid={'profile-form'}>
        <h2 class={styles.title}>{'Редактирование профиля'}</h2>
        <Input
          label={'Имя'}
          name={'displayName'}
          autocomplete={'name'}
          required
          maxlength={LIMITS.displayName.max}
          modelValue={displayName.value ?? ''}
          onUpdate:modelValue={(value: string) => (displayName.value = value)}
          error={errors.value.displayName}
        />
        <Input
          label={'Кратко о себе'}
          name={'headline'}
          placeholder={'Например: продакт-менеджер, ищу команду для стартапа'}
          maxlength={LIMITS.headline.max}
          modelValue={headline.value ?? ''}
          onUpdate:modelValue={(value: string) => (headline.value = value)}
          error={errors.value.headline}
        />
        <Textarea
          label={'О себе'}
          name={'about'}
          rows={5}
          maxlength={LIMITS.about.max}
          placeholder={'Опыт, интересы, чем можете помочь команде'}
          modelValue={about.value ?? ''}
          onUpdate:modelValue={(value: string) => (about.value = value)}
          error={errors.value.about}
        />
        <ChipsInput
          label={'Навыки'}
          max={LIMITS.skills.maxItems}
          maxLength={LIMITS.skills.item.max}
          placeholder={'Введите навык и нажмите Enter'}
          hint={`До ${LIMITS.skills.maxItems} навыков`}
          modelValue={skills.value ?? []}
          onUpdate:modelValue={(value: string[]) => (skills.value = value)}
          error={errors.value.skills}
        />
        <Input
          label={'Telegram'}
          name={'telegram'}
          placeholder={'@username'}
          autocomplete={'off'}
          maxlength={LIMITS.telegram.max + 1}
          modelValue={telegram.value ?? ''}
          onUpdate:modelValue={(value: string) => (telegram.value = value)}
          error={errors.value.telegram}
        />
        {formError.value && <p class={styles.error} role={'alert'}>{formError.value}</p>}
        <div class={styles.buttons}>
          <Button type={'submit'} loading={isSubmitting.value}>{'Сохранить'}</Button>
          <Button variant={'secondary'} disabled={isSubmitting.value} onClick={() => emit('cancel')}>{'Отмена'}</Button>
        </div>
        <ConfirmDialog
          open={leaveOpen.value}
          title={'Уйти без сохранения?'}
          message={'Изменения профиля будут потеряны.'}
          confirmText={'Уйти'}
          cancelText={'Остаться'}
          tone={'danger'}
          onConfirm={() => answerLeave(true)}
          onCancel={() => answerLeave(false)}
        />
      </form>
    </Card>
  );
}, {
  props,
  name: 'ProfileForm',
  emits: {
    saved: (_user: User) => true,
    cancel: () => true,
  },
});
