import { defineComponent, nextTick, onMounted, ref, useId, watch } from 'vue';
import { prop } from '@fe/prop-types';
import type { ExtractProps } from '@fe/prop-types';
import { Field, describedBy } from '../field';
import control from '../field/control.module.css';
import styles from './index.module.css';

const props = {
  modelValue: prop<string>().optional(''),
  label: prop<string>().optional(),
  placeholder: prop<string>().optional(),
  error: prop<string>().optional(),
  hint: prop<string>().optional(),
  disabled: prop<boolean>().optional(false),
  required: prop<boolean>().optional(false),
  name: prop<string>().optional(),
  id: prop<string>().optional(),
  rows: prop<number>().optional(4),
  /** Shows a "N / max" counter; the counter turns red when exceeded. */
  maxlength: prop<number>().optional(),
  /** Also set the native maxlength attribute (hard limit). Set false to let validation show the error instead. */
  hardLimit: prop<boolean>().optional(true),
  /** Grow with content (up to `maxRows`). */
  autoresize: prop<boolean>().optional(true),
  maxRows: prop<number>().optional(12),
};

export type TextareaProps = ExtractProps<typeof props>;

/** Length in Unicode code points (matches server-side rune counting). */
export const charCount = (s: string) => [...s].length;

export const Textarea = defineComponent((props: TextareaProps, { emit }) => {
  const autoId = useId();
  const id = () => props.id ?? autoId;
  const el = ref<HTMLTextAreaElement | null>(null);

  const resize = () => {
    const t = el.value;
    if (!t || !props.autoresize) {
      return;
    }
    const style = getComputedStyle(t);
    const lineHeight = Number.parseFloat(style.lineHeight) || 24;
    const padding = Number.parseFloat(style.paddingTop) + Number.parseFloat(style.paddingBottom);
    const border = Number.parseFloat(style.borderTopWidth) + Number.parseFloat(style.borderBottomWidth);
    t.style.height = 'auto';
    const max = lineHeight * (props.maxRows ?? 12) + padding + border;
    const next = t.scrollHeight + border;
    t.style.height = `${Math.min(next, max)}px`;
    t.style.overflowY = next > max ? 'auto' : 'hidden';
  };

  onMounted(resize);
  watch(() => props.modelValue, () => nextTick(resize));

  return () => {
    const count = charCount(props.modelValue ?? '');
    const over = props.maxlength !== undefined && count > props.maxlength;
    return (
      <Field for={id()} label={props.label} hint={props.hint} error={props.error} required={props.required}>
        {{
          default: () => (
            <textarea
              ref={el}
              id={id()}
              class={[control.control, styles.textarea, (props.error || over) && control.invalid]}
              value={props.modelValue}
              rows={props.rows}
              placeholder={props.placeholder}
              disabled={props.disabled}
              required={props.required}
              name={props.name}
              maxlength={props.hardLimit ? props.maxlength : undefined}
              aria-invalid={props.error || over ? 'true' : undefined}
              aria-describedby={describedBy(id(), props)}
              onInput={(e: Event) => emit('update:modelValue', (e.target as HTMLTextAreaElement).value)}
              onBlur={(e: FocusEvent) => emit('blur', e)}
            />
          ),
          aside: props.maxlength !== undefined
            ? () => (
              <span class={[styles.counter, over && styles.over]} aria-live={'polite'}>
                {`${count} / ${props.maxlength}`}
              </span>
            )
            : undefined,
        }}
      </Field>
    );
  };
}, {
  props,
  name: 'Textarea',
  emits: {
    'update:modelValue': (_v: string) => true,
    'blur': (_e: FocusEvent) => true,
  },
});
