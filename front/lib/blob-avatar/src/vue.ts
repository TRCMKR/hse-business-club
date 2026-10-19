import { computed, defineComponent, h } from 'vue';
import type { PropType, VNode } from 'vue';
import { blobAvatarTree } from './index';
import type { Background } from './render';
import { formatAttr } from './svg';
import type { SvgNode } from './svg';
import type { Traits } from './traits';

function toVNode(node: SvgNode): VNode {
  const attrs: Record<string, string> = {};
  for (const [name, value] of Object.entries(node.attrs)) {
    attrs[name] = formatAttr(value);
  }
  return h(node.tag, attrs, node.children.map(toVNode));
}

export interface BlobAvatarProps {
  /** Any string; the app uses `username`. */
  seed: string;
  /** Width and height in pixels; without it the SVG fills its container. */
  size?: number;
  background?: Background;
  /** Pins trait axes, e.g. `{ palette: 3 }`. */
  traits?: Partial<Traits>;
  /** aria-label, defaults to the seed. */
  label?: string;
}

const props = {
  seed: { type: String, required: true },
  size: { type: Number, required: false },
  background: { type: String as PropType<Background>, required: false },
  traits: { type: Object as PropType<Partial<Traits>>, required: false },
  label: { type: String, required: false },
};

/**
 * Deterministic blob avatar. Renders the SVG through vnodes (no innerHTML),
 * so it is SSR-safe and hydrates to the same markup as `blobAvatar()`.
 */
export const BlobAvatar = defineComponent((props: BlobAvatarProps) => {
  const tree = computed(() =>
    blobAvatarTree(props.seed, {
      size: props.size,
      background: props.background,
      traits: props.traits,
      aria: props.label,
    }),
  );
  return () => toVNode(tree.value);
}, { props, name: 'BlobAvatar' });

export type { Background } from './render';
export type { Traits } from './traits';
