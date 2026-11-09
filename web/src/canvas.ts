import { BaseComponent } from "@panyam/tsappkit";
import { pickOnly, toRenderable, type DecodedSheet } from "./packed.js";
import { Camera, boundsOfVertices, revealView, type Bounds, type CameraView } from "./camera.js";
import { wheelZoomFactor } from "@agni/web-shared/panzoom.js";
import { hexToRgba, Renderer } from "./webgl.js";
import { TextOverlay } from "./textoverlay.js";
import {
  DEFAULT_HIGHLIGHT_COLOR,
  HighlightShape,
  PATH_WIDTH_FRACTION,
  circleTriangles,
  entityBounds,
  entityFrame,
  pathQuads,
  rectTriangles,
  resolveHighlights,
  type HighlightSpec,
} from "./highlights.js";
import { SHEET_DRAWN } from "./drawmark.js";
import { PickIndex, drawnOnPacked } from "./canvaspick.js";
import { rectOf, type Members } from "./area.js";
import { AreaBand } from "./areaband.js";
import { CLICK_SLOP_PX, type Selection } from "./selection.js";
import type { PackedSheet } from "./gen/agni/v1/geom/geom_packed_pb.js";
import { UPRIGHT, orientTurn, type Orientation, type Point } from "./orient.js";
import { strataDrawOrder, type StrataRow } from "./boardstrata.js";
import { unitAround, type RevealBox } from "./svgview.js";

// CanvasComponent owns one WebGL2 drawing as a tsappkit lifecycle component. The canvas mounts it on
// #view, and each sheet view in WebGL mounts one on a canvas of its own (agni issue 1368). It renders
// whatever sheet it is handed through showSheet (WS9-003). Switching sheets swaps the renderer and
// camera, while the input handlers and the animation loop are set up once.
export class CanvasComponent extends BaseComponent {
  private gl?: WebGL2RenderingContext;
  private renderer?: Renderer;
  private camera?: Camera;
  private overlay?: TextOverlay;
  private started = false;
  // sheet is the sheet currently drawn, kept for the highlight join from specs to primitive keys.
  // highlights holds the active highlight specs. They persist across sheet redraws so highlights
  // survive a re-render.
  private sheet?: DecodedSheet;
  private highlights: HighlightSpec[] = [];
  // marked is false from a sheet's draw until the frame that first renders it. The frame after that
  // one records the SHEET_DRAWN mark, as the SVG host does (drawmark.ts).
  private marked = true;
  // picks indexes the drawn sheet for clicks (canvaspick.ts). It is built on the first click
  // rather than with the sheet, so a board that is only looked at never pays for it.
  private picks?: PickIndex;
  // orientation turns the view, and fromBelow draws a board's strata nearest last (agni issue 1368).
  // strata are the board's layer rows, which say how deep each group lies. order is the draw order
  // they give, and depth its inverse, a primitive's place in that order, which a pick reads.
  private orientation: Orientation = UPRIGHT;
  private fromBelow = false;
  private strata: StrataRow[] = [];
  private order: Int32Array | null = null;
  private depth: Int32Array | undefined;
  // touched says the reader moved this sheet's view, so turning it keeps the point at its centre
  // rather than refitting, as SvgView does.
  private touched = false;
  // lit holds the world box of each entity the highlight lit, which reveal frames.
  private lit: Bounds[] = [];
  private frameId = 0;
  private disposed = false;
  private resizeObserver?: ResizeObserver;

  // textHost is where the text layer goes, and readout where the canvas reports what it drew. The
  // page's canvas leaves both unset and gets #text-overlay and #readout. A sheet view sets its own
  // text host and no readout before activate.
  textHost?: HTMLElement;
  readout?: HTMLElement | null;
  // markView names the drawing in the SHEET_DRAWN mark's detail, so a browser spec timing the
  // canvas can tell its draws from a sheet view's.
  markView = "canvas";

