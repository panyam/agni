import { BaseComponent } from "@panyam/tsappkit";
import { KIND_LINE_LOOP, KIND_POINTS, KIND_TRIANGLES, pickOnly, toRenderable, type DecodedSheet } from "./packed.js";
import { Camera, boundsOfVertices, revealView, type Bounds, type CameraView } from "./camera.js";
import { wheelZoomFactor } from "@agni/web-shared/panzoom.js";
import { hexToRgba, Renderer } from "./webgl.js";
import { Glide, lerpCamera } from "./glide.js";
import { TextOverlay } from "./textoverlay.js";
import {
  DEFAULT_HIGHLIGHT_COLOR,
  boardNetAlpha,
  frameFloor,
  frameStroke,
  frameStrokeAlpha,
  HighlightShape,
  PATH_WIDTH_FRACTION,
  circleRing,
  circleTriangles,
  entityBounds,
  entityFrame,
  pathQuads,
  pinMarkRadius,
  rectRing,
  rectTriangles,
  resolveHighlights,
  type HighlightSpec,
} from "./highlights.js";
import { SHEET_DRAWN } from "./drawmark.js";
import { timingMode } from "./api.js";
import { DrawGate } from "./drawgate.js";
import type { DrawScene } from "./drawprofile.js";
import { PickIndex, drawnOnPacked } from "./canvaspick.js";
import { rectOf, type Members } from "./area.js";
import { AreaBand } from "./areaband.js";
import { CLICK_SLOP_PX, type Selection } from "./selection.js";
import type { PackedSheet } from "@panyam/agni-client/gen/agni/v1/geom/geom_packed_pb";
import { UPRIGHT, orientTurn, type Orientation, type Point } from "./orient.js";
import { strataDrawOrder, type StrataRow } from "./boardstrata.js";
import { unitAround, type RevealBox } from "./svgview.js";
import type { BoardLayerRow, BoardStackView } from "@panyam/agni-client/gen/agni/v1/geom/geom_packed_pb";
import { groupDepths, layoutStack, perMm, stackZ, type GroupDepths, type StackLayout } from "./stacklayout.js";
import { PRESETS, StackCamera, type StackPreset } from "./stackcamera.js";
import { HOLE_FLOATS, StackExtras, stackGeometry } from "./stackgl.js";
import { GRAZING, pickThroughStack, type Barrel } from "./stackpick.js";
import { faceShows } from "./stacktext.js";
import { bodyGeometry, bodyTopZ, pickWithBodies, type BodyFaces, type BodyGeometry } from "./stackbodies.js";
import { boardLookPlan, padPrimitives, type BoardLookPlan, type StackLook } from "./stacklook.js";

type Rgba = [number, number, number, number];

// StackState is a board view drawn as its exploded stack (agni issue 1593). The flat camera, side and
// hidden layers wait untouched while it lasts, so leaving the stack view finds them as they were.
interface StackState {
  view: BoardStackView;
  layout: StackLayout;
  depths: GroupDepths;
  camera: StackCamera;
  extras: StackExtras | null;
  // bodies holds the parts the stack view stands on the board's faces (agni issue 1352), and
  // showBodies whether they draw and pick.
  bodies: BodyGeometry;
  showBodies: boolean;
  // look is the stack view's look (stacklook.ts), and plan the board look's plan of the board's
  // layer rows, which it reads from rows (agni issue 1653).
  look: StackLook;
  plan: BoardLookPlan;
  explode: number;
  mmScale: number;
  width: number;
  // planes lists each depth a group draws at, with its groups, for picking a plane at a time.
  planes: { depth: number; groups: Set<number> }[];
  groups: Set<number>;
  barrelColor: Rgba;
  // holes and nets are the plated holes' rows and nets (stackgl.ts), for picking a barrel.
  holes: Float32Array;
  nets: string[];
  // candidates lists, for each copper layer, the middle of each net's shape on it, for the probes a
  // browser spec clicks.
  candidates: Map<string, ProbeCandidates>;
  probeTimer: ReturnType<typeof setTimeout> | null;
}

