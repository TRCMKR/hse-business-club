import { defineComponent } from 'vue';
import { prop } from '@fe/prop-types';
import type { ExtractProps } from '@fe/prop-types';
import { Icon } from '../icon';
import styles from './index.module.css';

const props = {
  /** Text of the chip; the default slot is used when omitted. */
  label: prop<string>().optional(),
  size: prop<'md' | 'sm'>().optional('md'),
  tone: prop<'neutral' | 'accent'>().optional('neutral'),
  removable: prop<boolean>().optional(false),
  /** aria-label of the remove button; defaults to «Удалить <label>». */
  removeLabel: prop<string>().optional(),
  disabled: prop<boolean>().optional(false),
};

export type ChipProps = ExtractProps<typeof props>;

export const Chip = defineComponent((props: ChipProps, { slots, emit }) => {
  return () => (
    <span class={[styles.chip, styles[props.size!], styles[props.tone!], props.removable && styles.removable]}>
      <span class={styles.text}>{props.label ?? slots.default?.()}</span>
      {props.removable && (
        <button
          type={'button'}
          class={styles.remove}
          disabled={props.disabled}
          aria-label={props.removeLabel ?? (props.label ? `Удалить ${props.label}` : 'Удалить')}
          onClick={(e: MouseEvent) => {
            e.stopPropagation();
            emit('remove');
          }}
        >
          <Icon name={'close'} size={props.size === 'sm' ? 12 : 14} strokeWidth={2.5} />
        </button>
      )}
    </span>
  );
}, {
  props,
  name: 'Chip',
  emits: {
    remove: () => true,
  },
});
