import { computed, defineComponent, ref, useId } from 'vue';
import { prop } from '@fe/prop-types';
import type { ExtractProps } from '@fe/prop-types';
import { Chip } from '../chip';
import { Field, describedBy } from '../field';
import control from '../field/control.module.css';
import styles from './index.module.css';

const props = {
  modelValue: prop<string[]>().required(),
  max: prop<number>().required(),
  /** Max length of a single item (native maxlength on the text input). */
  maxLength: prop<number>().optional(),
  label: prop<string>().optional(),
  placeholder: prop<string>().optional(),
  error: prop<string>().optional(),
  hint: prop<string>().optional(),
  disabled: prop<boolean>().optional(false),
  id: prop<string>().optional(),
};

export type ChipsInputProps = ExtractProps<typeof props>;

/**
 * Tag input: adds on Enter, comma and blur; trims; ignores empty values and
 * case-insensitive duplicates; Backspace on empty input removes the last item;
 * stops accepting input once `max` items are present.
 */
export const ChipsInput = defineComponent((props: ChipsInputProps, { emit }) => {
  const autoId = useId();
  const id = () => props.id ?? autoId;
  const text = ref('');
  const inputEl = ref<HTMLInputElement | null>(null);

  const full = computed(() => props.modelValue.length >= props.max);

  const addMany = (raw: string[]) => {
    const next = [...props.modelValue];
    const seen = new Set(next.map(v => v.toLocaleLowerCase()));
    for (const r of raw) {
      let v = r.trim();
      if (props.maxLength !== undefined) {
        v = [...v].slice(0, props.maxLength).join('');
      }
      if (!v || seen.has(v.toLocaleLowerCase()) || next.length >= props.max) {
        continue;
      }
      seen.add(v.toLocaleLowerCase());
      next.push(v);
    }
    if (next.length !== props.modelValue.length) {
      emit('update:modelValue', next);
    }
  };

  const commit = () => {
    if (text.value.trim()) {
      addMany([text.value]);
    }
    text.value = '';
  };

  const removeAt = (i: number) => {
    emit('update:modelValue', props.modelValue.filter((_, j) => j !== i));
    inputEl.value?.focus();
  };

  const onInput = (e: Event) => {
    const value = (e.target as HTMLInputElement).value;
    if (value.includes(',')) {
      const parts = value.split(',');
      const rest = parts.pop() ?? '';
      addMany(parts);
      text.value = rest;
      (e.target as HTMLInputElement).value = rest;
      return;
    }
    text.value = value;
  };

  const onKeydown = (e: KeyboardEvent) => {
    if (e.isComposing) {
      return;
    }
    if (e.key === 'Enter' || e.key === ',') {
      e.preventDefault();
      commit();
    } else if (e.key === 'Backspace' && text.value === '' && props.modelValue.length > 0) {
      e.preventDefault();
      emit('update:modelValue', props.modelValue.slice(0, -1));
    }
  };

  return () => {
    const limitHint = full.value ? `Не больше ${props.max}` : undefined;
    const hint = limitHint ?? props.hint;
    return (
      <Field for={id()} label={props.label} hint={hint} error={props.error}>
        <div
          class={[
            control.control,
            styles.box,
            props.error && control.invalid,
            props.disabled && control.disabled,
          ]}
          onClick={() => inputEl.value?.focus()}
        >
          {props.modelValue.map((v, i) => (
            <Chip
              key={v}
              label={v}
              size={'sm'}
              tone={'accent'}
              removable={!props.disabled}
              onRemove={() => removeAt(i)}
            />
          ))}
          <input
            ref={inputEl}
            id={id()}
            class={styles.input}
            type={'text'}
            value={text.value}
            placeholder={full.value ? '' : props.placeholder}
            disabled={props.disabled || full.value}
            maxlength={props.maxLength}
            autocomplete={'off'}
            aria-invalid={props.error ? 'true' : undefined}
            aria-describedby={describedBy(id(), { error: props.error, hint })}
            onInput={onInput}
            onKeydown={onKeydown}
            onBlur={commit}
          />
        </div>
      </Field>
    );
  };
}, {
  props,
  name: 'ChipsInput',
  emits: {
    'update:modelValue': (_v: string[]) => true,
  },
});