  // onPick fires when the reader CLICKS an entity on the canvas, as SvgView.onPick does. A press
  // and release that moved more than CLICK_SLOP_PX is a pan and picks nothing (agni issue 1332).
  onPick: ((sel: Selection) => void) | null = null;
  // onPickEmpty fires when a click hits nothing keyed, such as the paper or the frame.
  onPickEmpty: (() => void) | null = null;
  // onArea fires when the reader Shift-drags a box over the canvas, with what the box holds, and
  // onToggle when they Shift-click an entity, as SvgView's do (agni issue 1333). The canvas forms
  // the area itself, since only its pick index knows what lies where.
  onArea: ((m: Members) => void) | null = null;
  onToggle: ((sel: Selection) => void) | null = null;
  // onViewChange fires when the reader pans or zooms, so a locked view can follow.
  onViewChange: (() => void) | null = null;

  activate(): void {
    const canvas = this.rootElement as HTMLCanvasElement;
    const gl = canvas.getContext("webgl2");
    if (!gl) {
      this.setReadout("WebGL2 not available in this browser.");
      return;
    }
    this.gl = gl;
    this.setReadout("no sheet loaded — pick a file");
  }

  // ready says the browser gave this canvas a WebGL2 context, so it can draw.
  ready(): boolean {
    return this.gl !== undefined;
  }

  // showSheet renders a packed sheet delivered by the design API (GetSheet). Safe to call
  // repeatedly to switch sheets.
  showSheet(sheet: PackedSheet): void {
    if (!this.gl) return;
    this.draw(toRenderable(sheet));
  }

  // setHiddenGroups forwards board layer visibility to the renderer (WS7-035); kept and
  // re-applied when a new sheet arrives, so the chosen view survives sheet swaps.
  setHiddenGroups(groups: Iterable<number>): void {
    this.hidden = [...groups];
    this.renderer?.setHiddenGroups(this.hidden);
    this.overlay?.setHiddenGroups(this.hidden);
  }
  private hidden: number[] = [];

  // getView / setView snapshot and restore the camera's pan/zoom, so the presenter can
  // remember a view per (mode, file, sheet). null before any sheet has been drawn.
  getView(): CameraView | null {
    return this.camera ? this.camera.getView() : null;
  }

  setView(v: CameraView): void {
    this.camera?.setView(v);
    this.touched = true;
  }

  // getCentre is the world point at the middle of the view and the zoom, in pixels per world unit.
  // setCentre puts a world point at the middle at a zoom. Two locked WebGL views on one sheet
  // exchange these, as two SVG views exchange SvgView's.
  getCentre(): { at: Point; scale: number } | null {
    if (!this.camera) return null;
    const v = this.camera.getView();
    return { at: { x: v.centerX, y: v.centerY }, scale: 1 / v.worldPerPixel };
  }

  setCentre(at: Point, scale: number): void {
    this.setView({ centerX: at.x, centerY: at.y, worldPerPixel: 1 / scale });
  }

  // getOrientation and setOrientation turn the view (agni issues 1327, 1368). A view the reader has
  // moved keeps the point at its centre, since the camera turns about it, and a fresh one refits.
  getOrientation(): Orientation {
    return this.orientation;
  }

  setOrientation(o: Orientation): void {
    this.orientation = o;
    if (!this.camera) return;
    this.camera.setOrientation(o);
    if (!this.touched) this.fit();
  }

  // setStrata gives a board's layer rows and the side the reader looks from. From below, the strata
  // draw nearest last and a click picks the nearest first, as orderStrata orders an SVG board. A
  // schematic passes no rows and draws in pack order.
  setStrata(rows: StrataRow[], fromBelow: boolean): void {
    this.strata = rows;
    this.fromBelow = fromBelow;
    this.applyOrder();
  }

  private applyOrder(): void {
    const sheet = this.sheet;
    this.order = sheet ? strataDrawOrder(sheet.primitives.map((p) => p.group), this.strata, this.fromBelow) : null;
    this.depth = undefined;
    if (this.order) {
      this.depth = new Int32Array(this.order.length);
      this.order.forEach((primitive, n) => (this.depth![primitive] = n));
    }
    this.renderer?.setOrder(this.order);
  }

