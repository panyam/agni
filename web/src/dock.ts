// dock.ts is the dockable panel shell (WS9-021). dockview-core drives the viewer layout, and each
// Solid island mounts once at boot in a server-rendered hole that the dock adopts. The holes live in
// a hidden "park" container; createComponent moves a hole into its dockview panel and dispose parks
// it again, so an island survives its panel being closed and reopened and C11's shell + islands
// split is untouched. The reference implementation (lilbattle's GameViewerPage) clones hidden
// templates lazily instead. That would break this app's boot, where AppRoot resolves every hole by
// id up front and the presenter wires all island callbacks at construction.
import { DockviewComponent, type DockviewApi, type IContentRenderer } from "dockview-core";
import "dockview-core/dist/styles/dockview.css";

export interface DockPanelDef {
  id: string;
  title: string;
  // defaultOpen marks a panel the boot layout places (defaultLayout). Every panel sets it, since
  // the layout tabs panels rather than hiding them in a menu (#325). It also gates the reconcile, so
  // a panel added in a later release opens for existing users only if it sets this, and a menu-only
  // panel would omit it.
  defaultOpen?: boolean;
}

// VIEWER_PANELS is the registry the dock is built from. Panel ids double as dockview component
// names and as the data-dock-panel key of the server-rendered hole.
export const VIEWER_PANELS: readonly DockPanelDef[] = [
  // Files was retired in WS9-049. prunePanels drops it from a saved layout that still names it.
  // The birds-eye sheet list (WS9-025).
  { id: "overview", title: "Sheets", defaultOpen: true },
  { id: "canvas", title: "Canvas", defaultOpen: true },
  // Details is the click-to-inspect target, so a fresh page needs it visible or selecting a
  // component paints nowhere.
  { id: "details", title: "Details", defaultOpen: true },
  // Rules is a static reference catalog, tabbed in the east stack beside the findings it explains.
  { id: "rules", title: "Rules", defaultOpen: true },
  // Checks is the findings table (WS9), server-sourced and grouped and sorted on the client, with
  // the on-demand Run button. Its severity grouping covers what a separate Report panel did.
  { id: "checks", title: "Checks", defaultOpen: true },
  // The datalog query panel (WS9-036) for ad-hoc search over the fact base. It holds the centre
  // column's bottom strip, under the drawing it asks about.
  { id: "query", title: "Query", defaultOpen: true },
  // The interface-coverage panel (WS9-041), a per-interface signal matrix tabbed with Review.
  { id: "coverage", title: "Coverage", defaultOpen: true },
  // The trace panel (agni issue 600) follows one pin to another and lights the route on the
  // canvas. It tabs with Query (see defaultLayout).
  { id: "trace", title: "Trace", defaultOpen: true },
  // The datasheet-params panel (WS9-035), a per-component parameter tree tabbed with Details since
  // both answer "what is this thing I selected". Populated only when serve was started with --params.
  { id: "parts", title: "Parts", defaultOpen: true },
  // The review panel (WS9-052) shows the project's checklist verdict for the open design (see
  // review.ts). Useful when serve was started with --review-store.
  { id: "review", title: "Review", defaultOpen: true },
  // Diff (WS9-005) and its changed-item list (WS9-006) are tabs beside the canvas, and starting a
  // comparison activates them (openDiffPanel).
  { id: "diff", title: "Diff", defaultOpen: true },
  { id: "changes", title: "Changes", defaultOpen: true },
];

// Bump LAYOUT_KEY whenever the DEFAULT ARRANGEMENT changes. A saved layout wins over the default
// and the reconcile only ADDS panels, so without a bump anyone who has opened the viewer before
// keeps their old arrangement and never sees the change. Bumped for the WS9-049 work page and for
// the tabbed layout (#325).
export const LAYOUT_KEY = "agni-work-page-dockview-layout-v2";

type LayoutStorage = Pick<Storage, "getItem" | "setItem" | "removeItem">;

