// @vitest-environment jsdom
import { describe, it, expect, vi } from "vitest";
import { diffPanelIsland } from "./diffpanel.jsx";
import { emptyDiffState } from "./diffpresenter.js";
import type { DiffState } from "./diffpresenter.js";

function state(over: Partial<DiffState>): DiffState {
  return {
    ...emptyDiffState(),
    active: true,
    aLabel: "m:a.edn",
    bLabel: "m:b.edn",
    pairs: [
      { name: "Top", aId: "s1", bId: "s1" },
      { name: "Power", aId: "s2", bId: "" },
    ],
    activePair: 0,
    legend: [{ cls: "added", label: "added", color: "#1a9850", count: 2 }],
    ...over,
  };
}

function mountPanel() {
  const onPair = vi.fn();
  const onMode = vi.fn();
  const onClose = vi.fn();
  const el = document.createElement("div");
  document.body.appendChild(el);
  const { island, view } = diffPanelIsland(el, null, { onPair, onMode, onClose });
  island.activate();
  return { el, view, onPair, onMode, onClose };
}

describe("diffPanelIsland", () => {
  it("shows the inactive hint until a comparison opens", () => {
    const { el, view } = mountPanel();
    expect(el.textContent).toContain("No comparison open");
    view.setState(state({}));
    expect(el.textContent).toContain("m:a.edn");
    expect(el.textContent).toContain("m:b.edn");
  });

  it("renders legend chips with counts and marks one-sided pairs in the selector", () => {
    const { el, view, onPair } = mountPanel();
    view.setState(state({}));
    expect(el.textContent).toContain("2 added");
    const select = el.querySelector("select")!;
    const labels = [...select.querySelectorAll("option")].map((o) => o.textContent);
    expect(labels).toEqual(["Top", "Power (A only)"]);
    select.value = "1";
    select.dispatchEvent(new Event("change", { bubbles: true }));
    expect(onPair).toHaveBeenCalledWith(1);
  });

  it("shows 'no changes' for an identical pair and the error when the diff failed", () => {
    const { el, view } = mountPanel();
    view.setState(state({ legend: [] }));
    expect(el.textContent).toContain("no changes");
    view.setState(state({ legend: [], error: "boom" }));
    expect(el.textContent).toContain("boom");
    expect(el.textContent).not.toContain("no changes");
  });

  it("gates the Overlay toggle on alignment and emits onMode (WS9-007)", () => {
    const { el, view, onMode } = mountPanel();
    view.setState(state({ overlayOk: false, overlayReason: "page sizes differ between revisions" }));
    const overlayBtn = [...el.querySelectorAll(".diff-modes .mode-btn")].find((b) => b.textContent === "Overlay") as HTMLButtonElement;
    expect(overlayBtn.disabled).toBe(true);
    expect(overlayBtn.title).toContain("page sizes");
    view.setState(state({ overlayOk: true }));
    expect(overlayBtn.disabled).toBe(false);
    overlayBtn.click();
    expect(onMode).toHaveBeenCalledWith("overlay");
    view.setState(state({ overlayOk: true, mode: "overlay" }));
    expect(overlayBtn.classList.contains("active")).toBe(true);
  });

  it("emits onClose", () => {
    const { el, view, onClose } = mountPanel();
    view.setState(state({}));
    (el.querySelector(".diff-close") as HTMLButtonElement).click();
    expect(onClose).toHaveBeenCalled();
  });
});
