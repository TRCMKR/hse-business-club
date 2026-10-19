function el(tag, attrs = {}, children = []) {
  return { tag, attrs, children };
}
function num(value) {
  if (!Number.isFinite(value)) {
    throw new Error(`blob-avatar: non-finite number ${value}`);
  }
  const rounded = Math.round(value * 100) / 100;
  return rounded === 0 ? "0" : String(rounded);
}
function d(strings, ...values) {
  let out = strings[0] ?? "";
  for (let i = 0; i < values.length; i++) {
    out += num(values[i]) + (strings[i + 1] ?? "");
  }
  return out;
}
function formatAttr(value) {
  return typeof value === "number" ? num(value) : value;
}
function escapeAttr(value) {
  return value.replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;").replace(/"/g, "&quot;");
}
function toSvgString(node) {
  let out = `<${node.tag}`;
  for (const [name, value] of Object.entries(node.attrs)) {
    out += ` ${name}="${escapeAttr(formatAttr(value))}"`;
  }
  if (node.children.length === 0) {
    return `${out}/>`;
  }
  out += ">";
  for (const child of node.children) {
    out += toSvgString(child);
  }
  return `${out}</${node.tag}>`;
}
export {
  d,
  el,
  formatAttr,
  num,
  toSvgString
};
