import { palette } from "./colors.js";
import { buildFace } from "./faces.js";
import { createRng } from "./random.js";
import { buildBody } from "./shapes.js";
import { el, d } from "./svg.js";
function backdrop(background, fill) {
  switch (background) {
    case "none":
      return [];
    case "circle":
      return [el("circle", { cx: 50, cy: 50, r: 50, fill })];
    case "squircle":
      return [el("rect", { width: 100, height: 100, rx: 30, fill })];
    case "square":
      return [el("rect", { width: 100, height: 100, fill })];
  }
}
function buildTree(traits, options = {}) {
  const colors = palette(traits.hue, traits.tone);
  const body = buildBody(traits.shape, traits.shapeSeed);
  const lean = createRng(traits.shapeSeed).range(-5, 5);
  const { face, shine } = body;
  const rootAttrs = {
    xmlns: "http://www.w3.org/2000/svg",
    viewBox: "0 0 100 100"
  };
  if (options.size !== void 0) {
    rootAttrs.width = options.size;
    rootAttrs.height = options.size;
  }
  rootAttrs.role = "img";
  rootAttrs["aria-label"] = options.aria ?? "avatar";
  const bodyNode = { ...body.node, attrs: { ...body.node.attrs, fill: colors.body } };
  return el("svg", rootAttrs, [
    ...backdrop(options.background ?? "circle", colors.background),
    el("g", { transform: d`rotate(${lean} 50 56)` }, [
      bodyNode,
      el("ellipse", {
        transform: d`translate(${shine.x} ${shine.y})rotate(${shine.angle})`,
        rx: shine.rx,
        ry: shine.ry,
        fill: "#fff",
        opacity: 0.35
      }),
      el("g", { transform: d`translate(${face.x} ${face.y})scale(${face.scale})` }, buildFace(traits, colors))
    ])
  ]);
}
export {
  buildTree
};
