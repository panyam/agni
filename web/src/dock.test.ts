// @vitest-environment jsdom
import { afterEach, beforeAll, describe, expect, it, vi } from "vitest";
import type { DockviewApi } from "dockview-core";
import { DRAWER_PANELS, LAYOUT_KEY, VIEWER_PANELS, closeDiffPanel, createViewerDock, defaultLayout, openDiffPanel, queryHeight, type ViewerDock } from "./dock.js";

// The dock is goapplib's tsappkit-dock (agni issue 1549), which tests its own restore, reconcile,
// prune, reset and Panels menu. These cover what agni gives it: the registry and where each panel
// starts, the grid's default arrangement, and the viewer's drawers, buttons and sheet views (agni
// issue 1542).

beforeAll(() => {
  // jsdom has no ResizeObserver, and dockview-core watches its container with one.
  globalThis.ResizeObserver ??= class {
    observe(): void {}
    unobserve(): void {}
    disconnect(): void {}
  } as unknown as typeof ResizeObserver;
});

describe("panel registry", () => {
  it("lists every viewer panel once, with no retired one", () => {
    const ids = VIEWER_PANELS.map((p) => p.id);
    expect(new Set(ids).size).toBe(ids.length);
    expect([...ids].sort()).toEqual(
      ["canvas", "changes", "checks", "coverage", "diff", "lexicon", "overview", "parts", "properties", "query", "review", "rules", "tables", "trace"].sort(),
    );
    expect(ids).not.toContain("files");
    expect(ids).not.toContain("details");
  });

  it("starts Sheets in the left drawer, and the engine's findings and their references in the right one", () => {
    const at = (id: string) => VIEWER_PANELS.find((p) => p.id === id)?.defaultPlacement;
    expect(at("overview")).toEqual({ kind: "drawer", edge: "left" });
    for (const id of ["checks", "review", "rules", "lexicon", "coverage"]) expect(at(id)).toEqual({ kind: "drawer", edge: "right" });
    for (const id of ["canvas", "properties", "parts", "query", "trace", "tables", "diff", "changes"]) expect(at(id)).toBeUndefined();
    // Checks and Review open first in the right drawer, since a drawer's tabs follow the registry.
    const right = VIEWER_PANELS.filter((p) => p.defaultPlacement?.kind === "drawer" && p.defaultPlacement.edge === "right").map((p) => p.id);
    expect(right.slice(0, 2)).toEqual(["checks", "review"]);
    expect([...DRAWER_PANELS]).toEqual(["overview", "checks", "review"]);
  });

  it("saves under a key the old dock never used, so every reader starts on the drawers once", () => {
    expect(LAYOUT_KEY).not.toBe("agni-work-page-dockview-layout-v3");
  });
});

describe("defaultLayout", () => {
  function fakeApi() {
    const added: { id: string; direction?: string; ref?: string }[] = [];
    const sizes: Record<string, { width?: number; height?: number }> = {};
    const active: string[] = [];
    const panel = (id: string) => ({
      api: { setActive: () => active.push(id), group: { api: { setSize: (s: { width?: number; height?: number }) => (sizes[id] = s) } } },
    });
    const api = {
      width: 1400,
      height: 1000,
      addPanel: (o: { id: string; position?: { direction: string; referencePanel?: string } }) =>
        added.push({ id: o.id, direction: o.position?.direction, ref: o.position?.referencePanel }),
      getPanel: (id: string) => (added.some((a) => a.id === id) ? panel(id) : undefined),
    } as unknown as DockviewApi;
    return { api, added, sizes, active };
  }

  it("builds the west rail, then splits the centre for the query strip, with no drawer panel in the grid", () => {
    const { api, added } = fakeApi();
    defaultLayout(api);
    expect(added).toEqual([
      { id: "canvas", direction: undefined, ref: undefined },
      { id: "properties", direction: "left", ref: "canvas" },
      { id: "parts", direction: "within", ref: "properties" },
      { id: "query", direction: "below", ref: "canvas" },
      { id: "trace", direction: "within", ref: "query" },
      { id: "tables", direction: "within", ref: "query" },
      { id: "diff", direction: "within", ref: "canvas" },
      { id: "changes", direction: "within", ref: "canvas" },
    ]);
  });

  it("opens each stack on its first tab at once, and sizes the rail and the query strip once laid out", () => {
    const { api, sizes, active } = fakeApi();
    let later: (() => void) | null = null;
    defaultLayout(api, (apply) => (later = apply));
    // A page that isn't in front gets no animation frames, so the tabs can't wait on the sizing.
    expect(active).toEqual(["query", "properties", "canvas"]);
    expect(sizes).toEqual({});
    later!();
    expect(sizes.properties).toEqual({ width: 260 });
    expect(sizes.query).toEqual({ height: 320 });
  });
});

describe("queryHeight", () => {
  it("gives a tall dock its fifth, a 1000px dock the floor, and a short one the cap", () => {
    expect(queryHeight(2400)).toBe(480);
    expect(queryHeight(1000)).toBe(320);
    expect(queryHeight(500)).toBe(225);
    expect(queryHeight(0)).toBe(320);
  });
});

