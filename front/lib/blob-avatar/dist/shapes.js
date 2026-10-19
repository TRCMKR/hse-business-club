import { createRng } from "./random.js";
import { el, d } from "./svg.js";
function roundedPolygon(points, radius) {
  const n = points.length;
  let path = "";
  for (let i = 0; i < n; i++) {
    const prev = points[(i - 1 + n) % n];
    const cur = points[i];
    const next = points[(i + 1) % n];
    const toward = (from, to) => {
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
function smoothClosed(points) {
  const n = points.length;
  const at = (i) => points[(i % n + n) % n];
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
function blobOutline(shapeSeed) {
  const rng = createRng(shapeSeed ^ 2654435769);
  const count = rng.int(6, 9);
  const step = Math.PI * 2 / count;
  const start = rng.range(0, step);
  const points = [];
  for (let i = 0; i < count; i++) {
    const angle = start + i * step + rng.range(-0.22, 0.22) * step;
    const radius = rng.range(29, 36);
    points.push({ x: 50 + Math.cos(angle) * radius, y: 54 + Math.sin(angle) * radius * 0.94 });
  }
  return smoothClosed(points);
}
function buildBody(shape, shapeSeed) {
  switch (shape) {
    case "round":
      return {
        node: el("circle", { cx: 50, cy: 54, r: 33 }),
        face: { x: 50, y: 53, scale: 1.2 },
        shine: { x: 36, y: 35, rx: 7, ry: 4, angle: -35 }
      };
    case "squircle":
      return {
        node: el("rect", { x: 17, y: 21, width: 66, height: 66, rx: 24 }),
        face: { x: 50, y: 54, scale: 1.2 },
        shine: { x: 32, y: 32, rx: 6, ry: 3.5, angle: -40 }
      };
    case "blob":
      return {
        node: el("path", { d: blobOutline(shapeSeed) }),
        face: { x: 50, y: 54, scale: 1.12 },
        shine: { x: 37, y: 38, rx: 6, ry: 3.5, angle: -35 }
      };
    case "droplet":
      return {
        node: el("path", { d: "M50 13C60 29 83 43 83 62C83 80 68 89 50 89C32 89 17 80 17 62C17 43 40 29 50 13Z" }),
        face: { x: 50, y: 62, scale: 1.1 },
        shine: { x: 38, y: 44, rx: 5, ry: 3, angle: -55 }
      };
    case "hexagon": {
      const points = [];
      for (let i = 0; i < 6; i++) {
        const angle = Math.PI / 3 * i;
        points.push({ x: 50 + Math.cos(angle) * 37, y: 54 + Math.sin(angle) * 35 });
      }
      return {
        node: el("path", { d: roundedPolygon(points, 9) }),
        face: { x: 50, y: 55, scale: 1.15 },
        shine: { x: 37, y: 30, rx: 6, ry: 3, angle: -20 }
      };
    }
    case "cloud":
      return {
        node: el("g", {}, [
          el("circle", { cx: 33, cy: 55, r: 17 }),
          el("circle", { cx: 51, cy: 42, r: 21 }),
          el("circle", { cx: 69, cy: 54, r: 16 }),
          el("rect", { x: 16, y: 54, width: 68, height: 32, rx: 16 })
        ]),
        face: { x: 50, y: 60, scale: 1.12 },
        shine: { x: 42, y: 29, rx: 6, ry: 3, angle: -25 }
      };
    case "capsule":
      return {
        node: el("rect", { x: 24, y: 14, width: 52, height: 76, rx: 26 }),
        face: { x: 50, y: 48, scale: 1 },
        shine: { x: 36, y: 28, rx: 4.5, ry: 3, angle: -50 }
      };
    case "triangle":
      return {
        node: el("path", { d: roundedPolygon([{ x: 50, y: 12 }, { x: 90, y: 84 }, { x: 10, y: 84 }], 11) }),
        face: { x: 50, y: 67, scale: 0.88 },
        shine: { x: 42, y: 32, rx: 4.5, ry: 2.5, angle: -60 }
      };
  }
}
export {
  buildBody
};