  // reveal frames what the highlight lit, as SvgView's revealOverlay frames its overlay. A pin frames
  // the unit of its part that draws it, since a dot alone is too small to read (agni issue 1419). It
  // returns false when nothing is lit on this sheet.
  reveal(pin?: { ref: string; pin: string }): boolean {
    if (!this.camera || !this.sheet || this.lit.length === 0) return false;
    let box = this.lit.reduce(unionBounds);
    if (pin) box = this.unitOf(box, pin.ref);
    const canvas = this.rootElement as HTMLCanvasElement;
    const quarter = this.orientation.rot === 90 || this.orientation.rot === 270;
    this.camera.setView(revealView(box, boundsOfVertices(this.sheet.vertices), canvas.width, canvas.height, quarter));
    this.touched = true;
    this.onViewChange?.();
    return true;
  }

  // unitOf grows a box through every drawing of the part that touches it (svgview.ts, unitAround).
  private unitOf(seed: Bounds, ref: string): Bounds {
    const sheet = this.sheet!;
    const parts: RevealBox[] = [];
    for (const k of sheet.keys) {
      if (k.refDes !== ref || k.pin) continue;
      const b = entityBounds(sheet.vertices, sheet.primitives, [k.primitive]);
      if (b) parts.push(toBox(b));
    }
    return fromBox(unitAround(toBox(seed), parts));
  }

  // dispose stops drawing and frees the GPU's copy of the sheet, for a sheet view the reader closed
  // or switched to SVG.
  dispose(): void {
    this.disposed = true;
    cancelAnimationFrame(this.frameId);
    this.resizeObserver?.disconnect();
    this.overlay?.hide();
    this.gl?.getExtension("WEBGL_lose_context")?.loseContext();
    this.gl = undefined;
    this.renderer = undefined;
  }

  private fit(): void {
    const canvas = this.rootElement as HTMLCanvasElement;
    this.camera?.fit(canvas.width, canvas.height);
  }

  private draw(sheet: DecodedSheet): void {
    const gl = this.gl!;
    const canvas = this.rootElement as HTMLCanvasElement;
    this.sheet = sheet;
    this.picks = undefined;
    this.renderer = new Renderer(gl, sheet);
    this.renderer.setHiddenGroups(this.hidden);
    this.camera = new Camera(boundsOfVertices(sheet.vertices));
    this.camera.setOrientation(this.orientation);
    this.touched = false;
    this.applyOrder();
    this.overlay ??= this.makeOverlay();
    this.overlay?.setHiddenGroups(this.hidden);
    this.overlay?.setContent(sheet.images, sheet.labels, sheet.fontFamily);
    this.applyHighlights(); // highlights chosen before this sheet was (re)drawn are reapplied
    this.resize();
    this.camera.fit(canvas.width, canvas.height);
    this.marked = false;
    this.setReadout(`sheet ${sheet.sheetId} — ${sheet.primitives.length} primitives, ${sheet.vertices.length / 2} vertices`);
    this.start();
  }

  // setHighlights tints the elements each spec selects (components, nets, pins) in the spec's
  // color/alpha; an empty array clears every highlight. Safe to call before or after a sheet is
  // drawn, since the specs are remembered and reapplied on the next draw.
  setHighlights(specs: HighlightSpec[]): void {
    this.highlights = specs;
    (this.rootElement as HTMLElement).dataset.highlight = JSON.stringify(specs.map(askedSubjects));
    this.applyHighlights();
  }

