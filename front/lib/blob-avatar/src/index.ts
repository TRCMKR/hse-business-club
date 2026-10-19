import { buildTree } from './render';
import type { RenderOptions } from './render';
import { toSvgString } from './svg';
import type { SvgNode } from './svg';
import { normalizeSeed } from './random';
import { deriveTraits } from './traits';
import type { Traits } from './traits';

export interface BlobAvatarOptions extends RenderOptions {
  /** Pins any trait axis, e.g. `{ shape: 'round', palette: 3 }`. */
  traits?: Partial<Traits>;
}

/** Longest default aria-label; longer seeds are cut so the SVG stays small. */
export const MAX_LABEL_LENGTH = 64;

/** Default aria-label: the normalized seed, truncated, or "avatar" when empty. */
function defaultLabel(seed: string): string {
  return normalizeSeed(seed).slice(0, MAX_LABEL_LENGTH) || 'avatar';
}

/** Element tree of the avatar for `seed`; render it with any view layer. */
export function blobAvatarTree(seed: string, options: BlobAvatarOptions = {}): SvgNode {
  const { traits, ...render } = options;
  const aria = render.aria === undefined ? defaultLabel(seed) : render.aria.slice(0, MAX_LABEL_LENGTH) || 'avatar';
  return buildTree(deriveTraits(seed, traits), { ...render, aria });
}

/** SVG markup of the avatar for `seed`. Same seed, same string, in Node and in the browser. */
export function blobAvatar(seed: string, options: BlobAvatarOptions = {}): string {
  return toSvgString(blobAvatarTree(seed, options));
}

export { ACCENTS, deriveTraits, DETAILS, SHAPES } from './traits';
export type { Accent, Detail, Shape, Traits } from './traits';
export { PALETTES } from './colors';
export type { Palette } from './colors';
export { buildTree } from './render';
export type { Background, RenderOptions } from './render';
export { toSvgString, formatAttr } from './svg';
export type { SvgAttrValue, SvgNode } from './svg';
export { createRng, hashString, normalizeSeed } from './random';
export type { Rng } from './random';
