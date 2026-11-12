// dock.ts is the dockable panel shell (WS9-021), built on goapplib's tsappkit-dock (agni issues 1549
// and 1542). Each Solid island mounts once at boot in a server-rendered hole, and the holes live in a
// hidden "park" container. The dock asks for a panel's hole (hostFor) whenever the panel arrives in
// the grid, a float or a drawer, and hands it back (release) when it leaves, so an island survives
// its panel being closed, reopened or moved, and C11's shell + islands split is untouched.
//
// The design's own navigation (Sheets) opens from a drawer on the left, and what the engine found and
// what a person asked for (Checks, Review and their reference panels) from a drawer on the right, so
// the drawing and the board beside it keep the width (agni issue 1542). A reader can dock any of
// them back into the grid from its ⋯ menu, and Reset layout puts them back in their drawers.
import { createDock, type Dock, type PanelDef } from "@panyam/tsappkit-dock";
import type { DockviewApi } from "dockview-core";
import "dockview-core/dist/styles/dockview.css";

const LEFT = { kind: "drawer", edge: "left" } as const;
const RIGHT = { kind: "drawer", edge: "right" } as const;

// VIEWER_PANELS is the registry that the dock is built from. Panel ids double as dockview ids and as
// the data-dock-panel key of the server-rendered hole. Every panel opens on a fresh page, in the grid
// or in its drawer. The order is the Panels menu's, and the drawers' tab order.
export const VIEWER_PANELS: readonly PanelDef[] = [
  { id: "canvas", title: "Canvas" },
  // Properties describes whatever is selected (agni issue 902), so a fresh page needs it visible or a
  // click on the drawing paints nowhere. Parts tabs with it, since both answer "what is this".
  { id: "properties", title: "Properties" },
  { id: "parts", title: "Parts" },
  // The datalog query panel (WS9-036) holds the centre column's bottom strip, under the drawing it
  // asks about. Trace (agni issue 600) and Tables (agni issues 898 and 956) tab with it.
  { id: "query", title: "Query" },
  { id: "trace", title: "Trace" },
  { id: "tables", title: "Tables" },
  // Diff (WS9-005) and its changed-item list (WS9-006) tab beside the canvas.
  { id: "diff", title: "Diff" },
  { id: "changes", title: "Changes" },
  // The birds-eye sheet list (WS9-025), in the left drawer (agni issue 1542).
  { id: "overview", title: "Sheets", defaultPlacement: LEFT },
  // The findings with the Run button (WS9), the checklist verdict (WS9-052), and the references that
  // explain them, as tabs in the right drawer (agni issue 1542).
  { id: "checks", title: "Checks", defaultPlacement: RIGHT },
  { id: "review", title: "Review", defaultPlacement: RIGHT },
  { id: "rules", title: "Rules", defaultPlacement: RIGHT },
  { id: "lexicon", title: "Lexicon", defaultPlacement: RIGHT },
  { id: "coverage", title: "Coverage", defaultPlacement: RIGHT },
];

// LAYOUT_KEY is where the dock saves its layout. tsappkit-dock saves a different shape from the
// layout agni's own dock saved under "agni-work-page-dockview-layout-v3", so a new key starts every
// reader on the default once, with Sheets, Checks and Review in their drawers. Bump it when the
// default arrangement changes, since a saved layout wins over the default.
export const LAYOUT_KEY = "agni-dock-v4";

// RAIL_FRACTION and QUERY_FRACTION size the boot layout, a west rail for Properties beside a centre
// column whose bottom fifth is the query panel. Fractions rather than pixels, so the rail's tab labels
// are readable on a laptop and it doesn't become a canyon on a desktop.
const RAIL_FRACTION = 0.15;
const RAIL_MIN_PX = 260;
const QUERY_FRACTION = 0.2;
const QUERY_FALLBACK_PX = 320;
// QUERY_MIN_PX is the shortest the Query panel opens. Its tab strip, query box and Run row take about
// 190px, so a fifth of a 1000px window showed no result row at all (agni issue 759). QUERY_MAX_FRACTION
// keeps the floor from taking most of a short window from the drawing.
const QUERY_MIN_PX = 320;
const QUERY_MAX_FRACTION = 0.45;

// queryHeight is the Query panel's opening height for a dock that tall: a fifth of it, but never
// less than QUERY_MIN_PX, and never more than QUERY_MAX_FRACTION of it.
export function queryHeight(dockHeight: number): number {
  if (!dockHeight) return QUERY_FALLBACK_PX;
  return Math.round(Math.min(Math.max(dockHeight * QUERY_FRACTION, QUERY_MIN_PX), dockHeight * QUERY_MAX_FRACTION));
}

