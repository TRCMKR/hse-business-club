import type { Background } from './render';
import type { Traits } from './traits';
export interface BlobAvatarProps {
    /** Any string; the app uses `username`. */
    seed: string;
    /** Width and height in pixels; without it the SVG fills its container. */
    size?: number;
    background?: Background;
    /** Pins trait axes, e.g. `{ palette: 3 }`. */
    traits?: Partial<Traits>;
    /** aria-label, defaults to the seed. */
    label?: string;
}
/**
 * Deterministic blob avatar. Renders the SVG through vnodes (no innerHTML),
 * so it is SSR-safe and hydrates to the same markup as `blobAvatar()`.
 */
export declare const BlobAvatar: import("vue").DefineSetupFnComponent<BlobAvatarProps, {}, {}, BlobAvatarProps & {}, import("vue").PublicProps>;
export type { Background } from './render';
export type { Traits } from './traits';
