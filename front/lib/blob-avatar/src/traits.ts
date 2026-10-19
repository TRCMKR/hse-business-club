import { PALETTES } from './colors';
import { createRng, hashString, normalizeSeed } from './random';

export const SHAPES = ['round', 'squircle', 'blob', 'droplet', 'hexagon', 'capsule'] as const;
export const ACCENTS = ['orbit', 'ring', 'arc', 'inset', 'overlap', 'split'] as const;
export const DETAILS = ['none', 'satellite', 'trail'] as const;

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
    palette: Math.min(PALETTES.length - 1, Math.floor(rng.next() * PALETTES.length)),
    accent: quantize(ACCENTS, rng.next()),
    detail: quantize(DETAILS, rng.next()),
    rotation: rng.next(),
    accentAngle: rng.next(),
    scale: rng.next(),
  };
  const result: Traits = { ...derived };
  for (const key of Object.keys(overrides) as (keyof Traits)[]) {
    const value = overrides[key];
    if (value !== undefined && isValid(key, value)) {
      (result as unknown as Record<string, unknown>)[key] = value;
    }
  }
  return result;
}

const VARIANTS: Partial<Record<keyof Traits, readonly unknown[]>> = {
  shape: SHAPES,
  accent: ACCENTS,
  detail: DETAILS,
};

const UNIT_AXES: readonly (keyof Traits)[] = ['rotation', 'accentAngle', 'scale'];

function isInteger(value: unknown, max: number): boolean {
  return typeof value === 'number' && Number.isInteger(value) && value >= 0 && value <= max;
}

/** Overrides outside the known variants or ranges are ignored (the seed-derived value wins). */
function isValid(key: keyof Traits, value: unknown): boolean {
  const variants = VARIANTS[key];
  if (variants) {
    return variants.includes(value);
  }
  if (UNIT_AXES.includes(key)) {
    return typeof value === 'number' && value >= 0 && value <= 1;
  }
  if (key === 'palette') {
    return isInteger(value, PALETTES.length - 1);
  }
  if (key === 'shapeSeed') {
    return isInteger(value, 0xffffffff);
  }
  return false;
}
