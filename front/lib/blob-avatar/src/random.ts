/**
 * Deterministic pseudo-randomness. Nothing in this package may use Math.random:
 * the same seed must give byte-identical output in Node (SSR) and in the browser.
 */

/** Normalizes a seed so that "Anna " and "anna" produce the same avatar. */
export function normalizeSeed(seed: string): string {
  return seed.trim().toLowerCase();
}

/** FNV-1a 32-bit hash over UTF-16 code units, returned as an unsigned integer. */
export function hashString(seed: string): number {
  let hash = 0x811c9dc5;
  for (let i = 0; i < seed.length; i++) {
    hash ^= seed.charCodeAt(i);
    hash = Math.imul(hash, 0x01000193);
  }
  // Final avalanche (murmur3 fmix32) so short, similar seeds spread over the whole range.
  hash ^= hash >>> 16;
  hash = Math.imul(hash, 0x85ebca6b);
  hash ^= hash >>> 13;
  hash = Math.imul(hash, 0xc2b2ae35);
  hash ^= hash >>> 16;
  return hash >>> 0;
}

export interface Rng {
  /** Next float in [0, 1). */
  next(): number;
  /** Uniformly picks an element of a non-empty array. */
  pick<T>(items: readonly T[]): T;
  /** Uniform float in [min, max). */
  range(min: number, max: number): number;
  /** Uniform integer in [min, max] (inclusive). */
  int(min: number, max: number): number;
}

/** mulberry32 generator seeded with a 32-bit integer. */
export function createRng(seed: number): Rng {
  let state = seed >>> 0;
  const next = (): number => {
    state = (state + 0x6d2b79f5) >>> 0;
    let t = state;
    t = Math.imul(t ^ (t >>> 15), t | 1);
    t ^= t + Math.imul(t ^ (t >>> 7), t | 61);
    return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
  };
  return {
    next,
    pick<T>(items: readonly T[]): T {
      if (items.length === 0) {
        throw new Error('pick() requires a non-empty array');
      }
      return items[Math.floor(next() * items.length)] as T;
    },
    range(min: number, max: number): number {
      return min + next() * (max - min);
    },
    int(min: number, max: number): number {
      return min + Math.floor(next() * (max - min + 1));
    },
  };
}