// SavedLayout is the persisted shape, the dockview layout plus the panel-registry ids that existed
// at save time. The ids are a version marker that needs no manual bump, since they derive from
// VIEWER_PANELS. They let the restore tell a NEWLY ADDED panel (absent from panels) apart from a
// USER-CLOSED one (present in panels, absent from the layout). Without them a saved layout freezes
// the panel set, and panels added later never appear for anyone with a saved arrangement.
interface SavedLayout {
  v: 1;
  panels: string[];
  layout: unknown;
}

// LoadedLayout is what loadLayout hands the restore. savedPanels is empty for a pre-versioning raw
// save, which makes every current defaultOpen panel count as newly added. That happens once, since
// the next save is wrapped.
export interface LoadedLayout {
  layout: unknown;
  savedPanels: string[];
}

// loadLayout returns the saved layout, or null when there is none or it does not parse; the
// caller falls back to the default layout in both cases. It reads both the versioned wrapper and
// the bare dockview JSON that predates it.
export function loadLayout(storage: LayoutStorage): LoadedLayout | null {
  const raw = storage.getItem(LAYOUT_KEY);
  if (!raw) return null;
  try {
    const parsed: unknown = JSON.parse(raw);
    if (parsed && typeof parsed === "object" && "v" in parsed && "layout" in parsed) {
      const s = parsed as SavedLayout;
      return { layout: s.layout, savedPanels: Array.isArray(s.panels) ? s.panels : [] };
    }
    return { layout: parsed, savedPanels: [] };
  } catch {
    return null;
  }
}

// saveLayout persists the layout stamped with the current panel registry. A storage failure
// (quota, private mode) is swallowed so it cannot take the viewer down.
export function saveLayout(storage: LayoutStorage, layout: unknown): void {
  try {
    const wrapped: SavedLayout = { v: 1, panels: VIEWER_PANELS.map((p) => p.id), layout };
    storage.setItem(LAYOUT_KEY, JSON.stringify(wrapped));
  } catch {
    // best-effort persistence
  }
}

// reconcilePanels runs after a saved layout is restored. A defaultOpen panel that is not open and
// NOT in the registry the layout was saved under was added by a later release, so it opens at
// openPanel's position. Panels the user closed stay closed, since they were in the saved registry.
// A new panel without defaultOpen stays closed too and appears only in the Panels menu (WS9-042),
// so a new feature does not re-crowd an existing user's layout.
export function reconcilePanels(api: DockviewApi, savedPanels: string[]): void {
  for (const def of VIEWER_PANELS) {
    if (!def.defaultOpen || api.getPanel(def.id) || savedPanels.includes(def.id)) continue;
    openPanel(api, def);
  }
}

// prunePanels closes panels a restored layout names that the registry no longer has. It is the
// counterpart to reconcilePanels, which only ever ADDS. Without it a removed panel restores as an
// empty tab, because adoptPanel falls back to a blank element when the hole is gone. The user's
// arrangement of the surviving panels is untouched.
export function prunePanels(api: DockviewApi): void {
  const known = new Set(VIEWER_PANELS.map((p) => p.id));
  // Snapshot first, because removePanel mutates the collection being walked.
  for (const panel of [...(api.panels ?? [])]) {
    if (!known.has(panel.id)) api.removePanel(panel);
  }
}

// adoptPanel is the dockview content renderer for a panel. It adopts the server-rendered hole,
// found by data-dock-panel in the park, and parks it again on dispose. Element identity survives
// close and reopen, so the island mounted in the hole keeps working. An unknown name yields an
// empty div rather than a crash, so a stale saved layout naming a removed panel still loads.
export function adoptPanel(park: HTMLElement, name: string): IContentRenderer {
  const doc = park.ownerDocument;
  const hole = doc.querySelector<HTMLElement>(`[data-dock-panel="${name}"]`);
  const element = hole ?? doc.createElement("div");
  return {
    element,
    init: () => {},
    dispose: () => {
      park.appendChild(element);
    },
  };
}

