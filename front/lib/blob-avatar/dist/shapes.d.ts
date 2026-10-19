import type { SvgNode } from './svg';
import type { Shape } from './traits';
/** Where the face sits on a body and how much it is scaled. */
export interface FaceAnchor {
    x: number;
    y: number;
    scale: number;
}
export interface Body {
    /** Body silhouette; the caller sets the fill. */
    node: SvgNode;
    face: FaceAnchor;
    /** Glossy highlight ellipse in viewBox coordinates. */
    shine: {
        x: number;
        y: number;
        rx: number;
        ry: number;
        angle: number;
    };
}
export declare function buildBody(shape: Shape, shapeSeed: number): Body;
