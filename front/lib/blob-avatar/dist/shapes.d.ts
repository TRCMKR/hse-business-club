import type { SvgNode } from './svg';
import type { Shape } from './traits';
/** Nominal radius of every base figure in local coordinates (origin at its center). */
export declare const FIGURE_RADIUS = 30;
/** Base figure centered at the origin, about FIGURE_RADIUS in size; the caller sets fill and transform. */
export declare function buildShape(shape: Shape, shapeSeed: number): SvgNode;
