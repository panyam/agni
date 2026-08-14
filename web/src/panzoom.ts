// panzoom is the single definition of how an Agni viewport navigates: the wheel (a two-finger
// trackpad drag) zooms toward the cursor, and a pointer drag pans. The WebGL schematic canvas, the
// SVG reference render, and the datasheet workbench all take the curve and the anchoring math from
// here, so the three cannot drift. Someone who learns one viewport has learned all of them.
//
// It also makes the policy one edit rather than three. The wheel-zooms choice is deliberate but not
// obviously right: every PDF reader scrolls on the wheel and zooms on ctrl+wheel, so the datasheet
// workbench trades that convention for consistency with the schematic viewers. Switching back would
// be a change to this file's callers, not a hunt through canvas.ts, svgview.ts, and regionview.tsx.
//
// PanZoom is in VIEW pixels: tx/ty place the content's top-left corner within the viewport (origin
// top-left, matching a CSS transform-origin of "0 0") and scale is view pixels per content unit.
// The content unit differs per viewport — SVG px for the reference render, PDF points for the
// datasheet workbench — and nothing here cares which it is.
export interface PanZoom {
  tx: number;
  ty: number;
  scale: number;
}

// WHEEL_ZOOM_K converts a wheel delta into an exponent. The curve is exponential rather than a
// fixed step per notch so zoom is scale-free: the same delta multiplies the scale by the same
// factor whether you are at 20% or 400%, and zooming in then back out by an equal delta lands
// exactly where you started. A linear step does neither.
export const WHEEL_ZOOM_K = 0.001;

export function wheelZoomFactor(deltaY: number): number {
  return Math.exp(-deltaY * WHEEL_ZOOM_K);
}

// zoomAbout scales by factor while holding the content point currently under (cx, cy) fixed on
// screen, which is what makes wheel-zoom feel like it is aimed rather than centered. cx/cy are
// viewport-local pixels.
export function zoomAbout(v: PanZoom, cx: number, cy: number, factor: number): PanZoom {
  return {
    scale: v.scale * factor,
    tx: cx - (cx - v.tx) * factor,
    ty: cy - (cy - v.ty) * factor,
  };
}

// zoomAboutClamped is zoomAbout with scale limits applied BEFORE the anchoring, so a gesture that
// runs into a limit stops zooming instead of continuing to slide the content sideways. Anchoring on
// the requested factor and clamping afterwards drifts the view every frame you hold the wheel at
// the limit, which reads as the page crawling away on its own.
export function zoomAboutClamped(
  v: PanZoom,
  cx: number,
  cy: number,
  factor: number,
  minScale: number,
  maxScale: number,
): PanZoom {
  const target = clampScale(v.scale * factor, minScale, maxScale);
  return zoomAbout(v, cx, cy, target / v.scale);
}

export function clampScale(scale: number, minScale: number, maxScale: number): number {
  return Math.min(maxScale, Math.max(minScale, scale));
}

// panBy moves the content by a pixel delta (a drag). Scale is untouched, so the grabbed point stays
// under the pointer at any zoom.
export function panBy(v: PanZoom, dxPx: number, dyPx: number): PanZoom {
  return { scale: v.scale, tx: v.tx + dxPx, ty: v.ty + dyPx };
}

// contentPointAt converts a viewport-local pixel position to content units, for a readout or for
// placing something the user pointed at.
export function contentPointAt(v: PanZoom, cx: number, cy: number): { x: number; y: number } {
  return { x: (cx - v.tx) / v.scale, y: (cy - v.ty) / v.scale };
}

// fitInto contains content of the given size in a host and centers it. `margin` is the fraction of
// the host left as breathing room (0.04 leaves a 2% gutter on each side). A degenerate content or
// host size falls back to scale 1 rather than an infinite or zero zoom. The scale limits are
// applied BEFORE centering, so a fit that runs into one is still centered on the size it actually
// gets rather than on the size it asked for.
export function fitInto(
  contentW: number,
  contentH: number,
  hostW: number,
  hostH: number,
  opts: { margin?: number; minScale?: number; maxScale?: number } = {},
): PanZoom {
  const { margin = 0.04, minScale = 0, maxScale = Infinity } = opts;
  const cw = Math.max(1, contentW);
  const ch = Math.max(1, contentH);
  const hw = Math.max(1, hostW);
  const hh = Math.max(1, hostH);
  const scale = clampScale(Math.min(hw / cw, hh / ch) * (1 - margin) || 1, minScale, maxScale);
  return { scale, tx: (hw - cw * scale) / 2, ty: (hh - ch * scale) / 2 };
}
