import { defineComponent, h } from 'vue';
import { NuxtLink } from '#components';
import { prop } from '@fe/prop-types';
import type { ExtractProps } from '@fe/prop-types';
import styles from './index.module.css';

const props = {
  /** Root tag when `to` is not set. */
  as: prop<string>().optional('div'),
  /** Makes the whole card a link. */
  to: prop<string>().optional(),
  padding: prop<'md' | 'sm' | 'none'>().optional('md'),
  /** Vertical flex layout with this gap between children (sm 8, md 16, lg 20px). */
  gap: prop<'sm' | 'md' | 'lg'>().optional(),
  /** Hover/press feedback (implied by `to`). */
  interactive: prop<boolean>().optional(false),
};

export type CardProps = ExtractProps<typeof props>;

export const Card = defineComponent((props: CardProps, { slots }) => {
  return () => {
    const cls = [
      styles.card,
      styles[`pad-${props.padding}`],
      props.gap && styles[`gap-${props.gap}`],
      (props.interactive || props.to) && styles.interactive,
    ];
    if (props.to) {
      return (
        <NuxtLink to={props.to} class={[cls, styles.link]}>
          {slots.default?.()}
        </NuxtLink>
      );
    }
    return h(props.as ?? 'div', { class: cls }, slots.default?.());
  };
}, { props, name: 'Card' });
