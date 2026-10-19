import { createRng } from './random';
import { d, el } from './svg';
import type { SvgNode } from './svg';
import type { Shape } from './traits';

interface Point {
  x: number;
  y: number;
}

/** Nominal radius of every base figure in local coordinates (origin at its center). */
export const FIGURE_RADIUS = 30;

/** Closed polygon with corners rounded by quadratic curves. */
function roundedPolygon(points: Point[], radius: number): string {
  const n = points.length;
  let path = '';
  for (let i = 0; i < n; i++) {
    const prev = points[(i - 1 + n) % n] as Point;
    const cur = points[i] as Point;
    const next = points[(i + 1) % n] as Point;
    const toward = (from: Point, to: Point): Point => {
      const len = Math.hypot(to.x - from.x, to.y - from.y);
      const t = Math.min(radius, len / 2) / len;
      return { x: from.x + (to.x - from.x) * t, y: from.y + (to.y - from.y) * t };
    };
    const a = toward(cur, prev);
    const b = toward(cur, next);
    path += i === 0 ? d`M${a.x} ${a.y}` : d`L${a.x} ${a.y}`;
    path += d`Q${cur.x} ${cur.y} ${b.x} ${b.y}`;
  }
  return `${path}Z`;
}

/** Closed smooth curve through points: Catmull-Rom converted to cubic Beziers. */
function smoothClosed(points: Point[]): string {
  const n = points.length;
  const at = (i: number): Point => points[((i % n) + n) % n] as Point;
  let path = d`M${at(0).x} ${at(0).y}`;
  for (let i = 0; i < n; i++) {
    const p0 = at(i - 1);
    const p1 = at(i);
    const p2 = at(i + 1);
    const p3 = at(i + 2);
    path += d`C${p1.x + (p2.x - p0.x) / 6} ${p1.y + (p2.y - p0.y) / 6} ${p2.x - (p3.x - p1.x) / 6} ${p2.y - (p3.y - p1.y) / 6} ${p2.x} ${p2.y}`;
  }
  return `${path}Z`;
}

/** Calm organic outline: few points, small radial jitter. */
function blobOutline(shapeSeed: number): string {
  const rng = createRng(shapeSeed ^ 0x9e3779b9);
  const count = rng.int(5, 7);
  const step = (Math.PI * 2) / count;
  const start = rng.range(0, step);
  const points: Point[] = [];
  for (let i = 0; i < count; i++) {
    const angle = start + i * step + rng.range(-0.15, 0.15) * step;
    const radius = rng.range(27, 32);
    points.push({ x: Math.cos(angle) * radius, y: Math.sin(angle) * radius });
  }
  return smoothClosed(points);
}

/** Base figure centered at the origin, about FIGURE_RADIUS in size; the caller sets fill and transform. */
export function buildShape(shape: Shape, shapeSeed: number): SvgNode {
  const r = FIGURE_RADIUS;
  switch (shape) {
    case 'round':
      return el('circle', { r });
    case 'squircle':
      return el('rect', { x: -27, y: -27, width: 54, height: 54, rx: 17 });
    case 'blob':
      return el('path', { d: blobOutline(shapeSeed) });
    case 'droplet':
      // Circle with one sharp corner, like a leaf or a map pin.
      return el('path', { d: 'M-27-27H0A27 27 0 0 1 27 0A27 27 0 0 1 0 27A27 27 0 0 1-27 0Z' });
    case 'hexagon': {
      const points: Point[] = [];
      for (let i = 0; i < 6; i++) {
        const angle = (Math.PI / 3) * i + Math.PI / 6;
        points.push({ x: Math.cos(angle) * 31, y: Math.sin(angle) * 31 });
      }
      return el('path', { d: roundedPolygon(points, 8) });
    }
    case 'capsule':
      return el('rect', { x: -19, y: -31, width: 38, height: 62, rx: 19 });
  }
}
