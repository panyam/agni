// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach } from "vitest";
import { sheetOverviewPanelIsland } from "./sheetoverviewpanel.jsx";
import type { OverviewState, SheetTile } from "./sheetoverview.js";

function mkTile(t: Pick<SheetTile, "id" | "name" | "count" | "unresolved"> & Partial<SheetTile>): SheetTile {
  return { parentId: "", local: 0, ...t };
}

function mountPanel() {
  const onSelect = vi.fn();
  const el = document.createElement("div");
  document.body.appendChild(el);
  const { island, view } = sheetOverviewPanelIsland(el, null, { onSelect });
  island.activate();
  return { el, view, onSelect };
}

function state(over: Partial<OverviewState>): OverviewState {
  return {
    tiles: [
      mkTile({ id: "s1", name: "Root", count: 3, unresolved: 0 }),
      mkTile({ id: "s2", name: "Power", count: 0, unresolved: 0 }),
    ],
    activeId: "s1",
    ruleCount: 2,
    ranCount: 2,
    ...over,
  };
}

beforeEach(() => document.body.replaceChildren());

describe("sheetOverviewPanelIsland", () => {
  it("renders a tile per sheet with a firing badge, nothing on a clean sheet, marking the shown sheet", () => {
    const { el, view } = mountPanel();
    view.setState(state({}));
    const tiles = [...el.querySelectorAll(".sheet-tile")];
    expect(tiles.map((t) => t.querySelector(".sheet-tile-name")?.textContent)).toEqual(["Root", "Power"]);
    expect(tiles[0].querySelector(".sheet-tile-count.firing")?.textContent).toBe("3");
    expect(tiles[1].querySelector(".sheet-tile-count")).toBeNull();
    expect(tiles[0].classList.contains("active")).toBe(true);
    expect(tiles[1].classList.contains("active")).toBe(false);
  });

  it("shows a dash instead of a misleading 0 when no rules are selected", () => {
    const { el, view } = mountPanel();
    view.setState(state({ ruleCount: 0 }));
    expect(el.querySelectorAll(".sheet-tile-count.norules").length).toBe(2);
    expect(el.querySelectorAll(".sheet-tile-count:not(.norules)").length).toBe(0);
  });

  // agni issue 1132: before Run checks every sheet badged a green 0, a clean claim over rules that never ran.
  it("draws no count before any selected rule has run", () => {
    const { el, view } = mountPanel();
    view.setState(state({ ranCount: 0 }));
    expect(el.querySelectorAll(".sheet-tile").length).toBe(2);
    expect(el.querySelectorAll(".sheet-tile-count").length).toBe(0);
  });

  it("emits the sheet id on tile click and hints when no design is open", () => {
    const { el, view, onSelect } = mountPanel();
    expect(el.textContent).toContain("No design open");
    view.setState(state({}));
    (el.querySelectorAll(".sheet-tile")[1] as HTMLButtonElement).click();
    expect(onSelect).toHaveBeenCalledWith("s2");
  });
});

// agni issue 350: the tile used to badge an inconclusive result red beside the defects, and a sheet
// whose findings were all undecided badged a green 0.
describe("sheet tiles and inconclusive results", () => {
  it("counts them beside the defects rather than in them", () => {
    const { el, view } = mountPanel();
    view.setState(state({ tiles: [mkTile({ id: "s1", name: "Root", count: 2, unresolved: 3 })] }));
    const tile = el.querySelector(".sheet-tile")!;
    expect(tile.querySelector(".sheet-tile-count.firing")?.textContent).toBe("2");
    expect(tile.querySelector(".sheet-tile-count.unresolved")?.textContent).toBe("3?");
  });

  it("does not call a sheet clean when its only findings are undecided", () => {
    const { el, view } = mountPanel();
    view.setState(state({ tiles: [mkTile({ id: "s1", name: "Root", count: 0, unresolved: 4 })] }));
    const tile = el.querySelector(".sheet-tile")!;
    expect(tile.querySelector(".sheet-tile-count.firing")).toBeNull();
    expect([...tile.querySelectorAll(".sheet-tile-count")].map((b) => b.textContent)).toEqual(["4?"]);
  });
});

