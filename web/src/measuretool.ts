// MeasureTool is the board measuring tool's overlay on one surface (agni issue 133). It shows where
// the cursor is in millimetres, takes a click as a point, and draws the line between two points with
// their distance. measure.ts holds the model, and each renderer gives the tool a BoardMeasurer, so
// the overlay is the same in WebGL and SVG.
//
// The overlay sits in the surface's stage, over whichever renderer draws, and takes no pointer
// events except on its unit button. The surface hands it clicks before they pick, and asks it to
// redraw when the view moves.

import { Measurement, formatPoint, formatSpan, type BoardMeasurer, type LengthUnit, type MeasurePoint } from "./measure.js";

const SVG_NS = "http://www.w3.org/2000/svg";

export class MeasureTool {
  private on = false;
  private unit: LengthUnit = "mm";
  private readonly measurement = new Measurement();
  private readonly root: HTMLDivElement;
  private readonly lines: SVGSVGElement;
  private readonly cursor: HTMLSpanElement;
  private readonly result: HTMLSpanElement;
  private readonly unitButton: HTMLButtonElement;

  // onChange fires when the tool turns on or off, so a toggle button can show its state.
  onChange: ((on: boolean) => void) | null = null;
  // onSheet fires when the surface draws a sheet, with whether the tool can measure it, so a button
  // shows only over a board.
  onSheet: ((available: boolean) => void) | null = null;

  // stage is the element the renderers draw in, positioned, and measurer the current board's map, or
  // null when the surface draws no board.
  constructor(
    private readonly stage: HTMLElement,
    private readonly measurer: () => BoardMeasurer | null,
  ) {
    const doc = stage.ownerDocument;
    this.root = doc.createElement("div");
    this.root.className = "measure-overlay";
    this.root.dataset.measure = "";
    this.root.hidden = true;
    this.lines = doc.createElementNS(SVG_NS, "svg") as SVGSVGElement;
    this.lines.classList.add("measure-lines");
    const chip = doc.createElement("div");
    chip.className = "measure-readout";
    this.cursor = doc.createElement("span");
    this.cursor.dataset.measureCursor = "";
    this.result = doc.createElement("span");
    this.result.dataset.measureSpan = "";
    this.unitButton = doc.createElement("button");
    this.unitButton.type = "button";
    this.unitButton.dataset.action = "measure-unit";
    this.unitButton.title = "switch between millimetres and mils";
    this.unitButton.addEventListener("click", () => this.setUnit(this.unit === "mm" ? "mil" : "mm"));
    // The unit button comes first, so it stays put while the cursor's text changes width under it.
    chip.append(this.unitButton, this.cursor, this.result);
    this.root.append(this.lines, chip);
    stage.appendChild(this.root);
    stage.addEventListener("mousemove", (e) => {
      if (this.on) this.showCursor(this.measurer()?.at(e.clientX, e.clientY) ?? null);
    });
    doc.addEventListener("keydown", (e) => {
      if (this.on && e.key === "Escape") this.setActive(false);
    });
    this.setUnit("mm");
  }

  get active(): boolean {
    return this.on;
  }

  // available says whether the surface draws a board the tool can measure.
  available(): boolean {
    return this.measurer() !== null;
  }

  setActive(on: boolean): void {
    if (on && !this.available()) on = false;
    if (on === this.on) return;
    this.on = on;
    this.measurement.clear();
    this.root.hidden = !on;
    this.stage.classList.toggle("measuring", on);
    this.showCursor(null);
    this.redraw();
    this.onChange?.(on);
  }

  toggle(): void {
    this.setActive(!this.on);
  }

  // click takes a click as a point while the tool is on, and returns whether it took it. A click off
  // the board is taken too, so measuring never selects anything.
  click(clientX: number, clientY: number): boolean {
    if (!this.on) return false;
    const p = this.measurer()?.at(clientX, clientY);
    if (p) this.measurement.click(p);
    this.redraw();
    return true;
  }

  // sheetChanged clears the points when the surface draws another sheet, and turns the tool off when
  // the new sheet is no board.
  sheetChanged(): void {
    this.measurement.clear();
    const available = this.available();
    if (this.on && !available) this.setActive(false);
    else this.redraw();
    this.onSheet?.(available);
  }

  // redraw places the line and the points where the view now draws them, and writes the result.
  redraw(): void {
    const pts = this.measurement.points;
    this.root.dataset.measurePoints = String(pts.length);
    const span = this.measurement.span();
    this.result.textContent = span ? formatSpan(span, this.unit) : pts.length === 1 ? `from ${formatPoint(pts[0], this.unit)}` : "click a point";
    this.lines.replaceChildren();
    const m = this.on ? this.measurer() : null;
    if (!m || pts.length === 0) return;
    const r = this.stage.getBoundingClientRect();
    const at = pts.map((p) => m.client(p.at)).map((c) => (c ? { x: c.x - r.left, y: c.y - r.top } : null));
    if (at[0] && at[1]) this.lines.append(this.el("line", { x1: at[0].x, y1: at[0].y, x2: at[1].x, y2: at[1].y }));
    for (const c of at) if (c) this.lines.append(this.el("circle", { cx: c.x, cy: c.y, r: 4 }));
  }

  private setUnit(unit: LengthUnit): void {
    this.unit = unit;
    this.unitButton.textContent = unit;
    this.redraw();
  }

  private showCursor(p: MeasurePoint | null): void {
    this.cursor.textContent = p ? formatPoint(p, this.unit) : "";
  }

  private el(tag: string, attrs: Record<string, number>): SVGElement {
    const e = this.stage.ownerDocument.createElementNS(SVG_NS, tag) as SVGElement;
    for (const [k, v] of Object.entries(attrs)) e.setAttribute(k, String(v));
    return e;
  }
}
