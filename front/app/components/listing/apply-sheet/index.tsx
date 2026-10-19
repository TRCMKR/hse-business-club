import { defineComponent, ref, watch } from 'vue';
import { toast } from 'vue-sonner';
import { prop } from '@fe/prop-types';
import type { ExtractProps } from '@fe/prop-types';
import { Button } from '~/components/ui/button';
import { ResponsiveModal } from '~/components/ui/responsive-modal';
import { Textarea } from '~/components/ui/textarea';
import { getErrorMessage } from '~/utils/errors';
import { LIMITS, messageSchema } from '~/utils/validation';
import styles from './index.module.css';

export const APPLY_SUCCESS_MESSAGE = 'Отклик отправлен';

const props = {
  open: prop<boolean>().required(),
  /** Shown under the title so the user sees what they apply to. */
  listingTitle: prop<string>().optional(),
  /**
   * Sends the application (`api.applications.create(listingId, message)`); receives the
   * trimmed message. A rejection is shown under the field and as a toast.
   */
  submit: prop<(message: string) => Promise<unknown>>().required(),
};

export type ApplySheetProps = ExtractProps<typeof props>;

/** Client-side check with the same rules and message as the server (SPEC §2.4). */
function validateMessage(value: string): string | null {
  try {
    messageSchema.validateSync(value);
    return null;
  } catch (e) {
    return getErrorMessage(e);
  }
}

/**
 * «Откликнуться» form: bottom sheet below 768px, centered dialog otherwise
 * (`ResponsiveModal`). Textarea 1–2000 with a counter; «Отправить» shows `loading` and
 * ignores repeated clicks. Success → toast «Отклик отправлен», `success`, close, draft
 * cleared. Server error (400/409) → message under the field + toast, `failed(error)`.
 */
export const ApplySheet = defineComponent((props: ApplySheetProps, { emit }) => {
  const message = ref('');
  const error = ref<string | null>(null);
  const loading = ref(false);

  watch(() => props.open, (open) => {
    if (open) {
      error.value = null;
    }
  });

  const close = () => {
    if (loading.value) {
      return;
    }
    emit('update:open', false);
  };

  const send = async () => {
    if (loading.value) {
      return;
    }
    const invalid = validateMessage(message.value);
    if (invalid) {
      error.value = invalid;
      return;
    }
    loading.value = true;
    error.value = null;
    try {
      await props.submit(message.value.trim());
      message.value = '';
      toast.success(APPLY_SUCCESS_MESSAGE);
      emit('success');
      emit('update:open', false);
    } catch (e) {
      const text = getErrorMessage(e);
      error.value = text;
      toast.error(text);
      emit('failed', e);
    } finally {
      loading.value = false;
    }
  };

  const onInput = (value: string) => {
    message.value = value;
    if (error.value) {
      error.value = null;
    }
  };

  return () => (
    <ResponsiveModal
      open={props.open}
      title={'Отклик'}
      size={'md'}
      closable={!loading.value}
      onUpdate:open={(v: boolean) => (v ? emit('update:open', true) : close())}
    >
      {{
        default: () => (
          <div class={styles.body} data-testid={'apply-sheet'}>
            {props.listingTitle && <p class={styles.listing}>{props.listingTitle}</p>}
            <Textarea
              label={'Сопроводительное сообщение'}
              placeholder={'Расскажите, чем вы можете помочь проекту и как с вами связаться'}
              modelValue={message.value}
              onUpdate:modelValue={onInput}
              maxlength={LIMITS.message.max}
              rows={5}
              required
              disabled={loading.value}
              error={error.value ?? undefined}
              name={'message'}
            />
          </div>
        ),
        footer: () => (
          <>
            <Button variant={'secondary'} disabled={loading.value} onClick={close}>{'Отмена'}</Button>
            <Button icon={'send'} loading={loading.value} onClick={send} data-testid={'apply-submit'}>
              {'Отправить'}
            </Button>
          </>
        ),
      }}
    </ResponsiveModal>
  );
}, {
  props,
  name: 'ApplySheet',
  emits: {
    'update:open': (_v: boolean) => true,
    'success': () => true,
    'failed': (_e: unknown) => true,
  },
});
