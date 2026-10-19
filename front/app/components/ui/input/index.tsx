import { defineComponent, useId } from 'vue';
import { prop } from '@fe/prop-types';
import type { ExtractProps } from '@fe/prop-types';
import { Field, describedBy } from '../field';
import control from '../field/control.module.css';
import styles from './index.module.css';

const props = {
  modelValue: prop<string>().optional(''),
  label: prop<string>().optional(),
  placeholder: prop<string>().optional(),
  type: prop<'text' | 'password' | 'search' | 'email' | 'url' | 'tel' | 'number'>().optional('text'),
  error: prop<string>().optional(),
  hint: prop<string>().optional(),
  disabled: prop<boolean>().optional(false),
  required: prop<boolean>().optional(false),
  autocomplete: prop<string>().optional(),
  inputmode: prop<'text' | 'search' | 'email' | 'url' | 'tel' | 'numeric' | 'decimal' | 'none'>().optional(),
  name: prop<string>().optional(),
  id: prop<string>().optional(),
  maxlength: prop<number>().optional(),
  autofocus: prop<boolean>().optional(false),
};

export type InputProps = ExtractProps<typeof props>;

export const Input = defineComponent((props: InputProps, { emit }) => {
  const autoId = useId();
  const id = () => props.id ?? autoId;

  return () => (
    <Field for={id()} label={props.label} hint={props.hint} error={props.error} required={props.required}>
      <input
        id={id()}
        class={[control.control, styles.input, props.error && control.invalid]}
        type={props.type}
        value={props.modelValue}
        placeholder={props.placeholder}
        disabled={props.disabled}
        required={props.required}
        autocomplete={props.autocomplete}
        inputmode={props.inputmode}
        name={props.name}
        maxlength={props.maxlength}
        autofocus={props.autofocus}
        aria-invalid={props.error ? 'true' : undefined}
        aria-describedby={describedBy(id(), props)}
        onInput={(e: Event) => emit('update:modelValue', (e.target as HTMLInputElement).value)}
        onBlur={(e: FocusEvent) => emit('blur', e)}
        onFocus={(e: FocusEvent) => emit('focus', e)}
        onKeydown={(e: KeyboardEvent) => {
          if (e.key === 'Enter' && !e.isComposing) {
            emit('enter', e);
          }
        }}
      />
    </Field>
  );
}, {
  props,
  name: 'Input',
  emits: {
    'update:modelValue': (_v: string) => true,
    'blur': (_e: FocusEvent) => true,
    'focus': (_e: FocusEvent) => true,
    'enter': (_e: KeyboardEvent) => true,
  },
});