describe("the viewer's dock", () => {
  let made: ViewerDock | null = null;
  afterEach(() => {
    made?.dock.dispose();
    made = null;
    document.body.replaceChildren();
  });

  function boot() {
    const container = document.createElement("div");
    const park = document.createElement("div");
    const menu = document.createElement("div");
    for (const p of VIEWER_PANELS) {
      const hole = document.createElement("div");
      hole.dataset.dockPanel = p.id;
      hole.textContent = p.id;
      park.appendChild(hole);
    }
    document.body.append(container, park, menu);
    made = createViewerDock(container, park, menu, null);
    return { v: made, container, park, menu };
  }

  it("puts the server-rendered holes in the grid and the drawers, the same element each time", () => {
    const { v, park } = boot();
    const canvasHole = document.querySelector('[data-dock-panel="canvas"]');
    expect(canvasHole?.parentElement).not.toBe(park);
    expect(v.dock.where("overview")).toEqual({ kind: "drawer", edge: "left" });
    expect(v.dock.where("checks")).toEqual({ kind: "drawer", edge: "right" });
    expect(v.dock.where("canvas")).toEqual({ kind: "grid" });
    v.dock.place("checks", { kind: "grid" });
    expect(document.querySelectorAll('[data-dock-panel="checks"]').length).toBe(1);
    expect(v.dock.where("checks")).toEqual({ kind: "grid" });
  });

  it("adds a drawer button for Sheets, Checks and Review beside the Panels menu, which opens and closes its drawer", () => {
    const { v, menu } = boot();
    const buttons = [...menu.querySelectorAll<HTMLButtonElement>(".dock-drawer-btn")];
    expect(buttons.map((b) => b.textContent)).toEqual(["Sheets", "Checks", "Review"]);
    expect(menu.querySelector(".dock-panels-btn")).not.toBeNull();
    const toggled = vi.spyOn(v.dock, "toggleDrawer");
    buttons[0].click();
    expect(buttons[0].classList.contains("active")).toBe(true);
    buttons[0].click();
    expect(toggled).toHaveBeenLastCalledWith("left", false);
    expect(buttons[0].classList.contains("active")).toBe(false);
  });

  it("closes a panel's drawer for a chosen sheet, and leaves a grid panel alone", () => {
    const { v } = boot();
    v.dock.show("overview");
    const toggled = vi.spyOn(v.dock, "toggleDrawer");
    v.hideDrawerOf("overview");
    expect(toggled).toHaveBeenLastCalledWith("left", false);
    toggled.mockClear();
    v.hideDrawerOf("canvas");
    expect(toggled).not.toHaveBeenCalled();
  });

  it("reveals a grid panel with both drawers closed, and a drawer panel in its open drawer", () => {
    const { v } = boot();
    v.dock.show("checks");
    const toggled = vi.spyOn(v.dock, "toggleDrawer");
    expect(v.reveal("query")).toBe(true);
    expect(toggled.mock.calls).toEqual([
      ["left", false],
      ["right", false],
    ]);
    toggled.mockClear();
    expect(v.reveal("checks")).toBe(true);
    expect(toggled).not.toHaveBeenCalled();
    expect(v.reveal("no-such-panel")).toBe(false);
  });

  it("opens a comparison with Changes in a rail beside Diff, and tabs it back with Diff once closed", () => {
    const { v } = boot();
    const group = (id: string) => v.dock.api.getPanel(id)?.group;
    expect(group("changes"), "the default layout tabs Changes with Diff").toBe(group("diff"));
    openDiffPanel(v.dock);
    expect(group("changes")).not.toBe(group("diff"));
    expect(group("changes")?.panels.map((p) => p.id)).toEqual(["changes"]);
    expect(group("diff")?.activePanel?.id).toBe("diff");
    expect(group("changes")?.activePanel?.id).toBe("changes");
    closeDiffPanel(v.dock);
    expect(group("changes")).toBe(group("diff"));
    expect(group("canvas")?.activePanel?.id).toBe("canvas");
  });

  it("leaves a Changes the reader moved where they put it", () => {
    const { v } = boot();
    v.dock.place("changes", { kind: "drawer", edge: "right" });
    openDiffPanel(v.dock);
    expect(v.dock.where("changes")).toEqual({ kind: "drawer", edge: "right" });
    closeDiffPanel(v.dock);
    expect(v.dock.where("changes")).toEqual({ kind: "drawer", edge: "right" });
  });

  it("docks a sheet view beside the canvas, tabs the next one with it, and says when one closes", () => {
    const { v } = boot();
    const closed: string[] = [];
    v.addSheetView("sheetview-1", "Power", document.createElement("div"), () => closed.push("sheetview-1"));
    v.addSheetView("sheetview-2", "Board", document.createElement("div"), () => closed.push("sheetview-2"));
    expect(v.dock.where("sheetview-1")).toEqual({ kind: "grid" });
    expect(v.dock.api.getPanel("sheetview-1")?.group.id).toBe(v.dock.api.getPanel("sheetview-2")?.group.id);
    expect(v.dock.api.getPanel("sheetview-1")?.group.id).not.toBe(v.dock.api.getPanel("canvas")?.group.id);
    v.setTitle("sheetview-2", "Board, bottom");
    expect(v.dock.api.getPanel("sheetview-2")?.title).toBe("Board, bottom");
    v.dock.toggle("sheetview-1");
    expect(closed).toEqual(["sheetview-1"]);
    expect(v.dock.where("sheetview-1")).toBeNull();
  });
});
