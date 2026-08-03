// @vitest-environment jsdom
import { describe, it, expect, vi } from "vitest";
import { partsPanelIsland } from "./partspanel.jsx";
import type { PartsState } from "./parts.js";

function mount(state: PartsState) {
  const onLocate = vi.fn();
  const el = document.createElement("div");
  document.body.appendChild(el);
  const panel = partsPanelIsland(el, null, { onLocate });
  panel.island.activate();
  panel.view.setState(state);
  return { el, onLocate };
}

// A design component joined to a one-parameter spec (fields the panel reads; cast because the proto
// Message type carries generated metadata a test does not need to fabricate).
const state = {
  parts: [
    {
      refDes: "U1",
      mpn: "LM1117",
      spec: {
        parameters: [
          {
            name: "VIN abs max",
            symbol: "",
            unit: "V",
            limitKind: 1,
            value: { max: 20 },
            conditions: [{ raw: "TA=25C", symbol: "TA" }],
            prov: { docRef: "SNOS412Q", page: 4, tableOrFigure: "7.1" },
          },
        ],
      },
    },
  ],
} as unknown as PartsState;

describe("partsPanel", () => {
  it("lists a datasheet-backed component and expands to its parameter tree", () => {
    const { el } = mount(state);
    expect(el.querySelector(".parts-ref")?.textContent).toContain("U1");
    expect(el.querySelector(".parts-mpn")?.textContent).toContain("LM1117");
    expect(el.querySelector(".parts-count")?.textContent).toContain("1 params");
    expect(el.querySelectorAll(".parts-param")).toHaveLength(0); // collapsed
    (el.querySelector(".parts-toggle") as HTMLButtonElement).click();
    const row = el.querySelector(".parts-param");
    expect(row).toBeTruthy();
    expect(row?.textContent).toContain("VIN abs max");
    expect(row?.textContent).toContain("max 20");
    expect(row?.textContent).toContain("abs-max"); // limit-kind badge
    expect(row?.textContent).toContain("SNOS412Q p4"); // citation
  });

  it("locates the component on ref-des click", () => {
    const { el, onLocate } = mount(state);
    (el.querySelector(".parts-ref") as HTMLButtonElement).click();
    expect(onLocate).toHaveBeenCalledWith("U1");
  });

  it("shows an empty hint when no parts are datasheet-backed", () => {
    const { el } = mount({ parts: [] });
    expect(el.querySelector(".parts-empty")).toBeTruthy();
  });
});
