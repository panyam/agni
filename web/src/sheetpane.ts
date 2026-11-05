// A sheet view is a dock panel that shows one sheet of the open design beside the canvas (agni issue
// 1326). It has its own sheet, camera, board layers and highlight, and shares the page's selection.
// A reader opens one from the Sheets panel, so a board sits beside its schematic and a click on one
// finds the part on the other.
//
// The pane draws through SvgView, the canvas's SVG renderer, so a pick here reads the same keys a
// pick on the canvas does. It asks the engine through the three ports in SheetPaneDeps, and holds
// no client of its own, so a test drives it with fakes.
import { SvgView, type SvgViewState } from "./svgview.js";
import type { Selection } from "./selection.js";
import type { HighlightSpec } from "./highlights.js";
import { chooseSheet, LINKS, type Link } from "./sheetfollow.js";

export const BOARD_SHEET = "board";

// SheetPaneDeps are what a pane asks of the page.
export interface SheetPaneDeps {
  // sheets lists the open design's sheets, for the picker and for naming the other sheets.
  sheets: () => { id: string; name: string }[];
  // render returns a sheet's SVG document with pick targets, as the canvas draws it.
  render: (sheetId: string) => Promise<string>;
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
}

export class SheetPane {
  readonly element: HTMLElement;
  readonly view: SvgView;
  sheet = "";
  link: Link = "follow";
  private readonly host: HTMLElement;
  private readonly picker: HTMLSelectElement;
  private readonly layers: HTMLSelectElement;
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
    this.layers = el("select", "sheetview-layers") as HTMLSelectElement;
    this.layers.title = "board layer visibility";
    for (const s of ["all", "front", "back"]) this.layers.append(new Option(`${s} layers`, s));
    this.layers.addEventListener("change", () => this.applyLayers());
    this.linkSelect = el("select", "sheetview-link") as HTMLSelectElement;
    this.linkSelect.title = "follow: open and centre on what is selected elsewhere. locked: also mirror pan and zoom with other locked views on the same sheet. off: never move.";
    for (const l of LINKS) this.linkSelect.append(new Option(l, l));
    this.linkSelect.addEventListener("change", () => {
      this.link = this.linkSelect.value as Link;
    });
    const float = el("button", "mode-btn sheetview-float") as HTMLButtonElement;
    float.type = "button";
    float.textContent = "Float";
    float.title = "float this view over the page";
    float.addEventListener("click", () => this.deps.onFloat(this));
    const close = el("button", "mode-btn sheetview-close") as HTMLButtonElement;
    close.type = "button";
    close.textContent = "Close";
    close.addEventListener("click", () => this.deps.onClose(this));
    bar.append(this.picker, this.layers, this.linkSelect, float, close);
    this.also = el("div", "sheetview-also");
    this.host = el("div", "sheetview-canvas");
    this.element.append(bar, this.also, this.host);
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
    let svg = "";
    try {
      svg = await this.deps.render(sheetId);
    } catch (e) {
      svg = "";
      this.also.textContent = `could not draw this sheet: ${String(e)}`;
    }
    if (n !== this.drawing) return;
    this.view.setSvg(svg);
    this.deps.onOpened(this);
    this.applyLayers();
    this.layers.style.display = sheetId === BOARD_SHEET ? "" : "none";
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
    if (reveal) this.view.revealOverlay();
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

  private applyLayers(): void {
    this.host.classList.remove("board-front", "board-back");
    const side = this.layers.value;
    if (this.sheet === BOARD_SHEET && (side === "front" || side === "back")) this.host.classList.add(`board-${side}`);
  }

  private nameOf(id: string): string {
    return this.deps.sheets().find((s) => s.id === id)?.name || id;
  }
}
