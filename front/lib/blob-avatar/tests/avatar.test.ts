import { describe, expect, it } from 'vitest';
import { blobAvatar, blobAvatarTree, deriveTraits, EXPRESSIONS, MAX_LABEL_LENGTH, EXTRAS, EYE_SHAPES, SHAPES, TONES } from '../src/index';
import type { SvgNode } from '../src/index';

const DEMO_USERS = ['anna', 'boris', 'vera', 'dmitry'];

function parse(svg: string): Document {
  return new DOMParser().parseFromString(svg, 'image/svg+xml');
}

function assertValid(svg: string): void {
  expect(svg).not.toMatch(/NaN|undefined|Infinity|null/);
  const doc = parse(svg);
  expect(doc.getElementsByTagName('parsererror')).toHaveLength(0);
  expect(doc.documentElement.tagName).toBe('svg');
  expect(doc.querySelectorAll('[id], style, script, [href], [xlink\\:href]')).toHaveLength(0);
  expect(svg.length).toBeLessThan(4096);
}

describe('blobAvatar', () => {
  it('is deterministic and normalizes the seed', () => {
    expect(blobAvatar('Anna ')).toBe(blobAvatar('anna'));
    expect(blobAvatar('  ANNA')).toBe(blobAvatar('anna'));
    expect(blobAvatar('anna')).toBe(blobAvatar('anna'));
  });

  it('gives different avatars to different seeds', () => {
    const svgs = new Set<string>();
    for (let i = 0; i < 200; i++) {
      // The label is the seed itself, so compare without it.
      svgs.add(blobAvatar(`random-user-${i * 7919}`, { aria: 'avatar' }));
    }
    expect(svgs.size).toBeGreaterThanOrEqual(195);
  });

  it('renders every shape x expression x eyeShape x tone combination', () => {
    let count = 0;
    for (const shape of SHAPES) {
      for (const expression of EXPRESSIONS) {
        for (const eyeShape of EYE_SHAPES) {
          for (const tone of TONES) {
            const extras = EXTRAS[count % EXTRAS.length];
            assertValid(blobAvatar(`combo-${count}`, { traits: { shape, expression, eyeShape, tone, extras } }));
            count++;
          }
        }
      }
    }
    expect(count).toBe(8 * 10 * 4 * 2);
  });

  it('stays under 4 KB in the heaviest configuration', () => {
    for (let i = 0; i < 200; i++) {
      const svg = blobAvatar(`heavy-${i}`, {
        size: 128,
        background: 'squircle',
        aria: 'Очень длинное имя пользователя для подписи',
        traits: { shape: 'blob', expression: 'cheeky', eyeShape: 'wide', extras: 'freckles' },
      });
      assertValid(svg);
    }
  });

  it('keeps continuous extremes valid', () => {
    for (const value of [0, 0.999999]) {
      assertValid(blobAvatar('edge', {
        traits: { eyeSize: value, eyeSpacing: value, eyeTilt: value, pupilShift: value, expression: 'sleepy' },
      }));
    }
  });

  it('supports size, background and aria options', () => {
    const svg = blobAvatar('anna', { size: 48, background: 'none', aria: 'Анна <&> "С"' });
    const root = parse(svg).documentElement;
    expect(root.getAttribute('width')).toBe('48');
    expect(root.getAttribute('height')).toBe('48');
    expect(root.getAttribute('viewBox')).toBe('0 0 100 100');
    expect(root.getAttribute('role')).toBe('img');
    expect(root.getAttribute('aria-label')).toBe('Анна <&> "С"');

    const fluid = parse(blobAvatar('anna')).documentElement;
    expect(fluid.hasAttribute('width')).toBe(false);
    expect(fluid.getAttribute('aria-label')).toBe('anna');

    const backgrounds = (['none', 'circle', 'squircle', 'square'] as const).map((background) => blobAvatar('anna', { background }));
    expect(new Set(backgrounds).size).toBe(4);
  });

  it('falls back to "avatar" for an empty label and truncates long ones', () => {
    expect(parse(blobAvatar('   ')).documentElement.getAttribute('aria-label')).toBe('avatar');
    expect(parse(blobAvatar('x', { aria: '' })).documentElement.getAttribute('aria-label')).toBe('avatar');
    const long = 'a'.repeat(100_000);
    const svg = blobAvatar(long);
    expect(parse(svg).documentElement.getAttribute('aria-label')).toBe('a'.repeat(MAX_LABEL_LENGTH));
    expect(svg.length).toBeLessThan(4096);
    expect(blobAvatar('x', { aria: long }).length).toBeLessThan(4096);
  });

  it('exposes the tree used for rendering', () => {
    const tree: SvgNode = blobAvatarTree('boris');
    expect(tree.tag).toBe('svg');
    expect(tree.attrs.role).toBe('img');
    expect(tree.children.length).toBeGreaterThan(0);
  });

  it.each(DEMO_USERS)('keeps the face of demo user %s', (username) => {
    expect(deriveTraits(username)).toMatchSnapshot('traits');
    expect(blobAvatar(username)).toMatchSnapshot('svg');
  });
});
