import { buildTree } from "./render.js";
import { toSvgString } from "./svg.js";
import { formatAttr } from "./svg.js";
import { normalizeSeed } from "./random.js";
import { createRng, hashString } from "./random.js";
import { deriveTraits } from "./traits.js";
import { EXPRESSIONS, EXTRAS, EYE_SHAPES, HUES, SHAPES, TONES } from "./traits.js";
const MAX_LABEL_LENGTH = 64;
function defaultLabel(seed) {
  return normalizeSeed(seed).slice(0, MAX_LABEL_LENGTH) || "avatar";
}
function blobAvatarTree(seed, options = {}) {
  const { traits, ...render } = options;
  const aria = render.aria === void 0 ? defaultLabel(seed) : render.aria.slice(0, MAX_LABEL_LENGTH) || "avatar";
  return buildTree(deriveTraits(seed, traits), { ...render, aria });
}
function blobAvatar(seed, options = {}) {
  return toSvgString(blobAvatarTree(seed, options));
}
export {
  EXPRESSIONS,
  EXTRAS,
  EYE_SHAPES,
  HUES,
  MAX_LABEL_LENGTH,
  SHAPES,
  TONES,
  blobAvatar,
  blobAvatarTree,
  buildTree,
  createRng,
  deriveTraits,
  formatAttr,
  hashString,
  normalizeSeed,
  toSvgString
};
