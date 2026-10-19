import { defineComponent } from 'vue';
import { prop } from '@fe/prop-types';
import type { ExtractProps } from '@fe/prop-types';
import styles from './index.module.css';

const props = {
  size: prop<number>().optional(16),
  /** Accessible label; when omitted the spinner is decorative (aria-hidden). */
  label: prop<string>().optional(),
};

export type SpinnerProps = ExtractProps<typeof props>;

export const Spinner = defineComponent((props: SpinnerProps) => {
  return () => (
    <span
      class={styles.spinner}
      style={{ height: `${props.size}px`, width: `${props.size}px` }}
      role={props.label ? 'status' : undefined}
      aria-label={props.label}
      aria-hidden={props.label ? undefined : 'true'}
      data-testid={'spinner'}
    />
  );
}, { props, name: 'Spinner' });
