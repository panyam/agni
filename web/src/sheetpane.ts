// A sheet view is a dock panel that shows one sheet of the open design beside the canvas (agni issue
// 1326). It has its own sheet, camera, board layers and highlight, and shares the page's selection.
// A reader opens one from the Sheets panel, so a board sits beside its schematic and a click on one
// finds the part on the other.
//
// The pane draws through a SheetSurface, as the canvas does, in SVG or in WebGL as its reader chooses
// (agni issue 1368). So a pick here reads the same keys a pick on the canvas does. It asks the engine
// through the ports in SheetPaneDeps, and holds no client of its own, so a test drives it with fakes.
import type { SvgView, SvgViewState } from "./svgview.js";
import { SheetSurface, type SurfaceMode } from "./sheetsurface.js";
import { CanvasComponent } from "./canvas.js";
import type { PackedSheet } from "@panyam/agni-client/gen/agni/v1/geom/geom_packed_pb";
import { labelFor, pinOf, type Selection } from "./selection.js";
import type { HighlightSpec } from "./highlights.js";
import { chooseSheet, LINKS, type Link } from "./sheetfollow.js";
import { UPRIGHT, flip, flipped, fromBelow, rotateBy, type Orientation } from "./orient.js";
import type { BoardLayerRow, BoardStackView } from "@panyam/agni-client/gen/agni/v1/geom/geom_packed_pb";
import { defaultHidden, presetHidden, soloHidden, toggled } from "./boardlayers.js";
import { boardLayerListIsland } from "./boardlayerlist.js";
import type { BoardFollow } from "@panyam/agni-client/gen/agni/v1/webapi/design_pb";
import { delayedBusy, type Busy } from "./busy.js";
import { sheetStep, stepLabel } from "./sheetstep.js";
import type { StackPreset } from "./stackcamera.js";
import { defaultedNote } from "./stackbodies.js";
import type { StackLook } from "./stacklook.js";

// ViewTurn is how a view is turned for a sheet, kept per design and sheet so a view opened on the
// board again comes back the way the bench has it (agni issue 1327). A board flipped an odd number of
// times is seen from underneath, the back copper nearest (fromBelow, agni issue 1472).
export interface ViewTurn {
  orientation: Orientation;
}

// turnKey names a sheet's kept turn in storage, by the design's mount and path and the sheet.
export function turnKey(mount: string, path: string, sheetId: string): string {
  return `agni-sheetview-turn:${mount}/${path}:${sheetId}`;
}

// parseTurn reads a kept turn back, and returns null for anything that is not one, such as a value
// an older page wrote under the same key. An older page also kept a bottom field, which the flips
// now decide, so it is ignored.
export function parseTurn(text: string | null): ViewTurn | null {
  if (!text) return null;
  try {
    const v = JSON.parse(text) as { orientation?: Partial<Orientation> };
    const o = v.orientation;
    if (!o || ![0, 90, 180, 270].includes(o.rot as number) || typeof o.flipH !== "boolean" || typeof o.flipV !== "boolean") return null;
    return { orientation: { rot: o.rot as Orientation["rot"], flipH: o.flipH, flipV: o.flipV } };
  } catch {
    return null;
  }
}

// TurnStore keeps a sheet's turn. The page backs it with localStorage, and a page with no storage
// keeps nothing, so a view opens upright.
export interface TurnStore {
  load(sheetId: string): ViewTurn | null;
  save(sheetId: string, turn: ViewTurn): void;
}

export const BOARD_SHEET = "board";

// Located is where a selection is drawn. sheets lists the sheets that draw it, the board included
// when it was asked for. follow is LocateSubjects' answer for how a board view follows it, the side
// to turn to and the layer rows to show, when the board was read and draws it (agni issue 1635).
export interface Located {
  sheets: string[];
  follow?: BoardFollow;
}

// FOLLOW_SIDE_KEY keeps a reader's choice to turn off a board view's following of side and layers
// (agni issue 1587). Every board view shares it, and an absent key means on.
export const FOLLOW_SIDE_KEY = "agni.board.followSide";

function loadFollowSide(): boolean {
  try {
    return globalThis.localStorage?.getItem(FOLLOW_SIDE_KEY) !== "off";
  } catch {
    return true;
  }
}

function saveFollowSide(on: boolean): void {
  try {
    if (on) globalThis.localStorage?.removeItem(FOLLOW_SIDE_KEY);
    else globalThis.localStorage?.setItem(FOLLOW_SIDE_KEY, "off");
  } catch {
    // A page without storage forgets the choice on reload.
  }
}

// SheetDrawing is one sheet as a pane draws it, the SVG document or, in WebGL, the packed sheet.
// boardLayers is empty for a schematic sheet, and boardStack is unset for one (agni 1603).
export interface SheetDrawing {
  svg: string;
  packed?: PackedSheet;
  boardLayers: BoardLayerRow[];
  boardStack?: BoardStackView;
}

// GlFactory builds a pane's WebGL drawing on its canvas and text host, or returns null where WebGL2
// is missing. The page uses glDrawing, and a test passes a fake.
export type GlFactory = (id: string, canvas: HTMLCanvasElement, textHost: HTMLElement) => CanvasComponent | null;

// glDrawing is the page's GlFactory. A pane's drawing reports to no readout, and names the pane in
// its SHEET_DRAWN marks.
export const glDrawing: GlFactory = (id, canvas, textHost) => {
  const gl = new CanvasComponent(`${id}-gl`, canvas);
  gl.textHost = textHost;
  gl.readout = null;
  gl.activate();
  return gl.ready() ? gl : null;
};

