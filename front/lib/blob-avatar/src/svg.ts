/** A minimal SVG element tree, rendered either to a string or to Vue vnodes. */
export type SvgAttrValue = string | number;

export interface SvgNode {
  tag: string;
  attrs: Record<string, SvgAttrValue>;
  children: SvgNode[];
}

export function el(tag: string, attrs: Record<string, SvgAttrValue> = {}, children: SvgNode[] = []): SvgNode {
  return { tag, attrs, children };
}

/** Formats a number with at most two decimals, without trailing zeros or "-0". */
export function num(value: number): string {
  if (!Number.isFinite(value)) {
    throw new Error(`blob-avatar: non-finite number ${value}`);
  }
  const rounded = Math.round(value * 100) / 100;
  // `-0 === 0`, so this also normalizes negative zero.
  return rounded === 0 ? '0' : String(rounded);
}

/** Tagged template for path data and transforms: every interpolated number goes through num(). */
export function d(strings: TemplateStringsArray, ...values: number[]): string {
  let out = strings[0] ?? '';
  for (let i = 0; i < values.length; i++) {
    out += num(values[i] as number) + (strings[i + 1] ?? '');
  }
  return out;
}

export function formatAttr(value: SvgAttrValue): string {
  return typeof value === 'number' ? num(value) : value;
}

function escapeAttr(value: string): string {
  return value
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;');
}

export function toSvgString(node: SvgNode): string {
  let out = `<${node.tag}`;
  for (const [name, value] of Object.entries(node.attrs)) {
    out += ` ${name}="${escapeAttr(formatAttr(value))}"`;
  }
  if (node.children.length === 0) {
    return `${out}/>`;
  }
  out += '>';
  for (const child of node.children) {
    out += toSvgString(child);
  }
  return `${out}</${node.tag}>`;
}
