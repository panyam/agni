// @vitest-environment jsdom
import { describe, it, expect, vi } from "vitest";
import { coveragePanelIsland } from "./coveragepanel.jsx";
import type { CoverageState } from "./coverage.js";

function mount(state: CoverageState) {
  const onLocate = vi.fn();
  const el = document.createElement("div");
  document.body.appendChild(el);
  const panel = coveragePanelIsland(el, null, { onLocate });
  panel.island.activate();
  panel.view.setState(state);
  return { el, onLocate };
}

const state: CoverageState = {
  interfaces: [
    {
      profile: "SPI_NOR",
      anchor: "SPI_CS",
      signals: [
        { name: "CS", net: "SPI_CS", state: "present" },
        { name: "IO2", net: "", state: "missing" },
      ],
    },
  ],
};

describe("coveragePanel", () => {
  it("renders a chip per signal, colored by state", () => {
    const { el } = mount(state);
    expect(el.querySelectorAll(".coverage-signal")).toHaveLength(2);
    expect(el.querySelector(".cov-present")?.textContent).toContain("CS");
    expect(el.querySelector(".cov-missing")?.textContent).toContain("IO2");
    expect(el.querySelector(".coverage-count")?.textContent).toBe("1/2");
  });

  it("locates a present signal's net on click; a missing signal is not clickable", () => {
    const { el, onLocate } = mount(state);
    (el.querySelector(".cov-present") as HTMLButtonElement).click();
    expect(onLocate).toHaveBeenCalledWith("SPI_CS");

    const missing = el.querySelector(".cov-missing") as HTMLButtonElement;
    expect(missing.disabled).toBe(true);
    onLocate.mockClear();
    missing.click();
    expect(onLocate).not.toHaveBeenCalled();
  });

  it("shows an empty message when no interface is detected", () => {
    const { el } = mount({ interfaces: [] });
    expect(el.querySelector(".coverage-empty")).toBeTruthy();
    expect(el.querySelector(".coverage-signal")).toBeNull();
  });
});
