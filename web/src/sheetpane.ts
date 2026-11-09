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
import type { PackedSheet } from "./gen/agni/v1/geom/geom_packed_pb.js";
import { pinOf, type Selection } from "./selection.js";
import type { HighlightSpec } from "./highlights.js";
import { chooseSheet, LINKS, type Link } from "./sheetfollow.js";
import { UPRIGHT, flip, rotateBy, type Orientation } from "./orient.js";
import type { BoardLayerRow } from "./gen/agni/v1/geom/geom_packed_pb.js";
import { presetHidden, soloHidden, toggled } from "./boardlayers.js";
import { boardLayerListIsland } from "./boardlayerlist.js";

// ViewTurn is how a view is turned for a sheet, kept per design and sheet so a view opened on the
// board again comes back the way the bench has it (agni issue 1327). bottom draws the board from
// underneath, the back copper nearest.
export interface ViewTurn {
  orientation: Orientation;
  bottom: boolean;
}

// turnKey names a sheet's kept turn in storage, by the design's mount and path and the sheet.
export function turnKey(mount: string, path: string, sheetId: string): string {
  return `agni-sheetview-turn:${mount}/${path}:${sheetId}`;
}

// parseTurn reads a kept turn back, and returns null for anything that is not one, such as a value
// an older page wrote under the same key.
export function parseTurn(text: string | null): ViewTurn | null {
  if (!text) return null;
  try {
    const v = JSON.parse(text) as { orientation?: Partial<Orientation>; bottom?: unknown };
    const o = v.orientation;
    if (!o || ![0, 90, 180, 270].includes(o.rot as number) || typeof o.flipH !== "boolean" || typeof o.flipV !== "boolean") return null;
    return { orientation: { rot: o.rot as Orientation["rot"], flipH: o.flipH, flipV: o.flipV }, bottom: v.bottom === true };
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

// SheetDrawing is one sheet as a pane draws it, the SVG document or, in WebGL, the packed sheet.
// boardLayers is empty for a schematic sheet.
export interface SheetDrawing {
  svg: string;
  packed?: PackedSheet;
  boardLayers: BoardLayerRow[];
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

// SheetPaneDeps are what a pane asks of the page.
export interface SheetPaneDeps {
  // sheets lists the open design's sheets, for the picker and for naming the other sheets.
  sheets: () => { id: string; name: string }[];
  // render returns a sheet as the canvas draws it in mode, an SVG document with pick targets or a
  // packed sheet, and the board's layer rows when the sheet is the board.
  render: (sheetId: string, mode: SurfaceMode) => Promise<SheetDrawing>;
  // overlay returns the highlight overlay for specs on a sheet, framed like render's document.
  overlay: (sheetId: string, specs: HighlightSpec[]) => Promise<string>;
  // drawnOn lists the sheets that draw a selection, the board included when board is set.
  drawnOn: (sel: Selection, board: boolean) => Promise<string[]>;
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
  // renderer is the renderer a new view starts in, the canvas's own (agni issue 1368).
  renderer?: () => SurfaceMode;
  // gl builds the WebGL drawing when the reader turns WebGL on. glDrawing when unset.
  gl?: GlFactory;
}

export class SheetPane {
  readonly element: HTMLElement;
  readonly surface: SheetSurface;
  sheet = "";
  link: Link = "follow";
  private readonly host: HTMLElement;
  private readonly glCanvas: HTMLCanvasElement;
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
  private hiddenLayers = new Set<string>();
  private readonly bottomButton: HTMLButtonElement;
  private turn: ViewTurn = { orientation: UPRIGHT, bottom: false };
  private readonly linkSelect: HTMLSelectElement;
  private readonly also: HTMLElement;
  private sel: Selection | null = null;
  // drawnOn is the last answer to where the selection is drawn, so a chip can offer the rest.
  private drawnOnLast: string[] = [];
  // drawing counts renders, so a late answer for a sheet the reader has left draws nothing.
  private drawing = 0;

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
    const flipH = button("sheetview-flip-h", "flip-h", "⇆", "flip this view left to right", () =>
      this.setTurn({ ...this.turn, orientation: flip(this.turn.orientation, "h") }),
    );
    const flipV = button("sheetview-flip-v", "flip-v", "⇅", "flip this view top to bottom", () =>
      this.setTurn({ ...this.turn, orientation: flip(this.turn.orientation, "v") }),
    );
    // The bottom view turns the board over left to right and draws its back copper nearest, as the
    // board looks from underneath.
    this.bottomButton = button("sheetview-bottom", "bottom", "Bottom", "look at the board from underneath", () =>
      this.setTurn({ orientation: flip(this.turn.orientation, "h"), bottom: !this.turn.bottom }),
    );
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
    const close = el("button", "mode-btn sheetview-close") as HTMLButtonElement;
    close.type = "button";
    close.textContent = "Close";
    close.addEventListener("click", () => this.deps.onClose(this));
    bar.append(this.picker, renderer, this.layersButton, rotate, flipH, flipV, this.bottomButton, this.linkSelect, float, close, this.layerPop);
    this.also = el("div", "sheetview-also");
    // The stage holds the WebGL canvas and its text layer under the SVG host, as the page's canvas
    // panel does (#view, #text-overlay, #svg-view).
    const stage = el("div", "sheetview-stage");
    this.glCanvas = el("canvas", "sheetview-gl") as HTMLCanvasElement;
    this.textHost = el("div", "text-overlay sheetview-text");
    this.host = el("div", "sheetview-canvas");
    stage.append(this.glCanvas, this.textHost, this.host);
    this.element.append(this.layerStyle, bar, this.also, stage);
    this.surface = new SheetSurface({
      svgHost: this.host,
      gl: (create) => {
        if (!this.gl && create) this.gl = (this.deps.gl ?? glDrawing)(this.id, this.glCanvas, this.textHost);
        return this.gl;
      },
      dropGl: () => {
        this.gl?.dispose();
        this.gl = null;
      },
      layerScope: `[data-sheetview="${id}"] .sheetview-canvas`,
      layerStyle: this.layerStyle,
      markView: id,
      readout: null,
    });
    this.surface.onPick = (sel) => this.deps.onPick(this, sel);
    this.surface.onPickEmpty = () => this.deps.onPickEmpty(this);
    this.surface.onViewChange = () => this.deps.onViewChange(this, this.surface.svg.getView());
    this.surface.setMode(this.deps.renderer?.() ?? "svg");
    this.webglBox.checked = this.surface.mode === "webgl";
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

  // open draws a sheet and lights the current selection on it.
  async open(sheetId: string): Promise<void> {
    this.sheet = sheetId;
    this.fillPicker();
    const n = ++this.drawing;
    let drawing: SheetDrawing = { svg: "", boardLayers: [] };
    const mode = this.surface.mode;
    try {
      drawing = await this.deps.render(sheetId, mode);
    } catch (e) {
      this.also.textContent = `could not draw this sheet: ${String(e)}`;
    }
    if (n !== this.drawing) return;
    this.layerRows = sheetId === BOARD_SHEET ? drawing.boardLayers : [];
    this.turn = this.deps.turns?.load(sheetId) ?? { orientation: UPRIGHT, bottom: false };
    // The turn goes to the surface first, so a WebGL sheet draws turned from its first frame.
    this.surface.setTurn(this.turn.orientation, sheetId === BOARD_SHEET && this.turn.bottom, this.layerRows);
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
    if (this.layerRows.length === 0) this.showLayerPop(false);
    this.bottomButton.style.display = sheetId === BOARD_SHEET ? "" : "none";
    await this.paint(false);
  }

  // follow takes a selection made elsewhere on the page. A following view opens a sheet that draws
  // it when its own does not, and centres on it. A view that is off lights it where it is drawn.
  async follow(sel: Selection | null): Promise<void> {
    this.sel = sel;
    if (!sel || this.link === "off") {
      this.drawnOnLast = [];
      this.offer([]);
      await this.paint(false);
      return;
    }
    let drawnOn: string[] = [];
    try {
      drawnOn = await this.deps.drawnOn(sel, this.sheet === BOARD_SHEET);
    } catch {
      drawnOn = [];
    }
    if (this.sel !== sel) return;
    const choice = chooseSheet(this.sheet, drawnOn);
    this.drawnOnLast = drawnOn;
    if (choice.open) await this.open(choice.open);
    if (this.sel !== sel) return;
    this.offer(choice.others);
    await this.paint(true);
  }

  // lightPick takes a selection picked in this view, which it lights and does not move to.
  async lightPick(sel: Selection): Promise<void> {
    this.sel = sel;
    this.drawnOnLast = [];
    this.offer([]);
    await this.paint(false);
  }

  private async paint(reveal: boolean): Promise<void> {
    const sel = this.sel;
    const specs = sel ? this.deps.specs(sel) : [];
    this.surface.setHighlights(specs);
    if (!this.sheet || specs.length === 0) {
      this.surface.setSvgOverlay("");
      return;
    }
    // WebGL lights the specs against the keys it holds, and only an SVG drawing needs the overlay.
    if (this.surface.drawnBy === "svg") {
      let markup = "";
      try {
        markup = await this.deps.overlay(this.sheet, specs);
      } catch {
        markup = "";
      }
      if (this.sel !== sel) return;
      this.surface.setSvgOverlay(markup);
    }
    if (reveal && sel) this.surface.reveal(pinOf(sel));
  }

  // offer names the other sheets that draw the selection, each a button that opens it.
  private offer(others: string[]): void {
    this.also.replaceChildren();
    if (others.length === 0) return;
    const doc = this.element.ownerDocument;
    this.also.append(doc.createTextNode("also on "));
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
    this.surface.setTurn(this.turn.orientation, this.sheet === BOARD_SHEET && this.turn.bottom, this.layerRows);
    this.bottomButton.classList.toggle("active", this.turn.bottom);
    // The view states how it is turned, for a walk or a test to read without decoding a transform.
    // data-side is the side of the board nearest the reader, "back" in the bottom view.
    this.element.dataset.rotation = String(this.turn.orientation.rot);
    this.element.dataset.side = this.turn.bottom ? "back" : "front";
  }

  // hiddenLayersOf names the board layers this view hides.
  hiddenLayersOf(): ReadonlySet<string> {
    return this.hiddenLayers;
  }

  // dispose unmounts the layer list and frees the WebGL drawing, for a view the dock has closed.
  dispose(): void {
    this.layerList.island.deactivate();
    this.gl?.dispose();
    this.gl = null;
  }

  private setHiddenLayers(hidden: Set<string>): void {
    this.hiddenLayers = hidden;
    this.applyLayers();
  }

  private showLayerPop(open: boolean): void {
    this.layerPop.hidden = !open;
    this.layersButton.classList.toggle("active", open);
    this.layersButton.setAttribute("aria-expanded", String(open));
  }

  // applyLayers hides this view's hidden layers with rules scoped to its own drawing, so the canvas
  // and every other view keep theirs.
  private applyLayers(): void {
    this.surface.setBoardLayers(this.layerRows, this.hiddenLayers);
    this.layerList.setState({ rows: this.layerRows, hidden: this.hiddenLayers });
  }

  private nameOf(id: string): string {
    return this.deps.sheets().find((s) => s.id === id)?.name || id;
  }
}
