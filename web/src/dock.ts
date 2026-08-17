// dock.ts is the dockable panel shell (WS9-021): dockview-core drives the viewer layout,
// with each existing Solid island mounted in a server-rendered hole that the dock adopts.
// The holes live in a hidden "park" container in the page; createComponent moves a hole into
// its dockview panel and dispose parks it again. Islands therefore mount exactly once at
// boot (C11's server-rendered shell + islands split is untouched) and survive a panel being
// closed and reopened. This is the one deliberate deviation from the reference
// implementation (lilbattle's GameViewerPage), which clones hidden templates lazily: lazy
// creation would break this app's boot, where AppRoot resolves every hole by id up front and
// the presenter wires all island callbacks at construction.
import { DockviewComponent, type DockviewApi, type IContentRenderer } from "dockview-core";
import "dockview-core/dist/styles/dockview.css";

export interface DockPanelDef {
  id: string;
  title: string;
  // defaultOpen marks a panel the boot layout places (defaultLayout). Every panel carries it
  // today: the layout tabs the crowding away instead of hiding panels in a menu, so there is no
  // longer a "secondary" tier. The flag stays because it also gates the reconcile — a panel added
  // in a later release opens for existing users only if it is part of the default arrangement, and
  // a future menu-only panel omits it.
  defaultOpen?: boolean;
}

// VIEWER_PANELS is the registry the dock is built from: panel ids double as dockview
// component names and as the data-dock-panel key of the server-rendered hole.
export const VIEWER_PANELS: readonly DockPanelDef[] = [
  // Files is GONE as of WS9-049 phase 3. It was demoted to secondary in phase 1 and kept only
  // because the old Compare flow needed a tree to click side B in; the compare picker replaced
  // that, and /designs replaced cross-file navigation, so the panel had no remaining job. A saved
  // layout still naming it is handled by prunePanels, not by keeping a registry entry alive.
  // The birds-eye sheet list (WS9-025); for existing
  // saved layouts it appears via the reconcile pass (it is not in their saved registry).
  { id: "overview", title: "Sheets", defaultOpen: true },
  { id: "canvas", title: "Canvas", defaultOpen: true },
  // Details is core: it is the click-to-inspect target, so a fresh page needs it visible or
  // selecting a component paints nowhere.
  { id: "details", title: "Details", defaultOpen: true },
  // Rules is a static reference catalog: a tab in the east stack beside the findings it explains.
  { id: "rules", title: "Rules", defaultOpen: true },
  // Checks is the merged findings+report panel (WS9): a server-sourced, client-grouped/sorted table
  // with the on-demand Run button. The separate Report panel was folded in (severity is one grouping).
  { id: "checks", title: "Checks", defaultOpen: true },
  // The datalog query panel (WS9-036): ad-hoc search over the fact base. It holds the centre
  // column's bottom strip, because asking a question about the drawing belongs under the drawing.
  { id: "query", title: "Query", defaultOpen: true },
  // The interface-coverage panel (WS9-041): per-interface signal matrix, tabbed with the checks.
  { id: "coverage", title: "Coverage", defaultOpen: true },
  // The datasheet-params panel (WS9-035): per-component parameter tree, tabbed with Details since
  // both answer "what is this thing I selected". Populated only when serve was started with --params.
  { id: "parts", title: "Parts", defaultOpen: true },
  // The review panel (WS9-052): the project's checklist verdict for the open design, over the stored
  // runs (WS9-053). Tabbed with the checks; useful when serve was started with --review-store.
  { id: "review", title: "Review", defaultOpen: true },
  // Diff (WS9-005) and its changed-item list (WS9-006) ride as tabs beside the canvas. Both render
  // an empty state naming the Compare button, so the tab teaches the feature rather than opening
  // onto a blank pane; starting a comparison just activates them (openDiffPanel).
  { id: "diff", title: "Diff", defaultOpen: true },
  { id: "changes", title: "Changes", defaultOpen: true },
];

// LAYOUT_KEY is bumped whenever the DEFAULT ARRANGEMENT changes, and that is not optional: a saved
// layout wins over the default, and the reconcile only ADDS panels, so anyone who has opened the
// viewer before would keep their old arrangement and never see the change. It has been bumped twice
// for this reason — once for the WS9-049 work page, and once for the tabbed layout — because a
// default nobody with history can see is a default nobody has tested.
export const LAYOUT_KEY = "agni-work-page-dockview-layout-v2";

