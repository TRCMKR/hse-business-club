import { defineComponent } from 'vue';
import { prop } from '@fe/prop-types';
import type { ExtractProps } from '@fe/prop-types';
import { Chip } from '~/components/ui/chip';
import { STATUS_FILTER_OPTIONS } from '../status';
import type { StatusFilterOption, StatusFilterValue } from '../status';
import styles from './index.module.css';

const props = {
  modelValue: prop<StatusFilterValue>().required(),
  /** Defaults to all statuses + «Все». */
  options: prop<StatusFilterOption[]>().optional(),
  ariaLabel: prop<string>().optional('Фильтр по статусу'),
  disabled: prop<boolean>().optional(false),
};

export type StatusFilterProps = ExtractProps<typeof props>;

/** Row of toggle chips for the application status filter (single choice, «Все» = none). */
export const StatusFilter = defineComponent((props: StatusFilterProps, { emit }) => {
  return () => (
    <div class={styles.filter} role={'group'} aria-label={props.ariaLabel}>
      {(props.options ?? STATUS_FILTER_OPTIONS).map((o) => {
        const selected = o.value === props.modelValue;
        return (
          <button
            key={o.value || 'all'}
            type={'button'}
            class={styles.option}
            aria-pressed={selected ? 'true' : 'false'}
            disabled={props.disabled}
            data-testid={`status-filter-${o.value || 'all'}`}
            onClick={() => selected || emit('update:modelValue', o.value)}
          >
            <Chip label={o.label} tone={selected ? 'accent' : 'neutral'} />
          </button>
        );
      })}
    </div>
  );
}, {
  props,
  name: 'StatusFilter',
  emits: {
    'update:modelValue': (_v: StatusFilterValue) => true,
  },
});
