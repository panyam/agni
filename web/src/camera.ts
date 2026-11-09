// View-local affine camera. Maps sheet-local int32 vertex coordinates to clip space [-1,1].
// Only this matrix changes per frame; the geometry buffer is uploaded once and never changes.
//
// A world point w lands on the screen at
//
//   pixel = viewport/2 + T * (w.x - centre.x, -(w.y - centre.y)) / worldPerPixel
//
// where T is the view's orientation (orient.ts's Turn), in a frame whose y runs down the screen.
// Upright T is the identity and the camera is a plain pan and zoom. A turned view rotates and
// flips about the point at its centre, as SvgView turns its drawing (agni issues 1327, 1368).
// matrix() is that map into clip space as a 3x3 column-major mat3 for the GLSL uniform.
//
// geom is Y-up (internal/geomath/transform.go) and WebGL NDC is Y-up too, so an upright view does
// not flip Y and a higher world-Y is higher on screen. The SVG oracle flips once only because SVG
// pixel space is Y-down, so both views come out the same way up.

import { UPRIGHT, orientTurn, type Orientation, type Turn } from "./orient.js";

export interface Bounds {
  minX: number;
  minY: number;
  maxX: number;
  maxY: number;
}

// Axis-aligned bounds of all vertices, in sheet-local int32 space.
export function boundsOfVertices(vertices: Int32Array): Bounds {
  if (vertices.length < 2) {
    return { minX: 0, minY: 0, maxX: 1, maxY: 1 };
  }
  let minX = Infinity;
  let minY = Infinity;
  let maxX = -Infinity;
  let maxY = -Infinity;
  for (let i = 0; i + 1 < vertices.length; i += 2) {
    const x = vertices[i];
    const y = vertices[i + 1];
    if (x < minX) minX = x;
    if (y < minY) minY = y;
    if (x > maxX) maxX = x;
    if (y > maxY) maxY = y;
  }
  return { minX, minY, maxX, maxY };
}

// CameraView is a snapshot of the camera's pan/zoom, for remembering and restoring a view.
export interface CameraView {
  centerX: number;
  centerY: number;
  worldPerPixel: number;
}

export class Camera {
  // World-space center that maps to the middle of the viewport.
  private centerX = 0;
  private centerY = 0;
  // World units per screen pixel, the same on both axes.
  private worldPerPixel = 1;
  private viewportW = 1;
  private viewportH = 1;
  private orientation: Orientation = UPRIGHT;
  private turn: Turn = orientTurn(UPRIGHT);

  constructor(private readonly sheetBounds: Bounds) {}

  // getOrientation and setOrientation turn the view about the point at its centre. The pan and
  // zoom stay as they are, so a view keeps its place when it turns.
  getOrientation(): Orientation {
    return this.orientation;
  }

  setOrientation(o: Orientation): void {
    this.orientation = o;
    this.turn = orientTurn(o);
  }

  // Fit the sheet bounds into the current viewport with a small margin.
  fit(viewportW: number, viewportH: number): void {
    this.viewportW = Math.max(1, viewportW);
    this.viewportH = Math.max(1, viewportH);
    const b = this.sheetBounds;
    // A quarter turn lays the sheet's width along the screen's height.
    const quarter = this.orientation.rot === 90 || this.orientation.rot === 270;
    const w = Math.max(1, quarter ? b.maxY - b.minY : b.maxX - b.minX);
    const h = Math.max(1, quarter ? b.maxX - b.minX : b.maxY - b.minY);
    this.centerX = (b.minX + b.maxX) / 2;
    this.centerY = (b.minY + b.maxY) / 2;
    const margin = 1.1;
    // Choose worldPerPixel so both dimensions fit.
    const wppX = (w * margin) / this.viewportW;
    const wppY = (h * margin) / this.viewportH;
    this.worldPerPixel = Math.max(wppX, wppY);
  }

  setViewport(viewportW: number, viewportH: number): void {
    this.viewportW = Math.max(1, viewportW);
    this.viewportH = Math.max(1, viewportH);
  }

  // getView / setView snapshot and restore the pan/zoom (center + scale). The viewport is not
  // part of the view, since it tracks the canvas size and resize sets it separately.
  getView(): CameraView {
    return { centerX: this.centerX, centerY: this.centerY, worldPerPixel: this.worldPerPixel };
  }

  setView(v: CameraView): void {
    this.centerX = v.centerX;
    this.centerY = v.centerY;
    this.worldPerPixel = v.worldPerPixel;
  }

  // Pan by a pixel delta (mouse drag). dyPx is screen-down positive. The delta turns back into the
  // drawing's frame first, so the grabbed point stays under the cursor in a turned view.
  panPixels(dxPx: number, dyPx: number): void {
    const u = this.unturn(dxPx, dyPx);
    this.centerX -= u.x * this.worldPerPixel;
    // Screen Y is down and world Y is up, so dragging down adds in world Y.
    this.centerY += u.y * this.worldPerPixel;
  }