// HEAVY_DRAWING_BYTES is the size of a drawing that blocks the page long enough to need the loader
// on screen before it starts. Jetson's 5.6 MB board SVG blocked the page for about 190 ms.
export const HEAVY_DRAWING_BYTES = 1_000_000;

// heavyDrawing says whether drawing a sheet in mode will block the page for long, by the size of the
// SVG document or of the packed sheet's buffers.
export function heavyDrawing(d: SheetDrawing, mode: SurfaceMode): boolean {
  if (mode === "webgl" && d.packed) return (d.packed.vertices?.length ?? 0) + (d.packed.primitives?.length ?? 0) > HEAVY_DRAWING_BYTES;
  return d.svg.length > HEAVY_DRAWING_BYTES;
}

// nextPaint resolves once the browser has painted, so an element shown just before is on screen
// before the page blocks. A page with no animation frames waits a timer's tick instead.
function nextPaint(): Promise<void> {
  return new Promise((resolve) => {
    if (typeof requestAnimationFrame === "function") requestAnimationFrame(() => setTimeout(resolve, 0));
    else setTimeout(resolve, 0);
  });
}

// SheetPaneDeps are what a pane asks of the page.
export interface SheetPaneDeps {
  // sheets lists the open design's sheets, for the picker and for naming the other sheets.
  sheets: () => { id: string; name: string }[];
  // render returns a sheet as the canvas draws it in mode, an SVG document with pick targets or a
  // packed sheet, and the board's layer rows when the sheet is the board.
  render: (sheetId: string, mode: SurfaceMode) => Promise<SheetDrawing>;
  // overlay returns the highlight overlay for specs on a sheet, framed like render's document.
  overlay: (sheetId: string, specs: HighlightSpec[]) => Promise<string>;
  // drawnOn lists the sheets that draw a selection, the board included when board is set. With
  // board set it also says where the board places the selection.
  drawnOn: (sel: Selection, board: boolean) => Promise<Located>;
  // specs is the highlight a selection gets, the canvas's focus style.
  specs: (sel: Selection) => HighlightSpec[];
  onPick: (pane: SheetPane, sel: Selection) => void;
  onPickEmpty: (pane: SheetPane) => void;
  onViewChange: (pane: SheetPane, v: SvgViewState) => void;
  onFloat: (pane: SheetPane) => void;
  onClose: (pane: SheetPane) => void;
  // onOpened hears each sheet the pane draws, so the dock tab can carry its name.
  onOpened: (pane: SheetPane) => void;
  // turns keeps each sheet's turn for the open design.
  turns?: TurnStore;
  // renderer picks the renderer for a sheet, the canvas's own by default (agni issue 1368). The view
  // asks again each time it moves to another sheet, until its reader picks one (agni issue 1536).
  renderer?: (sheetId: string) => SurfaceMode;
  // gl builds the WebGL drawing when the reader turns WebGL on. glDrawing when unset.
  gl?: GlFactory;
  // parkAfterMs is how long an SVG drawing sits idle before an image stands in for it. PARK_AFTER_MS
  // when unset.
  parkAfterMs?: number;
  // stops lists the sheets a sheet step visits for a selection, the union of its members' sheets for
  // an area (agni issue 1589). The view asks it on each selection, to read its place. When unset, the
  // view asks drawnOn at a step and reads no place for a selection.
  stops?: (sel: Selection) => Promise<string[]>;
  // history steps the page's selection trail back or forward (agni issue 1588). The view's Back and
  // Forward buttons are hidden when unset.
  history?: { back: () => void; forward: () => void };
}

// PARK_AFTER_MS is how long a view's SVG drawing sits with the pointer away before the view shows
// an image of it (agni issue 1482).
export const PARK_AFTER_MS = 1000;

