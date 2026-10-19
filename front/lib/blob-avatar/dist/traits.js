import { PALETTES } from "./colors.js";
import { createRng, hashString, normalizeSeed } from "./random.js";
const SHAPES = ["round", "squircle", "blob", "droplet", "hexagon", "capsule"];
const ACCENTS = ["orbit", "ring", "arc", "inset", "overlap", "split"];
const DETAILS = ["none", "satellite", "trail"];
function quantize(items, value) {
  return items[Math.min(items.length - 1, Math.floor(value * items.length))];
}
function deriveTraits(seed, overrides = {}) {
  const rng = createRng(hashString(normalizeSeed(seed)));
  const derived = {
    shape: quantize(SHAPES, rng.next()),
    shapeSeed: Math.floor(rng.next() * 4294967296) >>> 0,
    palette: Math.min(PALETTES.length - 1, Math.floor(rng.next() * PALETTES.length)),
    accent: quantize(ACCENTS, rng.next()),
    detail: quantize(DETAILS, rng.next()),
    rotation: rng.next(),
    accentAngle: rng.next(),
    scale: rng.next()
  };
  const result = { ...derived };
  for (const key of Object.keys(overrides)) {
    const value = overrides[key];
    if (value !== void 0 && isValid(key, value)) {
      result[key] = value;
    }
  }
  return result;
}
const VARIANTS = {
  shape: SHAPES,
  accent: ACCENTS,
  detail: DETAILS
};
const UNIT_AXES = ["rotation", "accentAngle", "scale"];
function isInteger(value, max) {
  return typeof value === "number" && Number.isInteger(value) && value >= 0 && value <= max;
}
function isValid(key, value) {
  const variants = VARIANTS[key];
  if (variants) {
    return variants.includes(value);
  }
  if (UNIT_AXES.includes(key)) {
    return typeof value === "number" && value >= 0 && value <= 1;
  }
  if (key === "palette") {
    return isInteger(value, PALETTES.length - 1);
  }
  if (key === "shapeSeed") {
    return isInteger(value, 4294967295);
  }
  return false;
}
export {
  ACCENTS,
  DETAILS,
  SHAPES,
  deriveTraits
};
