// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach } from "vitest";
import { sheetOverviewPanelIsland } from "./sheetoverviewpanel.jsx";
import type { OverviewState } from "./sheetoverview.js";

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
      { id: "s1", name: "Root", count: 3 },
      { id: "s2", name: "Power", count: 0 },
    ],
    activeId: "s1",
    ruleCount: 2,
    ...over,
  };
}

beforeEach(() => document.body.replaceChildren());

describe("sheetOverviewPanelIsland", () => {
  it("renders a tile per sheet with firing and explicit clean badges, marking the shown sheet", () => {
    const { el, view } = mountPanel();
    view.setState(state({}));
    const tiles = [...el.querySelectorAll(".sheet-tile")];
    expect(tiles.map((t) => t.querySelector(".sheet-tile-name")?.textContent)).toEqual(["Root", "Power"]);
    expect(tiles[0].querySelector(".sheet-tile-count.firing")?.textContent).toBe("3");
    expect(tiles[1].querySelector(".sheet-tile-count.clean")?.textContent).toBe("0");
    expect(tiles[0].classList.contains("active")).toBe(true);
    expect(tiles[1].classList.contains("active")).toBe(false);
  });

  it("shows a dash instead of a misleading 0 when no rules are selected", () => {
    const { el, view } = mountPanel();
    view.setState(state({ ruleCount: 0 }));
    expect(el.querySelectorAll(".sheet-tile-count.norules").length).toBe(2);
    expect(el.querySelector(".sheet-tile-count.clean")).toBeNull();
  });

  it("emits the sheet id on tile click and hints when no design is open", () => {
    const { el, view, onSelect } = mountPanel();
    expect(el.textContent).toContain("No design open");
    view.setState(state({}));
    (el.querySelectorAll(".sheet-tile")[1] as HTMLButtonElement).click();
    expect(onSelect).toHaveBeenCalledWith("s2");
  });
});