export class SheetPane {
  readonly element: HTMLElement;
  readonly surface: SheetSurface;
  sheet = "";
  link: Link = "follow";
  private readonly host: HTMLElement;
  private glCanvas: HTMLCanvasElement;
  private readonly textHost: HTMLElement;
  private gl: CanvasComponent | null = null;
  private readonly webglBox: HTMLInputElement;
  private readonly picker: HTMLSelectElement;
  // The board's layer rows and the ones this view hides, apart from the canvas's (agni issue 1401).
  // layerStyle holds the rules that hide them, scoped to this view's drawing.
  private readonly layersButton: HTMLButtonElement;
  private readonly layerPop: HTMLElement;
  private readonly layerStyle: HTMLStyleElement;
  private readonly layerList: ReturnType<typeof boardLayerListIsland>;
  private layerRows: BoardLayerRow[] = [];
  private layerStack: BoardStackView | undefined;
  private hiddenLayers = new Set<string>();
  // layersChosen says the reader picked this view's layers, so a board drawn again keeps them rather
  // than going back to defaultHidden.
  // rendererChosen is set once the reader picks this view's renderer, which then holds across sheets.
  private rendererChosen = false;
  private layersChosen = false;
  private readonly flipButtons: Record<"h" | "v", HTMLButtonElement>;
  // followSide turns a board view to a selection's side and shows its copper layers (agni issue
  // 1587). followBox is its checkbox, shown on the board alone.
  private followSide = loadFollowSide();
  private readonly followBox: HTMLInputElement;
  private readonly followLabel: HTMLElement;
  private turn: ViewTurn = { orientation: UPRIGHT };
  private readonly linkSelect: HTMLSelectElement;
  private readonly also: HTMLElement;
  // absentNote says the board does not place the selection, on a view that keeps the board.
  private readonly absentNote: HTMLElement;
  private sel: Selection | null = null;
  // absent is set while the board view stays on the board without the selection, so it lights none.
  private absent = false;
  // underlay is the page's highlight under the selection's, such as a walk's shades (agni issue 1480).
  private underlay: HighlightSpec[] = [];
  // drawnOn is the last answer to where the selection is drawn, so a chip can offer the rest.
  private drawnOnLast: string[] = [];
  // drawing counts renders, so a late answer for a sheet the reader has left draws nothing.
  private drawing = 0;
  // The step controls (agni issues 1588 and 1589). historyButtons step the page's selection trail.
  // sheetButtons open the previous and next sheet that draws the selection, and place reads where
  // this view is among them. stopsFor holds the last answer to which sheets those are.
  private readonly historyButtons: Record<"back" | "forward", HTMLButtonElement>;
  private readonly sheetButtons: Record<"prev" | "next", HTMLButtonElement>;
  private readonly place: HTMLElement;
  private stopsFor: { sel: Selection | null; sheets: string[] } = { sel: null, sheets: [] };
  // setBusy shows or hides this view's loader, and busyEl is the loader itself.
  private readonly setBusy: (busy: Busy, label?: string) => void;
  private readonly busyEl: HTMLElement;
  // The stack view (agni issue 1593). stackButton enters and leaves it, and stackBar holds its explode
  // control and camera presets. stackFrom is the renderer and view the board had before, which leaving
  // restores.
  private readonly stackButton: HTMLButtonElement;
  private readonly stackBar: HTMLElement;
  private readonly explodeInput: HTMLInputElement;
  // bodiesBox shows and hides the parts' bodies (agni issue 1352), and bodiesNote says how many parts
  // stand at a default height because the file states none.
  private readonly bodiesBox: HTMLInputElement;
  private readonly bodiesNote: HTMLElement;
  // lookButtons switch the stack view between the layer look and the board look (agni issue 1653).
  private readonly lookButtons: Record<StackLook, HTMLButtonElement>;
  private exploded = false;
  private noGl = false;
  private stackFrom: { mode: SurfaceMode; view: unknown } | null = null;