  // applyHighlights resolves the current specs to primitive-index groups via the sheet's keys
  // (the local mirror of render.HighlightPacked) and pushes them to the renderer. Outline groups
  // recolor their primitives in place. Bounding-shape groups (WS9-017) tessellate one framing
  // rect/circle per matched entity from the vertices already held, with no round-trip, and go to
  // the renderer's overlay path. A no-op without a sheet or renderer yet.
  private applyHighlights(): void {
    if (!this.renderer || !this.sheet) return;
    const sheet = this.sheet;
    const lit = new Set<string>();
    const fallback = hexToRgba(DEFAULT_HIGHLIGHT_COLOR, [1, 0, 1, 1]);
    const draws: { color: ReturnType<typeof hexToRgba>; primitives: Set<number> }[] = [];
    const overlays: { color: ReturnType<typeof hexToRgba>; vertices: Int32Array }[] = [];
    const litBoxes: Bounds[] = [];
    // PATH marker half-width in world units, a fraction of the sheet's smaller span (WS9-043) so
    // it is visible at any coordinate scale (geometry is in nanometers for KiCad).
    const sb = boundsOfVertices(sheet.vertices);
    const pathHalfWidth = PATH_WIDTH_FRACTION * Math.min(sb.maxX - sb.minX, sb.maxY - sb.minY);
    // A pick area is never drawn, so no highlight lights it (packed.ts, pickOnly).
    const drawn = sheet.keys.filter((k) => !pickOnly(sheet.primitives[k.primitive]?.group ?? 0));
    for (const g of resolveHighlights(drawn, this.highlights)) {
      for (const s of g.subjects) lit.add(s);
      for (const entity of g.entities) {
        const b = entityBounds(sheet.vertices, sheet.primitives, entity);
        if (b) litBoxes.push(b);
      }
      const color = hexToRgba(g.color, fallback, g.alpha);
      // PATH (WS9-043) draws a wider translucent marker. GL lines are always 1px, so its width
      // comes from filled quads tessellated along each wire segment (as the board copper tracks
      // do) and sent to the overlay path. It skips the recolor, so the wire shows through the
      // quad like the SVG stroke does.
      if (g.shape === HighlightShape.OUTLINE) {
        draws.push({ color, primitives: g.primitives });
        continue;
      }
      const tris: number[] = [];
      if (g.shape === HighlightShape.PATH) {
        const w = pathHalfWidth * g.strokeScale; // user width scale (WS9-044)
        // Appended one by one, because a spread passes every number as an argument, and a board's
        // ground net makes too many quads for the call stack (agni issue 1330, on Jetson's GND).
        for (const entity of g.entities) for (const v of pathQuads(sheet.vertices, sheet.primitives, entity, w)) tris.push(v);
      } else {
        for (const entity of g.entities) {
          const b = entityBounds(sheet.vertices, sheet.primitives, entity);
          if (!b) continue;
          // A point (a picked pin) frames by the floor alone, so the floor is the path marker's
          // width, a fraction of the sheet, which reads at any coordinate scale.
          const frame = entityFrame(b.minX, b.minY, b.maxX, b.maxY, pathHalfWidth);
          tris.push(...(g.shape === HighlightShape.BOUNDING_RECT ? rectTriangles(frame) : circleTriangles(frame)));
        }
      }
      overlays.push({ color, vertices: Int32Array.from(tris, Math.round) });
    }
    this.lit = litBoxes;
    this.renderer.setHighlights(draws);
    this.renderer.setOverlays(overlays);
    (this.rootElement as HTMLElement).dataset.lit = JSON.stringify([...lit].sort());
  }

  // makeOverlay attaches the text layer to the #text-overlay host beside the canvas. Returns
  // undefined if the host is absent, so text is simply not drawn.
  private makeOverlay(): TextOverlay | undefined {
    const host = this.textHost ?? document.getElementById("text-overlay");
    return host ? new TextOverlay(host) : undefined;
  }

  // showText / hideText follow the active render mode. The text layer belongs to the WebGL
  // view, so the composition root shows it in WebGL mode and hides it for SVG/Native.
  showText(): void {
    this.overlay?.show();
  }
  hideText(): void {
    this.overlay?.hide();
  }

  private dpr(): number {
    return window.devicePixelRatio || 1;
  }

