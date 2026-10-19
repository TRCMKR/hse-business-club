import { describe, expect, it } from 'vitest';
import { blobAvatar } from '../src/index';
import { PALETTES } from '../src/colors';
import { ACCENTS, deriveTraits, DETAILS, SHAPES } from '../src/traits';
import type { Traits } from '../src/traits';

describe('deriveTraits', () => {
  it('quantizes every axis into its variant set', () => {
    for (let i = 0; i < 300; i++) {
      const t = deriveTraits(`user-${i}`);
      expect(SHAPES).toContain(t.shape);
      expect(ACCENTS).toContain(t.accent);
      expect(DETAILS).toContain(t.detail);
      expect(Number.isInteger(t.palette)).toBe(true);
      expect(t.palette).toBeGreaterThanOrEqual(0);
      expect(t.palette).toBeLessThan(PALETTES.length);
      for (const value of [t.rotation, t.accentAngle, t.scale]) {
        expect(value).toBeGreaterThanOrEqual(0);
        expect(value).toBeLessThan(1);
      }
    }
  });

  it('covers all variants over many seeds', () => {
    const shapes = new Set<string>();
    const accents = new Set<string>();
    const details = new Set<string>();
    const palettes = new Set<number>();
    for (let i = 0; i < 1000; i++) {
      const t = deriveTraits(`seed${i}`);
      shapes.add(t.shape);
      accents.add(t.accent);
      details.add(t.detail);
      palettes.add(t.palette);
    }
    expect(shapes.size).toBe(SHAPES.length);
    expect(accents.size).toBe(ACCENTS.length);
    expect(details.size).toBe(DETAILS.length);
    expect(palettes.size).toBe(PALETTES.length);
  });

  it('applies overrides without shifting other axes', () => {
    const base = deriveTraits('vera');
    const pinned = deriveTraits('vera', { accent: 'ring', palette: 3 });
    expect(pinned).toEqual({ ...base, accent: 'ring', palette: 3 });
  });

  it('ignores overrides outside the known variants', () => {
    const bogus = {
      shape: 'triangle',
      accent: 'laser',
      detail: 'hat',
      palette: 12,
      rotation: 5,
      accentAngle: Number.NaN,
      scale: -0.1,
      shapeSeed: -1,
      expression: 'happy',
    } as unknown as Partial<Traits>;
    expect(deriveTraits('anna', bogus)).toEqual(deriveTraits('anna'));
    expect(blobAvatar('anna', { traits: bogus })).toBe(blobAvatar('anna'));
    for (const palette of [-1, 1.5, '2', Number.POSITIVE_INFINITY]) {
      expect(deriveTraits('anna', { palette } as unknown as Partial<Traits>)).toEqual(deriveTraits('anna'));
    }
  });

  it('ignores undefined overrides', () => {
    expect(deriveTraits('boris', { shape: undefined })).toEqual(deriveTraits('boris'));
  });
});
