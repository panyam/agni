// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach } from "vitest";
import { diffChangesPanelIsland } from "./diffchangespanel.jsx";
import { emptyDiffState, type DiffState } from "./diffpresenter.js";
import type { ChangedItem } from "./diff.js";

function item(over: Partial<ChangedItem>): ChangedItem {
  return { kind: "component", cls: "added", key: "R4", oldName: "", detail: "", aSheets: [], bSheets: [], ...over };
}

function state(over: Partial<DiffState>): DiffState {
  return {
    ...emptyDiffState(),
    active: true,
    aLabel: "m:a",
    bLabel: "m:b",
    pairs: [{ name: "Top", aId: "s1", bId: "s1" }],
    activePair: 0,
    ...over,
  };
}

function mountPanel() {
  const onSelect = vi.fn();
  const el = document.createElement("div");
  document.body.appendChild(el);
  const { island, view } = diffChangesPanelIsland(el, null, { onSelect });
  island.activate();
  return { el, view, onSelect };
}

beforeEach(() => document.body.replaceChildren());

describe("diffChangesPanelIsland", () => {
  it("groups items by change class with counts and detail lines", () => {
    const { el, view } = mountPanel();
    view.setState(
      state({
        items: [
          item({}),
          item({ cls: "changed", key: "R3", detail: "Value: 10k → 22k" }),
          item({ kind: "net", cls: "renamed", key: "SIG_CLK", oldName: "SIG", detail: "was SIG" }),
        ],
      }),
    );
    const heads = [...el.querySelectorAll(".finding-group-name")].map((n) => n.textContent);
    expect(heads).toEqual(["added", "changed", "renamed net"]);
    expect(el.textContent).toContain("Value: 10k → 22k");
    expect(el.textContent).toContain("was SIG");
  });

  it("emits onSelect with the item id on row click, and marks the selected row", () => {
    const { el, view, onSelect } = mountPanel();
    view.setState(state({ items: [item({}), item({ cls: "removed", key: "R2" })], selected: "component:R2" }));
    expect(el.querySelector(".finding.selected .finding-subject")?.textContent).toBe("R2");
    (el.querySelector(".finding .finding-subject") as HTMLElement).closest("button")!.click();
    expect(onSelect).toHaveBeenCalledWith("component:R4");
  });

  // The same strip the findings and query panels use, so a diff over a many-sheet design keeps its
  // rows one line tall rather than carrying one chip per pair.
  it("caps the pair badges and counts the rest", () => {
    const { el, view, onSelect } = mountPanel();
    const pairs = Array.from({ length: 21 }, (_, i) => ({ name: `P${i}`, aId: `a${i}`, bId: `b${i}` }));
    view.setState(state({ pairs, items: [item({ key: "R9", bSheets: pairs.map((p) => p.bId) })] }));
    const labels = () => [...el.querySelectorAll(".sheet-badge")].map((b) => b.textContent);
    expect(labels()).toEqual(["P0", "P1", "P2", "+18"]);

    (el.querySelector(".sheet-badge-more") as HTMLElement).click();
    expect(labels()).toHaveLength(22);
    [...el.querySelectorAll<HTMLElement>(".sheet-badge")].find((b) => b.textContent === "P20")!.click();
    expect(onSelect).toHaveBeenCalledWith("component:R9", 20);
  });

  it("shows pair badges only for multi-pair diffs and emits the pair index from a badge click", () => {
    const { el, view, onSelect } = mountPanel();
    const pairs = [
      { name: "Top", aId: "a1", bId: "b1" },
      { name: "Power", aId: "a2", bId: "b2" },
    ];
    view.setState(state({ pairs, items: [item({ key: "R9", bSheets: ["b2"] })] }));
    const badge = el.querySelector(".sheet-badge") as HTMLElement;
    expect(badge.textContent).toBe("Power");
    badge.click();
    expect(onSelect).toHaveBeenCalledWith("component:R9", 1);
    // Single-pair diffs get no badges.
    view.setState(state({ items: [item({ bSheets: ["s1"] })] }));
    expect(el.querySelector(".sheet-badge")).toBeNull();
  });

  // agni issue 1559. A click on a row rebuilt every row, so a long list jumped back to its top and the
  // reader lost the row they clicked.
  it("keeps every row's element when only the selection changes, so the list keeps its place", () => {
    const { el, view } = mountPanel();
    const items = [item({}), item({ kind: "net", cls: "new", key: "FAN_PWM" })];
    view.setState(state({ items }));
    const rows = () => [...el.querySelectorAll<HTMLElement>("button.finding")];
    const before = rows();
    view.setState(state({ items, selected: "net:FAN_PWM" }));
    expect(rows()).toHaveLength(2);
    rows().forEach((r, i) => expect(r, `row ${i}`).toBe(before[i]));
    expect(el.querySelector(".finding.selected .finding-subject")?.textContent).toBe("FAN_PWM");
  });

  it("distinguishes empty states: inactive vs no changes", () => {
    const { el, view } = mountPanel();
    expect(el.textContent).toContain("No comparison open");
    view.setState(state({ items: [] }));
    expect(el.textContent).toContain("No changes");
  });
});
