/**
 * Hand-picked pastel palettes: low saturation, high lightness, no pure white or black.
 * `figure` is the large base shape, `accent` the smaller element; both stay readable
 * on `background` at 24 px.
 */
export interface Palette {
    name: string;
    background: string;
    figure: string;
    accent: string;
}
export declare const PALETTES: readonly Palette[];
