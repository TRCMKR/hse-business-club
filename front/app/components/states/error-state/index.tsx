import { defineComponent } from 'vue';
import { prop } from '@fe/prop-types';
import type { ExtractProps } from '@fe/prop-types';
import { Button } from '../../ui/button';
import { Icon } from '../../ui/icon';
import styles from '../empty-state/index.module.css';

const props = {
  title: prop<string>().optional('Не удалось загрузить'),
  message: prop<string>().optional(),
  /** Show the «Повторить» button (emits `retry`). */
  retry: prop<boolean>().optional(false),
  /** Spinner on the retry button. */
  retrying: prop<boolean>().optional(false),
  /** Heading level of the title: 1 when the state replaces the whole page content. */
  headingLevel: prop<1 | 2>().optional(2),
};

export type ErrorStateProps = ExtractProps<typeof props>;

export const ErrorState = defineComponent((props: ErrorStateProps, { slots, emit }) => {
  return () => {
    const Heading = props.headingLevel === 1 ? 'h1' : 'h2';
    return (
      <div class={styles.state} role={'alert'}>
        <div class={[styles.icon, styles.danger]} aria-hidden={'true'}>
          <Icon name={'alert'} size={32} />
        </div>
        <Heading class={styles.title}>{props.title}</Heading>
        {props.message && <p class={styles.description}>{props.message}</p>}
        {(props.retry || slots.actions) && (
          <div class={styles.actions}>
            {props.retry && (
              <Button variant={'secondary'} loading={props.retrying} onClick={() => emit('retry')}>
                {'Повторить'}
              </Button>
            )}
            {slots.actions?.()}
          </div>
        )}
      </div>
    );
  };
}, {
  props,
  name: 'ErrorState',
  emits: {
    retry: () => true,
  },
});