// RAIL_FRACTION and QUERY_FRACTION size the boot layout, two 15% side rails around a centre column
// whose bottom fifth is the query panel. Fractions rather than pixels, so the rails' tab labels stay
// readable on a laptop without becoming a canyon on a desktop.
const RAIL_FRACTION = 0.15;
const QUERY_FRACTION = 0.2;
const RAIL_FALLBACK_PX = 260;
const QUERY_FALLBACK_PX = 180;
// EAST_MIN_PX keeps the east stacks' two-tab strips readable on a small window. dockview clips a
// strip that does not fit rather than scrolling it, so below roughly 1270px wide, 15% starts eating
// the word "Coverage". Two tabs need ~190px and four needed ~380, which ruled out one stack of four.
const EAST_MIN_PX = 190;

// defaultLayout is the boot arrangement, and it opens EVERY panel, with four surfaces visible and
// the rest tabbed behind them (#325). Tabs handle the crowding and keep every panel one labelled
// click away. The layout it replaced left five panels in a menu, and a first-time reader landed on
// a schematic with no visible way to ask anything about it.
//
//   west 15%   centre 80% h                    east 15%
//   ┌────────┬────────────────────────────────┬─────────────┐
//   │ Sheets │ [Canvas] Diff  Changes         │[Checks]Rules│
//   │        │                                │             │
//   ├────────┤                                ├─────────────┤
//   │[Details│                                │[Review] Cov.│
//   │ Parts  ├────────────────────────────────┤             │
//   │        │ Query                    20% h │             │
//   └────────┴────────────────────────────────┴─────────────┘
//
// Order matters. Each position is relative to the reference panel's GROUP, so the side rails are
// built before the centre is split. Adding query below canvas afterwards divides the centre column
// alone, where doing it first would put a full-width strip under all three.
export function defaultLayout(api: DockviewApi): void {
  api.addPanel({ id: "canvas", component: "canvas", title: "Canvas" });

  // West rail, with the design's own navigation above what one selection is made of.
  api.addPanel({ id: "overview", component: "overview", title: "Sheets", position: { direction: "left", referencePanel: "canvas" } });
  api.addPanel({ id: "details", component: "details", title: "Details", position: { direction: "below", referencePanel: "overview" } });
  api.addPanel({ id: "parts", component: "parts", title: "Parts", position: { direction: "within", referencePanel: "details" } });

  // East rail, split north/south like the west rail. dockview clips a tab strip rather than
  // scrolling it or offering an overflow menu, so four tabs in one 15% stack left the fourth
  // reachable only through the Panels menu (those four labels need ~380px, 30% of a 1280px laptop).
  // Two stacks of two fit, with what the engine found above what a person asked for.
  api.addPanel({ id: "checks", component: "checks", title: "Checks", position: { direction: "right", referencePanel: "canvas" } });
  api.addPanel({ id: "rules", component: "rules", title: "Rules", position: { direction: "within", referencePanel: "checks" } });
  api.addPanel({ id: "review", component: "review", title: "Review", position: { direction: "below", referencePanel: "checks" } });
  api.addPanel({ id: "coverage", component: "coverage", title: "Coverage", position: { direction: "within", referencePanel: "review" } });

  // The centre column's bottom fifth, after both rails exist so it splits the centre alone.
  api.addPanel({ id: "query", component: "query", title: "Query", position: { direction: "below", referencePanel: "canvas" } });
  // Trace tabs with Query and has to be added AFTER it. dockview resolves referencePanel against
  // panels that already exist, so naming one the layout has not placed yet throws rather than
  // deferring. Both are questions a reader asks ABOUT the drawing, so they share a strip.
  api.addPanel({ id: "trace", component: "trace", title: "Trace", position: { direction: "within", referencePanel: "query" } });

  // Diff and Changes are placed at boot rather than opened on demand. Both render an empty state
  // naming the Compare button, so a reader who clicks one learns the feature exists instead of
  // meeting a blank pane.
  api.addPanel({ id: "diff", component: "diff", title: "Diff", position: { direction: "within", referencePanel: "canvas" } });
  api.addPanel({ id: "changes", component: "changes", title: "Changes", position: { direction: "within", referencePanel: "canvas" } });

  // Sizes and active tabs only stick once dockview has laid the grid out, so this waits for the grid
  // to HAVE a size rather than guessing a delay. dockview measures itself from a ResizeObserver on
  // its container, which has not fired by the end of the current task. Sizing on setTimeout(0)
  // divides up a 0x0 grid and is dropped, which is why the earlier layout's pixel widths never
  // applied and why a 250ms sleep "fixed" it.
  whenSized(api, () => {
    const rail = api.width ? Math.round(api.width * RAIL_FRACTION) : RAIL_FALLBACK_PX;
    resizeGroup(api, "overview", { width: rail });
    resizeGroup(api, "checks", { width: Math.max(rail, EAST_MIN_PX) });
    resizeGroup(api, "query", { height: api.height ? Math.round(api.height * QUERY_FRACTION) : QUERY_FALLBACK_PX });
    // Each stack opens on its first tab. Query needs this explicitly, because dockview makes the
    // last panel added the active one and Trace is added to that stack after Query.
    api.getPanel("query")?.api.setActive();
    api.getPanel("details")?.api.setActive();
    api.getPanel("checks")?.api.setActive();
    api.getPanel("review")?.api.setActive();
    api.getPanel("canvas")?.api.setActive();
  });
}

