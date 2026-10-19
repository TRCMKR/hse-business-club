import type { RenderOptions } from './render';
import type { SvgNode } from './svg';
import type { Traits } from './traits';
export interface BlobAvatarOptions extends RenderOptions {
    /** Pins any trait axis, e.g. `{ expression: 'happy', hue: 210 }`. */
    traits?: Partial<Traits>;
}
/** Longest default aria-label; longer seeds are cut so the SVG stays small. */
export declare const MAX_LABEL_LENGTH = 64;
/** Element tree of the avatar for `seed`; render it with any view layer. */
export declare function blobAvatarTree(seed: string, options?: BlobAvatarOptions): SvgNode;
/** SVG markup of the avatar for `seed`. Same seed, same string, in Node and in the browser. */
export declare function blobAvatar(seed: string, options?: BlobAvatarOptions): string;
export { deriveTraits, EXPRESSIONS, EXTRAS, EYE_SHAPES, HUES, SHAPES, TONES } from './traits';
export type { Expression, Extras, EyeShape, Hue, Shape, Tone, Traits } from './traits';
export { buildTree } from './render';
export type { Background, RenderOptions } from './render';
export { toSvgString, formatAttr } from './svg';
export type { SvgAttrValue, SvgNode } from './svg';
export { createRng, hashString, normalizeSeed } from './random';
export type { Rng } from './random';
