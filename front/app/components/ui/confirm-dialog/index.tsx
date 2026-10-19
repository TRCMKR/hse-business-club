import { defineComponent } from 'vue';
import { prop } from '@fe/prop-types';
import type { ExtractProps } from '@fe/prop-types';
import { Button } from '../button';
import { Modal } from '../modal';
import styles from './index.module.css';

const props = {
  open: prop<boolean>().required(),
  title: prop<string>().required(),
  message: prop<string>().optional(),
  confirmText: prop<string>().optional('Подтвердить'),
  cancelText: prop<string>().optional('Отмена'),
  tone: prop<'primary' | 'danger'>().optional('primary'),
  /** Shows a spinner on the confirm button and blocks closing. */
  loading: prop<boolean>().optional(false),
};

export type ConfirmDialogProps = ExtractProps<typeof props>;

/**
 * Confirmation dialog. Emits `confirm` (the parent closes it after the action
 * succeeds) and `cancel` + `update:open(false)` on cancel / Esc / overlay.
 * Slot default replaces `message`.
 */
export const ConfirmDialog = defineComponent((props: ConfirmDialogProps, { slots, emit }) => {
  const cancel = () => {
    if (props.loading) {
      return;
    }
    emit('cancel');
    emit('update:open', false);
  };

  return () => (
    <Modal
      open={props.open}
      title={props.title}
      size={'sm'}
      variant={'dialog'}
      closable={!props.loading}
      hideClose={true}
      onClose={cancel}
    >
      {{
        default: () => (slots.default
          ? slots.default()
          : props.message && <p class={styles.message}>{props.message}</p>),
        footer: () => (
          <>
            {/* Initial focus: never on a destructive action. */}
            <Button
              variant={'secondary'}
              disabled={props.loading}
              data-autofocus={props.tone === 'danger' ? '' : undefined}
              onClick={cancel}
            >
              {props.cancelText}
            </Button>
            <Button
              variant={props.tone}
              loading={props.loading}
              data-autofocus={props.tone === 'danger' ? undefined : ''}
              onClick={() => emit('confirm')}
            >
              {props.confirmText}
            </Button>
          </>
        ),
      }}
    </Modal>
  );
}, {
  props,
  name: 'ConfirmDialog',
  emits: {
    'confirm': () => true,
    'cancel': () => true,
    'update:open': (_v: boolean) => true,
  },
});
