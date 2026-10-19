import { describe, expect, it } from 'vitest';
import { blobAvatar } from '../src/index';
import { deriveTraits, EXPRESSIONS, EXTRAS, EYE_SHAPES, HUES, SHAPES, TONES } from '../src/traits';
import type { Traits } from '../src/traits';

describe('deriveTraits', () => {
  it('quantizes every axis into its variant set', () => {
    for (let i = 0; i < 300; i++) {
      const t = deriveTraits(`user-${i}`);
      expect(SHAPES).toContain(t.shape);
      expect(HUES).toContain(t.hue);
      expect(TONES).toContain(t.tone);
      expect(EYE_SHAPES).toContain(t.eyeShape);
      expect(EXPRESSIONS).toContain(t.expression);
      expect(EXTRAS).toContain(t.extras);
      for (const value of [t.eyeSize, t.eyeSpacing, t.eyeTilt, t.pupilShift]) {
        expect(value).toBeGreaterThanOrEqual(0);
        expect(value).toBeLessThan(1);
      }
    }
  });

  it('covers all variants over many seeds', () => {
    const shapes = new Set<string>();
    const expressions = new Set<string>();
    for (let i = 0; i < 1000; i++) {
      const t = deriveTraits(`seed${i}`);
      shapes.add(t.shape);
      expressions.add(t.expression);
    }
    expect(shapes.size).toBe(SHAPES.length);
    expect(expressions.size).toBe(EXPRESSIONS.length);
  });

  it('applies overrides without shifting other axes', () => {
    const base = deriveTraits('vera');
    const pinned = deriveTraits('vera', { expression: 'happy', hue: 210 });
    expect(pinned).toEqual({ ...base, expression: 'happy', hue: 210 });
  });

  it('ignores overrides outside the known variants', () => {
    const bogus = { shape: 'bogus', eyeShape: 'laser', expression: 'angry', hue: 13, tone: 'grey', extras: 'hat', eyeSize: 5, pupilShift: Number.NaN, shapeSeed: -1 } as unknown as Partial<Traits>;
    expect(deriveTraits('anna', bogus)).toEqual(deriveTraits('anna'));
    expect(blobAvatar('anna', { traits: bogus })).toBe(blobAvatar('anna'));
  });

  it('ignores undefined overrides', () => {
    expect(deriveTraits('boris', { shape: undefined })).toEqual(deriveTraits('boris'));
  });
});