  // unturn maps a screen offset back into the drawing's unturned frame, by the transpose of the turn.
  private unturn(x: number, y: number): { x: number; y: number } {
    const t = this.turn;
    return { x: t.a * x + t.b * y, y: t.c * x + t.d * y };
  }

  // Zoom toward a cursor position (pixels, origin top-left). factor > 1 zooms in.
  zoomAt(cursorXPx: number, cursorYPx: number, factor: number): void {
    const worldBefore = this.pixelToWorld(cursorXPx, cursorYPx);
    this.worldPerPixel /= factor;
    const worldAfter = this.pixelToWorld(cursorXPx, cursorYPx);
    // Shift center so the world point under the cursor does not move.
    this.centerX += worldBefore.x - worldAfter.x;
    this.centerY += worldBefore.y - worldAfter.y;
  }

  // pixelToWorld maps a canvas pixel (device pixels, origin top-left) to the world point under it,
  // the inverse of matrix(). A click on the canvas picks through it (canvaspick.ts).
  pixelToWorld(px: number, py: number): { x: number; y: number } {
    const u = this.unturn(px - this.viewportW / 2, py - this.viewportH / 2);
    const x = this.centerX + u.x * this.worldPerPixel;
    // Screen Y is down and world Y is up, so a small py is a higher world Y.
    const y = this.centerY - u.y * this.worldPerPixel;
    return { x, y };
  }

  // worldPerDevicePixel is how many world units one canvas pixel spans, the same on both axes.
  worldPerDevicePixel(): number {
    return this.worldPerPixel;
  }

  // The view rect in world coordinates (for viewport culling). A quarter turn swaps its sides.
  viewRect(): Bounds {
    const quarter = this.orientation.rot === 90 || this.orientation.rot === 270;
    const halfW = ((quarter ? this.viewportH : this.viewportW) / 2) * this.worldPerPixel;
    const halfH = ((quarter ? this.viewportW : this.viewportH) / 2) * this.worldPerPixel;
    return {
      minX: this.centerX - halfW,
      minY: this.centerY - halfH,
      maxX: this.centerX + halfW,
      maxY: this.centerY + halfH,
    };
  }

  // 3x3 column-major matrix (mat3) mapping world -> clip [-1,1]. It is the map in the file header,
  // scaled from pixels into clip space, where y runs up.
  matrix(): Float32Array {
    const sx = 1 / ((this.viewportW / 2) * this.worldPerPixel);
    const sy = 1 / ((this.viewportH / 2) * this.worldPerPixel);
    const t = this.turn;
    // clip = K * (w - centre), with K = diag(sx, -sy) * T * diag(1, -1).
    const k00 = sx * t.a;
    const k01 = -sx * t.c;
    const k10 = -sy * t.b;
    const k11 = sy * t.d;
    const tx = -(k00 * this.centerX + k01 * this.centerY);
    const ty = -(k10 * this.centerX + k11 * this.centerY);
    // column-major: col0, col1, col2
    return new Float32Array([
      k00, k10, 0,
      k01, k11, 0,
      tx, ty, 1,
    ]);
  }
}

// REVEAL_WINDOW is how many times the target's size the revealed view spans, and REVEAL_FLOOR the
// smallest target, as a fraction of the sheet's longer side. They match SvgView's reveal, whose
// 40 px floor is 2.5% of a 1600 px drawing (core/render/svg.go's sheetMaxPx).
const REVEAL_WINDOW = 2.5;
const REVEAL_FLOOR = 40 / 1600;

// revealView is the view that centres a world box in a viewport of vpW by vpH device pixels, with the
// box filling 1/REVEAL_WINDOW of it, as SvgView's computeReveal frames a highlight. quarter says the
// view is turned a quarter, so the box's width lies along the viewport's height.
export function revealView(box: Bounds, sheet: Bounds, vpW: number, vpH: number, quarter: boolean): CameraView {
  const floor = REVEAL_FLOOR * Math.max(sheet.maxX - sheet.minX, sheet.maxY - sheet.minY, 1);
  const w = Math.max(box.maxX - box.minX, floor);
  const h = Math.max(box.maxY - box.minY, floor);
  const [along, across] = quarter ? [h, w] : [w, h];
  return {
    centerX: (box.minX + box.maxX) / 2,
    centerY: (box.minY + box.maxY) / 2,
    worldPerPixel: Math.max((along * REVEAL_WINDOW) / Math.max(1, vpW), (across * REVEAL_WINDOW) / Math.max(1, vpH)),
  };
}