type LayoutStorage = Pick<Storage, "getItem" | "setItem" | "removeItem">;

// SavedLayout is the persisted shape: the dockview layout plus the panel-registry ids that
// existed at save time. The ids are the layout's "version marker" — self-updating, since
// they are derived from VIEWER_PANELS rather than a manually bumped integer — and they let
// the restore tell a NEWLY ADDED panel (absent from panels) apart from a USER-CLOSED one
// (present in panels, absent from the layout). Without this, a saved layout froze the panel
// set: docks added by later features never appeared for anyone with a saved arrangement.
interface SavedLayout {
  v: 1;
  panels: string[];
  layout: unknown;
}

// LoadedLayout is what loadLayout hands the restore: the dockview layout and the registry
// at save time (empty for a pre-versioning raw save, which makes every current non-on-demand
// panel count as newly added — a one-time reconcile, after which the save is wrapped).
export interface LoadedLayout {
  layout: unknown;
  savedPanels: string[];
}

// loadLayout returns the saved layout, or null when there is none or it does not parse; the
// caller falls back to the default layout in both cases. Both persisted shapes load: the
// versioned wrapper and the bare dockview JSON that predates it.
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

// saveLayout persists the layout stamped with the current panel registry; a storage failure
// (quota, private mode) must not take the viewer down, so it is swallowed.
export function saveLayout(storage: LayoutStorage, layout: unknown): void {
  try {
    const wrapped: SavedLayout = { v: 1, panels: VIEWER_PANELS.map((p) => p.id), layout };
    storage.setItem(LAYOUT_KEY, JSON.stringify(wrapped));
  } catch {
    // best-effort persistence
  }
}

// reconcilePanels runs after a saved layout is restored: a registered panel that is
// default-open, not currently open, and NOT part of the registry the layout was saved under
// is a core panel some feature added since — open it (at openPanel's default position; the
// user drags it where they want and persistence keeps it). Panels the user closed stay
// closed (they were in the saved registry), and a newly-registered SECONDARY panel is left
// closed too (WS9-042): it appears in the Panels menu without auto-opening, so a new feature
// panel does not re-crowd an existing user's layout.
export function reconcilePanels(api: DockviewApi, savedPanels: string[]): void {
  for (const def of VIEWER_PANELS) {
    if (!def.defaultOpen || api.getPanel(def.id) || savedPanels.includes(def.id)) continue;
    openPanel(api, def);
  }
}

// prunePanels closes panels a restored layout names that the registry no longer has. It is the
// counterpart to reconcilePanels, which only ever ADDS: without it a removed panel restores as an
// empty tab, because adoptPanel falls back to a blank element when the hole is gone (below). The
// user's arrangement of the surviving panels is untouched, so this is a prune, not a reset.
export function prunePanels(api: DockviewApi): void {
  const known = new Set(VIEWER_PANELS.map((p) => p.id));
  // Snapshot first: removePanel mutates the collection being walked.
  for (const panel of [...(api.panels ?? [])]) {
    if (!known.has(panel.id)) api.removePanel(panel);
  }
}

// adoptPanel is the dockview content renderer for a panel: it adopts the server-rendered
// hole (found by data-dock-panel wherever it currently is — the park on first open, the park
// again after a close) and parks it on dispose. Element identity is preserved across
// close/reopen so the island mounted in the hole keeps working. An unknown name yields an
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

// RAIL_FRACTION / QUERY_FRACTION size the boot layout: two 15% side rails around a centre column
// whose bottom fifth is the query surface. Fractions rather than pixels because the rails hold
// tab strips whose labels have to stay readable on a laptop and not become a canyon on a desktop.
const RAIL_FRACTION = 0.15;
const QUERY_FRACTION = 0.2;
const RAIL_FALLBACK_PX = 260;
const QUERY_FALLBACK_PX = 180;
// EAST_MIN_PX keeps the east stacks' two-tab strips readable on a small window: dockview clips a
// strip that does not fit rather than scrolling it, so below roughly 1270px wide, 15% starts eating
// the word "Coverage". Two tabs need ~190px; four needed ~380, which is what made one stack of four
// untenable at this width in the first place.
const EAST_MIN_PX = 190;