  constructor(
    readonly id: string,
    private readonly deps: SheetPaneDeps,
    doc: Document = document,
  ) {
    const el = (tag: string, cls: string): HTMLElement => {
      const e = doc.createElement(tag);
      e.className = cls;
      return e;
    };
    this.element = el("div", "sheetview");
    this.element.dataset.sheetview = id;
    const bar = el("div", "sheetview-bar");
    this.picker = el("select", "sheetview-sheet") as HTMLSelectElement;
    this.picker.title = "sheet shown in this view";
    this.picker.addEventListener("change", () => void this.open(this.picker.value));
    this.linkSelect = el("select", "sheetview-link") as HTMLSelectElement;
    this.linkSelect.title = "follow: open and centre on what is selected elsewhere. locked: also mirror pan and zoom with other locked views on the same sheet. off: never move.";
    for (const l of LINKS) this.linkSelect.append(new Option(l, l));
    this.linkSelect.addEventListener("change", () => {
      this.link = this.linkSelect.value as Link;
    });
    // Each turn control names its action in data-action, which the cross-probe walk and the view's
    // own tests click by (web/browser/cross-probe-walk.spec.ts).
    const button = (cls: string, action: string, text: string, title: string, act: () => void): HTMLButtonElement => {
      const b = el("button", `mode-btn ${cls}`) as HTMLButtonElement;
      b.type = "button";
      b.dataset.action = action;
      b.textContent = text;
      b.title = title;
      b.addEventListener("click", act);
      return b;
    };
    const rotate = button("sheetview-rotate", "rotate", "⟳ 90°", "turn this view a quarter turn clockwise", () =>
      this.setTurn({ ...this.turn, orientation: rotateBy(this.turn.orientation) }),
    );
    // A flip button stays pressed while its flip is on. On the board one flip turns the board over,
    // so the view draws the back copper nearest, as the board looks from underneath.
    const flipH = button("sheetview-flip-h", "flip-h", "⇆", "flip this view left to right (on the board, one flip shows it from underneath)", () =>
      this.setTurn({ ...this.turn, orientation: flip(this.turn.orientation, "h") }),
    );
    const flipV = button("sheetview-flip-v", "flip-v", "⇅", "flip this view top to bottom (on the board, one flip shows it from underneath)", () =>
      this.setTurn({ ...this.turn, orientation: flip(this.turn.orientation, "v") }),
    );
    this.flipButtons = { h: flipH, v: flipV };
    const float = el("button", "mode-btn sheetview-float") as HTMLButtonElement;
    float.type = "button";
    float.textContent = "Float";
    float.title = "float this view over the page";
    float.addEventListener("click", () => this.deps.onFloat(this));
    // The Layers button opens this view's own layer rows, the list the Sheets panel shows for the
    // canvas, so a reader can show In2.Cu alone here and keep the whole board on the canvas.
    this.layersButton = button("sheetview-layers", "layers", "Layers", "show or hide this view's board layers", () =>
      this.showLayerPop(this.layerPop.hidden),
    );
    this.layersButton.setAttribute("aria-haspopup", "true");
    this.layersButton.setAttribute("aria-expanded", "false");
    this.layerPop = el("div", "sheetview-layer-pop");
    this.layerPop.hidden = true;
    this.layerList = boardLayerListIsland(this.layerPop, {
      onToggle: (name) => this.setHiddenLayers(toggled(this.hiddenLayers, name)),
      onSolo: (name) => this.setHiddenLayers(soloHidden(this.layerRows, name)),
      onPreset: (preset) => this.setHiddenLayers(presetHidden(this.layerRows, preset)),
    });
    this.layerList.island.activate();
    // A press anywhere outside the list and its button closes the list.
    doc.addEventListener("pointerdown", (e) => {
      const t = e.target as Node | null;
      if (!this.layerPop.hidden && t && !this.layerPop.contains(t) && !this.layersButton.contains(t)) this.showLayerPop(false);
    });
    this.layerStyle = doc.createElement("style");
    // The renderer this view draws in, chosen per view (agni issue 1368). It is one WebGL checkbox, off
    // for SVG, as the canvas's is (agni issue 1469).
    const renderer = el("label", "render-toggle sheetview-renderer");
    renderer.title = "draw this view with WebGL, or with SVG when off";
    this.webglBox = el("input", "sheetview-webgl") as HTMLInputElement;
    this.webglBox.type = "checkbox";
    this.webglBox.addEventListener("change", () => void this.setRenderer(this.webglBox.checked ? "webgl" : "svg"));
    renderer.append(this.webglBox, doc.createTextNode("WebGL"));
    // The follow checkbox turns the board to each selection's side and shows its copper layers. Off,
    // the view keeps the side and layers its reader chose. The choice holds across views and reloads.
    this.followLabel = el("label", "render-toggle sheetview-follow-side");
    this.followLabel.title = "turn the board to the side a selection is on, and show the copper layers it uses";
    this.followBox = el("input", "sheetview-follow-side-box") as HTMLInputElement;
    this.followBox.type = "checkbox";
    this.followBox.dataset.action = "follow-side";
    this.followBox.checked = this.followSide;
    this.followBox.addEventListener("change", () => {
      this.followSide = this.followBox.checked;
      saveFollowSide(this.followSide);
    });
    this.followLabel.append(this.followBox, doc.createTextNode("Follow side"));
    // The step controls name their actions as the canvas's do, and the page's keys step the view the
    // reader last pressed in.
    this.historyButtons = {
      back: button("sheetview-step-back", "selection-back", "←", "step back to the previous selection ([)", () => this.deps.history?.back()),
      forward: button("sheetview-step-forward", "selection-forward", "→", "step forward to the next selection (])", () => this.deps.history?.forward()),
    };
    if (!this.deps.history) for (const b of Object.values(this.historyButtons)) b.style.display = "none";
    this.setHistory(false, false);
    this.sheetButtons = {
      prev: button("sheetview-prev-sheet", "prev-sheet", "‹", "open the previous sheet that draws the selection, or the previous sheet with nothing selected (,)", () => void this.stepSheet(-1)),
      next: button("sheetview-next-sheet", "next-sheet", "›", "open the next sheet that draws the selection, or the next sheet with nothing selected (.)", () => void this.stepSheet(1)),
    };
    this.stackButton = button("sheetview-stack", "explode", "Stack", "show the board's layers apart, in stack order", () => void this.toggleStack());
    this.stackButton.setAttribute("aria-pressed", "false");
    this.stackBar = el("span", "sheetview-stack-bar");
    this.stackBar.hidden = true;
    this.explodeInput = el("input", "sheetview-explode") as HTMLInputElement;
    this.explodeInput.type = "range";
    this.explodeInput.min = "0";
    this.explodeInput.max = "100";
    this.explodeInput.value = "60";
    this.explodeInput.dataset.stackExplode = "";
    this.explodeInput.title = "how far apart the layers draw: 0 is the board's true thickness";
    this.explodeInput.addEventListener("input", () => this.gl?.setExplode(Number(this.explodeInput.value) / 100));
    this.stackBar.append(this.explodeInput);
    const bodiesLabel = el("label", "render-toggle sheetview-stack-bodies");
    bodiesLabel.title = "stand each part on its side of the board, as tall as its file states, or at a default height where the file gives none";
    this.bodiesBox = el("input", "sheetview-stack-bodies-box") as HTMLInputElement;
    this.bodiesBox.type = "checkbox";
    this.bodiesBox.checked = true;
    this.bodiesBox.dataset.action = "stack-bodies";
    this.bodiesBox.addEventListener("change", () => this.gl?.setStackBodies(this.bodiesBox.checked));
    this.bodiesNote = el("span", "sheetview-stack-bodies-note");
    this.bodiesNote.dataset.stackBodiesNote = "";
    bodiesLabel.append(this.bodiesBox, doc.createTextNode("Parts"), this.bodiesNote);
    this.stackBar.append(bodiesLabel);
    this.lookButtons = {
      layers: button("sheetview-stack-look", "stack-look-layers", "Layers", "draw each layer apart in the editor's colours", () => this.setLook("layers")),
      board: button("sheetview-stack-look", "stack-look-board", "Board", "draw the board as it is made: mask, pads, silkscreen and parts", () => this.setLook("board")),
    };
    for (const [look, b] of Object.entries(this.lookButtons)) {
      b.dataset.stackLook = look;
      this.stackBar.append(b);
    }
    for (const [preset, text] of [["top", "Top"], ["iso", "Iso"], ["front", "Front"], ["side", "Side"]] as [StackPreset, string][]) {
      const b = button("sheetview-stack-preset", `stack-${preset}`, text, `look at the stack from the ${preset === "iso" ? "corner" : preset}`, () => this.gl?.setStackPreset(preset));
      b.dataset.stackPreset = preset;
      this.stackBar.append(b);
    }
    this.place = el("span", "sheetview-place");
    this.place.dataset.sheetPlace = "";
    const close = el("button", "mode-btn sheetview-close") as HTMLButtonElement;
    close.type = "button";
    close.textContent = "Close";
    close.addEventListener("click", () => this.deps.onClose(this));
    bar.append(
      this.picker,
      this.sheetButtons.prev,
      this.place,
      this.sheetButtons.next,
      this.historyButtons.back,
      this.historyButtons.forward,
      renderer,
      this.layersButton,
      this.stackButton,
      this.stackBar,
      rotate,
      flipH,
      flipV,
      this.followLabel,
      this.linkSelect,
      float,
      close,
      this.layerPop,
    );
    this.also = el("div", "sheetview-also");
    this.absentNote = el("span", "sheetview-absent");
    // The stage holds the WebGL canvas and its text layer under the SVG host, as the page's canvas
    // panel does (#view, #text-overlay, #svg-view).
    const stage = el("div", "sheetview-stage");
    this.glCanvas = el("canvas", "sheetview-gl") as HTMLCanvasElement;
    this.textHost = el("div", "text-overlay sheetview-text");
    this.host = el("div", "sheetview-canvas");
    // A large board takes seconds to draw, so the view shows its own loader over its stage while it
    // does, after the page loader's show delay, so a quick draw does not flash it.
    const busy = el("div", "render-busy sheetview-busy");
    this.busyEl = busy;
    busy.setAttribute("role", "status");
    busy.setAttribute("aria-live", "polite");
    const busyLabel = el("span", "render-busy-label");
    busyLabel.textContent = "rendering…";
    busy.append(busyLabel);
    this.setBusy = delayedBusy(busy);
    stage.append(this.glCanvas, this.textHost, this.host, busy);
    this.element.append(this.layerStyle, bar, this.also, stage);
    this.surface = new SheetSurface({
      svgHost: this.host,
      gl: (create) => {
        if (!this.gl && create) this.gl = (this.deps.gl ?? glDrawing)(this.id, this.glCanvas, this.textHost);
        return this.gl;
      },
      // Disposing the drawing loses its context on purpose, to free the GPU's copy. A canvas keeps the
      // context it first made, lost or not, so the next WebGL drawing gets a fresh canvas. Without one,
      // its shaders fail to compile and a second press of Stack draws nothing.
      dropGl: () => {
        this.gl?.dispose();
        this.gl = null;
        const fresh = this.glCanvas.cloneNode(false) as HTMLCanvasElement;
        this.glCanvas.replaceWith(fresh);
        this.glCanvas = fresh;
      },
      layerScope: `[data-sheetview="${id}"] .sheetview-canvas`,
      layerStyle: this.layerStyle,
      markView: id,
      readout: null,
    });
    this.surface.onPick = (sel) => this.deps.onPick(this, sel);
    this.surface.onPickEmpty = () => this.deps.onPickEmpty(this);
    this.surface.onViewChange = () => this.deps.onViewChange(this, this.surface.svg.getView());
    this.surface.setMode(this.deps.renderer?.("") ?? "svg");
    this.surface.parkWhenIdle(this.deps.parkAfterMs ?? PARK_AFTER_MS);
    this.webglBox.checked = this.surface.mode === "webgl";
    this.element.dataset.exploded = "false";
    this.stackButton.style.display = "none";
  }

