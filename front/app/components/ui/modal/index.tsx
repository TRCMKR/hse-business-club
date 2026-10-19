import {
  Teleport,
  defineComponent,
  nextTick,
  onBeforeUnmount,
  onMounted,
  ref,
  useId,
  watch,
} from 'vue';
import { prop } from '@fe/prop-types';
import type { ExtractProps } from '@fe/prop-types';
import { Icon } from '../icon';
import styles from './index.module.css';

export type ModalVariant = 'dialog' | 'sheet';
export type ModalSize = 'sm' | 'md' | 'lg';

/** Props shared by Modal, Dialog, Sheet and ResponsiveModal. */
export const modalBaseProps = {
  open: prop<boolean>().required(),
  title: prop<string>().optional(),
  /** id of an external element labelling the dialog (used when `title` is not set). */
  labelledby: prop<string>().optional(),
  ariaLabel: prop<string>().optional(),
  /** Dialog max width: sm 400px, md 520px, lg 720px. */
  size: prop<ModalSize>().optional('md'),
  /** Esc, overlay click and the close button close the modal. Set false while a request is running. */
  closable: prop<boolean>().optional(true),
  /** Hide the "×" button in the header. */
  hideClose: prop<boolean>().optional(false),
};

const props = {
  ...modalBaseProps,
  variant: prop<ModalVariant>().optional('dialog'),
};

export type ModalProps = ExtractProps<typeof props>;
export type ModalBaseProps = ExtractProps<typeof modalBaseProps>;

export const modalEmits = {
  'update:open': (_v: boolean) => true,
  'close': () => true,
};

const FOCUSABLE = [
  'a[href]',
  'button:not([disabled])',
  'input:not([disabled]):not([type="hidden"])',
  'select:not([disabled])',
  'textarea:not([disabled])',
  '[tabindex]:not([tabindex="-1"])',
  '[contenteditable="true"]',
].join(',');

const focusables = (root: HTMLElement) =>
  Array.from(root.querySelectorAll<HTMLElement>(FOCUSABLE))
    .filter(el => !el.hasAttribute('inert') && el.getAttribute('aria-hidden') !== 'true');

/** Open modals, topmost last; only the topmost reacts to Esc. */
const stack: symbol[] = [];
let savedOverflow = '';

/**
 * Accessible modal base: Teleport to body (client only), Esc / overlay click close,
 * focus trap, focus restore, body scroll lock, role=dialog + aria-modal.
 * Slots: default (body), footer, header (replaces the title).
 */
