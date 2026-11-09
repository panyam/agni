// SheetSurface is the drawing of one sheet in one view, in either renderer (agni issue 1368). The
// canvas has one and so does every sheet view, so one code path draws, highlights, turns, picks and
// frames every view. It holds an SvgView, and a WebGL drawing (CanvasComponent) when it draws in
// WebGL. A caller fetches the sheet in the format the surface's mode wants and hands it over.
//
// The SVG host sits over the WebGL canvas. In WebGL the host hides and the canvas draws, with its
// own text layer. In SVG the host shows and the canvas keeps whatever it last drew underneath.
import { SvgView, drawnOnSheet, type SvgViewState } from "./svgview.js";
import type { CanvasComponent } from "./canvas.js";
import type { CameraView } from "./camera.js";
import type { HighlightSpec } from "./highlights.js";
import { UPRIGHT, type Orientation, type Point } from "./orient.js";
import { orderStrata, type StrataRow } from "./boardstrata.js";
import { layerHiding } from "./boardlayers.js";
import { membersIn, type Members } from "./area.js";
import type { Selection } from "./selection.js";
import type { PackedSheet, BoardLayerRow } from "./gen/agni/v1/geom/geom_packed_pb.js";
import { SHEET_DRAWN } from "./drawmark.js";

// SurfaceMode is the renderer a surface draws in. Native draws the tool's own SVG into the SVG host,
// so a surface sees it as "svg".
export type SurfaceMode = "webgl" | "svg";

// SurfaceParts are the elements and the WebGL drawing a surface works over. The canvas passes the
// page's own elements and its lifecycle's CanvasComponent. A sheet view makes its own and builds its
// WebGL drawing only when the reader turns WebGL on, since each one holds a GPU copy of the sheet.
export interface SurfaceParts {
  svgHost: HTMLElement;
  // gl returns the WebGL drawing. With create set it builds one on first use, and without it it
  // returns only one already built. It returns null where WebGL2 is missing.
  gl: (create: boolean) => CanvasComponent | null;
  // dropGl frees the WebGL drawing when the surface leaves WebGL. The canvas keeps its own and passes
  // nothing.
  dropGl?: () => void;
  // layerScope scopes the CSS that hides board layers to this surface's SVG drawing, and layerStyle
  // holds that CSS.
  layerScope: string;
  layerStyle: HTMLStyleElement;
  // markView names this surface in the SHEET_DRAWN mark's detail.
  markView: string;
  // readout reports what the surface drew, or null for a surface with no readout.
  readout: HTMLElement | null;
}

export class SheetSurface {
  readonly svg: SvgView;
  // mode is the renderer the next sheet draws in, and drawnBy the one that drew the sheet on screen.
  mode: SurfaceMode = "svg";
  drawnBy: SurfaceMode = "svg";
  private orientation: Orientation = UPRIGHT;
  private strata: StrataRow[] = [];
  private fromBelow = false;
  private hiddenGroups: number[] = [];
  private highlights: HighlightSpec[] = [];

  onPick: ((sel: Selection) => void) | null = null;
  onPickEmpty: (() => void) | null = null;
  onArea: ((m: Members) => void) | null = null;
  onToggle: ((sel: Selection) => void) | null = null;
  onViewChange: (() => void) | null = null;

  constructor(private readonly parts: SurfaceParts) {
    this.svg = new SvgView(parts.svgHost);
    this.svg.onPick = (sel) => this.onPick?.(sel);
    this.svg.onPickEmpty = () => this.onPickEmpty?.();
    this.svg.onToggle = (sel) => this.onToggle?.(sel);
    this.svg.onViewChange = () => this.onViewChange?.();
    this.svg.onArea = (r) => {
      const doc = parts.svgHost.querySelector("svg");
      if (doc) this.onArea?.(membersIn(doc, r));
    };
  }

  // webgl is the WebGL drawing, wired to this surface's handlers, or null where WebGL2 is missing.
  private webgl(): CanvasComponent | null {
    const gl = this.parts.gl(true);
    if (!gl) return null;
    gl.markView = this.parts.markView;
    gl.onPick = (sel) => this.onPick?.(sel);
    gl.onPickEmpty = () => this.onPickEmpty?.();
    gl.onArea = (m) => this.onArea?.(m);
    gl.onToggle = (sel) => this.onToggle?.(sel);
    gl.onViewChange = () => this.onViewChange?.();
    return gl;
  }

  // setMode picks the renderer for the next sheet. Leaving WebGL frees a sheet view's GPU copy.
  setMode(mode: SurfaceMode): void {
    if (mode === this.mode) return;
    this.mode = mode;
    if (mode === "svg") this.parts.dropGl?.();
  }

  // showPacked draws a packed sheet in WebGL. It returns false where WebGL2 is missing, and then the
  // caller draws the sheet as SVG instead.
  showPacked(sheet: PackedSheet): boolean {
    const gl = this.webgl();
    if (!gl) return false;
    this.drawnBy = "webgl";
    gl.setHiddenGroups(this.hiddenGroups);
    gl.setHighlights(this.highlights);
    gl.setOrientation(this.orientation);
    gl.showSheet(sheet);
    gl.setStrata(this.strata, this.fromBelow);
    this.svg.hide();
    gl.showText();
    return true;
  }

