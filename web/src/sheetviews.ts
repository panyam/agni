// SheetViews owns the sheet views open beside the canvas (agni issue 1326). It opens one on request,
// hands each a selection made anywhere on the page, and mirrors the camera between locked views on
// one sheet. The dock is reached through SheetViewDock, so a test runs the manager with no dock.
import { SheetPane, type SheetPaneDeps } from "./sheetpane.js";
import { lockedPeers } from "./sheetfollow.js";
import type { Selection } from "./selection.js";

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

type EngineDeps = Pick<SheetPaneDeps, "sheets" | "render" | "overlay" | "drawnOn" | "specs">;

export class SheetViews {
  private readonly panes = new Map<string, SheetPane>();
  private next = 1;

  constructor(
    private readonly dock: SheetViewDock,
    private readonly engine: EngineDeps,
    private readonly hooks: SheetViewHooks,
    private readonly doc: Document = document,
  ) {}

  // open adds a view showing sheetId and returns it once the sheet is drawn.
  async open(sheetId: string): Promise<SheetPane> {
    const id = `sheetview-${this.next++}`;
    const pane = new SheetPane(
      id,
      {
        ...this.engine,
        onPick: (p, sel) => this.hooks.onPick(p, sel),
        onPickEmpty: (p) => this.hooks.onPickEmpty(p),
        onViewChange: (p, v) => {
          for (const peer of lockedPeers(this.list(), p)) this.panes.get(peer.id)?.view.setView(v);
        },
        onFloat: (p) => this.dock.float(p.id),
        onClose: (p) => this.dock.close(p.id),
        onOpened: (p) => this.dock.setTitle(p.id, p.title()),
      },
      this.doc,
    );
    this.panes.set(id, pane);
    this.dock.add(id, pane.title(), pane.element, () => this.panes.delete(id));
    await pane.open(sheetId);
    return pane;
  }

  // follow hands a selection to every view. The view it was picked in lights it where it is, and the
  // rest follow it as their link says. A null selection clears every view's highlight.
  async follow(sel: Selection | null, pickedIn = ""): Promise<void> {
    await Promise.all(
      [...this.panes.values()].map((p) => (p.id === pickedIn && sel ? p.lightPick(sel) : p.follow(sel))),
    );
  }

  // reset closes every view, for a page that opens another design.
  reset(): void {
    for (const id of [...this.panes.keys()]) this.dock.close(id);
    this.panes.clear();
  }

  list(): SheetPane[] {
    return [...this.panes.values()];
  }
}