// defaultLayout is the boot arrangement, and it opens EVERY panel: four visible surfaces, the rest
// tabbed behind them.
//
// It replaces a "lean" layout that opened four panels and left five in a menu. The lean version was
// answering the wrong question. Crowding is what it avoided, but a panel nobody can find is worse
// than a crowded screen, and a first-time reader was landing on a schematic with no visible way to
// ask anything about it. Tabs solve the same crowding problem while leaving everything on the board:
// at most four surfaces are visible at once, and every other panel is one labelled click away rather
// than behind a dropdown you have to know exists.
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
// Ordering is load-bearing. Each position is relative to the reference panel's GROUP, so the side
// rails are built before the centre is split: adding query below canvas afterwards divides the
// centre column alone, where doing it first would have put a full-width strip under all three.
export function defaultLayout(api: DockviewApi): void {
  api.addPanel({ id: "canvas", component: "canvas", title: "Canvas" });

  // West rail: the design's own navigation over what one selection is made of.
  api.addPanel({ id: "overview", component: "overview", title: "Sheets", position: { direction: "left", referencePanel: "canvas" } });
  api.addPanel({ id: "details", component: "details", title: "Details", position: { direction: "below", referencePanel: "overview" } });
  api.addPanel({ id: "parts", component: "parts", title: "Parts", position: { direction: "within", referencePanel: "details" } });

  // East rail, split north/south the way the west rail is. Four tabs in one stack does not fit a 15%
  // column: dockview clips a tab strip rather than scrolling it or offering an overflow menu, so the
  // fourth tab was not cramped, it was unreachable except through the Panels menu (measured: those
  // four labels need ~380px, which is 30% of a 1280px laptop). Two stacks of two fit, and the pairs
  // are the honest split anyway — what the engine found, then what a person asked for.
  api.addPanel({ id: "checks", component: "checks", title: "Checks", position: { direction: "right", referencePanel: "canvas" } });
  api.addPanel({ id: "rules", component: "rules", title: "Rules", position: { direction: "within", referencePanel: "checks" } });
  api.addPanel({ id: "review", component: "review", title: "Review", position: { direction: "below", referencePanel: "checks" } });
  api.addPanel({ id: "coverage", component: "coverage", title: "Coverage", position: { direction: "within", referencePanel: "review" } });

  // The centre column's bottom fifth, after both rails exist so it splits the centre alone.
  api.addPanel({ id: "query", component: "query", title: "Query", position: { direction: "below", referencePanel: "canvas" } });

  // Diff and Changes ride as tabs beside the canvas rather than opening on demand. Both render an
  // empty state naming the Compare button, so a reader who clicks one learns the feature exists
  // instead of meeting a blank pane.
  api.addPanel({ id: "diff", component: "diff", title: "Diff", position: { direction: "within", referencePanel: "canvas" } });
  api.addPanel({ id: "changes", component: "changes", title: "Changes", position: { direction: "within", referencePanel: "canvas" } });

  // Sizes and active tabs only stick once dockview has laid the grid out, so this waits for the grid
  // to HAVE a size rather than guessing a delay. dockview measures itself from a ResizeObserver on
  // its container, which has not fired yet at the end of the current task: sizing on setTimeout(0)
  // divides up a 0x0 grid and is dropped, which is why the previous layout's pixel widths never
  // applied and why a 250ms sleep "fixed" it. Waiting on the measurement is the honest version.
  whenSized(api, () => {
    const rail = api.width ? Math.round(api.width * RAIL_FRACTION) : RAIL_FALLBACK_PX;
    resizeGroup(api, "overview", { width: rail });
    resizeGroup(api, "checks", { width: Math.max(rail, EAST_MIN_PX) });
    resizeGroup(api, "query", { height: api.height ? Math.round(api.height * QUERY_FRACTION) : QUERY_FALLBACK_PX });
    // Each stack opens on its first tab: the drawing, the findings, and what one selection is.
    api.getPanel("details")?.api.setActive();
    api.getPanel("checks")?.api.setActive();
    api.getPanel("review")?.api.setActive();
    api.getPanel("canvas")?.api.setActive();
  });
}

