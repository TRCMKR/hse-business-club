import { defineComponent } from 'vue';
import { NuxtLink } from '#components';
import { prop } from '@fe/prop-types';
import type { ExtractProps } from '@fe/prop-types';
import { Spinner } from '../spinner';
import { Icon } from '../icon';
import type { IconName } from '../icon';
import styles from './index.module.css';

export type ButtonVariant = 'primary' | 'secondary' | 'ghost' | 'danger';
export type ButtonSize = 'md' | 'sm';

const props = {
  variant: prop<ButtonVariant>().optional('primary'),
  size: prop<ButtonSize>().optional('md'),
  type: prop<'button' | 'submit' | 'reset'>().optional('button'),
  loading: prop<boolean>().optional(false),
  disabled: prop<boolean>().optional(false),
  /** When set, the button is rendered as a NuxtLink. */
  to: prop<string>().optional(),
  /** Full width. */
  block: prop<boolean>().optional(false),
  /** Leading icon. */
  icon: prop<IconName>().optional(),
  /** Square icon-only button; pass `ariaLabel`. */
  iconOnly: prop<boolean>().optional(false),
  ariaLabel: prop<string>().optional(),
};

export type ButtonProps = ExtractProps<typeof props>;

export const Button = defineComponent((props: ButtonProps, { slots, emit }) => {
  const inactive = () => props.loading || props.disabled;

  const onClick = (e: MouseEvent) => {
    if (inactive()) {
      e.preventDefault();
      e.stopPropagation();
      return;
    }
    emit('click', e);
  };

  return () => {
    const iconSize = props.size === 'sm' ? 16 : 20;
    const cls = [
      styles.button,
      styles[props.variant!],
      styles[props.size!],
      props.block && styles.block,
      props.iconOnly && styles.iconOnly,
      props.loading && styles.loading,
    ];
    const content = (
      <>
        {props.loading && (
          <span class={styles.spinner}>
            <Spinner size={iconSize - 2} />
          </span>
        )}
        <span class={styles.content}>
          {props.icon && <Icon name={props.icon} size={iconSize} />}
          {slots.default?.()}
        </span>
      </>
    );

    if (props.to) {
      return (
        <NuxtLink
          to={props.to}
          class={cls}
          aria-label={props.ariaLabel}
          aria-disabled={inactive() ? 'true' : undefined}
          aria-busy={props.loading ? 'true' : undefined}
          tabindex={inactive() ? -1 : undefined}
          onClick={onClick}
        >
          {content}
        </NuxtLink>
      );
    }

    return (
      <button
        type={props.type}
        class={cls}
        disabled={inactive()}
        aria-label={props.ariaLabel}
        aria-busy={props.loading ? 'true' : undefined}
        onClick={onClick}
      >
        {content}
      </button>
    );
  };
}, {
  props,
  name: 'Button',
  emits: {
    click: (_e: MouseEvent) => true,
  },
});
