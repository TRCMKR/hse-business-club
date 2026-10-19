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

const props = {
  seed: { type: String, required: true },
  size: { type: Number, required: false },
  background: { type: String as PropType<Background>, required: false },
  traits: { type: Object as PropType<Partial<Traits>>, required: false },
  label: { type: String, required: false },
} as const;

/**
 * Deterministic blob avatar. Renders the SVG through vnodes (no innerHTML),
 * so it is SSR-safe and hydrates to the same markup as `blobAvatar()`.
 */
export const BlobAvatar = defineComponent((props) => {
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
