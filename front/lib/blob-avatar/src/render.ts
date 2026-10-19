import { PALETTES } from './colors';
import type { Palette } from './colors';
import { buildShape, FIGURE_RADIUS } from './shapes';
import { d, el } from './svg';
import type { SvgNode } from './svg';
import type { Traits } from './traits';

export type Background = 'none' | 'circle' | 'squircle' | 'square';

export interface RenderOptions {
  /** Width and height in pixels. Without it the SVG has no size attributes and fills its box. */
  size?: number;
  /** Backdrop behind the figure, `circle` by default. */
  background?: Background;
  /** Accessible label (aria-label). */
  aria?: string;
}

function backdrop(background: Background, fill: string): SvgNode[] {
  switch (background) {
    case 'none':
      return [];
    case 'circle':
      return [el('circle', { cx: 50, cy: 50, r: 50, fill })];
    case 'squircle':
      return [el('rect', { width: 100, height: 100, rx: 30, fill })];
    case 'square':
      return [el('rect', { width: 100, height: 100, fill })];
  }
}

interface Layout {
  /** Figure center. */
  x: number;
  y: number;
  /** Figure rotation in degrees. */
  angle: number;
  /** Figure scale. */
  scale: number;
  /** Unit vector of the accent direction. */
  dx: number;
  dy: number;
  /** Effective figure radius in viewBox units. */
  radius: number;
}

/** How far the figure moves away from an accent that sticks out, to keep the whole centered. */
const BALANCE: Record<Traits['accent'], number> = {
  orbit: 4,
  ring: 0,
  arc: 0,
  inset: 0,
  overlap: 6,
  split: 0,
};

function layout(traits: Traits): Layout {
  const theta = traits.accentAngle * Math.PI * 2;
  const dx = Math.cos(theta);
  const dy = Math.sin(theta);
  const scale = 0.88 + traits.scale * 0.14;
  const shift = BALANCE[traits.accent];
  return {
    x: 50 - dx * shift,
    y: 50 - dy * shift,
    angle: traits.rotation * 360,
    scale,
    dx,
    dy,
    radius: FIGURE_RADIUS * scale,
  };
}

function figure(traits: Traits, fill: string, transform: string): SvgNode {
  const node = buildShape(traits.shape, traits.shapeSeed);
  return { ...node, attrs: { ...node.attrs, fill, transform } };
}

function accent(traits: Traits, l: Layout, colors: Palette): SvgNode[] {
  const { x, y, dx, dy, radius } = l;
  switch (traits.accent) {
    case 'orbit': {
      const dist = radius * 0.82;
      return [el('circle', { cx: x + dx * dist, cy: y + dy * dist, r: radius * 0.4, fill: colors.accent })];
    }
    case 'ring': {
      const off = radius * 0.3;
      return [el('circle', {
        'cx': x + dx * off,
        'cy': y + dy * off,
        'r': radius * 0.36,
        'fill': 'none',
        'stroke': colors.accent,
        'stroke-width': radius * 0.17,
      })];
    }
    case 'arc': {
      // Half ring opening away from the accent direction.
      const r = radius * 0.52;
      const px = -dy;
      const py = dx;
      return [el('path', {
        'd': d`M${x + px * r} ${y + py * r}A${r} ${r} 0 0 0 ${x - px * r} ${y - py * r}`,
        'fill': 'none',
        'stroke': colors.accent,
        'stroke-width': radius * 0.2,
        'stroke-linecap': 'round',
        'transform': d`translate(${dx * radius * 0.08} ${dy * radius * 0.08})`,
      })];
    }
    case 'inset': {
      const off = radius * 0.24;
      return [figure(traits, colors.accent, d`translate(${x + dx * off} ${y + dy * off})rotate(${l.angle})scale(${l.scale * 0.42})`)];
    }
    case 'overlap': {
      const dist = radius * 0.62;
      return [el('circle', {
        cx: x + dx * dist,
        cy: y + dy * dist,
        r: radius * 0.72,
        fill: colors.accent,
        opacity: 0.5,
      })];
    }
    case 'split': {
      // Half disc whose flat side faces the accent direction.
      const r = radius * 0.5;
      const px = -dy;
      const py = dx;
      return [el('path', {
        d: d`M${x + px * r} ${y + py * r}A${r} ${r} 0 0 0 ${x - px * r} ${y - py * r}Z`,
        fill: colors.accent,
      })];
    }
  }
}

function detail(traits: Traits, l: Layout, colors: Palette): SvgNode[] {
  // Roughly opposite to the accent so the two never collide.
  switch (traits.detail) {
    case 'none':
      return [];
    case 'satellite': {
      const theta = traits.accentAngle * Math.PI * 2 + Math.PI * 0.8;
      return [el('circle', { cx: 50 + Math.cos(theta) * 43, cy: 50 + Math.sin(theta) * 43, r: 3.6, fill: colors.accent })];
    }
    case 'trail': {
      // Thin arc hugging the figure on the side opposite to the accent.
      const theta = traits.accentAngle * Math.PI * 2 + Math.PI;
      const r = l.radius * 1.08 + 5;
      const from = theta - Math.PI * 0.3;
      const to = theta + Math.PI * 0.3;
      return [el('path', {
        'd': d`M${l.x + Math.cos(from) * r} ${l.y + Math.sin(from) * r}A${r} ${r} 0 0 1 ${l.x + Math.cos(to) * r} ${l.y + Math.sin(to) * r}`,
        'fill': 'none',
        'stroke': colors.accent,
        'stroke-width': 3,
        'stroke-linecap': 'round',
      })];
    }
  }
}

/** Builds the SVG element tree for the given traits. */
export function buildTree(traits: Traits, options: RenderOptions = {}): SvgNode {
  const colors = PALETTES[traits.palette] ?? (PALETTES[0] as Palette);
  const l = layout(traits);

  const rootAttrs: Record<string, string | number> = {
    xmlns: 'http://www.w3.org/2000/svg',
    viewBox: '0 0 100 100',
  };
  if (options.size !== undefined) {
    rootAttrs.width = options.size;
    rootAttrs.height = options.size;
  }
  rootAttrs.role = 'img';
  rootAttrs['aria-label'] = options.aria ?? 'avatar';

  return el('svg', rootAttrs, [
    ...backdrop(options.background ?? 'circle', colors.background),
    figure(traits, colors.figure, d`translate(${l.x} ${l.y})rotate(${l.angle})scale(${l.scale})`),
    ...accent(traits, l, colors),
    ...detail(traits, l, colors),
  ]);
}
