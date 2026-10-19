import { buildTree } from "./render.js";
import { toSvgString } from "./svg.js";
import { formatAttr } from "./svg.js";
import { deriveTraits } from "./traits.js";
import { EXPRESSIONS, EXTRAS, EYE_SHAPES, HUES, SHAPES, TONES } from "./traits.js";
import { createRng, hashString, normalizeSeed } from "./random.js";
function blobAvatarTree(seed, options = {}) {
  const { traits, ...render } = options;
  return buildTree(deriveTraits(seed, traits), { ...render, aria: render.aria ?? seed.trim() });
}
function blobAvatar(seed, options = {}) {
  return toSvgString(blobAvatarTree(seed, options));
}
export {
  EXPRESSIONS,
  EXTRAS,
  EYE_SHAPES,
  HUES,
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