// whenSized runs apply once the dock has a non-zero size, or after about 30 frames if it never does
// (a hidden container, a headless host with no animation frames). The bound is there because a
// layout that never sizes must still get its active tabs set.
function whenSized(api: DockviewApi, apply: () => void, framesLeft = 30): void {
  if ((api.width > 0 && api.height > 0) || framesLeft <= 0) {
    apply();
    return;
  }
  const next = (): void => whenSized(api, apply, framesLeft - 1);
  if (typeof requestAnimationFrame === "function") requestAnimationFrame(next);
  else setTimeout(next, 16);
}

// resizeGroup sizes the GROUP a panel sits in, the only thing dockview will resize.
//
// `panel.api.setSize()` looks like it does this and does nothing. It fires onDidSizeChange, and the
// listener for that event is installed by the GRIDVIEW panel (the group), not by the dockview panel
// inside it, so the call type-checks, runs, and is dropped. Every boot width was silently ignored
// from WS9-021 until #325 (the rails were three equal columns), and a unit test asserting the layout
// saw only that the CALL was made, never that the pixels moved.
function resizeGroup(api: DockviewApi, panelId: string, size: { width?: number; height?: number }): void {
  api.getPanel(panelId)?.api.group?.api.setSize(size);
}

// openDiffPanel focuses the diff view and its changed-item list, adding either back first if the
// user closed it. The default layout already places both beside the canvas, so on an untouched
// layout this is a tab switch, and the add path covers a layout where they were closed.
export function openDiffPanel(api: DockviewApi): void {
  const existing = api.getPanel("diff");
  if (!existing) {
    const canvas = api.getPanel("canvas");
    api.addPanel({
      id: "diff",
      component: "diff",
      title: "Diff",
      position: canvas ? { direction: "within", referencePanel: "canvas" } : { direction: "right" },
    });
  }
  if (!api.getPanel("changes")) {
    api.addPanel({
      id: "changes",
      component: "changes",
      title: "Changes",
      position: { direction: "right", referencePanel: "diff" },
    });
    // Meant to keep the right column a rail, but a panel-level setSize is dropped (see
    // resizeGroup), so this has no effect.
    setTimeout(() => api.getPanel("changes")?.api.setSize({ width: 260 }), 0);
  }
  api.getPanel("diff")?.api.setActive();
}

