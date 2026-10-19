import type { Tone } from './traits';
export interface Palette {
    background: string;
    body: string;
    /** Darker body color: freckles, outlines. */
    shade: string;
    /** Eyes, mouth, brows. */
    ink: string;
    sclera: string;
    cheek: string;
    tongue: string;
}
export declare function palette(hue: number, tone: Tone): Palette;
