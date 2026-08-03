// View-local affine camera. Maps sheet-local int32 vertex coordinates to clip space
// [-1,1]. Only this matrix changes per frame; the geometry buffer is uploaded once and
// stays static. The matrix is a pure pan+zoom affine (no rotation/shear):
//
//   clip.x = scaleX * world.x + tx
//   clip.y = scaleY * world.y + ty
//
// stored as a 3x3 column-major matrix for a mat3 GLSL uniform:
//   [ scaleX  0       0 ]      columns: [scaleX,0,0], [0,scaleY,0], [tx,ty,1]
//   [ 0       scaleY  0 ]
//   [ tx      ty      1 ]
//
// geom is Y-up (EDIF; see transform.go) and WebGL NDC is Y-up too, so the matrix does not
// flip Y: a higher world-Y is higher on screen. This keeps the WebGL view oriented the same
// as the SVG oracle (which flips once only because SVG pixel space is Y-down).

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
  // World units per half-viewport-height (isotropic zoom; aspect handled per-frame).
  private worldPerPixel = 1;
  private viewportW = 1;
  private viewportH = 1;

  constructor(private readonly sheetBounds: Bounds) {}

  // Fit the sheet bounds into the current viewport with a small margin.
  fit(viewportW: number, viewportH: number): void {
    this.viewportW = Math.max(1, viewportW);
    this.viewportH = Math.max(1, viewportH);
    const b = this.sheetBounds;
    const w = Math.max(1, b.maxX - b.minX);
    const h = Math.max(1, b.maxY - b.minY);
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

  // getView / setView snapshot and restore the pan/zoom (center + scale), so a caller can
  // remember a view and reapply it later. The viewport is not part of the view — it tracks the
  // canvas size and is set independently by resize.
  getView(): CameraView {
    return { centerX: this.centerX, centerY: this.centerY, worldPerPixel: this.worldPerPixel };
  }

  setView(v: CameraView): void {
    this.centerX = v.centerX;
    this.centerY = v.centerY;
    this.worldPerPixel = v.worldPerPixel;
  }

  // Pan by a pixel delta (mouse drag). dyPx is screen-down positive.
  panPixels(dxPx: number, dyPx: number): void {
    this.centerX -= dxPx * this.worldPerPixel;
    // World Y is up while screen Y is down, so dragging down (dyPx > 0) shows a higher-Y
    // region: add in world Y. This keeps the grabbed point under the cursor.
    this.centerY += dyPx * this.worldPerPixel;
  }

  // Zoom toward a cursor position (pixels, origin top-left). factor > 1 zooms in.
  zoomAt(cursorXPx: number, cursorYPx: number, factor: number): void {
    const worldBefore = this.pixelToWorld(cursorXPx, cursorYPx);
    this.worldPerPixel /= factor;
    const worldAfter = this.pixelToWorld(cursorXPx, cursorYPx);
    // Shift center so the world point under the cursor stays fixed.
    this.centerX += worldBefore.x - worldAfter.x;
    this.centerY += worldBefore.y - worldAfter.y;
  }

  private pixelToWorld(px: number, py: number): { x: number; y: number } {
    const x = this.centerX + (px - this.viewportW / 2) * this.worldPerPixel;
    // Screen Y is down, world Y is up: the top of the viewport (py small) is higher world Y.
    const y = this.centerY - (py - this.viewportH / 2) * this.worldPerPixel;
    return { x, y };
  }

  // The view rect in world coordinates (for viewport culling).
  viewRect(): Bounds {
    const halfW = (this.viewportW / 2) * this.worldPerPixel;
    const halfH = (this.viewportH / 2) * this.worldPerPixel;
    return {
      minX: this.centerX - halfW,
      minY: this.centerY - halfH,
      maxX: this.centerX + halfW,
      maxY: this.centerY + halfH,
    };
  }

  // 3x3 column-major matrix (mat3) mapping world -> clip [-1,1].
  matrix(): Float32Array {
    const halfW = (this.viewportW / 2) * this.worldPerPixel;
    const halfH = (this.viewportH / 2) * this.worldPerPixel;
    const scaleX = 1 / halfW;
    // geom is Y-up (EDIF; see transform.go) and WebGL NDC is Y-up too (clip y=+1 is the top
    // of the canvas), so no flip: a higher world-Y maps to a higher clip-Y. This matches the
    // SVG oracle, which flips once only because SVG pixel space is Y-down.
    const scaleY = 1 / halfH;
    const tx = -this.centerX * scaleX;
    const ty = -this.centerY * scaleY;
    // column-major: col0, col1, col2
    return new Float32Array([
      scaleX, 0, 0,
      0, scaleY, 0,
      tx, ty, 1,
    ]);
  }
}