  private resize(): void {
    const canvas = this.rootElement as HTMLCanvasElement;
    const w = Math.floor(canvas.clientWidth * this.dpr());
    const h = Math.floor(canvas.clientHeight * this.dpr());
    // A parked (closed) dock panel measures 0x0; keep the last real framebuffer so reopening
    // the panel doesn't flash a collapsed viewport before the observer fires again.
    if (w === 0 || h === 0) return;
    if (canvas.width !== w || canvas.height !== h) {
      canvas.width = w;
      canvas.height = h;
    }
    this.gl!.viewport(0, 0, canvas.width, canvas.height);
    this.camera?.setViewport(canvas.width, canvas.height);
  }

  // start wires input + the animation loop once; both read the current renderer/camera, so a
  // later showSheet swap is picked up without re-binding.
  private start(): void {
    if (this.started) return;
    this.started = true;
    const canvas = this.rootElement as HTMLCanvasElement;

    window.addEventListener("resize", () => this.resize());
    // The window listener misses dock-panel resizes (dragging a dockview splitter fires no
    // window resize), so observe the canvas element itself. Guarded because jsdom has no
    // ResizeObserver.
    if (typeof ResizeObserver !== "undefined") {
      this.resizeObserver = new ResizeObserver(() => this.resize());
      this.resizeObserver.observe(canvas);
    }

    let dragging = false;
    let lastX = 0;
    let lastY = 0;
    // downX/downY and moved separate a click from a pan, as SvgView's bindInput does.
    let downX = 0;
    let downY = 0;
    let moved = false;
    // selecting is a press with Shift held, which draws a box rather than panning.
    let selecting = false;
    const band = new AreaBand(canvas.parentElement ?? canvas);
    canvas.addEventListener("mousedown", (e) => {
      dragging = true;
      selecting = e.shiftKey && (this.onArea !== null || this.onToggle !== null);
      if (selecting) e.preventDefault();
      lastX = e.clientX;
      lastY = e.clientY;
      downX = e.clientX;
      downY = e.clientY;
      moved = false;
    });
    window.addEventListener("mouseup", (e) => {
      const wasDragging = dragging;
      dragging = false;
      if (wasDragging && selecting) {
        selecting = false;
        band.clear();
        if (moved) {
          const m = this.membersIn(rectOf(downX, downY, e.clientX, e.clientY));
          if (m) this.onArea?.(m);
        } else {
          const sel = this.pickAt(e.clientX, e.clientY);
          if (sel) this.onToggle?.(sel);
        }
        return;
      }
      if (!wasDragging || moved || !this.onPick) return;
      const sel = this.pickAt(e.clientX, e.clientY);
      if (sel) this.onPick(sel);
      else this.onPickEmpty?.();
    });
    window.addEventListener("mousemove", (e) => {
      if (!dragging || !this.camera) return;
      if (Math.abs(e.clientX - downX) > CLICK_SLOP_PX || Math.abs(e.clientY - downY) > CLICK_SLOP_PX) moved = true;
      if (selecting) {
        if (moved) band.draw(rectOf(downX, downY, e.clientX, e.clientY));
        return;
      }
      this.camera.panPixels((e.clientX - lastX) * this.dpr(), (e.clientY - lastY) * this.dpr());
      this.touched = true;
      this.onViewChange?.();
      lastX = e.clientX;
      lastY = e.clientY;
    });
    canvas.addEventListener(
      "wheel",
      (e) => {
        e.preventDefault();
        if (!this.camera) return;
        const rect = canvas.getBoundingClientRect();
        this.camera.zoomAt((e.clientX - rect.left) * this.dpr(), (e.clientY - rect.top) * this.dpr(), wheelZoomFactor(e.deltaY));
        this.touched = true;
        this.onViewChange?.();
      },
      { passive: false },
    );

    const frame = (): void => {
      if (this.disposed) return;
      if (this.renderer && this.camera) {
        this.renderer.render(this.camera.matrix(), this.camera.viewRect());
        // The overlay reads the same camera in the same tick, so text moves and turns with the
        // geometry.
        this.overlay?.update(this.camera.getView(), canvas.clientWidth, canvas.clientHeight, this.dpr(), orientTurn(this.orientation));
        if (!this.marked) {
          this.marked = true;
          const view = this.markView;
          requestAnimationFrame(() => performance.mark(SHEET_DRAWN, { detail: { mode: "webgl", view } }));
        }
      }
      this.frameId = requestAnimationFrame(frame);
    };
    this.frameId = requestAnimationFrame(frame);
  }

