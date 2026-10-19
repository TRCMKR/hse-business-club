import type { Palette } from './colors';
import type { SvgNode } from './svg';
import type { Traits } from './traits';
/** Face elements in local coordinates; the caller positions and scales the group. */
export declare function buildFace(traits: Traits, colors: Palette): SvgNode[];
