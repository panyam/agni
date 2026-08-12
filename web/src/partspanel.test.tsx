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
            pinRefs: [],
            prov: { docRef: "SNOS412Q", page: 4, tableOrFigure: "7.1" },
          },
        ],
        pins: [],
      },
    },
  ],
} as unknown as PartsState;

// A two-supply part: the shape the panel used to render as four indistinguishable rows. VCCA and
// VCCB carry different limits, one row is bound to a GROUP of two terminals, and one is part-wide.
const pinBound = {
  parts: [
    {
      refDes: "U7",
      mpn: "ACME-XLAT",
      spec: {
        parameters: [
          { name: "Supply voltage", symbol: "VCCA", unit: "V", limitKind: 1, value: { max: 4.6 }, conditions: [], pinRefs: ["vcca"] },
          { name: "Supply voltage", symbol: "VCCB", unit: "V", limitKind: 1, value: { max: 6.5 }, conditions: [], pinRefs: ["vccb"] },
          { name: "Output voltage", symbol: "VO", unit: "V", limitKind: 2, value: { max: 3.6 }, conditions: [], pinRefs: ["a1", "a2"] },
          { name: "Junction temperature", symbol: "TJ", unit: "C", limitKind: 1, value: { max: 150 }, conditions: [], pinRefs: [] },
          // A port-wide rating: one datasheet row covering every I/O terminal. Eight is what the real
          // TXB0104 has, and it is the case that decides whether a chip row needs truncating.
          {
            name: "ESD (HBM)", symbol: "V(ESD)", unit: "kV", limitKind: 1, value: { max: 2.5 }, conditions: [],
            pinRefs: ["a1", "a2", "a3", "a4", "b1", "b2", "b3", "b4"],
          },
        ],
        pins: [
          { id: "vcca", name: "VCCA" },
          { id: "vccb", name: "VCCB" },
          { id: "a1", name: "A1" },
          { id: "a2", name: "A2" },
          { id: "a3", name: "A3" },
          { id: "a4", name: "A4" },
          { id: "b1", name: "B1" },
          { id: "b2", name: "B2" },
          { id: "b3", name: "B3" },
          { id: "b4", name: "B4" },
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

  // The reason this change exists: two rows printing the same kind of limit are only
  // distinguishable by the terminal they apply to, and the panel showed no terminal at all.
  it("names the terminal each parameter is bound to, by its printed pin name", () => {
    const { el } = mount(pinBound);
    (el.querySelector(".parts-toggle") as HTMLButtonElement).click();
    const rows = [...el.querySelectorAll(".parts-param")].map((r) => r.textContent ?? "");

    expect(rows[0]).toContain("VCCA");
    expect(rows[0]).toContain("max 4.6");
    expect(rows[1]).toContain("VCCB");
    expect(rows[1]).toContain("max 6.5");

    // The ids are spec-local and opaque; the panel must resolve them to printed names.
    expect(el.querySelector(".parts-pin")?.textContent).toBe("VCCA");
    expect(rows.join("")).not.toContain("vcca");
  });

  it("lists every terminal of a group binding, and labels a part-wide row as such", () => {
    const { el } = mount(pinBound);
    (el.querySelector(".parts-toggle") as HTMLButtonElement).click();
    const chips = [...el.querySelectorAll(".parts-pin")].map((c) => c.textContent ?? "");

    expect(chips).toContain("A1, A2");
    // An unbound row on a spec that DECLARES pins is a fact about the die, not an unrendered
    // binding, and the two must not look alike.
    expect(chips).toContain("part-wide");
    expect(el.querySelector(".parts-pin-wide")).toBeTruthy();
  });

  // Characterisation, not an endorsement: a row bound to eight terminals currently renders all eight
  // names inline. Truncation is deliberately deferred (no user has met a part where it hurts yet),
  // and this test is what has to change when it lands, so the decision cannot be reversed silently.
  it("renders every terminal of a wide group binding, untruncated", () => {
    const { el } = mount(pinBound);
    (el.querySelector(".parts-toggle") as HTMLButtonElement).click();
    const chips = [...el.querySelectorAll(".parts-pin")].map((c) => c.textContent ?? "");
    const wide = chips.find((c) => c.includes("B4")); // the 8-pin ESD row, not the 2-pin VO one
    expect(wide).toBe("A1, A2, A3, A4, B1, B2, B3, B4");
    expect(wide).not.toContain("…");
    expect(wide).not.toMatch(/\+\d+/); // no "+4 more" affordance either
  });

  it("counts the declared pins in the header", () => {
    const { el } = mount(pinBound);
    expect([...el.querySelectorAll(".parts-count")].map((c) => c.textContent)).toContain("10 pins");
  });

  // Degrade-safety: a spec seeded before pin binding declares no pins, so the panel must render
  // exactly as it did — no chips at all, not even the part-wide label.
  it("shows no pin chips for a spec with no pin data", () => {
    const { el } = mount(state);
    (el.querySelector(".parts-toggle") as HTMLButtonElement).click();
    expect(el.querySelectorAll(".parts-pin")).toHaveLength(0);
    expect([...el.querySelectorAll(".parts-count")].map((c) => c.textContent)).not.toContain("0 pins");
  });

  it("shows an empty hint when no parts are datasheet-backed", () => {
    const { el } = mount({ parts: [] });
    expect(el.querySelector(".parts-empty")).toBeTruthy();
  });
});
