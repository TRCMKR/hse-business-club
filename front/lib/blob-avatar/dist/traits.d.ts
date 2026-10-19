export declare const SHAPES: readonly ["round", "squircle", "blob", "droplet", "hexagon", "capsule"];
export declare const ACCENTS: readonly ["orbit", "ring", "arc", "inset", "overlap", "split"];
export declare const DETAILS: readonly ["none", "satellite", "trail"];
export type Shape = (typeof SHAPES)[number];
export type Accent = (typeof ACCENTS)[number];
export type Detail = (typeof DETAILS)[number];
/**
 * Everything that defines how an avatar looks. Continuous axes are normalized to
 * [0, 1); the renderer maps them to geometry. Two equal Traits render the same SVG.
 */
export interface Traits {
    /** Base figure. */
    shape: Shape;
    /** Seed for silhouette details (the `blob` outline), uint32. */
    shapeSeed: number;
    /** Index into PALETTES: background, figure and accent colors. */
    palette: number;
    /** Main accent element drawn over or next to the base figure. */
    accent: Accent;
    /** Optional small secondary element. */
    detail: Detail;
    /** Rotation of the base figure, 0..1 of a full turn. */
    rotation: number;
    /** Direction of the accent from the center, 0..1 of a full turn. */
    accentAngle: number;
    /** Size of the base figure, 0 = smaller, 1 = larger. */
    scale: number;
}
/**
 * Derives traits from a seed. Every axis consumes exactly one rng draw in a fixed
 * order, so overriding one axis never changes the others.
 */
export declare function deriveTraits(seed: string, overrides?: Partial<Traits>): Traits;
