import { el, d } from "./svg.js";
const STROKE = 2.4;
function geometry(traits) {
  const radius = 4.2 + traits.eyeSize * 2.6;
  return {
    eyeY: -4,
    spread: 9.5 + traits.eyeSpacing * 5.5,
    radius,
    tilt: (traits.eyeTilt - 0.5) * 24,
    gaze: (traits.pupilShift - 0.5) * 2 * radius * 0.32
  };
}
function line(path, ink, width = STROKE) {
  return el("path", {
    "d": path,
    "fill": "none",
    "stroke": ink,
    "stroke-width": width,
    "stroke-linecap": "round",
    "stroke-linejoin": "round"
  });
}
function lid(extent, colors) {
  const top = -extent - 1;
  const edge = -extent * 0.1;
  return [
    el("path", { d: d`M${-extent - 1} ${edge}H${extent + 1}V${top}H${-extent - 1}Z`, fill: colors.body }),
    line(d`M${-extent} ${edge}H${extent}`, colors.ink, 2)
  ];
}
function openEye(shape, g, expression, colors) {
  const r = g.radius;
  const pupilScale = expression === "surprised" ? 0.75 : 1;
  const lookUp = expression === "thinking" ? -r * 0.3 : 0;
  const nodes = [];
  let extent = r;
  let pupil = r * 0.46 * pupilScale;
  if (shape === "dot") {
    nodes.push(el("ellipse", { cx: g.gaze * 0.5, cy: lookUp * 0.5, rx: r * 0.58, ry: r * 0.74, fill: colors.ink }));
    nodes.push(el("circle", { cx: g.gaze * 0.5 + r * 0.2, cy: lookUp * 0.5 - r * 0.28, r: r * 0.2, fill: colors.sclera }));
    extent = r * 0.74;
  } else {
    if (shape === "wide") {
      extent = r * 1.12;
      pupil = r * 0.42 * pupilScale;
      nodes.push(el("circle", { r: extent, fill: colors.sclera }));
    } else {
      extent = r * 1.05;
      nodes.push(el("ellipse", { rx: r * 0.82, ry: extent, fill: colors.sclera }));
    }
    const px = g.gaze;
    const py = r * 0.12 + lookUp;
    nodes.push(el("circle", { cx: px, cy: py, r: pupil, fill: colors.ink }));
    nodes.push(el("circle", { cx: px + pupil * 0.38, cy: py - pupil * 0.4, r: pupil * 0.36, fill: colors.sclera }));
  }
  if (expression === "sleepy") {
    nodes.push(...lid(extent, colors));
  }
  return nodes;
}
function closedEye(g, colors) {
  const r = g.radius * 0.95;
  return [line(d`M${-r} ${r * 0.25}Q0 ${-r * 1.1} ${r} ${r * 0.25}`, colors.ink)];
}
function eyes(traits, g, colors) {
  const result = [];
  for (const side of [-1, 1]) {
    const x = side * g.spread;
    const angle = -side * g.tilt;
    const winking = traits.eyeShape === "wink" && side === 1;
    const shape = traits.eyeShape === "wink" ? "oval" : traits.eyeShape;
    const children = winking ? closedEye(g, colors) : openEye(shape, g, traits.expression, colors);
    result.push(el("g", { transform: d`translate(${x} ${g.eyeY})rotate(${angle})` }, children));
  }
  return result;
}
const BROWS = {
  happy: [null, null],
  neutral: [null, null],
  surprised: [[-2.5, -1.5], [-2.5, -1.5]],
  smirk: [[-2, -2.5], [0.5, 0.5]],
  sleepy: [null, null],
  mad: [[2.6, -1.6], [2.6, -1.6]],
  thinking: [[0.5, 0.5], [-2, -3]],
  cheeky: [null, null],
  sad: [[-2.4, 1.4], [-2.4, 1.4]],
  shy: [[-1.2, 0.8], [-1.2, 0.8]]
};
function brows(expression, g, colors) {
  const result = [];
  const pair = BROWS[expression];
  const baseY = g.eyeY - g.radius * 1.15 - 3.5;
  const half = Math.max(3.6, g.radius * 0.8);
  pair.forEach((brow, index) => {
    if (brow === null) {
      return;
    }
    const side = index === 0 ? -1 : 1;
    const cx = side * g.spread;
    const innerX = cx - side * half;
    const outerX = cx + side * half;
    const innerY = baseY + brow[0];
    const outerY = baseY + brow[1];
    const midY = (innerY + outerY) / 2 - 1.2;
    result.push(line(d`M${outerX} ${outerY}Q${cx} ${midY} ${innerX} ${innerY}`, colors.ink, 2.2));
  });
  return result;
}
function mouth(expression, colors) {
  const ink = colors.ink;
  switch (expression) {
    case "happy":
      return [el("path", { d: "M-8 9Q0 9 8 9Q7 19 0 19Q-7 19-8 9Z", fill: ink, stroke: ink, "stroke-width": 1.5, "stroke-linejoin": "round" })];
    case "neutral":
      return [line("M-5 12H5", ink)];
    case "surprised":
      return [el("ellipse", { cx: 0, cy: 13, rx: 3.6, ry: 4.6, fill: ink })];
    case "smirk":
      return [line("M-6 12.5Q1 14.5 7 9", ink)];
    case "sleepy":
      return [el("ellipse", { cx: 0, cy: 12.5, rx: 2.6, ry: 2, fill: ink })];
    case "mad":
      return [line("M-7 13.5Q-3.5 10 0 13.5T7 13.5", ink)];
    case "thinking":
      return [line("M-1 12.5L6 11", ink)];
    case "cheeky":
      return [
        el("path", { d: "M0 12.5V16.5A3.2 3.2 0 0 0 6.4 16.5V12", fill: colors.tongue, stroke: ink, "stroke-width": 1.6 }),
        line("M-7 9.5Q0 16.5 7 9.5", ink)
      ];
    case "sad":
      return [line("M-6 15Q0 9 6 15", ink)];
    case "shy":
      return [line("M-3.5 12Q0 14.5 3.5 12", ink, 2)];
  }
}
function cheeks(extras, expression, g, colors) {
  const x = g.spread + 3;
  const y = 6;
  const result = [];
  if (extras === "blush" || expression === "shy") {
    for (const side of [-1, 1]) {
      result.push(el("ellipse", { cx: side * x, cy: y, rx: 4.6, ry: 2.6, fill: colors.cheek, opacity: 0.7 }));
    }
  }
  if (extras === "freckles") {
    for (const side of [-1, 1]) {
      for (const [dx, dy] of [[-1.8, -0.6], [1.6, -1], [0, 1.6]]) {
        result.push(el("circle", { cx: side * x + dx, cy: y + dy, r: 0.95, fill: colors.shade }));
      }
    }
  }
  return result;
}
function buildFace(traits, colors) {
  const g = geometry(traits);
  return [
    ...cheeks(traits.extras, traits.expression, g, colors),
    ...eyes(traits, g, colors),
    ...brows(traits.expression, g, colors),
    ...mouth(traits.expression, colors)
  ];
}
export {
  buildFace
};
