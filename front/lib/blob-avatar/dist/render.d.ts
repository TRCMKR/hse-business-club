import type { SvgNode } from './svg';
import type { Traits } from './traits';
export type Background = 'none' | 'circle' | 'squircle' | 'square';
export interface RenderOptions {
    /** Width and height in pixels. Without it the SVG has no size attributes and fills its box. */
    size?: number;
    /** Backdrop behind the figure, `circle` by default. */
    background?: Background;
    /** Accessible label (aria-label). */
    aria?: string;
}
/** Builds the SVG element tree for the given traits. */
export declare function buildTree(traits: Traits, options?: RenderOptions): SvgNode;