// defaultLayout is the grid's boot arrangement. The drawers' panels go to their drawers after it.
//
//   west 15%     centre
//   ┌──────────┬───────────────────────────────────────┐
//   │Properties│ [Canvas] Diff  Changes                │
//   │ Parts    │                                       │
//   │          ├───────────────────────────────────────┤
//   │          │ [Query] Trace Tables  20% h, ≥ 320px  │
//   └──────────┴───────────────────────────────────────┘
//
// Each position is relative to the reference panel's GROUP, so the rail is built before the centre
// is split, and the query strip then divides the centre column alone.
export function defaultLayout(api: DockviewApi, whenReady: (apply: () => void) => void = (apply) => whenSized(api, apply)): void {
  api.addPanel({ id: "canvas", component: "canvas", title: "Canvas" });
  api.addPanel({ id: "properties", component: "properties", title: "Properties", position: { direction: "left", referencePanel: "canvas" } });
  api.addPanel({ id: "parts", component: "parts", title: "Parts", position: { direction: "within", referencePanel: "properties" } });
  api.addPanel({ id: "query", component: "query", title: "Query", position: { direction: "below", referencePanel: "canvas" } });
  api.addPanel({ id: "trace", component: "trace", title: "Trace", position: { direction: "within", referencePanel: "query" } });
  api.addPanel({ id: "tables", component: "tables", title: "Tables", position: { direction: "within", referencePanel: "query" } });
  api.addPanel({ id: "diff", component: "diff", title: "Diff", position: { direction: "within", referencePanel: "canvas" } });
  api.addPanel({ id: "changes", component: "changes", title: "Changes", position: { direction: "within", referencePanel: "canvas" } });
  whenReady(() => {
    api.getPanel("properties")?.api.group?.api.setSize({ width: Math.max(Math.round(api.width * RAIL_FRACTION), RAIL_MIN_PX) });
    api.getPanel("query")?.api.group?.api.setSize({ height: queryHeight(api.height) });
    // Each stack opens on its first tab, since dockview makes the last panel added the active one.
    api.getPanel("query")?.api.setActive();
    api.getPanel("properties")?.api.setActive();
    api.getPanel("canvas")?.api.setActive();
  });
}

// whenLaidOut runs apply once dockview has laid its grid out over the container. Before its first
// layout dockview reports a 100 by 100 grid, which passes a plain size check and then drops every
// group size set against it, so this waits for the grid to match the container.
function whenLaidOut(container: HTMLElement, api: DockviewApi, apply: () => void, framesLeft = 60): void {
  const w = container.clientWidth;
  if ((w > 0 && Math.abs(api.width - w) <= 1 && api.height > 0) || framesLeft <= 0) {
    apply();
    return;
  }
  const next = (): void => whenLaidOut(container, api, apply, framesLeft - 1);
  if (typeof requestAnimationFrame === "function") requestAnimationFrame(next);
  else setTimeout(next, 16);
}

// whenSized runs apply once the dock has a non-zero size, or after about 30 frames if it never does
// (a hidden container, a headless host with no animation frames). Sizes take effect only once
// dockview has laid the grid out, and it measures itself from a ResizeObserver that hasn't fired by
// the end of the current task.
function whenSized(api: DockviewApi, apply: () => void, framesLeft = 30): void {
  if ((api.width > 0 && api.height > 0) || framesLeft <= 0) {
    apply();
    return;
  }
  const next = (): void => whenSized(api, apply, framesLeft - 1);
  if (typeof requestAnimationFrame === "function") requestAnimationFrame(next);
  else setTimeout(next, 16);
}

// SHEET_VIEW_PREFIX marks a sheet view's panel id (agni issue 1326). A sheet view is made at run time
// for one sheet of the open design, so it is not in VIEWER_PANELS and has no hole in the page. The
// dock resolves it while it is open, and a saved layout that names one restores without it, since
// nothing resolves it on the next page.
export const SHEET_VIEW_PREFIX = "sheetview-";

interface SheetViewEntry {
  title: string;
  element: HTMLElement;
  closed: () => void;
  position: PanelDef["position"];
}

// ViewerDock is the viewer's dock: tsappkit-dock's Dock plus the sheet views it holds.
export interface ViewerDock {
  dock: Dock;
  // addSheetView docks a sheet view. The first opens to the right of the canvas, and a later one tabs
  // with the last sheet view still open, so views gather in one group until the reader moves them.
  // closed runs when the view closes, from its tab or from its own Close.
  addSheetView(id: string, title: string, element: HTMLElement, closed: () => void): void;
  setTitle(id: string, title: string): void;
  // toggleShown brings a panel to the front, opening its drawer, or closes the drawer when the panel
  // is already in front in it. The header's drawer buttons and ⌘/Ctrl+G use it.
  toggleShown(id: string): void;
}