// CanvasProfile hears a WebGL drawing's costs. SheetSurface hands its DrawProfile over as one.
export interface CanvasProfile {
  sheet(scene: Partial<DrawScene>): void;
  rendered(drawMs: number, textMs: number, drawCalls: number): void;
}

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
  // gate lets a frame draw only when the camera, the size or the sheet's drawing changed, so a view
  // nobody is touching costs nothing per frame (agni issue 1479). draws counts the frames drawn,
  // and the canvas element carries it in data-draws for a spec to read.
  private readonly gate = new DrawGate();
  private draws = 0;
  private resizeObserver?: ResizeObserver;
  private stack: StackState | null = null;
  private litCache: Map<string, Rgba> | null = null;
  private litPartsCache: Map<string, Rgba> | null = null;

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
  // glide moves the camera to a revealed subject over GLIDE_MS (agni issue 1546).
  glide = new Glide();
  // profile hears what each drawn sheet holds and what each drawn frame cost, for a page opened with
  // ?timing (drawprofile.ts). It is null otherwise, and then nothing is timed.
  profile: CanvasProfile | null = null;

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
    const t = performance.now();
    const decoded = toRenderable(sheet);
    this.stages = { renderableMs: performance.now() - t };
    this.draw(decoded);
  }
  // pendingText is the sheet whose labels and images wait for its first frame (agni issue 1503).
  private pendingText: DecodedSheet | null = null;

  // fillText puts the drawn sheet's labels and images into the text layer, once its geometry has
  // painted. A sheet replaced before then never fills.
  private fillText(stages: Record<string, number> | null): void {
    const sheet = this.pendingText;
    this.pendingText = null;
    if (!sheet || sheet !== this.sheet || !this.overlay || !this.camera) return;
    const t = performance.now();
    this.overlay.setContent(sheet.images, sheet.labels, sheet.fontFamily);
    const canvas = this.rootElement as HTMLCanvasElement;
    this.overlay.update(this.camera.getView(), canvas.clientWidth, canvas.clientHeight, this.dpr(), orientTurn(this.orientation));
    // The stack view puts the new text on its planes at its next frame.
    if (this.stack) this.gate.invalidate();
    if (stages) stages.labelsMs = performance.now() - t;
  }

  // stages times a sheet's way to its first frame under ?timing (agni issue 1503), logged once as
  // "agni sheet <view>: {...}" after the frame is marked.
  private stages: Record<string, number> | null = null;

  // setHiddenGroups forwards board layer visibility to the renderer (WS7-035); kept and
  // re-applied when a new sheet arrives, so the chosen view survives sheet swaps.
  setHiddenGroups(groups: Iterable<number>): void {
    this.hidden = [...groups];
    this.renderer?.setHiddenGroups(this.drawnHidden());
    this.overlay?.setHiddenGroups(this.drawnHidden());
  }

  // drawnHidden is what the renderer hides: the reader's hidden layers, and in the stack view's board
  // look the layers that look hides as well.
  private drawnHidden(): number[] {
    const st = this.stack;
    if (!st || st.look !== "board") return this.hidden;
    return [...new Set([...this.hidden, ...st.plan.hidden])];
  }
  private hidden: number[] = [];

  // getView / setView snapshot and restore the camera's pan/zoom, so the presenter can
  // remember a view per (mode, file, sheet). null before any sheet has been drawn.
  getView(): CameraView | null {
    return this.camera ? this.camera.getView() : null;
  }

  setView(v: CameraView): void {
    this.glide.stop();
    this.camera?.setView(v);
    this.touched = true;
  }

  // getCentre is the design point at the middle of the view and the zoom, in pixels per world unit.
  // setCentre puts a design point at the middle at a zoom. The point is in the design's own
  // coordinates rather than the sheet's rebased ones, so two drawings of different sheets line up:
  // two locked views of one sheet, and the two revisions of a visual diff (agni issue 1335).
  getCentre(): { at: Point; scale: number } | null {
    if (!this.camera || !this.sheet) return null;
    const v = this.camera.getView();
    return { at: { x: v.centerX + this.sheet.originX, y: v.centerY + this.sheet.originY }, scale: 1 / v.worldPerPixel };
  }

  setCentre(at: Point, scale: number): void {
    if (!this.sheet) return;
    this.setView({ centerX: at.x - this.sheet.originX, centerY: at.y - this.sheet.originY, worldPerPixel: 1 / scale });
  }

  // setGhost draws another sheet's matches for specs over this one, as the visual diff's union draws
  // the old revision's removed parts over the new one (agni issue 1335). The ghost sheet is placed by
  // its origin, so a part sits where the old revision drew it. Its lines draw as thin quads and its
  // fills as they are, in each spec's colour, and nothing of it picks. null clears it.
  setGhost(sheet: PackedSheet | null, specs: HighlightSpec[]): void {
    this.ghost = sheet ? { sheet: toRenderable(sheet), specs } : null;
    this.applyHighlights();
  }
  private ghost: { sheet: DecodedSheet; specs: HighlightSpec[] } | null = null;

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
    const camera = this.camera;
    const to = revealView(box, boundsOfVertices(this.sheet.vertices), canvas.width, canvas.height, quarter);
    this.touched = true;
    // The view travels to the subject rather than jumping (agni issue 1546). The frame loop draws
    // each step, since it draws whenever the camera's matrix changes.
    this.glide.run(camera.getView(), to, lerpCamera, (v) => {
      camera.setView(v);
      this.onViewChange?.();
    });
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

  // enterStack draws the board as its exploded stack (agni issue 1593), from the stack view and the
  // board's layer rows. It returns false before the board is drawn, or for a board with no stack.
  enterStack(view: BoardStackView, rows: BoardLayerRow[]): boolean {
    if (!this.gl || !this.sheet || !this.renderer || view.rows.length === 0) return false;
    const sheet = this.sheet;
    const layout = layoutStack(view.rows.map((r) => ({ name: r.name, kind: r.kind, layer: r.layer, thicknessMm: r.thicknessMm, color: r.color })));
    const depths = groupDepths(layout, rows);
    const b = boundsOfVertices(sheet.vertices);
    const width = Math.max(b.maxX - b.minX, b.maxY - b.minY);
    const byDepth = new Map<number, Set<number>>();
    for (const [g, d] of depths.depth) {
      const set = byDepth.get(d) ?? new Set<number>();
      set.add(g);
      byDepth.set(d, set);
    }
    const candidates = probeCandidates(sheet, rows);
    const through = rows.find((r) => r.kind === "through");
    let extras: StackExtras | null = null;
    const bodies = bodyGeometry(view.bodies, sheet.originX, sheet.originY);
    const geometry = stackGeometry(view, layout, sheet.originX, sheet.originY);
    try {
      extras = new StackExtras(this.gl, geometry, bodies);
    } catch (e) {
      console.warn("agni: the stack's barrels and slabs did not build", e);
    }
    const centre = { x: (b.minX + b.maxX) / 2, y: (b.minY + b.maxY) / 2 };
    this.stack = {
      view,
      layout,
      depths,
      camera: new StackCamera({ ...centre, z: 0 }, width),
      extras,
      bodies,
      showBodies: true,
      look: "layers",
      plan: boardLookPlan(rows),
      explode: 0,
      mmScale: view.unitsPerMm,
      width,
      planes: [...byDepth].map(([depth, groups]) => ({ depth, groups })),
      groups: new Set(sheet.primitives.map((p) => p.group)),
      barrelColor: hexToRgba(through?.color ?? "#9aa0a6", [0.6, 0.6, 0.6, 1]),
      holes: geometry.holes,
      nets: geometry.nets,
      candidates,
      probeTimer: null,
    };
    this.glide.stop();
    this.overlay?.setHiddenGroups(this.drawnHidden());
    this.setExplode(0.6);
    this.resize();
    this.stack.camera.setPose(PRESETS.iso);
    this.stack.camera.fit();
    const el = this.rootElement as HTMLElement;
    el.dataset.stack = "on";
    el.dataset.stackLook = "layers";
    el.dataset.stackBodies = JSON.stringify({ parts: bodies.bodies.length, defaulted: bodies.bodies.filter((b) => b.defaulted).length });
    this.gate.invalidate();
    return true;
  }

  // setStackBodies shows or hides the parts' bodies in the stack view. A hidden body neither draws
  // nor picks.
  setStackBodies(on: boolean): void {
    if (!this.stack || this.stack.showBodies === on) return;
    this.stack.showBodies = on;
    this.gate.invalidate();
  }

  // setStackLook draws the stack view in the layer look or the board look (agni issue 1653). The board
  // look recolours the packed groups and the pads through the renderer, and hides the layers it
  // leaves out, so it repacks nothing and the reader's own hidden layers come back as they were.
  setStackLook(look: StackLook): void {
    const st = this.stack;
    const sheet = this.sheet;
    if (!st || !sheet || !this.renderer || st.look === look) return;
    st.look = look;
    this.applyLook(st, sheet, this.renderer);
    (this.rootElement as HTMLElement).dataset.stackLook = look;
    this.gate.invalidate();
  }

  private applyLook(st: StackState | null, sheet: DecodedSheet, renderer: Renderer): void {
    this.overlay?.setHiddenGroups(this.drawnHidden());
    if (!st || st.look !== "board") {
      renderer.setLook(new Map(), new Map());
      renderer.setHiddenGroups(this.drawnHidden());
      return;
    }
    const pal = st.view.boardLook;
    const rgba = (hex: string | undefined, fallback: Rgba): Rgba => hexToRgba(hex ?? "", fallback);
    const colorOf = {
      mask: rgba(pal?.mask, [0.12, 0.32, 0.19, 1]),
      maskOverCopper: rgba(pal?.maskOverCopper, [0.23, 0.49, 0.31, 1]),
      padFinish: rgba(pal?.padFinish, [0.83, 0.69, 0.3, 1]),
      silk: rgba(pal?.silk, [0.95, 0.95, 0.93, 1]),
    };
    const groups = new Map<number, Rgba>();
    for (const [g, key] of st.plan.color) groups.set(g, colorOf[key]);
    const pads = new Map<number, Rgba>();
    for (const i of padPrimitives(sheet.keys, (p) => sheet.primitives[p]?.group ?? -1, st.plan)) pads.set(i, colorOf.padFinish);
    renderer.setLook(groups, pads);
    renderer.setHiddenGroups(this.drawnHidden());
  }

  // pickPlanes lists each depth the look draws groups at, with its groups. The board look draws every
  // group it shows on the top or the bottom face.
  private pickPlanes(st: StackState): StackState["planes"] {
    if (st.look !== "board") return st.planes;
    const top = new Set<number>();
    const bottom = new Set<number>();
    for (const [g, face] of st.plan.face) (face === "top" ? top : bottom).add(g);
    return [
      { depth: 0, groups: top },
      { depth: st.layout.totalMm, groups: bottom },
    ];
  }

  // bodyFaces is where the board's faces sit in world z at the current explode, and the scale a
  // part's height draws at, which is the board's true one.
  private bodyFaces(st: StackState): BodyFaces {
    return { zTop: stackZ(0, st.layout, st.mmScale), zBottom: stackZ(st.layout.totalMm, st.layout, st.mmScale), hScale: st.view.unitsPerMm };
  }

  // leaveStack returns to the flat board, with the camera, side and hidden layers it had.
  leaveStack(): void {
    const st = this.stack;
    if (!st) return;
    if (st.probeTimer) clearTimeout(st.probeTimer);
    st.extras?.dispose();
    this.stack = null;
    if (this.sheet && this.renderer) this.applyLook(null, this.sheet, this.renderer);
    const el = this.rootElement as HTMLElement;
    delete el.dataset.stack;
    delete el.dataset.stackProbes;
    delete el.dataset.stackBodyProbes;
    delete el.dataset.stackHeightMm;
    delete el.dataset.stackSpread;
    delete el.dataset.stackBytes;
    delete el.dataset.stackBodies;
    delete el.dataset.stackLook;
    this.overlay?.flat();
    this.overlay?.show();
    this.resize();
    this.gate.invalidate();
  }

  // setExplode spreads the stack from 0, the board's true proportions, to 1.
  setExplode(explode: number): void {
    const st = this.stack;
    if (!st) return;
    st.explode = Math.min(Math.max(explode, 0), 1);
    st.mmScale = perMm(st.explode, st.layout, st.width, st.view.unitsPerMm);
    const half = (st.layout.totalMm / 2) * st.mmScale;
    st.camera.setRadius(Math.hypot(st.width / 2, st.width / 2, half));
    // The drawn height from the top face to the bottom face, in millimetres of the board's own
    // coordinates. At explode 0 it is the board's thickness.
    const height = stackZ(0, st.layout, st.mmScale) - stackZ(st.layout.totalMm, st.layout, st.mmScale);
    (this.rootElement as HTMLElement).dataset.stackHeightMm = String(Math.round((height / st.view.unitsPerMm) * 1e6) / 1e6);
    (this.rootElement as HTMLElement).dataset.stackSpread = String(st.explode);
  }

  setStackPreset(p: StackPreset): void {
    if (!this.stack) return;
    this.stack.camera.setPose(PRESETS[p]);
    this.stack.camera.fit();
  }

  // stackZOf is the world z a group draws at in the stack view, or null for the through copper and
  // drill holes, which draw as barrels.
  private stackZOf(st: StackState, group: number): number | null {
    if (st.look === "board") {
      const face = st.plan.face.get(group);
      if (!face) return null;
      return stackZ(face === "top" ? 0 : st.layout.totalMm, st.layout, st.mmScale);
    }
    if (st.depths.barrels.has(group)) return null;
    return stackZ(st.depths.depth.get(group) ?? 0, st.layout, st.mmScale);
  }

  // litNets lists the nets the highlight lights, by name, with their colours, for the barrels. It is
  // kept until the highlight or the sheet changes, since a frame asks for it and a net id's lookup
  // reads every key on the board.
  private litNets(): Map<string, Rgba> {
    if (this.litCache) return this.litCache;
    const lit = new Map<string, Rgba>();
    const sheet = this.sheet;
    if (!sheet) return lit;
    this.litCache = lit;
    const fallback = hexToRgba(DEFAULT_HIGHLIGHT_COLOR, [1, 0, 1, 1]);
    let byId: Map<string, string> | null = null;
    for (const spec of this.highlights) {
      const color = hexToRgba(spec.color || DEFAULT_HIGHLIGHT_COLOR, fallback, boardNetAlpha(spec.alpha || 1));
      for (const n of spec.nets ?? []) lit.set(n, color);
      if (spec.netIds?.length) {
        byId ??= new Map(sheet.keys.filter((k) => k.netId && k.net).map((k) => [k.netId, k.net]));
        for (const id of spec.netIds) {
          const n = byId.get(id);
          if (n) lit.set(n, color);
        }
      }
    }
    return lit;
  }

  // pickStack picks through the stack view at device pixel (px, py), through the flat board's pick
  // index (stackpick.ts), and the parts' bodies where they show (stackbodies.ts).
  // only limits the pick to some of the planes, and leaves the bodies out.
  private pickStack(px: number, py: number, only?: StackState["planes"]): Selection | null {
    const st = this.stack;
    const sheet = this.sheet;
    if (!st || !sheet) return null;
    const picks = (this.picks ??= new PickIndex(sheet));
    const planes = (only ?? this.pickPlanes(st)).map((p) => ({ z: stackZ(p.depth, st.layout, st.mmScale), groups: p.groups }));
    const barrels = only ? [] : this.drawnBarrels(st);
    const layers = () =>
      pickThroughStack(st.camera, planes, st.groups, this.drawnHidden(), px, py, this.dpr(), (x, y, perPx, hidden) => picks.pick(x, y, perPx, hidden, this.depth), barrels);
    if (only || !st.showBodies) return layers();
    return pickWithBodies(st.camera, st.bodies.bodies, this.bodyFaces(st), px, py, layers);
  }

  // barrelsShow says whether the stack view draws its barrels: the layer look draws them unless the
  // reader hid the through copper they stand for.
  private barrelsShow(st: StackState): boolean {
    return st.look !== "board" && (!st.depths.barrels.size || ![...st.depths.barrels].some((g) => this.hidden.includes(g)));
  }

  // drawnBarrels is each barrel the stack view draws, in world coordinates, for picking (agni issue
  // 1607).
  private drawnBarrels(st: StackState): Barrel[] {
    if (!this.barrelsShow(st)) return [];
    const out: Barrel[] = [];
    for (let i = 0; i < st.nets.length; i++) {
      const h = st.holes.subarray(i * HOLE_FLOATS, (i + 1) * HOLE_FLOATS);
      out.push({ x: h[0], y: h[1], r: h[2], zTop: stackZ(h[3], st.layout, st.mmScale), zBottom: stackZ(h[4], st.layout, st.mmScale), net: st.nets[i] });
    }
    return out;
  }

  // placeText puts each layer's text on its plane in the stack view (agni issue 1607). A face's text
  // shows only from its own side, and the board look draws silkscreen text in the silkscreen colour.
  private placeText(st: StackState): void {
    const overlay = this.overlay;
    if (!overlay) return;
    const eyeZ = st.camera.basis().eye.z;
    const silk = st.look === "board" ? st.view.boardLook?.silk : undefined;
    const faceOf = (g: number): "top" | "bottom" | undefined => {
      const f = st.plan.face.get(g);
      return st.look === "board" || st.plan.color.get(g) === "silk" ? f : undefined;
    };
    overlay.placeOnPlanes((p) => st.camera.project(p), this.dpr(), (g) => {
      if (Math.abs(eyeZ) < GRAZING) return null;
      const z = this.stackZOf(st, g);
      if (z === null || !faceShows(faceOf(g), eyeZ)) return null;
      return { z, fill: silk && st.plan.color.get(g) === "silk" ? silk : undefined };
    });
  }

  // litParts lists the parts the highlight lights, by ref, with their colours, for the bodies. It is
  // kept until the highlight or the sheet changes, as litNets is.
  private litParts(): Map<string, Rgba> {
    if (this.litPartsCache) return this.litPartsCache;
    const lit = new Map<string, Rgba>();
    const fallback = hexToRgba(DEFAULT_HIGHLIGHT_COLOR, [1, 0, 1, 1]);
    for (const spec of this.highlights) {
      const color = hexToRgba(spec.color || DEFAULT_HIGHLIGHT_COLOR, fallback);
      for (const ref of spec.components ?? []) lit.set(ref, color);
    }
    this.litPartsCache = lit;
    return lit;
  }

  // writeProbes names, for each copper layer, a point on one of its tracks that a click picks, in CSS
  // pixels from the canvas's corner, as data-stack-probes. A browser spec reads it to click a track on
  // an inner layer, as it reads data-lit for what a highlight lit.
  private writeProbes(): void {
    const st = this.stack;
    if (!st) return;
    const dpr = this.dpr();
    const probes: { layer: string; net: string; x: number; y: number }[] = [];
    for (const [layer, c] of st.candidates) {
      if (c.groups.every((g) => this.hidden.includes(g))) continue;
      const z = this.stackZOf(st, c.groups[0]);
      if (z === null) continue;
      // The lowest on screen first, since a nearer layer covers a farther one least at the stack's
      // front edge. A click lands on a whole CSS pixel, so a probe does too, and it counts only if a
      // pick there finds its net.
      const all: { x: number; y: number }[] = [];
      for (let i = 0; i + 1 < c.points.length; i += 2) all.push(st.camera.project({ x: c.points[i], y: c.points[i + 1], z }));
      const tries = all.sort((a, b) => b.y - a.y).slice(0, PROBE_TRIES);
      const own = this.pickPlanes(st).filter((p) => c.groups.some((g) => p.groups.has(g)));
      for (const t of tries) {
        const x = Math.round(t.x / dpr);
        const y = Math.round(t.y / dpr);
        // A probe counts when a click there picks a net, and this layer alone picks the same net there.
        const sel = this.pickStack(x * dpr, y * dpr);
        if (sel?.kind !== "net" || this.pickStack(x * dpr, y * dpr, own)?.net !== sel.net) continue;
        probes.push({ layer, net: sel.net!, x, y });
        break;
      }
    }
    (this.rootElement as HTMLElement).dataset.stackProbes = JSON.stringify(probes);
    (this.rootElement as HTMLElement).dataset.stackBodyProbes = JSON.stringify(this.bodyProbes(st));
  }

  // bodyProbes names up to BODY_PROBES parts on the side facing the camera, each with the CSS pixel
  // at the middle of its top face, where a click picks that part.
  private bodyProbes(st: StackState): { ref: string; x: number; y: number }[] {
    const out: { ref: string; x: number; y: number }[] = [];
    if (!st.showBodies) return out;
    const dpr = this.dpr();
    const canvas = this.rootElement as HTMLCanvasElement;
    const f = this.bodyFaces(st);
    const fromBelow = st.camera.basis().eye.z < 0;
    for (const b of st.bodies.bodies) {
      if (b.back !== fromBelow) continue;
      const n = b.ring.length / 2;
      let cx = 0;
      let cy = 0;
      for (let i = 0; i < n; i++) {
        cx += b.ring[2 * i] / n;
        cy += b.ring[2 * i + 1] / n;
      }
      const at = st.camera.project({ x: cx, y: cy, z: bodyTopZ(b, f) });
      const x = Math.round(at.x / dpr);
      const y = Math.round(at.y / dpr);
      if (x < 0 || y < 0 || x > canvas.clientWidth || y > canvas.clientHeight) continue;
      const sel = this.pickStack(x * dpr, y * dpr);
      if (sel?.kind !== "component" || sel.ref !== b.ref) continue;
      out.push({ ref: b.ref, x, y });
      if (out.length >= BODY_PROBES) break;
    }
    return out;
  }

  // dispose stops drawing and frees the GPU's copy of the sheet, for a sheet view the reader closed
  // or switched to SVG.
  dispose(): void {
    this.disposed = true;
    if (this.stack) this.leaveStack();
    cancelAnimationFrame(this.frameId);
    this.resizeObserver?.disconnect();
    this.overlay?.hide();
    this.renderer?.dispose();
    this.gl?.getExtension("WEBGL_lose_context")?.loseContext();
    this.gl = undefined;
    this.renderer = undefined;
  }

  private fit(): void {
    this.glide.stop();
    const canvas = this.rootElement as HTMLCanvasElement;
    this.camera?.fit(canvas.width, canvas.height);
  }

  private draw(sheet: DecodedSheet): void {
    const gl = this.gl!;
    const canvas = this.rootElement as HTMLCanvasElement;
    // A new sheet's groups and origin are its own, so a stack view of the old one ends.
    if (this.stack) this.leaveStack();
    this.sheet = sheet;
    this.picks = undefined;
    const built = performance.now();
    this.renderer?.dispose();
    let at = performance.now();
    const lap = (name: string) => {
      const now = performance.now();
      if (this.stages) this.stages[name] = now - at;
      at = now;
    };
    // The first frame draws before the batches are laid out, so laying them out never delays it.
    this.renderer = new Renderer(gl, sheet, { defer: true });
    lap("rendererMs");
    this.renderer.setHiddenGroups(this.hidden);
    // A new sheet's camera replaces the old one, and a glide toward the old sheet stops with it.
    this.glide.stop();
    this.camera = new Camera(boundsOfVertices(sheet.vertices));
    this.camera.setOrientation(this.orientation);
    this.touched = false;
    this.applyOrder();
    this.overlay ??= this.makeOverlay();
    this.overlay?.setHiddenGroups(this.hidden);
    lap("cameraMs");
    // The labels go in after the first frame (agni issue 1503). Laying out a board's text, 5,732
    // labels on Jetson, held the first paint of its copper. Until then the layer shows nothing.
    this.overlay?.setContent([], [], sheet.fontFamily);
    this.pendingText = sheet;
    this.applyHighlights(); // highlights chosen before this sheet was (re)drawn are reapplied
    lap("highlightsMs");
    this.resize();
    this.camera.fit(canvas.width, canvas.height);
    this.marked = false;
    this.profile?.sheet({
      primitives: sheet.primitives.length,
      vertices: sheet.vertices.length / 2,
      labels: sheet.labels.length,
      images: sheet.images.length,
      buildMs: performance.now() - built,
    });
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
    this.litCache = null;
    this.litPartsCache = null;
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
    const minSpan = Math.min(sb.maxX - sb.minX, sb.maxY - sb.minY);
    const pathHalfWidth = PATH_WIDTH_FRACTION * minSpan;
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
        const marks = pinMarkTriangles(sheet, g, pinMarkRadius(Math.max(sb.maxX - sb.minX, sb.maxY - sb.minY)));
        if (marks.length > 0) overlays.push({ color, vertices: Int32Array.from(marks, Math.round) });
        continue;
      }
      // On the board a net repaints its own copper (agni issue 1543), since the path marker is about
      // as wide as a track and disappears into it.
      const repaint = boardNetDraws(sheet, g, fallback);
      if (repaint) {
        draws.push(...repaint);
        continue;
      }
      const tris: number[] = [];
      if (g.shape === HighlightShape.PATH) {
        const w = pathHalfWidth * g.strokeScale; // user width scale (WS9-044)
        // Appended one by one, because a spread passes every number as an argument, and a board's
        // ground net makes too many quads for the call stack (agni issue 1330, on Jetson's GND).
        for (const entity of g.entities) for (const v of pathQuads(sheet.vertices, sheet.primitives, entity, w)) tris.push(v);
      } else {
        // A frame is a translucent fill under an opaque stroke in the same color (agni issue 1536).
        // Both size from the sheet's smaller span, as core/render/highlight.go does, so a small part
        // reads on a dense board and the frame matches the SVG one.
        // A pin keeps the path marker's width as its floor, a fraction of the sheet that reads at any
        // coordinate scale, and only a part takes the span floor.
        const ring: number[] = [];
        const strokeWidth = frameStroke(minSpan, g.strokeScale);
        for (const [i, entity] of g.entities.entries()) {
          const b = entityBounds(sheet.vertices, sheet.primitives, entity);
          if (!b) continue;
          const frame = entityFrame(b.minX, b.minY, b.maxX, b.maxY, g.parts[i] ? Math.max(pathHalfWidth, frameFloor(minSpan)) : pathHalfWidth);
          const rect = g.shape === HighlightShape.BOUNDING_RECT;
          tris.push(...(rect ? rectTriangles(frame) : circleTriangles(frame)));
          for (const v of rect ? rectRing(frame, strokeWidth) : circleRing(frame, strokeWidth)) ring.push(v);
        }
        overlays.push({ color, vertices: Int32Array.from(tris, Math.round) });
        overlays.push({ color: hexToRgba(g.color, fallback, frameStrokeAlpha(g.alpha)), vertices: Int32Array.from(ring, Math.round) });
        continue;
      }
      overlays.push({ color, vertices: Int32Array.from(tris, Math.round) });
    }
    if (this.ghost) {
      for (const o of ghostOverlays(this.ghost.sheet, this.ghost.specs, sheet, pathHalfWidth / 4, fallback)) {
        for (const s of o.subjects) lit.add(s);
        overlays.push({ color: o.color, vertices: o.vertices });
      }
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
    this.gate.invalidate();
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
    this.stack?.camera.setViewport(canvas.width, canvas.height);
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
    let panning = false;
    const band = new AreaBand(canvas.parentElement ?? canvas);
    canvas.addEventListener("mousedown", (e) => {
      dragging = true;
      // The stack view orbits on a drag and pans on a Shift-drag, so it draws no box.
      selecting = !this.stack && e.shiftKey && (this.onArea !== null || this.onToggle !== null);
      panning = e.shiftKey;
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
      const sel = this.stack ? this.pickStackAt(e.clientX, e.clientY) : this.pickAt(e.clientX, e.clientY);
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
      this.glide.stop();
      if (this.stack) {
        if (panning) this.stack.camera.pan((e.clientX - lastX) * this.dpr(), (e.clientY - lastY) * this.dpr());
        else this.stack.camera.orbit(e.clientX - lastX, e.clientY - lastY);
        lastX = e.clientX;
        lastY = e.clientY;
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
        this.glide.stop();
        if (this.stack) {
          this.stack.camera.zoomAt((e.clientX - rect.left) * this.dpr(), (e.clientY - rect.top) * this.dpr(), wheelZoomFactor(e.deltaY));
          return;
        }
        this.camera.zoomAt((e.clientX - rect.left) * this.dpr(), (e.clientY - rect.top) * this.dpr(), wheelZoomFactor(e.deltaY));
        this.touched = true;
        this.onViewChange?.();
      },
      { passive: false },
    );

    const frame = (): void => {
      if (this.disposed) return;
      if (this.stack && this.renderer) {
        this.stackFrame(this.stack, this.renderer, canvas);
        this.frameId = requestAnimationFrame(frame);
        return;
      }
      const matrix = this.camera?.matrix();
      if (
        this.renderer &&
        this.camera &&
        matrix &&
        this.gate.shouldDraw({ matrix, width: canvas.clientWidth, height: canvas.clientHeight, dpr: this.dpr(), content: this.renderer, version: this.renderer.version })
      ) {
        const t0 = performance.now();
        const calls = this.renderer.render(matrix, this.camera.viewRect(), this.camera.worldPerDevicePixel());
        const t1 = performance.now();
        canvas.dataset.draws = String(++this.draws);
        // The overlay reads the same camera in the same tick, so text moves and turns with the
        // geometry.
        this.overlay?.update(this.camera.getView(), canvas.clientWidth, canvas.clientHeight, this.dpr(), orientTurn(this.orientation));
        this.profile?.rendered(t1 - t0, performance.now() - t1, calls);
        if (!this.marked) {
          this.marked = true;
          const view = this.markView;
          const stages = this.stages;
          this.stages = null;
          if (stages) {
            stages.firstRenderMs = t1 - t0;
            stages.overlayUpdateMs = performance.now() - t1;
          }
          const painted = performance.now();
          requestAnimationFrame(() => {
            performance.mark(SHEET_DRAWN, { detail: { mode: "webgl", view } });
            this.fillText(stages);
            if (stages && timingMode()) {
              stages.toPaintMs = performance.now() - painted;
              const ms = Object.fromEntries(Object.entries(stages).map(([k, v]) => [k, Math.round(v * 10) / 10]));
              console.info(`agni sheet ${view}: ${JSON.stringify(ms)}`);
            }
          });
        }
      }
      this.frameId = requestAnimationFrame(frame);
    };
    this.frameId = requestAnimationFrame(frame);
  }

  // stackFrame draws one frame of the stack view when its camera, size or drawing changed.
  private stackFrame(st: StackState, renderer: Renderer, canvas: HTMLCanvasElement): void {
    const view = st.camera.matrix();
    // The renderer's version moves with the highlight and the hidden layers, and the key with the
    // camera and the explode control.
    const key = Float32Array.from([...view, st.mmScale, st.showBodies ? 1 : 0, st.look === "board" ? 1 : 0]);
    if (!this.gate.shouldDraw({ matrix: key, width: canvas.clientWidth, height: canvas.clientHeight, dpr: this.dpr(), content: renderer, version: renderer.version })) return;
    const t0 = performance.now();
    const zTop = stackZ(0, st.layout, st.mmScale);
    let calls = renderer.renderStack({ view, zOf: (g) => this.stackZOf(st, g), overlayZ: zTop }, st.camera.worldPerPixel);
    calls +=
      st.extras?.draw({
        view,
        zTop,
        zPerMm: st.mmScale,
        eyeZ: st.camera.basis().eye.z,
        barrels: this.barrelsShow(st),
        barrelColor: st.barrelColor,
        lit: this.litNets(),
        parseColor: (hex, alpha) => hexToRgba(hex, [0.7, 0.6, 0.4, alpha], alpha),
        bodies: st.showBodies
          ? {
              ...this.bodyFaces(st),
              colorOf: (hex, defaulted) => bodyColor(st.view, hex, defaulted),
              litParts: this.litParts(),
            }
          : undefined,
        board: st.look === "board" ? { mask: hexToRgba(st.view.boardLook?.mask ?? "", [0.12, 0.32, 0.19, 1]) } : undefined,
      }) ?? 0;
    // A view that opens straight into the stack, as one switched to WebGL to show it does, has had
    // no flat frame to fill its labels after.
    if (this.pendingText) this.fillText(null);
    this.placeText(st);
    canvas.dataset.draws = String(++this.draws);
    // What the board and the stack view hold on the GPU, in bytes, for the PR's memory budget.
    canvas.dataset.stackBytes ??= JSON.stringify({ board: renderer.bytes, stack: st.extras?.bytes ?? 0 });
    this.profile?.rendered(performance.now() - t0, 0, calls);
    if (st.probeTimer) clearTimeout(st.probeTimer);
    // The probes go stale as soon as the view moves, so they are gone until the view settles.
    delete canvas.dataset.stackProbes;
    delete canvas.dataset.stackBodyProbes;
    st.probeTimer = setTimeout(() => {
      st.probeTimer = null;
      if (this.stack === st) this.writeProbes();
    }, 150);
  }

  // pickStackAt is pickStack for a client point.
  private pickStackAt(clientX: number, clientY: number): Selection | null {
    const rect = (this.rootElement as HTMLCanvasElement).getBoundingClientRect();
    return this.pickStack((clientX - rect.left) * this.dpr(), (clientY - rect.top) * this.dpr());
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

// bodyColor is the colour a run of part bodies draws in (agni issue 1653). A body whose file gives
// no height draws paler, so it reads as a guess: the plain body takes the stack's default body
// colour, and a class colour mixes DEFAULTED_PALER toward white. KiCad states no heights, so every
// part on a KiCad board is defaulted, and a stronger mix washed a black IC out to grey.
function bodyColor(view: BoardStackView, hex: string, defaulted: boolean): Rgba {
  const plain = hexToRgba(view.bodyColor, [0.3, 0.31, 0.34, 1]);
  if (!defaulted) return hexToRgba(hex, plain);
  if (!hex || hex === view.bodyColor) return hexToRgba(view.defaultBodyColor, [0.65, 0.68, 0.71, 1]);
  const c = hexToRgba(hex, plain);
  const k = DEFAULTED_PALER;
  return [c[0] + (1 - c[0]) * k, c[1] + (1 - c[1]) * k, c[2] + (1 - c[2]) * k, c[3]];
}

// DEFAULTED_PALER is how far a class colour mixes toward white for a body at the default height.
const DEFAULTED_PALER = 0.2;

// PROBE_TRIES is how many points on a layer writeProbes tries before it gives the layer up.
const PROBE_TRIES = 24;

// BODY_PROBES is how many parts the stack view names a clickable point on, in data-stack-body-probes.
const BODY_PROBES = 3;

// ProbeCandidates is a copper layer's groups and points inside the net shapes drawn in them.
interface ProbeCandidates {
  groups: readonly number[];
  // points holds x, y pairs.
  points: Float64Array;
}

// probeCandidates lists points inside each copper layer's net shapes, its tracks, pours and zones,
// leaving out pads and the pick areas, which a part's key claims first. A filled shape gives the middle
// of some of its triangles, so a plane with holes still offers points on its copper.
function probeCandidates(sheet: DecodedSheet, rows: readonly BoardLayerRow[]): Map<string, ProbeCandidates> {
  const layerOf = new Map<number, string>();
  const groupsOf = new Map<string, number[]>();
  const points = new Map<string, number[]>();
  for (const r of rows) {
    if (r.kind !== "copper") continue;
    const groups = r.groups.filter((g) => !pickOnly(g));
    if (groups.length === 0) continue;
    groupsOf.set(r.name, groups);
    points.set(r.name, []);
    for (const g of groups) if (!layerOf.has(g)) layerOf.set(g, r.name);
  }
  for (const k of sheet.keys) {
    if (!k.net || k.pin || k.refDes) continue;
    const layer = layerOf.get(sheet.primitives[k.primitive]?.group ?? -1);
    if (layer !== undefined) shapePoints(sheet, k.primitive, points.get(layer)!);
  }
  const out = new Map<string, ProbeCandidates>();
  for (const [layer, groups] of groupsOf) out.set(layer, { groups, points: Float64Array.from(points.get(layer)!) });
  return out;
}

// SHAPE_SAMPLES is how many points shapePoints takes from one filled shape at most.
const SHAPE_SAMPLES = 16;

// shapePoints appends to out the middle of up to SHAPE_SAMPLES of a filled shape's triangles, spread
// through it, or the middle of a line's first segment, as x, y pairs.
function shapePoints(sheet: DecodedSheet, primitive: number, out: number[]): void {
  const p = sheet.primitives[primitive];
  const v = sheet.vertices;
  if (!p || p.count < 2) return;
  if (p.kind !== KIND_TRIANGLES) {
    const a = p.firstVertex;
    out.push((v[a * 2] + v[a * 2 + 2]) / 2, (v[a * 2 + 1] + v[a * 2 + 3]) / 2);
    return;
  }
  const tris = Math.floor(p.count / 3);
  const step = Math.max(1, Math.floor(tris / SHAPE_SAMPLES));
  for (let t = 0; t < tris; t += step) {
    const a = p.firstVertex + t * 3;
    out.push((v[a * 2] + v[a * 2 + 2] + v[a * 2 + 4]) / 3, (v[a * 2 + 1] + v[a * 2 + 3] + v[a * 2 + 5]) / 3);
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

// pinMarkTriangles draws each listed pin of an outline highlight as a filled dot of the given world
// radius, centred on the pin, as the SVG overlay draws it. A board's pin is a pad, which its recolour
// already shows at full size, so a board gets none.
export function pinMarkTriangles(sheet: DecodedSheet, g: { entities: number[][]; pins: boolean[] }, radius: number): number[] {
  if (sheet.sheetId === "board") return [];
  const tris: number[] = [];
  for (const [i, entity] of g.entities.entries()) {
    if (!g.pins[i]) continue;
    const b = entityBounds(sheet.vertices, sheet.primitives, entity);
    if (!b) continue;
    const cx = (b.minX + b.maxX) / 2;
    const cy = (b.minY + b.maxY) / 2;
    for (const v of circleTriangles({ minX: cx - radius, minY: cy - radius, maxX: cx + radius, maxY: cy + radius, cx, cy, r: radius })) tris.push(v);
  }
  return tris;
}

// ghostOverlays tessellates a ghost sheet's matches for specs into overlay triangles in the drawn
// sheet's frame, the two sheets lined up by their origins. A line becomes quads halfWidth to either
// side, a fill keeps its triangles, and a point becomes a small disc.
// boardNetDraws repaints a net lit on the board in its highlight colour (agni issue 1543): its
// tracks, pads and vias at boardNetAlpha, and its pours at their own wash times that, so a lit ground
// tints the plane rather than covering the board. It answers null for anything but a net's PATH on
// the board sheet, which keeps its marker. core/render.HighlightBoardSVG draws the SVG board the same
// way.
export function boardNetDraws(
  sheet: DecodedSheet,
  g: { shape: HighlightShape; color: string; alpha: number; primitives: Set<number> },
  fallback: ReturnType<typeof hexToRgba>,
): { color: ReturnType<typeof hexToRgba>; primitives: Set<number> }[] | null {
  if (sheet.sheetId !== "board" || g.shape !== HighlightShape.PATH) return null;
  const alpha = boardNetAlpha(g.alpha);
  const byAlpha = new Map<number, Set<number>>();
  for (const p of g.primitives) {
    const wash = sheet.groupAlphas?.[sheet.primitives[p]?.group ?? 0] || 1;
    const a = alpha * wash;
    const set = byAlpha.get(a) ?? new Set<number>();
    set.add(p);
    byAlpha.set(a, set);
  }
  return [...byAlpha].map(([a, primitives]) => ({ color: hexToRgba(g.color, fallback, a), primitives }));
}

export function ghostOverlays(
  ghost: DecodedSheet,
  specs: HighlightSpec[],
  onto: Pick<DecodedSheet, "originX" | "originY">,
  halfWidth: number,
  fallback: ReturnType<typeof hexToRgba>,
): { color: ReturnType<typeof hexToRgba>; vertices: Int32Array; subjects: string[] }[] {
  const dx = ghost.originX - onto.originX;
  const dy = ghost.originY - onto.originY;
  const drawn = ghost.keys.filter((k) => !pickOnly(ghost.primitives[k.primitive]?.group ?? 0));
  const out: { color: ReturnType<typeof hexToRgba>; vertices: Int32Array; subjects: string[] }[] = [];
  for (const g of resolveHighlights(drawn, specs)) {
    const tris: number[] = [];
    for (const i of g.primitives) {
      const p = ghost.primitives[i];
      if (!p) continue;
      if (p.kind === KIND_TRIANGLES) {
        for (let v = p.firstVertex; v < p.firstVertex + p.count; v++) tris.push(ghost.vertices[v * 2], ghost.vertices[v * 2 + 1]);
      } else if (p.kind === KIND_POINTS || p.count === 1) {
        const x = ghost.vertices[p.firstVertex * 2], y = ghost.vertices[p.firstVertex * 2 + 1];
        for (const t of circleTriangles(entityFrame(x, y, x, y, halfWidth * 2))) tris.push(t);
      } else {
        const ring = p.kind === KIND_LINE_LOOP;
        for (const t of pathQuads(ghost.vertices, ghost.primitives, [i], halfWidth)) tris.push(t);
        if (ring && p.count > 2) {
          // A loop closes back on its first point, which pathQuads leaves open.
          const a = p.firstVertex + p.count - 1, b = p.firstVertex;
          const closing = { kind: p.kind, group: p.group, firstVertex: 0, count: 2 };
          const pts = Int32Array.from([ghost.vertices[a * 2], ghost.vertices[a * 2 + 1], ghost.vertices[b * 2], ghost.vertices[b * 2 + 1]]);
          for (const t of pathQuads(pts, [closing], [0], halfWidth)) tris.push(t);
        }
      }
    }
    for (let k = 0; k < tris.length; k += 2) {
      tris[k] += dx;
      tris[k + 1] += dy;
    }
    out.push({ color: hexToRgba(g.color, fallback, g.alpha), vertices: Int32Array.from(tris, Math.round), subjects: g.subjects });
  }
  return out;
}
