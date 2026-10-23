// @vitest-environment jsdom
import { describe, it, expect } from "vitest";
import { areaSelection, emptyMembers, membersIn, rectOf, size, toggle, type ClientRect } from "./area.js";
import { entityBindings, labelFor } from "./selection.js";

// A drawing whose keyed elements carry their box in a data-box attribute, since jsdom has no layout.
function drawing(items: { kind: string; ref?: string; pin?: string; net?: string; box: [number, number, number, number] }[]): {
  root: Element;
  boxOf: (el: Element) => ClientRect;
} {
  const root = document.createElement("div");
  for (const it of items) {
    const el = document.createElement("span");
    el.setAttribute("data-kind", it.kind);
    if (it.ref) el.setAttribute("data-ref", it.ref);
    if (it.pin) el.setAttribute("data-pin", it.pin);
    if (it.net) el.setAttribute("data-net", it.net);
    el.setAttribute("data-box", it.box.join(","));
    root.appendChild(el);
  }
  const boxOf = (el: Element) => {
    const [left, top, right, bottom] = (el.getAttribute("data-box") ?? "").split(",").map(Number);
    return { left, top, right, bottom };
  };
  return { root, boxOf };
}

describe("what an area holds", () => {
  const { root, boxOf } = drawing([
    // U1 is drawn as two shapes, both inside; R9 has one shape outside.
    { kind: "component", ref: "U1", box: [10, 10, 30, 30] },
    { kind: "component", ref: "U1", box: [12, 12, 20, 20] },
    { kind: "pin", ref: "U1", pin: "1", box: [10, 10, 12, 12] },
    { kind: "component", ref: "R9", box: [40, 10, 60, 20] },
    { kind: "component", ref: "R9", box: [90, 10, 120, 20] },
    { kind: "pin", ref: "R9", pin: "2", box: [41, 11, 43, 13] },
    { kind: "pin", ref: "R9", pin: "1", box: [118, 11, 120, 13] },
    { kind: "net", net: "VIN", box: [30, 15, 40, 16] },
    { kind: "net", net: "GND", box: [50, 50, 150, 51] },
  ]);
  const r = rectOf(100, 100, 0, 0);

  it("takes a part only when all its shapes are inside, and a pin of a part that is not", () => {
    const m = membersIn(root, r, boxOf);
    expect(m.refs).toEqual(["U1"]);
    expect(m.pins).toEqual(["R9.2"]);
  });

  it("takes a net whose wire is inside, and none whose wire runs out", () => {
    expect(membersIn(root, r, boxOf).wires).toEqual(["VIN"]);
  });
});

describe("toggling an area", () => {
  it("adds and removes a part, a loose pin and a wire, and a pin of a member part toggles the part", () => {
    let m = toggle(emptyMembers(), { kind: "component", ref: "U1" });
    m = toggle(m, { kind: "pin", ref: "R9", pin: "2" });
    m = toggle(m, { kind: "net", net: "VIN" });
    expect(m).toEqual({ refs: ["U1"], pins: ["R9.2"], wires: ["VIN"] });
    expect(size(m)).toBe(3);
    m = toggle(m, { kind: "pin", ref: "U1", pin: "4" });
    expect(m.refs).toEqual([]);
    m = toggle(m, { kind: "pin", ref: "R9", pin: "2" });
    expect(m.pins).toEqual([]);
    expect(toggle(m, { kind: "bus", busId: "D" })).toEqual(m);
  });
});

describe("an area as a selection", () => {
  const pinNets = (ref: string) =>
    ({ U1: [{ pin: "1", net: "VIN" }, { pin: "2", net: "GND" }], R9: [{ pin: "1", net: "EN" }, { pin: "2", net: "VIN" }] })[ref] ?? [];
  const sel = areaSelection({ refs: ["U1"], pins: ["R9.1"], wires: ["SW"] }, pinNets);

  it("names every net it touches, through its parts, its loose pins and its wires", () => {
    expect(sel.nets).toEqual(["EN", "GND", "SW", "VIN"]);
    expect(labelFor(sel)).toBe("Area (3)");
  });

  it("binds a question to the whole set", () => {
    expect(entityBindings(["ref", "net"], sel)).toEqual({ ref: ["U1"], net: ["EN", "GND", "SW", "VIN"] });
  });
});
