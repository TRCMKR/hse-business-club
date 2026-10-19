export declare const SHAPES: readonly ["round", "squircle", "blob", "droplet", "hexagon", "cloud", "capsule", "triangle"];
export declare const HUES: readonly [12, 30, 50, 140, 175, 210, 260, 320];
export declare const TONES: readonly ["light", "dark"];
export declare const EYE_SHAPES: readonly ["dot", "oval", "wide", "wink"];
export declare const EXPRESSIONS: readonly ["happy", "neutral", "surprised", "smirk", "sleepy", "mad", "thinking", "cheeky", "sad", "shy"];
export declare const EXTRAS: readonly ["blush", "freckles", "none"];
export type Shape = (typeof SHAPES)[number];
export type Hue = (typeof HUES)[number];
export type Tone = (typeof TONES)[number];
export type EyeShape = (typeof EYE_SHAPES)[number];
export type Expression = (typeof EXPRESSIONS)[number];
export type Extras = (typeof EXTRAS)[number];
/**
 * Everything that defines how an avatar looks. Continuous axes are normalized to
 * [0, 1); the renderer maps them to geometry. Two equal Traits render the same SVG.
 */
export interface Traits {
    /** Body silhouette. */
    shape: Shape;
    /** Seed for silhouette details (blob outline, slight body tilt), uint32. */
    shapeSeed: number;
    /** Base hue in degrees, one of HUES. */
    hue: Hue;
    /** Lightness family of body and background. */
    tone: Tone;
    eyeShape: EyeShape;
    /** Eye size, 0 = small, 1 = big. */
    eyeSize: number;
    /** Distance between eyes, 0 = close, 1 = far. */
    eyeSpacing: number;
    /** Eye rotation, 0 = tilted outward-down, 0.5 = straight, 1 = tilted inward-down. */
    eyeTilt: number;
    /** Horizontal gaze, 0 = left, 0.5 = center, 1 = right. */
    pupilShift: number;
    /** Mouth and brows. */
    expression: Expression;
    extras: Extras;
}
/**
 * Derives traits from a seed. Every axis consumes exactly one rng draw in a fixed
 * order, so overriding one axis never changes the others.
 */
export declare function deriveTraits(seed: string, overrides?: Partial<Traits>): Traits;
