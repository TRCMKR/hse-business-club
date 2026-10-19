import { defineComponent } from 'vue';
import { prop } from '@fe/prop-types';
import type { ExtractProps } from '@fe/prop-types';
import styles from './index.module.css';

const props = {
  /** id of the control the label points to. */
  for: prop<string>().required(),
  label: prop<string>().optional(),
  hint: prop<string>().optional(),
  error: prop<string>().optional(),
  required: prop<boolean>().optional(false),
};

export type FieldProps = ExtractProps<typeof props>;

/** ids used by form controls for aria-describedby. */
export const fieldIds = (id: string) => ({
  hint: `${id}-hint`,
  error: `${id}-error`,
});

export const describedBy = (id: string, p: { hint?: string, error?: string }) => {
  const ids = fieldIds(id);
  const list = [p.error ? ids.error : '', p.hint ? ids.hint : ''].filter(Boolean);
  return list.length ? list.join(' ') : undefined;
};

/**
 * Label + control + hint/error layout shared by Input, Textarea, Select, ChipsInput.
 * Slot `aside` renders on the right of the hint row (e.g. a character counter).
 */
export const Field = defineComponent((props: FieldProps, { slots }) => {
  return () => {
    const ids = fieldIds(props.for);
    const aside = slots.aside?.();
    return (
      <div class={styles.field}>
        {props.label && (
          <label class={styles.label} for={props.for}>
            {props.label}
            {props.required && <span class={styles.required} aria-hidden={'true'}>{' *'}</span>}
          </label>
        )}
        {slots.default?.()}
        {(props.error || props.hint || aside) && (
          <div class={styles.meta}>
            <div class={styles.messages}>
              {props.error && <p id={ids.error} class={styles.error} role={'alert'}>{props.error}</p>}
              {props.hint && <p id={ids.hint} class={styles.hint}>{props.hint}</p>}
            </div>
            {aside}
          </div>
        )}
      </div>
    );
  };
}, { props, name: 'Field' });
