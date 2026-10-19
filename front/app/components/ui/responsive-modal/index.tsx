import { defineComponent } from 'vue';
import { useMediaQuery } from '@vueuse/core';
import type { ExtractProps } from '@fe/prop-types';
import { Modal, modalBaseProps, modalEmits } from '../modal';

const props = modalBaseProps;

export type ResponsiveModalProps = ExtractProps<typeof props>;

/**
 * Bottom sheet below 768px, centered dialog otherwise. On SSR (no matchMedia) it
 * defaults to dialog; modals render only on the client anyway.
 */
export const ResponsiveModal = defineComponent((props: ResponsiveModalProps, { slots, emit }) => {
  const isMobile = useMediaQuery('(max-width: 767.98px)');

  return () => (
    <Modal
      {...props}
      variant={isMobile.value ? 'sheet' : 'dialog'}
      {...{
        'onUpdate:open': (v: boolean) => emit('update:open', v),
        'onClose': () => emit('close'),
      }}
    >
      {{ default: slots.default, footer: slots.footer, header: slots.header }}
    </Modal>
  );
}, {
  props,
  name: 'ResponsiveModal',
  emits: modalEmits,
});
