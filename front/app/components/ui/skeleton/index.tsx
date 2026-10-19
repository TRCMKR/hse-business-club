import { defineComponent } from 'vue';
import { prop } from '@fe/prop-types';
import type { ExtractProps } from '@fe/prop-types';
import styles from './index.module.css';

const props = {
  /** CSS length or px number; default 100%. */
  width: prop<string | number>().optional(),
  /** CSS length or px number; default 1em (16px for lines). */
  height: prop<string | number>().optional(),
  radius: prop<string | number>().optional(),
  circle: prop<boolean>().optional(false),
  /** Renders N text lines; the last one is shorter. */
  lines: prop<number>().optional(),
};

export type SkeletonProps = ExtractProps<typeof props>;

const len = (v: string | number | undefined) => (typeof v === 'number' ? `${v}px` : v);

export const Skeleton = defineComponent((props: SkeletonProps) => {
  return () => {
    if (props.lines && props.lines > 0) {
      return (
        <div class={styles.lines} aria-hidden={'true'} style={{ width: len(props.width) }}>
          {Array.from({ length: props.lines }, (_, i) => (
            <span
              key={i}
              class={styles.bone}
              style={{
                borderRadius: len(props.radius),
                height: len(props.height) ?? '14px',
                width: i === props.lines! - 1 && props.lines! > 1 ? '60%' : '100%',
              }}
            />
          ))}
        </div>
      );
    }
    const size = len(props.width ?? props.height);
    return (
      <span
        class={[styles.bone, props.circle && styles.circle]}
        aria-hidden={'true'}
        style={{
          borderRadius: props.circle ? '50%' : len(props.radius),
          height: props.circle ? size : len(props.height),
          width: props.circle ? size : len(props.width),
        }}
      />
    );
  };
}, { props, name: 'Skeleton' });