  // toggleStack shows the board's stack apart, or puts it back (agni issue 1593). The stack draws in
  // WebGL, so a board drawn in SVG switches to WebGL first and back to SVG on the way out, with the
  // SVG view's pan and zoom.
  async toggleStack(): Promise<void> {
    if (this.exploded) {
      await this.leaveStack();
      return;
    }
    const stack = this.layerStack;
    if (!stack || this.sheet !== BOARD_SHEET) return;
    const mode = this.surface.mode;
    this.stackFrom = { mode, view: this.surface.getView(mode) };
    if (mode !== "webgl") await this.switchRenderer("webgl");
    const gl = this.gl;
    if (!gl || this.surface.drawnBy !== "webgl" || !gl.enterStack(stack, this.layerRows)) {
      this.noGl = true;
      this.showStackButton();
      await this.restoreFrom();
      return;
    }
    gl.setExplode(Number(this.explodeInput.value) / 100);
    gl.setStackBodies(this.bodiesBox.checked);
    this.showLook("layers");
    this.bodiesNote.textContent = defaultedNote(stack);
    this.setExploded(true);
  }

  // setLook switches the stack view's look.
  private setLook(look: StackLook): void {
    this.gl?.setStackLook(look);
    this.showLook(look);
  }

  private showLook(look: StackLook): void {
    for (const [l, b] of Object.entries(this.lookButtons)) {
      b.setAttribute("aria-pressed", String(l === look));
      b.classList.toggle("active", l === look);
    }
  }

  // showStackButton says on the Stack button why it cannot be pressed, when it cannot.
  private showStackButton(): void {
    const why = !this.layerStack?.rows.length
      ? "the board's file states no layer stack"
      : this.noGl
        ? "the stack view needs WebGL2, which this browser does not offer"
        : "";
    this.stackButton.disabled = why !== "";
    this.stackButton.title = why || "show the board's layers apart, in stack order";
  }

  private async leaveStack(): Promise<void> {
    this.gl?.leaveStack();
    this.setExploded(false);
    await this.restoreFrom();
  }

  // restoreFrom puts back the renderer and view the board had before the stack view.
  private async restoreFrom(): Promise<void> {
    const from = this.stackFrom;
    this.stackFrom = null;
    if (!from || from.mode === this.surface.mode) return;
    await this.switchRenderer(from.mode);
    if (from.view) this.surface.setView(from.mode, from.view);
  }