// whenSized runs apply once the dock has a non-zero size, or after a bounded wait if it never does
// (a hidden container, a headless host with no animation frames). Bounded because a layout that
// never sizes must still get its active tabs set.
function whenSized(api: DockviewApi, apply: () => void, framesLeft = 30): void {
  if ((api.width > 0 && api.height > 0) || framesLeft <= 0) {
    apply();
    return;
  }
  const next = (): void => whenSized(api, apply, framesLeft - 1);
  if (typeof requestAnimationFrame === "function") requestAnimationFrame(next);
  else setTimeout(next, 16);
}

// resizeGroup sizes the GROUP a panel sits in, which is the only thing dockview will resize.
//
// `panel.api.setSize()` looks like it does this and does nothing at all: it fires onDidSizeChange,
// and the listener for that event is installed by the GRIDVIEW panel (the group), not by the
// dockview panel inside it. So the call type-checks, runs, and is dropped. Every width the boot
// layout asked for was silently ignored from WS9-021 until this was traced (the rails were three
// equal columns however many pixels the code requested), which is a good reminder that a layout
// assertion in a unit test proves the CALL was made and never that the pixels moved.
function resizeGroup(api: DockviewApi, panelId: string, size: { width?: number; height?: number }): void {
  api.getPanel(panelId)?.api.group?.api.setSize(size);
}

// openDiffPanel focuses the diff view and its changed-item list, adding either back first if the
// user closed it. The default layout already places both beside the canvas, so on an untouched
// layout this is a tab switch; the add path is what covers a layout where they were closed.
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
    // The right column should stay a rail, not half the screen.
    setTimeout(() => api.getPanel("changes")?.api.setSize({ width: 260 }), 0);
  }
  api.getPanel("diff")?.api.setActive();
}

// closeDiffPanel ends a comparison by returning attention to the drawing, and deliberately LEAVES
// the diff and changes tabs in place. They used to be removed, which was right while they were
// opened on demand and wrong now that the default layout places them: a tab strip that loses two
// tabs when you close a comparison reads as the app breaking rather than as a mode ending. Both
// panels render "No comparison open" on their own, so what stays behind explains itself.
export function closeDiffPanel(api: DockviewApi): void {
  api.getPanel("canvas")?.api.setActive();
}

// openPanel re-adds a closed panel from the menu, on the right edge; the user drags it where they
// want and persistence keeps it. (Files used to be special-cased to the left; it is gone.)
export function openPanel(api: DockviewApi, def: DockPanelDef): void {
  api.addPanel({
    id: def.id,
    component: def.id,
    title: def.title,
    position: { direction: "right" },
  });
}

// panelsMenu renders the "Panels" dropdown in the top bar: one entry per registered panel, a
// ✓ marking the open ones. Each item is a show/hide TOGGLE (WS9-042) — clicking an open panel
// closes it, a closed panel opens it — so the menu is the discoverable way to both re-open a
// panel the tab-× closed AND hide one, not a one-way reopen. Panel layout is dock chrome, not
// presenter state, so this stays a plain DOM widget rather than an island.
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

// createViewerDock boots the dock: restore the saved layout when there is one (falling back
// to the default when it fails to apply, e.g. a corrupt or incompatible save), then persist
// every layout change. The page is light-only, so the theme class is fixed (no theme
// observer, unlike the reference implementation).
export function createViewerDock(container: HTMLElement, park: HTMLElement, menuHost: HTMLElement, storage: LayoutStorage): DockviewApi {
  container.classList.add("dockview-theme-light");
  const component = new DockviewComponent(container, {
    createComponent: (options) => adoptPanel(park, options.name),
    // "always" keeps every panel's content element attached (hidden) when its tab is not in
    // front. The default ("onlyWhenVisible") DETACHES hidden tab content, which breaks the
    // adopt contract: AppRoot resolves every island hole by id at boot, so a hole sitting in
    // a background tab (the default layout's Report-behind-Checks, or any saved layout where
    // the user tabbed panels together) would be missing and no island would mount.
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
