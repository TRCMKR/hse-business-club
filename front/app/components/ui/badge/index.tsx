import { defineComponent } from 'vue';
import { prop } from '@fe/prop-types';
import type { ExtractProps } from '@fe/prop-types';
import styles from './index.module.css';

export type BadgeTone = 'neutral' | 'accent' | 'success' | 'warning' | 'danger' | 'info';

const props = {
  tone: prop<BadgeTone>().optional('neutral'),
  size: prop<'md' | 'sm'>().optional('md'),
  /** Leading colored dot. */
  dot: prop<boolean>().optional(false),
};

export type BadgeProps = ExtractProps<typeof props>;

export const Badge = defineComponent((props: BadgeProps, { slots }) => {
  return () => (
    <span class={[styles.badge, styles[props.tone!], styles[props.size!]]}>
      {props.dot && <span class={styles.dot} aria-hidden={'true'} />}
      {slots.default?.()}
    </span>
  );
}, { props, name: 'Badge' });
