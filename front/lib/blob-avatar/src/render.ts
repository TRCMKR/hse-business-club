import { palette } from './colors';
import { buildFace } from './faces';
import { createRng } from './random';
import { buildBody } from './shapes';
import { d, el } from './svg';
import type { SvgNode } from './svg';
import type { Traits } from './traits';

export type Background = 'none' | 'circle' | 'squircle' | 'square';

export interface RenderOptions {
  /** Width and height in pixels. Without it the SVG has no size attributes and fills its box. */
  size?: number;
  /** Backdrop behind the creature, `circle` by default. */
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

/** Builds the SVG element tree for the given traits. */
export function buildTree(traits: Traits, options: RenderOptions = {}): SvgNode {
  const colors = palette(traits.hue, traits.tone);
  const body = buildBody(traits.shape, traits.shapeSeed);
  // Slight body tilt, derived from the shape seed so it does not consume a trait draw.
  const lean = createRng(traits.shapeSeed).range(-5, 5);
  const { face, shine } = body;

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

  const bodyNode: SvgNode = { ...body.node, attrs: { ...body.node.attrs, fill: colors.body } };
  return el('svg', rootAttrs, [
    ...backdrop(options.background ?? 'circle', colors.background),
    el('g', { transform: d`rotate(${lean} 50 56)` }, [
      bodyNode,
      el('ellipse', {
        transform: d`translate(${shine.x} ${shine.y})rotate(${shine.angle})`,
        rx: shine.rx,
        ry: shine.ry,
        fill: '#fff',
        opacity: 0.35,
      }),
      el('g', { transform: d`translate(${face.x} ${face.y})scale(${face.scale})` }, buildFace(traits, colors)),
    ]),
  ]);
}
