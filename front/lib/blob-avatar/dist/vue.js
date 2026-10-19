import { defineComponent, computed, h } from "vue";
import { blobAvatarTree } from "./index.js";
import { formatAttr } from "./svg.js";
function toVNode(node) {
  const attrs = {};
  for (const [name, value] of Object.entries(node.attrs)) {
    attrs[name] = formatAttr(value);
  }
  return h(node.tag, attrs, node.children.map(toVNode));
}
const props = {
  seed: { type: String, required: true },
  size: { type: Number, required: false },
  background: { type: String, required: false },
  traits: { type: Object, required: false },
  label: { type: String, required: false }
};
const BlobAvatar = defineComponent((props2) => {
  const tree = computed(
    () => blobAvatarTree(props2.seed, {
      size: props2.size,
      background: props2.background,
      traits: props2.traits,
      aria: props2.label
    })
  );
  return () => toVNode(tree.value);
}, { props, name: "BlobAvatar" });
export {
  BlobAvatar
};
