import { defineComponent, ref } from 'vue';
import { prop } from '@fe/prop-types';
import type { ExtractProps } from '@fe/prop-types';
import styles from './index.module.css';

export interface SegmentedOption {
  value: string
  label: string
}

const props = {
  modelValue: prop<string>().required(),
  options: prop<SegmentedOption[]>().required(),
  size: prop<'md' | 'sm'>().optional('md'),
  /** Stretch segments to full width. */
  block: prop<boolean>().optional(false),
  /** Accessible name of the group. */
  ariaLabel: prop<string>().optional(),
};

export type SegmentedProps = ExtractProps<typeof props>;

/** Segmented control (single choice), role=radiogroup with roving tabindex and arrow keys. */
export const Segmented = defineComponent((props: SegmentedProps, { emit }) => {
  const root = ref<HTMLElement | null>(null);

  const select = (value: string) => {
    if (value !== props.modelValue) {
      emit('update:modelValue', value);
    }
  };

  const onKeydown = (e: KeyboardEvent) => {
    const idx = props.options.findIndex(o => o.value === props.modelValue);
    const n = props.options.length;
    let next = -1;
    if (e.key === 'ArrowRight' || e.key === 'ArrowDown') {
      next = (idx + 1) % n;
    } else if (e.key === 'ArrowLeft' || e.key === 'ArrowUp') {
      next = (idx - 1 + n) % n;
    } else if (e.key === 'Home') {
      next = 0;
    } else if (e.key === 'End') {
      next = n - 1;
    }
    if (next < 0) {
      return;
    }
    e.preventDefault();
    select(props.options[next]!.value);
    root.value?.querySelectorAll<HTMLElement>('[role="radio"]')[next]?.focus();
  };

  return () => {
    const hasSelected = props.options.some(o => o.value === props.modelValue);
    return (
      <div
        ref={root}
        class={[styles.segmented, styles[props.size!], props.block && styles.block]}
        role={'radiogroup'}
        aria-label={props.ariaLabel}
        onKeydown={onKeydown}
      >
        {props.options.map((o, i) => {
          const checked = o.value === props.modelValue;
          return (
            <button
              key={o.value}
              type={'button'}
              role={'radio'}
              aria-checked={checked ? 'true' : 'false'}
              tabindex={checked || (!hasSelected && i === 0) ? 0 : -1}
              class={[styles.item, checked && styles.active]}
              onClick={() => select(o.value)}
            >
              {o.label}
            </button>
          );
        })}
      </div>
    );
  };
}, {
  props,
  name: 'Segmented',
  emits: {
    'update:modelValue': (_v: string) => true,
  },
});
