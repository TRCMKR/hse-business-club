import { describe, expect, it } from 'vitest';
import { createRng, hashString, normalizeSeed } from '../src/random';

describe('random', () => {
  it('normalizes seeds', () => {
    expect(normalizeSeed('  Anna ')).toBe('anna');
  });

  it('hashes to a stable unsigned 32-bit integer', () => {
    const hash = hashString('anna');
    expect(hash).toBe(hashString('anna'));
    expect(Number.isInteger(hash)).toBe(true);
    expect(hash).toBeGreaterThanOrEqual(0);
    expect(hash).toBeLessThan(2 ** 32);
    expect(hashString('anna')).not.toBe(hashString('annb'));
  });

  it('produces a reproducible sequence in [0, 1)', () => {
    const a = createRng(42);
    const b = createRng(42);
    for (let i = 0; i < 1000; i++) {
      const value = a.next();
      expect(value).toBe(b.next());
      expect(value).toBeGreaterThanOrEqual(0);
      expect(value).toBeLessThan(1);
    }
  });

  it('picks, ranges and ints stay in bounds', () => {
    const rng = createRng(7);
    const seen = new Set<string>();
    for (let i = 0; i < 500; i++) {
      seen.add(rng.pick(['a', 'b', 'c']));
      const r = rng.range(-3, 5);
      expect(r).toBeGreaterThanOrEqual(-3);
      expect(r).toBeLessThan(5);
      const n = rng.int(6, 9);
      expect(n).toBeGreaterThanOrEqual(6);
      expect(n).toBeLessThanOrEqual(9);
    }
    expect(seen).toEqual(new Set(['a', 'b', 'c']));
    expect(() => rng.pick([])).toThrow();
  });
});
