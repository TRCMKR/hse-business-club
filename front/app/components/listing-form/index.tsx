import { computed, defineComponent, ref, watch } from 'vue';
import { useForm } from 'vee-validate';
import type { PublicPathState } from 'vee-validate';
import { toTypedSchema } from '@vee-validate/yup';
import { useEventListener } from '@vueuse/core';
import { toast } from 'vue-sonner';
import { prop } from '@fe/prop-types';
import type { ExtractProps } from '@fe/prop-types';
import { Badge } from '~/components/ui/badge';
import { Button } from '~/components/ui/button';
import { ChipsInput } from '~/components/ui/chips-input';
import { ConfirmDialog } from '~/components/ui/confirm-dialog';
import { Input } from '~/components/ui/input';
import { Segmented } from '~/components/ui/segmented';
import { Select } from '~/components/ui/select';
import { Textarea } from '~/components/ui/textarea';
import { getErrorMessage } from '~/utils/errors';
import { listingKindCreateOptions, listingKindLabel, listingKindTone, startupStageOptions } from '~/utils/labels';
import { LIMITS, listingFormSchema } from '~/utils/validation';
import { ListingKind, StartupStage } from '~~/api/types';
import { emptyListingForm } from './mapping';
import type { ListingFormState } from './mapping';
import styles from './index.module.css';

export type { ListingFormState } from './mapping';

const props = {
  /** `create` shows the kind switch; `edit` shows the kind as a locked badge. */
  mode: prop<'create' | 'edit'>().required(),
  /** Initial values (read once on mount); blank startup by default. */
  initialValues: prop<ListingFormState>().optional(),
  /**
   * Called with validated values. Should send the request and navigate away; a rejected
   * promise is shown as a toast and as a message under the form.
   */
  submit: prop<(values: ListingFormState) => Promise<void>>().required(),
  submitText: prop<string>().optional(),
  /** Where «Отмена» leads; the button is hidden when not set. */
  cancelTo: prop<string>().optional(),
};

export type ListingFormProps = ExtractProps<typeof props>;

/**
 * Create / edit listing form (SPEC §2.4): kind, title, description, «Кого ищете», tags,
 * stage (startups, required) or compensation (vacancies). Client-side validation mirrors
 * the server; errors appear after blur or the first submit. Warns before leaving with
 * unsaved changes (route change and tab close).
 */