  // switchRenderer draws the sheet again in another renderer without counting as the reader's choice.
  private async switchRenderer(mode: SurfaceMode): Promise<void> {
    this.surface.setMode(mode);
    this.webglBox.checked = mode === "webgl";
    if (this.sheet) await this.open(this.sheet);
  }

  private setExploded(on: boolean): void {
    this.exploded = on;
    this.element.dataset.exploded = String(on);
    this.stackBar.hidden = !on;
    this.stackButton.setAttribute("aria-pressed", String(on));
    this.stackButton.classList.toggle("active", on);
  }

  // renderer is the renderer this view draws in, and drawnBy the one that drew the sheet on screen,
  // which is SVG where WebGL2 is missing.
  renderer(): SurfaceMode {
    return this.surface.mode;
  }

  get drawnBy(): SurfaceMode {
    return this.surface.drawnBy;
  }

  // view is this view's SVG drawing.
  get view(): SvgView {
    return this.surface.svg;
  }

  // setRenderer switches this view to SVG or WebGL and draws its sheet again in it.
  async setRenderer(mode: SurfaceMode): Promise<void> {
    this.rendererChosen = true;
    this.surface.setMode(mode);
    this.webglBox.checked = mode === "webgl";
    if (this.sheet) await this.open(this.sheet);
  }

  // getCentre and setCentre are the point at the middle of this view and its zoom, which locked views
  // on one sheet exchange.
  getCentre(): { at: { x: number; y: number }; scale: number } | null {
    return this.surface.getCentre();
  }

  setCentre(at: { x: number; y: number }, scale: number): void {
    this.surface.setCentre(at, scale);
  }

  // title is the dock tab's label, the sheet's name.
  title(): string {
    return this.nameOf(this.sheet) || "Sheet";
  }

  // open draws a sheet and lights the current selection on it. The view's loader shows while it does.
  // A reader can pick another sheet before the first is drawn, so only the latest draw hides it.
  async open(sheetId: string): Promise<void> {
    const n = ++this.drawing;
    this.setBusy(true);
    try {
      await this.draw(sheetId, n);
    } finally {
      if (n === this.drawing) this.setBusy(false);
    }
  }

  private async draw(sheetId: string, n: number): Promise<void> {
    // A view moved to another sheet takes that sheet's renderer, so a board view opens in WebGL and a
    // schematic in the canvas's renderer, unless the reader picked one for this view.
    if (sheetId !== this.sheet && !this.rendererChosen && this.deps.renderer) {
      this.surface.setMode(this.deps.renderer(sheetId));
      this.webglBox.checked = this.surface.mode === "webgl";
    }
    this.sheet = sheetId;
    this.fillPicker();
    let drawing: SheetDrawing = { svg: "", boardLayers: [] };
    const mode = this.surface.mode;
    try {
      drawing = await this.deps.render(sheetId, mode);
    } catch (e) {
      this.also.textContent = `could not draw this sheet: ${String(e)}`;
    }
    if (n !== this.drawing) return;
    // Drawing a large sheet blocks the page, and the loader's show delay cannot run out while it
    // does. When the engine answered inside the delay, the loader is not up yet, so it goes up now
    // and paints before the drawing starts.
    if (heavyDrawing(drawing, mode)) {
      this.busyEl.classList.add("on");
      await nextPaint();
      if (n !== this.drawing) return;
    }
    this.layerRows = sheetId === BOARD_SHEET ? drawing.boardLayers : [];
    this.layerStack = sheetId === BOARD_SHEET ? drawing.boardStack : undefined;
    // Drawing a sheet again ends the stack view, since the canvas draws the new sheet flat.
    if (this.exploded) this.setExploded(false);
    if (!this.layersChosen) this.hiddenLayers = defaultHidden(this.layerRows);
    this.turn = this.deps.turns?.load(sheetId) ?? { orientation: UPRIGHT };
    // The turn goes to the surface first, so a WebGL sheet draws turned from its first frame.
    this.surface.setTurn(this.turn.orientation, this.seenFromBelow(), this.layerRows);
    // Where WebGL2 is missing, a WebGL view draws its sheet as SVG instead.
    const drawnInGl = mode === "webgl" && drawing.packed !== undefined && this.surface.showPacked(drawing.packed);
    if (!drawnInGl) {
      if (mode === "webgl") {
        this.surface.setMode("svg");
        this.webglBox.checked = false;
        drawing = await this.deps.render(sheetId, "svg");
        if (n !== this.drawing) return;
      }
      this.surface.showSvg(drawing.svg);
    }
    this.element.dataset.renderer = this.surface.drawnBy;
    this.deps.onOpened(this);
    this.applyLayers();
    this.applyTurn();
    this.layersButton.style.display = this.layerRows.length > 0 ? "" : "none";
    this.followLabel.style.display = this.layerRows.length > 0 ? "" : "none";
    this.stackButton.style.display = this.layerRows.length > 0 ? "" : "none";
    this.showStackButton();
    if (this.layerRows.length === 0) this.showLayerPop(false);
    this.showPlace();
    await this.paint(false);
  }

