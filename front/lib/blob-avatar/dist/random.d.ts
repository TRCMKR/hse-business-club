/**
 * Deterministic pseudo-randomness. Nothing in this package may use Math.random:
 * the same seed must give byte-identical output in Node (SSR) and in the browser.
 */
/** Normalizes a seed so that "Anna " and "anna" produce the same avatar. */
export declare function normalizeSeed(seed: string): string;
/** FNV-1a 32-bit hash over UTF-16 code units, returned as an unsigned integer. */
export declare function hashString(seed: string): number;
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
export declare function createRng(seed: number): Rng;