export const ListingForm = defineComponent((props: ListingFormProps) => {
  const initial = props.initialValues ?? emptyListingForm();

  const { defineField, errors, handleSubmit, isSubmitting, meta, setFieldError, submitCount } = useForm({
    validationSchema: toTypedSchema(listingFormSchema),
    initialValues: { ...initial, tags: [...initial.tags] },
  });

  // Validate on input only once the field has been shown an error or the form was submitted.
  const lazy = (state: Pick<PublicPathState, 'errors'>) => ({
    validateOnModelUpdate: submitCount.value > 0 || state.errors.length > 0,
  });

  const [kind] = defineField('kind');
  const [title, titleAttrs] = defineField('title', lazy);
  const [description, descriptionAttrs] = defineField('description', lazy);
  const [lookingFor, lookingForAttrs] = defineField('lookingFor', lazy);
  const [tags] = defineField('tags', lazy);
  const [stage] = defineField('stage');
  const [compensation, compensationAttrs] = defineField('compensation', lazy);

  const isStartup = computed(() => kind.value === ListingKind.LISTING_KIND_STARTUP);

  // Switching the kind hides the other kind's field; its value is kept aside and
  // restored when switching back, so the hidden field never blocks validation.
  const stash = { stage: StartupStage.STARTUP_STAGE_UNSPECIFIED as StartupStage, compensation: '' };
  watch(kind, (next, prev) => {
    if (next === prev) {
      return;
    }
    if (next === ListingKind.LISTING_KIND_VACANCY) {
      stash.stage = stage.value ?? StartupStage.STARTUP_STAGE_UNSPECIFIED;
      stage.value = StartupStage.STARTUP_STAGE_UNSPECIFIED;
      compensation.value = stash.compensation;
    } else {
      stash.compensation = compensation.value ?? '';
      compensation.value = '';
      stage.value = stash.stage;
    }
    setFieldError('stage', undefined);
    setFieldError('compensation', undefined);
  });

  const formError = ref('');
  /** Set while the submit handler runs and after success, so navigation away is not blocked. */
  const leaving = ref(false);

  const onSubmit = handleSubmit(async (values) => {
    formError.value = '';
    leaving.value = true;
    try {
      await props.submit(values as ListingFormState);
    } catch (error) {
      leaving.value = false;
      const message = getErrorMessage(error);
      formError.value = message;
      toast.error(message);
    }
  });

  const onFormSubmit = (e: Event) => {
    e.preventDefault();
    // Double click / Enter while a request is in flight (or after success) is ignored.
    if (isSubmitting.value || leaving.value) {
      return;
    }
    void onSubmit(e);
  };

  // Unsaved changes guard.
  const hasUnsaved = () => meta.value.dirty && !leaving.value;
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

  const busy = computed(() => isSubmitting.value || leaving.value);
  const kindOptions = listingKindCreateOptions.map(o => ({ value: o.value as string, label: o.label }));
  const stageOptions = startupStageOptions.map(o => ({ value: o.value as string, label: o.label }));

  return () => {
    const currentKind = kind.value ?? ListingKind.LISTING_KIND_STARTUP;
    const stageValue = stage.value && stage.value !== StartupStage.STARTUP_STAGE_UNSPECIFIED ? stage.value : '';

    return (
      <form class={styles.form} onSubmit={onFormSubmit} novalidate data-testid={'listing-form'}>
        <div class={styles.kind}>
          <span class={styles.kindLabel}>{'Вид публикации'}</span>
          {props.mode === 'create'
            ? (
              <Segmented
                modelValue={currentKind}
                options={kindOptions}
                block
                ariaLabel={'Вид публикации'}
                onUpdate:modelValue={(v: string) => (kind.value = v as ListingKind)}
              />
            )
            : (
              <div class={styles.kindLocked}>
                <Badge tone={listingKindTone[currentKind]} data-testid={'listing-form-kind-badge'}>
                  {listingKindLabel[currentKind]}
                </Badge>
                <span class={styles.kindHint}>{'Вид публикации нельзя изменить'}</span>
              </div>
            )}
        </div>

        <Input
          label={'Название'}
          name={'title'}
          required
          placeholder={isStartup.value ? 'Например, сервис аренды инструментов' : 'Например, Frontend-разработчик в EdTech'}
          maxlength={LIMITS.title.max}
          modelValue={title.value ?? ''}
          onUpdate:modelValue={(v: string) => (title.value = v)}
          onBlur={titleAttrs.value.onBlur}
          error={errors.value.title}
        />

        {isStartup.value && (
          <Select
            label={'Стадия'}
            name={'stage'}
            required
            placeholder={'Выберите стадию'}
            options={stageOptions}
            modelValue={stageValue}
            onUpdate:modelValue={(v: string) => (stage.value = (v || StartupStage.STARTUP_STAGE_UNSPECIFIED) as StartupStage)}
            error={errors.value.stage}
          />
        )}

        <Textarea
          label={'Описание'}
          name={'description'}
          required
          rows={6}
          maxlength={LIMITS.description.max}
          placeholder={isStartup.value ? 'Чем занимается проект, что уже сделано, какие планы' : 'Задачи, команда, требования'}
          modelValue={description.value ?? ''}
          onUpdate:modelValue={(v: string) => (description.value = v)}
          onBlur={descriptionAttrs.value.onBlur}
          error={errors.value.description}
        />

        <Textarea
          label={'Кого ищете'}
          name={'lookingFor'}
          required
          rows={2}
          maxlength={LIMITS.lookingFor.max}
          placeholder={'Например, CTO и дизайнера на долю'}
          modelValue={lookingFor.value ?? ''}
          onUpdate:modelValue={(v: string) => (lookingFor.value = v)}
          onBlur={lookingForAttrs.value.onBlur}
          error={errors.value.lookingFor}
        />

        <ChipsInput
          label={'Теги'}
          max={LIMITS.tags.maxItems}
          maxLength={LIMITS.tags.item.max}
          placeholder={'Роли и навыки: Enter или запятая'}
          hint={`До ${LIMITS.tags.maxItems} тегов`}
          modelValue={tags.value ?? []}
          onUpdate:modelValue={(v: string[]) => (tags.value = v)}
          error={errors.value.tags}
        />

        {!isStartup.value && (
          <Input
            label={'Условия'}
            name={'compensation'}
            placeholder={'Доля, оплата, формат работы'}
            hint={`Не больше ${LIMITS.compensation.max} символов`}
            maxlength={LIMITS.compensation.max}
            modelValue={compensation.value ?? ''}
            onUpdate:modelValue={(v: string) => (compensation.value = v)}
            onBlur={compensationAttrs.value.onBlur}
            error={errors.value.compensation}
          />
        )}

        {formError.value && (
          <p class={styles.error} role={'alert'} data-testid={'listing-form-error'}>{formError.value}</p>
        )}

        <div class={styles.actions}>
          {props.cancelTo && (
            <Button variant={'secondary'} to={props.cancelTo} disabled={busy.value}>{'Отмена'}</Button>
          )}
          <Button type={'submit'} loading={busy.value} data-testid={'listing-form-submit'}>
            {props.submitText ?? (props.mode === 'create' ? 'Опубликовать' : 'Сохранить')}
          </Button>
        </div>

        <ConfirmDialog
          open={leaveOpen.value}
          title={'Уйти без сохранения?'}
          message={'Изменения в форме будут потеряны.'}
          confirmText={'Уйти'}
          cancelText={'Остаться'}
          tone={'danger'}
          onConfirm={() => answerLeave(true)}
          onCancel={() => answerLeave(false)}
        />
      </form>
    );
  };
}, { props, name: 'ListingForm' });
