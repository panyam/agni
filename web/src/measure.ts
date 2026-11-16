// The board measuring tool's model (agni issue 133). A reader turns Measure on, and the board view
// shows where the cursor is in millimetres. A click sets a point, a second click sets another, and
// the view shows the distance between them. A click on a pad sets the pad's centre, so a reading
// between two pads is exact rather than as good as the reader's aim.
//
// Positions are in the board's own frame, Y up, as the file places them. Each renderer maps a
// client point to that frame its own way. WebGL goes through its camera and the packed sheet's
// units_per_mm, and SVG through the drawing's screen matrix and the data-mm-* attributes BoardSVG
// writes on its root. Both implement BoardMeasurer, and everything here is the same for both.

// BoardPoint is a position on the board in millimetres, Y up.
export interface BoardPoint {
  x: number;
  y: number;
}

// MeasurePoint is a point the reader set or is hovering over. pad names the pad it snapped to, as
// "U1.3", and is null for a free point.
export interface MeasurePoint {
  at: BoardPoint;
  pad: string | null;
}

// BoardMeasurer is what a renderer gives the tool. at maps a client point to the board, snapping to
// the centre of a pad under it, and returns null off the drawing. client maps a board point back to
// client coordinates, for drawing the line, and returns null when the view cannot place it.
export interface BoardMeasurer {
  at(clientX: number, clientY: number): MeasurePoint | null;
  client(p: BoardPoint): { x: number; y: number } | null;
}

export type LengthUnit = "mm" | "mil";

// MIL_PER_MM converts millimetres to mils, thousandths of an inch.
export const MIL_PER_MM = 1000 / 25.4;

// formatLength writes a length in the unit asked for, to 0.001 mm or 0.1 mil.
export function formatLength(mm: number, unit: LengthUnit): string {
  if (unit === "mil") return `${(mm * MIL_PER_MM).toFixed(1)} mil`;
  return `${mm.toFixed(3)} mm`;
}

function formatNumber(mm: number, unit: LengthUnit): string {
  return unit === "mil" ? (mm * MIL_PER_MM).toFixed(1) : mm.toFixed(3);
}

// formatPoint writes a board position, with the pad it snapped to when there is one.
export function formatPoint(p: MeasurePoint, unit: LengthUnit): string {
  const at = `x ${formatNumber(p.at.x, unit)}  y ${formatNumber(p.at.y, unit)} ${unit}`;
  return p.pad ? `${p.pad}  ${at}` : at;
}

// Span is the step from one point to another.
export interface Span {
  dx: number;
  dy: number;
  d: number;
}

export function spanOf(a: BoardPoint, b: BoardPoint): Span {
  const dx = b.x - a.x;
  const dy = b.y - a.y;
  return { dx, dy, d: Math.hypot(dx, dy) };
}

// formatSpan writes a span as its distance and its two steps.
export function formatSpan(s: Span, unit: LengthUnit): string {
  return `${formatLength(s.d, unit)}  (Δx ${formatNumber(s.dx, unit)}, Δy ${formatNumber(s.dy, unit)})`;
}

// Measurement holds the points the reader set. A click sets the first point, the next sets the
// second, and a third click starts a new measurement at its point.
export class Measurement {
  private pts: MeasurePoint[] = [];

  get points(): readonly MeasurePoint[] {
    return this.pts;
  }

  click(p: MeasurePoint): void {
    this.pts = this.pts.length === 1 ? [this.pts[0], p] : [p];
  }

  clear(): void {
    this.pts = [];
  }

  // span is the measured step, once both points are set.
  span(): Span | null {
    return this.pts.length === 2 ? spanOf(this.pts[0].at, this.pts[1].at) : null;
  }
}

// mmOfWorld maps a packed sheet's world point to the board. World coordinates are sheet-local, so
// the origin comes back on first.
export function mmOfWorld(world: { x: number; y: number }, originX: number, originY: number, unitsPerMm: number): BoardPoint {
  return { x: (world.x + originX) / unitsPerMm, y: (world.y + originY) / unitsPerMm };
}

export function worldOfMm(p: BoardPoint, originX: number, originY: number, unitsPerMm: number): { x: number; y: number } {
  return { x: p.x * unitsPerMm - originX, y: p.y * unitsPerMm - originY };
}

// SvgBoardFrame is the map BoardSVG states on its root: a point (px, py) of the drawing lies at
// x0 + px * perPx and y0 - py * perPx millimetres.
export interface SvgBoardFrame {
  perPx: number;
  x0: number;
  y0: number;
}

// svgBoardFrame reads the map off a BoardSVG root, or returns null for a drawing that states none,
// such as a schematic or a tool's own SVG.
export function svgBoardFrame(root: Element): SvgBoardFrame | null {
  const num = (name: string): number => Number.parseFloat(root.getAttribute(name) ?? "");
  const f = { perPx: num("data-mm-per-px"), x0: num("data-mm-x0"), y0: num("data-mm-y0") };
  return Number.isFinite(f.perPx) && f.perPx > 0 && Number.isFinite(f.x0) && Number.isFinite(f.y0) ? f : null;
}

export function mmOfSvg(px: number, py: number, f: SvgBoardFrame): BoardPoint {
  return { x: f.x0 + px * f.perPx, y: f.y0 - py * f.perPx };
}

export function svgOfMm(p: BoardPoint, f: SvgBoardFrame): { x: number; y: number } {
  return { x: (p.x - f.x0) / f.perPx, y: (f.y0 - p.y) / f.perPx };
}