  // pickAt resolves a client point to the entity drawn under it, through the camera's inverse and
  // the sheet's pick index. A hidden layer picks nothing.
  private pickAt(clientX: number, clientY: number): Selection | null {
    if (!this.sheet || !this.camera) return null;
    const rect = (this.rootElement as HTMLCanvasElement).getBoundingClientRect();
    const at = this.camera.pixelToWorld((clientX - rect.left) * this.dpr(), (clientY - rect.top) * this.dpr());
    this.picks ??= new PickIndex(this.sheet);
    return this.picks.pick(at.x, at.y, this.dpr() * this.camera.worldPerDevicePixel(), new Set(this.hidden), this.depth);
  }

  // membersIn returns what a client rectangle holds on the drawn sheet, or null before one is drawn.
  // The rectangle's corners go through the camera's inverse, and since screen Y runs down and world
  // Y up, the corners swap on the way.
  membersIn(r: { left: number; top: number; right: number; bottom: number }): Members | null {
    if (!this.sheet || !this.camera) return null;
    const rect = (this.rootElement as HTMLCanvasElement).getBoundingClientRect();
    const a = this.camera.pixelToWorld((r.left - rect.left) * this.dpr(), (r.top - rect.top) * this.dpr());
    const b = this.camera.pixelToWorld((r.right - rect.left) * this.dpr(), (r.bottom - rect.top) * this.dpr());
    this.picks ??= new PickIndex(this.sheet);
    const world = { minX: Math.min(a.x, b.x), minY: Math.min(a.y, b.y), maxX: Math.max(a.x, b.x), maxY: Math.max(a.y, b.y) };
    return this.picks.membersIn(world, new Set(this.hidden));
  }

  // drawn lists the parts and nets the sheet on the canvas draws, or null before one is drawn, for
  // Properties' account of the sheet (agni issue 1333).
  drawn(): { refs: string[]; nets: string[] } | null {
    return this.sheet ? drawnOnPacked(this.sheet) : null;
  }

  private setReadout(text: string): void {
    const readout = this.readout === undefined ? document.getElementById("readout") : this.readout;
    if (readout) readout.textContent = text;
  }
}

function unionBounds(a: Bounds, b: Bounds): Bounds {
  return { minX: Math.min(a.minX, b.minX), minY: Math.min(a.minY, b.minY), maxX: Math.max(a.maxX, b.maxX), maxY: Math.max(a.maxY, b.maxY) };
}

// toBox and fromBox move between world bounds and unitAround's boxes.
function toBox(b: Bounds): RevealBox {
  return { x: b.minX, y: b.minY, w: b.maxX - b.minX, h: b.maxY - b.minY };
}

function fromBox(b: RevealBox): Bounds {
  return { minX: b.x, minY: b.y, maxX: b.x + b.w, maxY: b.y + b.h };
}

// askedSubjects is the part of a spec that names what to light, without its style. The canvas
// writes it to data-highlight in every mode, so a browser spec can see what an SVG sheet was asked
// to light and compare it with what the canvas lit (data-lit, agni issue 1330).
function askedSubjects(spec: HighlightSpec): Pick<HighlightSpec, "components" | "nets" | "netIds" | "busIds" | "pins"> {
  return { components: spec.components, nets: spec.nets, netIds: spec.netIds, busIds: spec.busIds, pins: spec.pins };
}
