/** A minimal SVG element tree, rendered either to a string or to Vue vnodes. */
export type SvgAttrValue = string | number;
export interface SvgNode {
    tag: string;
    attrs: Record<string, SvgAttrValue>;
    children: SvgNode[];
}
export declare function el(tag: string, attrs?: Record<string, SvgAttrValue>, children?: SvgNode[]): SvgNode;
/** Formats a number with at most two decimals, without trailing zeros or "-0". */
export declare function num(value: number): string;
/** Tagged template for path data and transforms: every interpolated number goes through num(). */
export declare function d(strings: TemplateStringsArray, ...values: number[]): string;
export declare function formatAttr(value: SvgAttrValue): string;
export declare function toSvgString(node: SvgNode): string;
