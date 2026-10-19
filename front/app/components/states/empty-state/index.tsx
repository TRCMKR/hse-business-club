import { defineComponent } from 'vue';
import { prop } from '@fe/prop-types';
import type { ExtractProps } from '@fe/prop-types';
import styles from './index.module.css';

const props = {
  title: prop<string>().required(),
  description: prop<string>().optional(),
  /** Emoji or short text shown above the title; slot `icon` overrides it. */
  icon: prop<string>().optional(),
};

export type EmptyStateProps = ExtractProps<typeof props>;

export const EmptyState = defineComponent((props: EmptyStateProps, { slots }) => {
  return () => (
    <div class={styles.state}>
      {(slots.icon || props.icon) && (
        <div class={styles.icon} aria-hidden={'true'}>{slots.icon ? slots.icon() : props.icon}</div>
      )}
      <h2 class={styles.title}>{props.title}</h2>
      {props.description && <p class={styles.description}>{props.description}</p>}
      {slots.actions && <div class={styles.actions}>{slots.actions()}</div>}
    </div>
  );
}, { props, name: 'EmptyState' });
