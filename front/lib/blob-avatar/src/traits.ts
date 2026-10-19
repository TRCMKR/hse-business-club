import { createRng, hashString, normalizeSeed } from './random';

export const SHAPES = ['round', 'squircle', 'blob', 'droplet', 'hexagon', 'cloud', 'capsule', 'triangle'] as const;
export const HUES = [12, 30, 50, 140, 175, 210, 260, 320] as const;
export const TONES = ['light', 'dark'] as const;
export const EYE_SHAPES = ['dot', 'oval', 'wide', 'wink'] as const;
export const EXPRESSIONS = [
  'happy',
  'neutral',
  'surprised',
  'smirk',
  'sleepy',
  'mad',
  'thinking',
  'cheeky',
  'sad',
  'shy',
] as const;
export const EXTRAS = ['blush', 'freckles', 'none'] as const;

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

function quantize<T>(items: readonly T[], value: number): T {
  return items[Math.min(items.length - 1, Math.floor(value * items.length))] as T;
}

/**
 * Derives traits from a seed. Every axis consumes exactly one rng draw in a fixed
 * order, so overriding one axis never changes the others.
 */
export function deriveTraits(seed: string, overrides: Partial<Traits> = {}): Traits {
  const rng = createRng(hashString(normalizeSeed(seed)));
  const derived: Traits = {
    shape: quantize(SHAPES, rng.next()),
    shapeSeed: Math.floor(rng.next() * 4294967296) >>> 0,
    hue: quantize(HUES, rng.next()),
    tone: quantize(TONES, rng.next()),
    eyeShape: quantize(EYE_SHAPES, rng.next()),
    eyeSize: rng.next(),
    eyeSpacing: rng.next(),
    eyeTilt: rng.next(),
    pupilShift: rng.next(),
    expression: quantize(EXPRESSIONS, rng.next()),
    extras: quantize(EXTRAS, rng.next()),
  };
  const result: Traits = { ...derived };
  for (const key of Object.keys(overrides) as (keyof Traits)[]) {
    const value = overrides[key];
    if (value !== undefined) {
      (result as unknown as Record<string, unknown>)[key] = value;
    }
  }
  return result;
}