// DRAWER_PANELS are the panels the header's drawer buttons show, each opening the drawer that holds
// it (agni issue 1542).
export const DRAWER_PANELS = ["overview", "checks", "review"] as const;

// createViewerDock boots the dock, restoring the saved layout when there is one, and adds the Panels
// menu and the drawer buttons to menuHost.
export function createViewerDock(container: HTMLElement, park: HTMLElement, menuHost: HTMLElement, storage: Storage | null): ViewerDock {
  const doc = container.ownerDocument;
  const sheetViews = new Map<string, SheetViewEntry>();
  const holes = new Map<string, HTMLElement>();
  const hostFor = (id: string): HTMLElement => {
    const view = sheetViews.get(id);
    if (view) return view.element;
    let hole = holes.get(id);
    if (!hole) {
      hole = doc.querySelector<HTMLElement>(`[data-dock-panel="${id}"]`) ?? doc.createElement("div");
      holes.set(id, hole);
    }
    return hole;
  };
  const buttons: HTMLButtonElement[] = [];
  // fronts is the panel in front in each open drawer, as drawerChanged reports it.
  const fronts = new Map<string, string>();
  const inFront = (id: string): boolean => {
    const at = dock.where(id);
    return at?.kind === "drawer" && fronts.get(at.edge) === id;
  };
  const markButtons = (): void => {
    for (const b of buttons) b.classList.toggle("active", inFront(b.dataset.panel ?? ""));
  };
  const toggleShown = (id: string): void => {
    const at = dock.where(id);
    if (at?.kind === "drawer" && inFront(id)) dock.toggleDrawer(at.edge, false);
    else dock.show(id);
  };
  const dock: Dock = createDock({
    container,
    panels: VIEWER_PANELS,
    resolve: (id) => {
      const v = sheetViews.get(id);
      return v ? { id, title: v.title, position: v.position } : undefined;
    },
    defaultLayout: (api) => defaultLayout(api, (apply) => whenLaidOut(container, api, apply)),
    hostFor,
    release: (id, el) => {
      if (!sheetViews.has(id)) park.appendChild(el);
    },
    // A sheet view that leaves the dock altogether is closed, and one that only moves is not.
    moved: (id, _el, to) => {
      const v = sheetViews.get(id);
      if (v && to === null) {
        sheetViews.delete(id);
        v.closed();
      }
    },
    drawerChanged: (edge, open, active) => {
      if (open && active) fronts.set(edge, active);
      else fronts.delete(edge);
      markButtons();
    },
    storage,
    key: LAYOUT_KEY,
    // The viewer's page is light-only.
    theme: () => "light",
    drawers: { left: { size: 280 }, right: { size: 0.34 } },
  });
  for (const id of DRAWER_PANELS) {
    const def = VIEWER_PANELS.find((p) => p.id === id)!;
    const b = doc.createElement("button");
    b.type = "button";
    b.className = "mode-btn dock-drawer-btn";
    b.dataset.panel = id;
    b.textContent = def.title;
    b.title = id === "overview" ? `Show or hide ${def.title} (⌘/Ctrl+G)` : `Show or hide ${def.title}`;
    b.addEventListener("click", () => toggleShown(id));
    buttons.push(b);
    menuHost.appendChild(b);
  }
  dock.panelsMenu(menuHost);
  return {
    dock,
    toggleShown,
    addSheetView: (id, title, element, closed) => {
      const open = [...sheetViews.keys()].filter((k) => k !== id && dock.where(k) !== null);
      const last = open[open.length - 1];
      sheetViews.set(id, {
        title,
        element,
        closed,
        position: last ? { direction: "within", referencePanel: last } : { direction: "right", referencePanel: "canvas" },
      });
      dock.show(id);
    },
    setTitle: (id, title) => {
      const v = sheetViews.get(id);
      if (v) v.title = title;
      dock.api.getPanel(id)?.api.setTitle(title);
    },
  };
}

// openDiffPanel focuses the diff view and its changed-item list, opening either first if the reader
// closed it.
export function openDiffPanel(dock: Dock): void {
  if (!dock.where("changes")) dock.show("changes");
  dock.show("diff");
}

// closeDiffPanel ends a comparison by returning attention to the drawing. It leaves the diff and
// changes tabs in place, since both render "No comparison open" on their own.
export function closeDiffPanel(dock: Dock): void {
  dock.show("canvas");
}
