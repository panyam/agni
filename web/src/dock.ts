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
  // onDemand panels are opened by a feature flow (starting a comparison opens diff/changes),
  // never by the default layout or the saved-layout reconcile.
  onDemand?: boolean;
  // defaultOpen panels form the lean boot layout (WS9-042): only these open on a fresh page,
  // so adding a feature panel does not re-crowd the viewer. Secondaries (no flag) stay closed
  // by default and are opened from the Panels menu. The flag also gates the reconcile: a
  // newly-registered secondary appears in the menu WITHOUT auto-opening for existing users.
  // onDemand always wins — an onDemand panel is never a default-layout panel.
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
  // Rules is a static reference catalog — secondary, opened from the menu when wanted.
  { id: "rules", title: "Rules" },
  // Checks is the merged findings+report panel (WS9): a server-sourced, client-grouped/sorted table
  // with the on-demand Run button. The separate Report panel was folded in (severity is one grouping).
  { id: "checks", title: "Checks", defaultOpen: true },
  // The datalog query panel (WS9-036): ad-hoc search over the fact base. Secondary — opened
  // from the menu; existing saved layouts keep whatever the user arranged.
  { id: "query", title: "Query" },
  // The interface-coverage panel (WS9-041): per-interface signal matrix. Secondary — opened from
  // the menu, or via the reconcile for existing saved layouts.
  { id: "coverage", title: "Coverage" },
  // The datasheet-params panel (WS9-035): per-component parameter tree for datasheet-backed parts.
  // Secondary — opened from the menu; only populated when serve was started with --params.
  { id: "parts", title: "Parts" },
  // The review panel (WS9-052): the project's checklist verdict for the open design, over the stored
  // runs (WS9-053). Secondary — opened from the menu, or via the reconcile for existing saved
  // layouts; only useful when serve was started with --review-store.
  { id: "review", title: "Review" },
  // Diff (WS9-005) and its changed-item list (WS9-006) are registered so their holes are
  // adoptable and the menu can reopen them, but they are not part of the default layout —
  // starting a comparison opens both (openDiffPanel).
  { id: "diff", title: "Diff", onDemand: true },
  { id: "changes", title: "Changes", onDemand: true },
];

// LAYOUT_KEY changed with the WS9-049 work page. The saved-layout mechanism reconciles panels that
// were ADDED since a save, never ones that were removed or demoted, so an existing save would have
// restored the old tree-on-the-left arrangement and hidden the whole change. A new key retires
// every pre-split save at once, which is cheaper and more predictable than migrating them.
export const LAYOUT_KEY = "agni-work-page-dockview-layout";

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

// defaultLayout is the lean boot layout (WS9-042, narrowed by WS9-049): only the default-open
// (core) panels open — Sheets on the left (260px), canvas in the center, and a 300px right column
// stacking details and checks. Sheets holds the left rail alone: the work page opens exactly one
// design, so the design's own sheet hierarchy is the navigation that belongs there. The secondaries
// (rules, query, coverage, parts) stay closed and are opened from the Panels menu. Their holes
// still mount (parked, hidden), so opening one is instant.
export function defaultLayout(api: DockviewApi): void {
  api.addPanel({ id: "canvas", component: "canvas", title: "Canvas" });
  api.addPanel({ id: "overview", component: "overview", title: "Sheets", position: { direction: "left", referencePanel: "canvas" } });
  api.addPanel({ id: "details", component: "details", title: "Details", position: { direction: "right", referencePanel: "canvas" } });
  api.addPanel({ id: "checks", component: "checks", title: "Checks", position: { direction: "below", referencePanel: "details" } });
  // Column widths only stick once dockview has laid the grid out, hence the deferred set
  // (same pattern as the reference implementation).
  setTimeout(() => {
    api.getPanel("overview")?.api.setSize({ width: 260 });
    api.getPanel("details")?.api.setSize({ width: 300 });
  }, 0);
}

// openDiffPanel opens (or focuses) the diff view plus its changed-item list. The diff wants
// the big center surface, so it tabs into the canvas panel's group when that exists, else
// falls back to the right edge; the changes list opens beside it on the right.
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

// closeDiffPanel removes the diff view and its changes list (holes park, islands stay
// mounted).
export function closeDiffPanel(api: DockviewApi): void {
  for (const id of ["diff", "changes"]) {
    const panel = api.getPanel(id);
    if (panel) api.removePanel(panel);
  }
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