  // showSvg draws an SVG document, the sheet as the engine drew it or the native tool's page.
  showSvg(markup: string): void {
    this.drawnBy = "svg";
    this.parts.gl(false)?.hideText();
    // Show before setSvg so fit() measures a laid-out (non-zero) host.
    this.svg.show();
    this.svg.setSvg(markup);
    this.applyTurn();
    const view = this.parts.markView;
    requestAnimationFrame(() => requestAnimationFrame(() => performance.mark(SHEET_DRAWN, { detail: { mode: "svg", view } })));
    if (this.parts.readout) {
      this.parts.readout.textContent = `SVG — ${this.svg.stats().elements} elements`;
      this.parts.readout.style.display = "";
    }
  }

  // setHighlights lights specs in WebGL, which matches them against the packed keys it holds. The
  // SVG drawing lights through setSvgOverlay, since only the engine can join specs to its document.
  setHighlights(specs: HighlightSpec[]): void {
    this.highlights = specs;
    // A WebGL drawing records what it was asked to light in every mode (data-highlight), which a
    // browser spec reads for an SVG sheet too.
    this.parts.gl(false)?.setHighlights(specs);
  }

  setSvgOverlay(markup: string): void {
    this.svg.setOverlay(markup);
  }

  // setGhost draws another sheet's matches for specs over the WebGL drawing, as the visual diff's
  // union draws the old revision's removed parts over the new one (agni issue 1335). null clears it.
  setGhost(sheet: PackedSheet | null, specs: HighlightSpec[]): void {
    this.parts.gl(false)?.setGhost(sheet, specs);
  }

  // reveal frames the highlight on the drawing on screen. A pin frames the unit that draws it.
  reveal(pin?: { ref: string; pin: string }): void {
    if (this.drawnBy === "webgl") this.parts.gl(false)?.reveal(pin);
    else this.svg.revealOverlay(pin);
  }

  // getView and setView snapshot and restore the pan and zoom of one renderer. The value is opaque
  // to a caller, which keeps it per renderer and sheet.
  getView(mode: SurfaceMode): unknown {
    return mode === "webgl" ? (this.parts.gl(false)?.getView() ?? null) : this.svg.getView();
  }

  setView(mode: SurfaceMode, view: unknown): void {
    if (mode === "webgl") this.parts.gl(false)?.setView(view as CameraView);
    else this.svg.setView(view as SvgViewState);
  }

  // getCentre and setCentre exchange the point at the middle of the drawing on screen and its zoom,
  // in that renderer's own coordinates. Locked views exchange them, so only views drawn by one
  // renderer lock together (sheetfollow.ts, lockedPeers).
  getCentre(): { at: Point; scale: number } | null {
    return this.drawnBy === "webgl" ? (this.parts.gl(false)?.getCentre() ?? null) : this.svg.getCentre();
  }

  setCentre(at: Point, scale: number): void {
    if (this.drawnBy === "webgl") this.parts.gl(false)?.setCentre(at, scale);
    else this.svg.setCentre(at, scale);
  }

  // setTurn turns the drawing and says which side of a board faces the reader. rows are the board's
  // layer rows, which order its strata in WebGL. A schematic passes none.
  setTurn(orientation: Orientation, fromBelow: boolean, rows: StrataRow[]): void {
    this.orientation = orientation;
    this.fromBelow = fromBelow;
    this.strata = rows;
    this.applyTurn();
  }

  private applyTurn(): void {
    if (this.drawnBy === "webgl") {
      const gl = this.parts.gl(false);
      gl?.setOrientation(this.orientation);
      gl?.setStrata(this.strata, this.fromBelow);
      return;
    }
    const doc = this.parts.svgHost.querySelector("svg");
    if (doc) orderStrata(doc, this.fromBelow);
    this.svg.setOrientation(this.orientation);
  }

  // setBoardLayers hides a board's hidden layer rows in both renderers, by CSS scoped to this
  // surface's SVG drawing and by packed group in WebGL (agni issues 1351, 1401).
  setBoardLayers(rows: BoardLayerRow[], hidden: ReadonlySet<string>): void {
    const h = layerHiding(rows, hidden, this.parts.layerScope);
    this.parts.layerStyle.textContent = h.css;
    this.hiddenGroups = h.groups;
    this.parts.gl(false)?.setHiddenGroups(h.groups);
  }

  // drawn lists the parts and nets the drawing on screen draws, or null before one is drawn.
  drawn(): { refs: string[]; nets: string[] } | null {
    return this.drawnBy === "webgl" ? (this.parts.gl(false)?.drawn() ?? null) : drawnOnSheet(this.parts.svgHost);
  }

  // membersIn returns what a client rectangle holds on the drawing on screen.
  membersIn(r: { left: number; top: number; right: number; bottom: number }): Members | null {
    if (this.drawnBy === "webgl") return this.parts.gl(false)?.membersIn(r) ?? null;
    const doc = this.parts.svgHost.querySelector("svg");
    return doc ? membersIn(doc, r as ClientRect) : null;
  }

  // element is the element the drawing on screen draws into, for a host measuring it.
  element(canvas: HTMLElement): HTMLElement {
    return this.drawnBy === "webgl" ? canvas : this.parts.svgHost;
  }
}