// agni issue 1141: a hierarchical design nests in the dock as it does in the file tree.
describe("a hierarchical design in the Sheets dock", () => {
  const tiles = [
    mkTile({ id: "/", name: "Root", count: 0, unresolved: 0 }),
    mkTile({ id: "/Power", name: "Power", parentId: "/", count: 0, unresolved: 0 }),
    mkTile({ id: "/Power/1", name: "Power/1", parentId: "/Power", count: 2, unresolved: 0 }),
    mkTile({ id: "/IO", name: "IO", parentId: "/", count: 0, unresolved: 0 }),
    mkTile({ id: "/IO/1", name: "IO/1", parentId: "/IO", count: 0, unresolved: 0 }),
  ];
  const names = (el: HTMLElement) => [...el.querySelectorAll(".sheet-tile-name")].map((n) => n.textContent);
  const depthOf = (el: HTMLElement, name: string) => {
    let d = 0;
    let at = [...el.querySelectorAll(".sheet-tile-name")].find((n) => n.textContent === name)!.closest("ul");
    while (at && at !== el.firstElementChild) {
      d++;
      at = at.parentElement!.closest("ul");
    }
    return d;
  };

  it("nests each sheet under its parent and opens the sheet a nested tile names", () => {
    const { el, view, onSelect } = mountPanel();
    view.setState(state({ tiles, activeId: "/" }));
    expect(names(el)).toEqual(["Root", "Power", "Power/1", "IO", "IO/1"]);
    expect(["Root", "Power", "Power/1"].map((n) => depthOf(el, n))).toEqual([0, 1, 2]);
    [...el.querySelectorAll<HTMLButtonElement>(".sheet-tile")].find((b) => b.textContent?.startsWith("IO/1"))!.click();
    expect(onSelect).toHaveBeenCalledWith("/IO/1");
  });

  it("folds a parent's sheets away and back", () => {
    const { el, view } = mountPanel();
    view.setState(state({ tiles, activeId: "/" }));
    const power = [...el.querySelectorAll(".sheet-tile-row")].find((r) => r.textContent?.includes("Power") && !r.textContent.includes("Power/1"))!;
    power.querySelector<HTMLButtonElement>("button.sheet-tile-twist")!.click();
    expect(names(el)).toEqual(["Root", "Power", "IO", "IO/1"]);
    power.querySelector<HTMLButtonElement>("button.sheet-tile-twist")!.click();
    expect(names(el)).toContain("Power/1");
  });

  it("shows a sheet's full id on hover where its label differs, and keeps a flat design free of fold arrows", () => {
    const { el, view } = mountPanel();
    view.setState(state({ tiles, activeId: "/" }));
    const io1 = [...el.querySelectorAll<HTMLButtonElement>(".sheet-tile")].find((b) => b.textContent?.startsWith("IO/1"))!;
    expect(io1.title).toBe("/IO/1");
    view.setState(state({}));
    expect(el.querySelectorAll(".sheet-tile-twist")).toHaveLength(0);
  });
});

// agni issue 1142: the pills say what they count.
describe("what the count pills mean", () => {
  it("says on the red pill how many defects are drawn on the sheet alone", () => {
    const { el, view } = mountPanel();
    view.setState(state({ tiles: [mkTile({ id: "s1", name: "1", count: 5, unresolved: 0, local: 2 })] }));
    expect(el.querySelector(".sheet-tile-count.firing")!.getAttribute("title")).toBe("5 defects on this sheet, 2 drawn here alone and 3 also on other sheets");
  });

  it("explains both pills in a legend once a run has counted, and not before", () => {
    const { el, view } = mountPanel();
    view.setState(state({ ranCount: 0 }));
    expect(el.querySelector(".sheet-tiles-legend")).toBeNull();
    view.setState(state({}));
    const legend = el.querySelector(".sheet-tiles-legend")!.textContent!;
    expect(legend).toContain("defects");
    expect(legend).toContain("inconclusive");
    expect(legend).toContain("counts on each");
  });
});