// closeDiffPanel ends a comparison by returning attention to the drawing. It LEAVES the diff and
// changes tabs in place, because the default layout places them (#325) and a tab strip that loses
// two tabs on close reads as the app breaking. Both panels render "No comparison open" on their own.
export function closeDiffPanel(api: DockviewApi): void {
  api.getPanel("canvas")?.api.setActive();
}

// openPanel re-adds a closed panel from the menu, on the right edge. The user drags it where they
// want and persistence keeps it.
export function openPanel(api: DockviewApi, def: DockPanelDef): void {
  api.addPanel({
    id: def.id,
    component: def.id,
    title: def.title,
    position: { direction: "right" },
  });
}

// panelsMenu renders the "Panels" dropdown in the top bar, one entry per registered panel with a
// ✓ on the open ones. Each item is a show/hide TOGGLE (WS9-042), so the menu both reopens a panel
// the tab-× closed and hides one. Panel layout is dock chrome, not presenter state, so this is a
// plain DOM widget rather than an island.
export function panelsMenu(host: HTMLElement, api: DockviewApi): void {
  const doc = host.ownerDocument;
  const btn = doc.createElement("button");
  btn.type = "button";
  btn.className = "mode-btn panels-btn";
  btn.textContent = "Panels ▾";
  const list = doc.createElement("div");
  list.className = "panels-list";
  list.style.display = "none";
  const rebuild = (): void => {
    list.replaceChildren();
    for (const def of VIEWER_PANELS) {
      const item = doc.createElement("button");
      item.type = "button";
      item.className = "panels-item";
      item.textContent = `${api.getPanel(def.id) ? "✓" : "  "} ${def.title}`;
      item.addEventListener("click", () => {
        const panel = api.getPanel(def.id);
        if (panel) api.removePanel(panel);
        else openPanel(api, def);
        list.style.display = "none";
      });
      list.appendChild(item);
    }
  };
  btn.addEventListener("click", () => {
    const show = list.style.display === "none";
    if (show) rebuild();
    list.style.display = show ? "block" : "none";
  });
  doc.addEventListener("click", (e) => {
    if (!host.contains(e.target as Node)) list.style.display = "none";
  });
  host.appendChild(btn);
  host.appendChild(list);
}

// createViewerDock boots the dock. It restores the saved layout when there is one, falling back to
// the default when it fails to apply (a corrupt or incompatible save), then persists every layout
// change. The page is light-only, so the theme class is fixed and there is no theme observer.
export function createViewerDock(container: HTMLElement, park: HTMLElement, menuHost: HTMLElement, storage: LayoutStorage): DockviewApi {
  container.classList.add("dockview-theme-light");
  const component = new DockviewComponent(container, {
    createComponent: (options) => adoptPanel(park, options.name),
    // "always" keeps every panel's content element attached (hidden) when its tab is not in
    // front. The default ("onlyWhenVisible") DETACHES hidden tab content, and AppRoot resolves
    // every island hole by id at boot, so a hole in a background tab (Rules behind Checks, or any
    // tab in a saved layout) would be missing and its island would never mount.
    defaultRenderer: "always",
  });
  const api = component.api;
  const saved = loadLayout(storage);
  if (saved !== null) {
    try {
      api.fromJSON(saved.layout as Parameters<typeof api.fromJSON>[0]);
      // Panels added to the registry since this layout was saved appear now, and ones removed
      // from it go away, without touching the user's arrangement of the rest.
      reconcilePanels(api, saved.savedPanels);
      prunePanels(api);
    } catch (err) {
      console.warn("dock: saved layout rejected, using default", err);
      storage.removeItem(LAYOUT_KEY);
      api.clear();
      defaultLayout(api);
    }
  } else {
    defaultLayout(api);
  }
  api.onDidLayoutChange(() => saveLayout(storage, api.toJSON()));
  panelsMenu(menuHost, api);
  return api;
}
