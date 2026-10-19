import { createRng, hashString, normalizeSeed } from "./random.js";
const SHAPES = ["round", "squircle", "blob", "droplet", "hexagon", "cloud", "capsule", "triangle"];
const HUES = [12, 30, 50, 140, 175, 210, 260, 320];
const TONES = ["light", "dark"];
const EYE_SHAPES = ["dot", "oval", "wide", "wink"];
const EXPRESSIONS = [
  "happy",
  "neutral",
  "surprised",
  "smirk",
  "sleepy",
  "mad",
  "thinking",
  "cheeky",
  "sad",
  "shy"
];
const EXTRAS = ["blush", "freckles", "none"];
function quantize(items, value) {
  return items[Math.min(items.length - 1, Math.floor(value * items.length))];
}
function deriveTraits(seed, overrides = {}) {
  const rng = createRng(hashString(normalizeSeed(seed)));
  const derived = {
    shape: quantize(SHAPES, rng.next()),
    shapeSeed: Math.floor(rng.next() * 4294967296) >>> 0,
    hue: quantize(HUES, rng.next()),
    tone: quantize(TONES, rng.next()),
    eyeShape: quantize(EYE_SHAPES, rng.next()),
    eyeSize: rng.next(),
    eyeSpacing: rng.next(),
    eyeTilt: rng.next(),
    pupilShift: rng.next(),
    expression: quantize(EXPRESSIONS, rng.next()),
    extras: quantize(EXTRAS, rng.next())
  };
  const result = { ...derived };
  for (const key of Object.keys(overrides)) {
    const value = overrides[key];
    if (value !== void 0) {
      result[key] = value;
    }
  }
  return result;
}
export {
  EXPRESSIONS,
  EXTRAS,
  EYE_SHAPES,
  HUES,
  SHAPES,
  TONES,
  deriveTraits
};
