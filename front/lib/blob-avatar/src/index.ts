import { buildTree } from './render';
import type { RenderOptions } from './render';
import { toSvgString } from './svg';
import type { SvgNode } from './svg';
import { deriveTraits } from './traits';
import type { Traits } from './traits';

export interface BlobAvatarOptions extends RenderOptions {
  /** Pins any trait axis, e.g. `{ expression: 'happy', hue: 210 }`. */
  traits?: Partial<Traits>;
}

/** Element tree of the avatar for `seed`; render it with any view layer. */
export function blobAvatarTree(seed: string, options: BlobAvatarOptions = {}): SvgNode {
  const { traits, ...render } = options;
  return buildTree(deriveTraits(seed, traits), { ...render, aria: render.aria ?? seed.trim() });
}

/** SVG markup of the avatar for `seed`. Same seed, same string, in Node and in the browser. */
export function blobAvatar(seed: string, options: BlobAvatarOptions = {}): string {
  return toSvgString(blobAvatarTree(seed, options));
}

export { deriveTraits, EXPRESSIONS, EXTRAS, EYE_SHAPES, HUES, SHAPES, TONES } from './traits';
export type { Expression, Extras, EyeShape, Hue, Shape, Tone, Traits } from './traits';
export { buildTree } from './render';
export type { Background, RenderOptions } from './render';
export { toSvgString, formatAttr } from './svg';
export type { SvgAttrValue, SvgNode } from './svg';
export { createRng, hashString, normalizeSeed } from './random';
export type { Rng } from './random';
