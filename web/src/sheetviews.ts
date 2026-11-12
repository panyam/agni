// SheetViews owns the sheet views open beside the canvas (agni issue 1326). It opens one on request,
// hands each a selection made anywhere on the page, and mirrors the camera between locked views on
// one sheet. The dock is reached through SheetViewDock, so a test runs the manager with no dock.
import { BOARD_SHEET, SheetPane, type SheetPaneDeps } from "./sheetpane.js";
import type { SurfaceMode } from "./sheetsurface.js";
import { lockedPeers } from "./sheetfollow.js";
import type { Selection } from "./selection.js";
import type { HighlightSpec } from "./highlights.js";

// SheetViewDock is what the manager asks of the dock.
export interface SheetViewDock {
  add(id: string, title: string, element: HTMLElement, closed: () => void): void;
  setTitle(id: string, title: string): void;
  float(id: string): void;
  close(id: string): void;
}

// SheetViewHooks are the page's side of a pick made in a view. The page selects it, as it selects a
// pick on the canvas, and the manager then passes the selection to the other views.
export interface SheetViewHooks {
  onPick: (pane: SheetPane, sel: Selection) => void;
  onPickEmpty: (pane: SheetPane) => void;
}

type EngineDeps = Pick<SheetPaneDeps, "sheets" | "render" | "overlay" | "drawnOn" | "specs" | "turns" | "renderer" | "gl"> & {
  // boardRenderer picks a board view's renderer when the caller names none and the canvas draws in
  // SVG. It answers WebGL on a hardware GPU and SVG on a software one (agni issue 1536). Without it,
  // a board view opens in the canvas's renderer, as a schematic view does.
  boardRenderer?: () => SurfaceMode;
};

export class SheetViews {
  private readonly panes = new Map<string, SheetPane>();
  private next = 1;
  // underlay is the page's highlight that every view lights under its selection, such as a walk.
  private underlay: HighlightSpec[] = [];

  constructor(
    private readonly dock: SheetViewDock,
    private readonly engine: EngineDeps,
    private readonly hooks: SheetViewHooks,
    private readonly doc: Document = document,
  ) {}

  // open adds a view showing sheetId and returns it once the sheet is drawn. renderer picks SVG or
  // WebGL. Without it a board view opens in WebGL when the canvas does, and otherwise in
  // boardRenderer (agni issue 1536). Any other sheet opens in the canvas's renderer (agni issue 1368).
  async open(sheetId: string, renderer?: SurfaceMode): Promise<SheetPane> {
    const id = `sheetview-${this.next++}`;
    const pane = new SheetPane(
      id,
      {
        ...this.engine,
        renderer: renderer ? () => renderer : (sheet) => this.rendererFor(sheet),
        onPick: (p, sel) => this.hooks.onPick(p, sel),
        onPickEmpty: (p) => this.hooks.onPickEmpty(p),
        // A locked peer takes the drawing point at this view's centre and its zoom, so two views of
        // the board line up though one is the bottom view or a different size (agni issue 1327).
        onViewChange: (p) => {
          const c = p.getCentre();
          if (!c) return;
          for (const peer of lockedPeers(this.list(), p)) this.panes.get(peer.id)?.setCentre(c.at, c.scale);
        },
        onFloat: (p) => this.dock.float(p.id),
        onClose: (p) => this.dock.close(p.id),
        onOpened: (p) => this.dock.setTitle(p.id, p.title()),
      },
      this.doc,
    );
    this.panes.set(id, pane);
    this.dock.add(id, pane.title(), pane.element, () => {
      pane.dispose();
      this.panes.delete(id);
    });
    if (this.underlay.length > 0) await pane.setUnderlay(this.underlay);
    await pane.open(sheetId);
    return pane;
  }

  // rendererFor is the renderer a view takes on a sheet when nobody named one. A board takes WebGL
  // when the canvas draws in WebGL, and otherwise boardRenderer's answer (agni issue 1536). Any other
  // sheet takes the canvas's renderer (agni issue 1368).
  private rendererFor(sheetId: string): SurfaceMode {
    const canvas = this.engine.renderer?.("") ?? "svg";
    if (sheetId !== BOARD_SHEET || canvas === "webgl") return canvas;
    return this.engine.boardRenderer?.() ?? canvas;
  }

  // follow hands a selection to every view. The view it was picked in lights it where it is, and the
  // rest follow it as their link says. A null selection clears every view's highlight.
  async follow(sel: Selection | null, pickedIn = ""): Promise<void> {
    await Promise.all(
      [...this.panes.values()].map((p) => (p.id === pickedIn && sel ? p.lightPick(sel) : p.follow(sel))),
    );
  }

  // setUnderlay lights specs in every view under its selection, and in each view opened later (agni
  // issue 1480). An empty list takes it away. reveal asks each following view to frame them, as a
  // trace's route is framed (agni issue 1551).
  async setUnderlay(specs: HighlightSpec[], reveal = false): Promise<void> {
    this.underlay = specs;
    await Promise.all([...this.panes.values()].map((p) => p.setUnderlay(specs, reveal)));
  }

  // close closes one view. It returns false when no open view has that id.
  close(id: string): boolean {
    if (!this.panes.has(id)) return false;
    this.dock.close(id);
    this.panes.get(id)?.dispose();
    this.panes.delete(id);
    return true;
  }

  // get returns the open view with that id.
  get(id: string): SheetPane | undefined {
    return this.panes.get(id);
  }

  // reset closes every view, for a page that opens another design.
  reset(): void {
    for (const id of [...this.panes.keys()]) this.dock.close(id);
    for (const p of this.panes.values()) p.dispose();
    this.panes.clear();
    this.underlay = [];
  }

  list(): SheetPane[] {
    return [...this.panes.values()];
  }
}
