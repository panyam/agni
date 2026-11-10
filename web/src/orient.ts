// How a view is turned (agni issue 1327). A reader at the bench turns the board view to match the
// board in front of them, and turns it over to look at the back. An orientation is a quarter-turn
// rotation and two flips, which together give the board's eight orientations. A vertical flip equals
// a horizontal flip plus a half turn, and both are kept, because a reader thinks in the flip they
// want.
//
// An orientation maps the drawing's own coordinates (the SVG document's, 0..w by 0..h) into the
// oriented drawing's (0..W by 0..H, where W and H swap on a quarter turn). The drawing turns about
// its centre and is moved back to the origin, so pan and zoom stay a plain translate and scale over
// the oriented drawing. The browser hit-tests through CSS transforms, so picking needs none of this.

export type Rotation = 0 | 90 | 180 | 270;

export interface Orientation {
  rot: Rotation;
  flipH: boolean;
  flipV: boolean;
}

export const UPRIGHT: Orientation = { rot: 0, flipH: false, flipV: false };

export interface Point {
  x: number;
  y: number;
}

export interface Box {
  x: number;
  y: number;
  w: number;
  h: number;
}

// orientedSize is the oriented drawing's width and height, swapped on a quarter turn.
export function orientedSize(o: Orientation, w: number, h: number): { w: number; h: number } {
  return o.rot === 90 || o.rot === 270 ? { w: h, h: w } : { w, h };
}

// orientPoint maps a point of the drawing into the oriented drawing. The flips apply first, in the
// drawing's own frame, and the rotation turns clockwise on screen, as CSS rotate does with y down.
export function orientPoint(o: Orientation, w: number, h: number, p: Point): Point {
  let dx = p.x - w / 2;
  let dy = p.y - h / 2;
  if (o.flipH) dx = -dx;
  if (o.flipV) dy = -dy;
  const [rx, ry] = turn(o.rot, dx, dy);
  const s = orientedSize(o, w, h);
  return { x: rx + s.w / 2, y: ry + s.h / 2 };
}

// unorientPoint is orientPoint's inverse, from the oriented drawing back to the drawing.
export function unorientPoint(o: Orientation, w: number, h: number, p: Point): Point {
  const s = orientedSize(o, w, h);
  let [dx, dy] = turn(((360 - o.rot) % 360) as Rotation, p.x - s.w / 2, p.y - s.h / 2);
  if (o.flipH) dx = -dx;
  if (o.flipV) dy = -dy;
  return { x: dx + w / 2, y: dy + h / 2 };
}

// orientBox maps a box of the drawing to the oriented drawing's axis-aligned box around it.
export function orientBox(o: Orientation, w: number, h: number, b: Box): Box {
  const corners = [
    { x: b.x, y: b.y },
    { x: b.x + b.w, y: b.y },
    { x: b.x, y: b.y + b.h },
    { x: b.x + b.w, y: b.y + b.h },
  ].map((p) => orientPoint(o, w, h, p));
  const xs = corners.map((p) => p.x);
  const ys = corners.map((p) => p.y);
  const x = Math.min(...xs);
  const y = Math.min(...ys);
  return { x, y, w: Math.max(...xs) - x, h: Math.max(...ys) - y };
}

// orientCss is the CSS transform that draws the drawing in its orientation, for an element whose
// transform-origin is its top-left corner. It is the same map as orientPoint.
export function orientCss(o: Orientation, w: number, h: number): string {
  if (o.rot === 0 && !o.flipH && !o.flipV) return "";
  const s = orientedSize(o, w, h);
  return `translate(${s.w / 2}px, ${s.h / 2}px) rotate(${o.rot}deg) scale(${o.flipH ? -1 : 1}, ${o.flipV ? -1 : 1}) translate(${-w / 2}px, ${-h / 2}px)`;
}

// rotateBy turns an orientation a further quarter turn clockwise on screen. Turning a flipped
// drawing on screen is turning it the other way in its own frame, but orientations keep the
// rotation on screen, so the turn simply adds.
export function rotateBy(o: Orientation, quarter: 1 | -1 = 1): Orientation {
  return { ...o, rot: (((o.rot + quarter * 90) % 360) + 360) % 360 as Rotation };
}

// flip mirrors the drawing as it is on screen, left to right ("h") or top to bottom ("v"). A flip on
// screen of a quarter-turned drawing is the other flip in the drawing's own frame.
export function flip(o: Orientation, axis: "h" | "v"): Orientation {
  const quarter = o.rot === 90 || o.rot === 270;
  const own = (axis === "h") !== quarter ? "h" : "v";
  return own === "h" ? { ...o, flipH: !o.flipH } : { ...o, flipV: !o.flipV };
}

// flipped says whether the drawing is flipped along a screen axis, the flip that flip(o, axis)
// toggles. A flip button shows it as pressed.
export function flipped(o: Orientation, axis: "h" | "v"): boolean {
  const quarter = o.rot === 90 || o.rot === 270;
  return (axis === "h") !== quarter ? o.flipH : o.flipV;
}

// fromBelow says whether a board drawn this way is seen from underneath. A board turned over is the
// top's mirror image, and two flips are the top turned half round, so an odd number of flips is the
// bottom view.
export function fromBelow(o: Orientation): boolean {
  return o.flipH !== o.flipV;
}

// Turn is the 2x2 linear part of an orientation, in a frame whose y runs down the screen. It maps
// an offset in the drawing's own frame to the offset on screen, flips first and then the turn, as
// orientPoint does. A point maps as (a*x + c*y, b*x + d*y), CSS matrix()'s order. Its entries are 0
// and ±1, so its inverse is its transpose.
export interface Turn {
  a: number;
  b: number;
  c: number;
  d: number;
}

// orientTurn is an orientation's Turn. The WebGL camera and its text overlay turn by it, so a
// WebGL view turns as orientCss turns an SVG one (agni issue 1368).
export function orientTurn(o: Orientation): Turn {
  const along = (x: number, y: number): [number, number] => turn(o.rot, o.flipH ? -x : x, o.flipV ? -y : y);
  const [a, b] = along(1, 0);
  const [c, d] = along(0, 1);
  return { a, b, c, d };
}

function turn(rot: Rotation, x: number, y: number): [number, number] {
  switch (rot) {
    case 90:
      return [-y, x];
    case 180:
      return [-x, -y];
    case 270:
      return [y, -x];
    default:
      return [x, y];
  }
}