  // follow takes a selection made elsewhere on the page. A following view opens a sheet that draws
  // it when its own does not, and centres on it. A view that is off lights it where it is drawn.
  async follow(sel: Selection | null): Promise<void> {
    this.sel = sel;
    void this.refreshStops();
    this.setAbsent(null);
    if (!sel || this.link === "off") {
      this.drawnOnLast = [];
      this.offer([]);
      await this.paint(false);
      return;
    }
    let located: Located = { sheets: [] };
    try {
      located = await this.deps.drawnOn(sel, this.sheet === BOARD_SHEET);
    } catch {
      located = { sheets: [] };
    }
    if (this.sel !== sel) return;
    const drawnOn = located.sheets;
    const choice = chooseSheet(this.sheet, drawnOn);
    this.drawnOnLast = drawnOn;
    if (choice.open) await this.open(choice.open);
    if (this.sel !== sel) return;
    this.offer(choice.others, choice.absent ? "drawn on " : "also on ");
    if (choice.absent) {
      this.setAbsent(sel);
      await this.paint(false);
      this.markFollowed(sel, true);
      return;
    }
    if (located.follow) this.turnToSelection(located.follow);
    await this.paint(true);
    this.markFollowed(sel, false);
  }

  // markFollowed records on the view's element what it last followed and whether its sheet lacks it,
  // as data-followed and data-absent, once it has drawn the answer. The cross-probe walk waits on
  // them (agni issue 1590).
  private markFollowed(sel: Selection, absent: boolean): void {
    if (this.sel !== sel) return;
    this.element.dataset.followed = labelFor(sel);
    this.element.dataset.absent = String(absent);
  }

  // turnToSelection turns a board view to the side the engine says its selection is on and shows the
  // layer rows it names (agni issues 1587 and 1635). It only turns layers on, so a layer the reader
  // showed stays, and a layer the reader hid comes back only when a selection uses it.
  private turnToSelection(follow: BoardFollow): void {
    if (!this.followSide || this.sheet !== BOARD_SHEET) return;
    const side = follow.side;
    const show = new Set(follow.layers);
    if ((side === "front" || side === "back") && (side === "back") !== this.seenFromBelow()) this.setTurn({ ...this.turn, orientation: flip(this.turn.orientation, "h") });
    if ([...show].some((name) => this.hiddenLayers.has(name))) {
      this.setHiddenLayers(new Set([...this.hiddenLayers].filter((name) => !show.has(name))));
    }
  }

  // setUnderlay lays the page's highlight under the selection's in this view, and an empty list
  // takes it away. With reveal, a view that follows frames it, and a view whose link is off lights it
  // where it is (agni issue 1551).
  async setUnderlay(specs: HighlightSpec[], reveal = false): Promise<void> {
    this.underlay = specs;
    await this.paint(false);
    if (reveal && specs.length > 0 && this.link !== "off" && this.underlay === specs) this.surface.reveal(undefined, specs);
  }

  // setAbsent states that the board does not place sel, or clears the note when sel is null.
  private setAbsent(sel: Selection | null): void {
    this.absent = sel !== null;
    this.absentNote.textContent = sel ? `${labelFor(sel)} is not on the board` : "";
    if (sel) this.also.prepend(this.absentNote);
    else this.absentNote.remove();
  }

  // lightPick takes a selection picked in this view, which it lights and does not move to.
  async lightPick(sel: Selection): Promise<void> {
    this.sel = sel;
    void this.refreshStops();
    this.setAbsent(null);
    this.drawnOnLast = [];
    this.offer([]);
    await this.paint(false);
  }

  private async paint(reveal: boolean): Promise<void> {
    const sel = this.sel;
    const underlay = this.underlay;
    const own = sel && !this.absent ? this.deps.specs(sel) : [];
    const specs = [...underlay, ...own];
    this.surface.setHighlights(specs);
    if (!this.sheet || specs.length === 0) {
      this.surface.setSvgOverlay("");
      return;
    }
    // WebGL lights the specs against the keys it holds, and only an SVG drawing needs the overlay. An
    // underlay with a selection over it comes as two documents, so a reveal frames the selection.
    const split = underlay.length > 0 && own.length > 0;
    if (this.surface.drawnBy === "svg") {
      const fetch = (s: HighlightSpec[]) => this.deps.overlay(this.sheet, s).catch(() => "");
      const [under, over] = split ? await Promise.all([fetch(underlay), fetch(own)]) : ["", await fetch(specs)];
      if (this.sel !== sel || this.underlay !== underlay) return;
      if (split) this.surface.setSvgOverlays(under, over);
      else this.surface.setSvgOverlay(over);
    }
    if (reveal && sel && !this.absent) this.surface.reveal(pinOf(sel), split ? own : undefined);
  }

  // setHistory enables the view's Back and Forward buttons where the page's selection trail goes
  // further that way.
  setHistory(canBack: boolean, canForward: boolean): void {
    this.historyButtons.back.disabled = !canBack;
    this.historyButtons.forward.disabled = !canForward;
  }

  // stepSheet opens the next sheet that draws the selection, or the previous one for a step of -1,
  // and centres on the selection there (agni issue 1589). With nothing selected it opens the next or
  // previous sheet. At either end it does nothing. A board view takes no sheet steps.
  async stepSheet(dir: 1 | -1): Promise<void> {
    if (this.sheet === BOARD_SHEET) return;
    const sel = this.sel;
    const stops = await this.stopsOf(sel);
    if (this.sel !== sel) return;
    const step = sheetStep(this.order(), stops, this.sheet);
    const to = dir > 0 ? step.next : step.prev;
    if (!to) return;
    await this.open(to);
    if (this.sel !== sel) return;
    this.showPlace();
    this.setAbsent(null);
    this.offer(this.drawnOnLast.filter((o) => o !== to));
    if (sel) await this.paint(true);
    else this.surface.reveal();
  }

