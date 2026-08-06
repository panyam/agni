// @vitest-environment jsdom
import { describe, it, expect, vi } from "vitest";
import { closeTab, emptyTabStrip, sheetTabsIsland, visitSheet, type TabStripState } from "./sheettabs.jsx";
import type { SheetRef } from "./gen/agni/v1/webapi/design_pb.js";
import type { SheetsState } from "./sheets.js";

function sheet(id: string, name: string): SheetRef {
  return { id, name } as SheetRef;
}

const DESIGN: SheetRef[] = [sheet("root", "Root"), sheet("amp1", "Amp 1"), sheet("amp2", "Amp 2")];

function push(mount: string, path: string, activeId: string, sheets = DESIGN): SheetsState {
  return { mount, path, sheets, activeId };
}

function strip(...activeIds: string[]): TabStripState {
  return activeIds.reduce((acc, id) => visitSheet(acc, push("m", "b.kicad_sch", id)), emptyTabStrip());
}

describe("visitSheet", () => {
  it("starts empty and adds a tab only for the sheet actually shown", () => {
    const s = visitSheet(emptyTabStrip(), push("m", "b.kicad_sch", "amp1"));
    expect(s.tabs).toEqual([{ id: "amp1", name: "Amp 1" }]);
    expect(s.activeId).toBe("amp1");
  });

  it("does not mirror the design's sheet list", () => {
    // The whole point of the strip: a three-sheet design opens with ONE tab, not three.
    expect(strip("root").tabs).toHaveLength(1);
  });

  it("accumulates visited sheets in visit order", () => {
    expect(strip("root", "amp2", "amp1").tabs.map((t) => t.id)).toEqual(["root", "amp2", "amp1"]);
  });

  it("keeps a revisited tab in place instead of reordering the strip", () => {
    const s = strip("root", "amp1", "amp2", "root");
    expect(s.tabs.map((t) => t.id)).toEqual(["root", "amp1", "amp2"]);
    expect(s.activeId).toBe("root");
  });

  it("clears the strip when the design changes", () => {
    const s = visitSheet(strip("root", "amp1"), push("m", "other.kicad_sch", "root"));
    expect(s.tabs).toEqual([{ id: "root", name: "Root" }]);
    expect(s.path).toBe("other.kicad_sch");
  });

  it("adds nothing while a design has no active sheet yet", () => {
    const s = visitSheet(emptyTabStrip(), push("m", "b.kicad_sch", "", DESIGN));
    expect(s.tabs).toEqual([]);
    expect(s.activeId).toBe("");
  });

  it("reports an active sheet the design does not list without tabbing it", () => {
    const s = visitSheet(emptyTabStrip(), push("m", "b.kicad_sch", "ghost"));
    expect(s.tabs).toEqual([]);
    expect(s.activeId).toBe("ghost");
  });

  it("falls back to the sheet id when the design gives it no name", () => {
    const s = visitSheet(emptyTabStrip(), push("m", "b.kicad_sch", "s1", [sheet("s1", "")]));
    expect(s.tabs).toEqual([{ id: "s1", name: "s1" }]);
  });
});

describe("closeTab", () => {
  it("closing an inactive tab leaves the shown sheet alone", () => {
    const { next, select } = closeTab(strip("root", "amp1"), "root");
    expect(next.tabs.map((t) => t.id)).toEqual(["amp1"]);
    expect(next.activeId).toBe("amp1");
    expect(select).toBe("");
  });

  it("closing the active tab selects the neighbour that slides into its place", () => {
    const { next, select } = closeTab(strip("root", "amp1", "amp2", "amp1"), "amp1");
    expect(next.tabs.map((t) => t.id)).toEqual(["root", "amp2"]);
    expect(select).toBe("amp2");
    expect(next.activeId).toBe("amp2");
  });

  it("closing the active RIGHTMOST tab falls back to the new last tab", () => {
    const { select } = closeTab(strip("root", "amp1"), "amp1");
    expect(select).toBe("root");
  });

  it("ignores a tab that is not in the strip", () => {
    const before = strip("root");
    expect(closeTab(before, "nope")).toEqual({ next: before, select: "" });
  });
});

function mount() {
  const onSelect = vi.fn();
  const el = document.createElement("div");
  document.body.appendChild(el);
  const tabs = sheetTabsIsland(el, null, { onSelect });
  tabs.island.activate();
  return { el, onSelect, view: tabs.view };
}

describe("sheetTabsIsland", () => {
  it("renders one tab per visited sheet and marks the active one", () => {
    const { el, view } = mount();
    view.setState(push("m", "b.kicad_sch", "root"));
    view.setState(push("m", "b.kicad_sch", "amp1"));
    expect(el.querySelectorAll(".sheet-tab")).toHaveLength(2);
    expect(el.querySelector(".sheet-tab.active")?.textContent).toContain("Amp 1");
    document.body.replaceChildren();
  });

  it("emits a select intent for a background tab, and none for the active one", () => {
    const { el, onSelect, view } = mount();
    view.setState(push("m", "b.kicad_sch", "root"));
    view.setState(push("m", "b.kicad_sch", "amp1"));
    const labels = el.querySelectorAll<HTMLButtonElement>(".sheet-tab-label");
    labels[0].click();
    expect(onSelect).toHaveBeenCalledWith("root");
    onSelect.mockClear();
    labels[1].click(); // already active
    expect(onSelect).not.toHaveBeenCalled();
    document.body.replaceChildren();
  });

  it("hides the close affordance while one tab remains", () => {
    const { el, view } = mount();
    view.setState(push("m", "b.kicad_sch", "root"));
    expect(el.querySelectorAll(".sheet-tab-close")).toHaveLength(0);
    view.setState(push("m", "b.kicad_sch", "amp1"));
    expect(el.querySelectorAll(".sheet-tab-close")).toHaveLength(2);
    document.body.replaceChildren();
  });

  it("closing the active tab drops it and asks the presenter for the neighbour", () => {
    const { el, onSelect, view } = mount();
    view.setState(push("m", "b.kicad_sch", "root"));
    view.setState(push("m", "b.kicad_sch", "amp1"));
    el.querySelectorAll<HTMLButtonElement>(".sheet-tab-close")[1].click();
    expect(el.querySelectorAll(".sheet-tab")).toHaveLength(1);
    expect(onSelect).toHaveBeenCalledWith("root");
    document.body.replaceChildren();
  });
});