export const Modal = defineComponent((props: ModalProps, { slots, emit }) => {
  const mounted = ref(false);
  const panel = ref<HTMLElement | null>(null);
  const titleId = useId();
  const token = Symbol('modal');
  let prevFocus: HTMLElement | null = null;
  let downTarget: EventTarget | null = null;

  const close = () => {
    if (!props.closable) {
      return;
    }
    emit('update:open', false);
    emit('close');
  };

  const onDocKeydown = (e: KeyboardEvent) => {
    if (e.key === 'Escape' && stack[stack.length - 1] === token) {
      e.preventDefault();
      e.stopPropagation();
      close();
    }
  };

  const focusInitial = () => {
    const root = panel.value;
    if (!root) {
      return;
    }
    const list = focusables(root);
    const target = list.find(el => el.hasAttribute('autofocus') || el.hasAttribute('data-autofocus'))
      ?? list.find(el => !el.hasAttribute('data-modal-close'))
      ?? list[0]
      ?? root;
    target.focus();
  };

  const activate = async () => {
    if (stack.includes(token)) {
      return;
    }
    prevFocus = document.activeElement instanceof HTMLElement ? document.activeElement : null;
    if (stack.length === 0) {
      savedOverflow = document.body.style.overflow;
      document.body.style.overflow = 'hidden';
    }
    stack.push(token);
    document.addEventListener('keydown', onDocKeydown);
    await nextTick();
    focusInitial();
  };

  const deactivate = () => {
    const idx = stack.indexOf(token);
    if (idx < 0) {
      return;
    }
    stack.splice(idx, 1);
    if (stack.length === 0) {
      document.body.style.overflow = savedOverflow;
    }
    document.removeEventListener('keydown', onDocKeydown);
    const el = prevFocus;
    prevFocus = null;
    if (el && el.isConnected) {
      el.focus();
    }
  };

  const onPanelKeydown = (e: KeyboardEvent) => {
    if (e.key !== 'Tab' || !panel.value) {
      return;
    }
    const list = focusables(panel.value);
    if (list.length === 0) {
      e.preventDefault();
      panel.value.focus();
      return;
    }
    const first = list[0]!;
    const last = list[list.length - 1]!;
    const active = document.activeElement;
    const inside = active instanceof Node && panel.value.contains(active);
    if (e.shiftKey && (active === first || !inside || active === panel.value)) {
      e.preventDefault();
      last.focus();
    } else if (!e.shiftKey && (active === last || !inside)) {
      e.preventDefault();
      first.focus();
    }
  };

  onMounted(() => {
    mounted.value = true;
  });

  watch(() => props.open && mounted.value, (on) => {
    if (on) {
      activate();
    } else {
      deactivate();
    }
  }, { immediate: true });

  onBeforeUnmount(deactivate);

  return () => {
    if (!mounted.value) {
      return null;
    }
    const hasHeader = Boolean(props.title || slots.header || (props.closable && !props.hideClose));
    const labelledby = props.title ? titleId : props.labelledby;
    return (
      <Teleport to={'body'}>
        {props.open && (
          <div
            class={[styles.root, styles[props.variant!]]}
            onPointerdown={(e: PointerEvent) => {
              downTarget = e.target;
            }}
            onClick={(e: MouseEvent) => {
              const fromOverlay = e.target === e.currentTarget && (downTarget === null || downTarget === e.currentTarget);
              downTarget = null;
              if (fromOverlay) {
                close();
              }
            }}
          >
            <div
              ref={panel}
              class={[styles.panel, styles[`size-${props.size}`]]}
              role={'dialog'}
              aria-modal={'true'}
              aria-labelledby={labelledby}
              aria-label={labelledby ? undefined : props.ariaLabel}
              tabindex={-1}
              onKeydown={onPanelKeydown}
            >
              {props.variant === 'sheet' && <div class={styles.grabber} aria-hidden={'true'} />}
              {hasHeader && (
                <div class={styles.header}>
                  {slots.header
                    ? slots.header()
                    : props.title && <h2 id={titleId} class={styles.title}>{props.title}</h2>}
                  {props.closable && !props.hideClose && (
                    <button
                      type={'button'}
                      class={styles.close}
                      aria-label={'Закрыть'}
                      data-modal-close={''}
                      onClick={close}
                    >
                      <Icon name={'close'} size={20} />
                    </button>
                  )}
                </div>
              )}
              <div class={styles.body}>{slots.default?.()}</div>
              {slots.footer && <div class={styles.footer}>{slots.footer()}</div>}
            </div>
          </div>
        )}
      </Teleport>
    );
  };
}, {
  props,
  name: 'Modal',
  emits: modalEmits,
});

/** Builds a Modal wrapper with a fixed variant (Dialog / Sheet). */
export const defineModalVariant = (variant: ModalVariant, name: string) =>
  defineComponent((props: ModalBaseProps, { slots, emit }) => {
    return () => (
      <Modal
        {...props}
        variant={variant}
        {...{
          'onUpdate:open': (v: boolean) => emit('update:open', v),
          'onClose': () => emit('close'),
        }}
      >
        {{ default: slots.default, footer: slots.footer, header: slots.header }}
      </Modal>
    );
  }, {
    props: modalBaseProps,
    name,
    emits: modalEmits,
  });
