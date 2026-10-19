import { defineComponent, useId } from 'vue';
import { prop } from '@fe/prop-types';
import type { ExtractProps } from '@fe/prop-types';
import { Field, describedBy } from '../field';
import { Icon } from '../icon';
import control from '../field/control.module.css';
import styles from './index.module.css';

export interface SelectOption {
  value: string
  label: string
  disabled?: boolean
}

const props = {
  modelValue: prop<string>().optional(''),
  options: prop<SelectOption[]>().required(),
  label: prop<string>().optional(),
  placeholder: prop<string>().optional(),
  error: prop<string>().optional(),
  hint: prop<string>().optional(),
  disabled: prop<boolean>().optional(false),
  required: prop<boolean>().optional(false),
  name: prop<string>().optional(),
  id: prop<string>().optional(),
};

export type SelectProps = ExtractProps<typeof props>;

export const Select = defineComponent((props: SelectProps, { emit }) => {
  const autoId = useId();
  const id = () => props.id ?? autoId;

  return () => (
    <Field for={id()} label={props.label} hint={props.hint} error={props.error} required={props.required}>
      <div class={styles.wrap}>
        <select
          id={id()}
          class={[control.control, styles.select, props.error && control.invalid, !props.modelValue && styles.empty]}
          value={props.modelValue}
          disabled={props.disabled}
          required={props.required}
          name={props.name}
          aria-invalid={props.error ? 'true' : undefined}
          aria-describedby={describedBy(id(), props)}
          onChange={(e: Event) => emit('update:modelValue', (e.target as HTMLSelectElement).value)}
        >
          {props.placeholder !== undefined && (
            <option value={''} disabled={props.required} selected={!props.modelValue}>{props.placeholder}</option>
          )}
          {props.options.map(o => (
            <option key={o.value} value={o.value} disabled={o.disabled} selected={o.value === props.modelValue}>
              {o.label}
            </option>
          ))}
        </select>
        <span class={styles.chevron}><Icon name={'chevron-down'} size={18} /></span>
      </div>
    </Field>
  );
}, {
  props,
  name: 'Select',
  emits: {
    'update:modelValue': (_v: string) => true,
  },
});