  // stepPlace is where this view stands among the sheets a step visits, from the last answer.
  stepPlace(): ReturnType<typeof sheetStep> {
    return sheetStep(this.order(), this.stopsFor.sel === this.sel ? this.stopsFor.sheets : [], this.sheet);
  }

  private order(): string[] {
    return this.deps.sheets().map((s) => s.id);
  }

  private async stopsOf(sel: Selection | null): Promise<string[]> {
    if (!sel) return [];
    if (this.stopsFor.sel === sel) return this.stopsFor.sheets;
    let sheets: string[] = [];
    try {
      sheets = this.deps.stops ? await this.deps.stops(sel) : (await this.deps.drawnOn(sel, false)).sheets;
    } catch {
      sheets = [];
    }
    if (this.sel === sel) this.stopsFor = { sel, sheets };
    return sheets;
  }

  private async refreshStops(): Promise<void> {
    this.showPlace();
    if (!this.deps.stops) return;
    await this.stopsOf(this.sel);
    this.showPlace();
  }

  // showPlace enables the sheet steps that go somewhere and reads this view's place, such as "2 of 5".
  // A board view hides them, since the board is no stop.
  private showPlace(): void {
    const onBoard = this.sheet === BOARD_SHEET || !this.sheet;
    for (const e of [this.sheetButtons.prev, this.place, this.sheetButtons.next]) e.style.display = onBoard ? "none" : "";
    const step = this.stepPlace();
    // Before the view knows a selection's sheets, it reads no place and leaves both steps on, since a
    // step asks.
    const known = !this.sel || this.stopsFor.sel === this.sel;
    this.sheetButtons.prev.disabled = known && !step.prev;
    this.sheetButtons.next.disabled = known && !step.next;
    this.place.textContent = known ? stepLabel(step) : "";
  }

  // offer names the other sheets that draw the selection, each a button that opens it.
  private offer(others: string[], lead = "also on "): void {
    this.also.replaceChildren();
    if (others.length === 0) return;
    const doc = this.element.ownerDocument;
    this.also.append(doc.createTextNode(lead));
    for (const id of others) {
      const b = doc.createElement("button");
      b.type = "button";
      b.className = "sheetview-chip";
      b.textContent = this.nameOf(id);
      b.addEventListener("click", async () => {
        await this.open(id);
        this.offer(this.drawnOnLast.filter((o) => o !== id));
        this.surface.reveal();
      });
      this.also.append(b);
    }
  }

  private fillPicker(): void {
    const sheets = this.deps.sheets();
    this.picker.replaceChildren(...sheets.map((s) => new Option(s.name || s.id, s.id, false, s.id === this.sheet)));
    this.picker.value = this.sheet;
  }

  // turnOf is how the view is turned now.
  turnOf(): ViewTurn {
    return this.turn;
  }

  // setTurn turns the view, keeps the turn for this sheet, and keeps the point at the view's centre
  // where it is.
  setTurn(turn: ViewTurn): void {
    this.turn = turn;
    if (this.sheet) this.deps.turns?.save(this.sheet, turn);
    this.applyTurn();
  }

  private applyTurn(): void {
    this.surface.setTurn(this.turn.orientation, this.seenFromBelow(), this.layerRows);
    for (const axis of ["h", "v"] as const) {
      const on = flipped(this.turn.orientation, axis);
      this.flipButtons[axis].classList.toggle("active", on);
      this.flipButtons[axis].setAttribute("aria-pressed", String(on));
    }
    // The view states how it is turned, for a walk or a test to read without decoding a transform.
    // data-side is the side of the board nearest the reader, "back" in the bottom view.
    this.element.dataset.rotation = String(this.turn.orientation.rot);
    this.element.dataset.side = this.seenFromBelow() ? "back" : "front";
  }

  // seenFromBelow says whether this view shows the board from underneath. A schematic sheet has no
  // underside, so its flips are mirrors alone.
  private seenFromBelow(): boolean {
    return this.sheet === BOARD_SHEET && fromBelow(this.turn.orientation);
  }

  // hiddenLayersOf names the board layers this view hides.
  hiddenLayersOf(): ReadonlySet<string> {
    return this.hiddenLayers;
  }

  // dispose unmounts the layer list and frees the WebGL drawing, for a view the dock has closed.
  dispose(): void {
    this.surface.svg.dispose();
    this.layerList.island.deactivate();
    this.gl?.dispose();
    this.gl = null;
  }

  private setHiddenLayers(hidden: Set<string>): void {
    this.hiddenLayers = hidden;
    this.layersChosen = true;
    this.applyLayers();
  }

  private showLayerPop(open: boolean): void {
    this.layerPop.hidden = !open;
    this.layersButton.classList.toggle("active", open);
    this.layersButton.setAttribute("aria-expanded", String(open));
    // The list scrolls inside the view, so a short view still shows the stack's last line (agni
    // 1603). The stylesheet's 60vh holds where the view is taller.
    if (open) {
      const room = this.element.getBoundingClientRect().bottom - this.layerPop.getBoundingClientRect().top - 6;
      this.layerPop.style.maxHeight = room > 0 ? `min(60vh, ${Math.max(room, 120)}px)` : "";
    }
  }

  // applyLayers hides this view's hidden layers with rules scoped to its own drawing, so the canvas
  // and every other view keep theirs.
  private applyLayers(): void {
    this.surface.setBoardLayers(this.layerRows, this.hiddenLayers);
    this.layerList.setState({ rows: this.layerRows, hidden: this.hiddenLayers, stack: this.layerStack });
  }

  private nameOf(id: string): string {
    return this.deps.sheets().find((s) => s.id === id)?.name || id;
  }
}