// A sheet opens in a view of its own only when the reader asks (agni issue 1326).
describe("opening a sheet beside the canvas", () => {
  it("offers each tile's sheet in a new view when the page can open one", () => {
    const onSelect = vi.fn();
    const onOpenBeside = vi.fn();
    const el = document.createElement("div");
    document.body.appendChild(el);
    const { island, view } = sheetOverviewPanelIsland(el, null, { onSelect, onOpenBeside });
    island.activate();
    view.setState(state({}));
    const beside = [...el.querySelectorAll(".sheet-tile-beside")] as HTMLButtonElement[];
    expect(beside.length).toBe(2);
    beside[1].click();
    expect(onOpenBeside).toHaveBeenCalledWith("s2");
    expect(onSelect).not.toHaveBeenCalled();
  });

  it("offers nothing when the page cannot open a view", () => {
    const { el, view } = mountPanel();
    view.setState(state({}));
    expect(el.querySelectorAll(".sheet-tile-beside").length).toBe(0);
  });
});

// agni 1351: the Board tile lists the board's layers while the board is shown.
describe("the Board tile's layers", () => {
  const rows = [
    { name: "F.Cu", kind: "copper", side: "front", color: "#c83434", svgSelector: "", groups: [] },
    { name: "In1.Cu", kind: "copper", side: "inner", color: "#8a6d3b", svgSelector: "", groups: [] },
  ] as unknown as OverviewState["boardLayers"];
  function mountLayers() {
    const layers = { onToggle: vi.fn(), onSolo: vi.fn(), onPreset: vi.fn() };
    const el = document.createElement("div");
    document.body.appendChild(el);
    const { island, view } = sheetOverviewPanelIsland(el, null, { onSelect: vi.fn(), layers });
    island.activate();
    const tiles = [mkTile({ id: "s1", name: "Root", count: 0, unresolved: 0 }), mkTile({ id: "board", name: "Board", count: 0, unresolved: 0 })];
    return { el, view, layers, tiles };
  }

  it("shows a checkbox per layer under the Board tile, unchecked when hidden", () => {
    const { el, view, tiles } = mountLayers();
    view.setState(state({ tiles, activeId: "board", boardLayers: rows, hiddenLayers: ["In1.Cu"] }));
    const boxes = [...el.querySelectorAll<HTMLInputElement>(".board-layer-row input")];
    expect([...el.querySelectorAll(".board-layer-name")].map((n) => n.textContent)).toEqual(["F.Cu", "In1.Cu"]);
    expect(boxes.map((b) => b.checked)).toEqual([true, false]);
  });

  it("lists no layers while a schematic sheet is shown", () => {
    const { el, view, tiles } = mountLayers();
    view.setState(state({ tiles, activeId: "s1", boardLayers: rows, hiddenLayers: [] }));
    expect(el.querySelector(".board-layers")).toBeNull();
  });

  it("toggles on a click, shows one alone on a double-click, and applies a preset", () => {
    const { el, view, layers, tiles } = mountLayers();
    view.setState(state({ tiles, activeId: "board", boardLayers: rows, hiddenLayers: [] }));
    el.querySelectorAll<HTMLInputElement>(".board-layer-row input")[1].click();
    expect(layers.onToggle).toHaveBeenCalledWith("In1.Cu");
    // Two presses on one row show it alone, even when the row is drawn again between them.
    el.querySelectorAll(".board-layer-row")[0].dispatchEvent(new MouseEvent("mousedown", { bubbles: true }));
    view.setState(state({ tiles, activeId: "board", boardLayers: rows, hiddenLayers: ["F.Cu"] }));
    expect(layers.onSolo).not.toHaveBeenCalled();
    el.querySelectorAll(".board-layer-row")[0].dispatchEvent(new MouseEvent("mousedown", { bubbles: true }));
    expect(layers.onSolo).toHaveBeenCalledWith("F.Cu");
    // A press on another row starts again.
    el.querySelectorAll(".board-layer-row")[1].dispatchEvent(new MouseEvent("mousedown", { bubbles: true }));
    expect(layers.onSolo).toHaveBeenCalledTimes(1);
    const back = [...el.querySelectorAll<HTMLButtonElement>(".board-layer-preset")].find((b) => b.textContent === "Back");
    back!.click();
    expect(layers.onPreset).toHaveBeenCalledWith("back");
  });
});
