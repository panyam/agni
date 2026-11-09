// A sheet view is a dock panel that shows one sheet of the open design beside the canvas (agni issue
// 1326). It has its own sheet, camera, board layers and highlight, and shares the page's selection.
// A reader opens one from the Sheets panel, so a board sits beside its schematic and a click on one
// finds the part on the other.
//
// The pane draws through SvgView, the canvas's SVG renderer, so a pick here reads the same keys a
// pick on the canvas does. It asks the engine through the three ports in SheetPaneDeps, and holds
// no client of its own, so a test drives it with fakes.
import { SvgView, type SvgViewState } from "./svgview.js";
import { pinOf, type Selection } from "./selection.js";
import type { HighlightSpec } from "./highlights.js";
import { chooseSheet, LINKS, type Link } from "./sheetfollow.js";
import { UPRIGHT, flip, rotateBy, type Orientation } from "./orient.js";
import { orderStrata } from "./boardstrata.js";
import type { BoardLayerRow } from "./gen/agni/v1/geom/geom_packed_pb.js";
import { layerHiding, presetHidden, soloHidden, toggled } from "./boardlayers.js";
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

// SheetDrawing is one sheet as a pane draws it. boardLayers is empty for a schematic sheet.
export interface SheetDrawing {
  svg: string;
  boardLayers: BoardLayerRow[];
}

// SheetPaneDeps are what a pane asks of the page.
export interface SheetPaneDeps {
  // sheets lists the open design's sheets, for the picker and for naming the other sheets.
  sheets: () => { id: string; name: string }[];
  // render returns a sheet's SVG document with pick targets, as the canvas draws it, and the
  // board's layer rows when the sheet is the board.
  render: (sheetId: string) => Promise<SheetDrawing>;
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
}

export class SheetPane {
  readonly element: HTMLElement;
  readonly view: SvgView;
  sheet = "";
  link: Link = "follow";
  private readonly host: HTMLElement;
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
    const close = el("button", "mode-btn sheetview-close") as HTMLButtonElement;
    close.type = "button";
    close.textContent = "Close";
    close.addEventListener("click", () => this.deps.onClose(this));
    bar.append(this.picker, this.layersButton, rotate, flipH, flipV, this.bottomButton, this.linkSelect, float, close, this.layerPop);
    this.also = el("div", "sheetview-also");
    this.host = el("div", "sheetview-canvas");
    this.element.append(this.layerStyle, bar, this.also, this.host);
    this.view = new SvgView(this.host);
    this.view.onPick = (sel) => this.deps.onPick(this, sel);
    this.view.onPickEmpty = () => this.deps.onPickEmpty(this);
    this.view.onViewChange = (v) => this.deps.onViewChange(this, v);
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
    try {
      drawing = await this.deps.render(sheetId);
    } catch (e) {
      this.also.textContent = `could not draw this sheet: ${String(e)}`;
    }
    if (n !== this.drawing) return;
    const svg = drawing.svg;
    this.layerRows = sheetId === BOARD_SHEET ? drawing.boardLayers : [];
    this.turn = this.deps.turns?.load(sheetId) ?? { orientation: UPRIGHT, bottom: false };
    this.view.setSvg(svg);
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
    if (!this.sheet || specs.length === 0) {
      this.view.setOverlay("");
      return;
    }
    let markup = "";
    try {
      markup = await this.deps.overlay(this.sheet, specs);
    } catch {
      markup = "";
    }
    if (this.sel !== sel) return;
    this.view.setOverlay(markup);
    if (reveal && sel) this.view.revealOverlay(pinOf(sel));
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
        this.view.revealOverlay();
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
    const svg = this.host.querySelector("svg");
    if (svg) orderStrata(svg, this.sheet === BOARD_SHEET && this.turn.bottom);
    this.bottomButton.classList.toggle("active", this.turn.bottom);
    // The view states how it is turned, for a walk or a test to read without decoding a transform.
    // data-side is the side of the board nearest the reader, "back" in the bottom view.
    this.element.dataset.rotation = String(this.turn.orientation.rot);
    this.element.dataset.side = this.turn.bottom ? "back" : "front";
    this.view.setOrientation(this.turn.orientation);
  }

  // hiddenLayersOf names the board layers this view hides.
  hiddenLayersOf(): ReadonlySet<string> {
    return this.hiddenLayers;
  }

  // dispose unmounts the layer list, for a view the dock has closed.
  dispose(): void {
    this.layerList.island.deactivate();
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
    const scope = `[data-sheetview="${this.id}"] .sheetview-canvas`;
    this.layerStyle.textContent = layerHiding(this.layerRows, this.hiddenLayers, scope).css;
    this.layerList.setState({ rows: this.layerRows, hidden: this.hiddenLayers });
  }

  private nameOf(id: string): string {
    return this.deps.sheets().find((s) => s.id === id)?.name || id;
  }
}
