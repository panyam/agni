// @vitest-environment jsdom
import { describe, it, expect } from "vitest";
import { bestOf, labelFor, pickAt, queryFor, selectionFromElement, type Selection } from "./selection.js";

function el(attrs: Record<string, string>): Element {
  const e = document.createElementNS("http://www.w3.org/2000/svg", "polyline");
  for (const [k, v] of Object.entries(attrs)) e.setAttribute(k, v);
  return e;
}

describe("reading a keyed element", () => {
  it("reads each kind the renderer emits", () => {
    expect(selectionFromElement(el({ "data-kind": "pin", "data-ref": "U7", "data-pin": "12" }))).toEqual({ kind: "pin", ref: "U7", pin: "12" });
    expect(selectionFromElement(el({ "data-kind": "component", "data-ref": "R1" }))).toEqual({ kind: "component", ref: "R1" });
    expect(selectionFromElement(el({ "data-kind": "wire", "data-net": "SDA", "data-net-id": "n1" }))).toEqual({ kind: "net", net: "SDA", netId: "n1" });
    expect(selectionFromElement(el({ "data-kind": "bus", "data-bus": "D[7:0]" }))).toEqual({ kind: "bus", busId: "D[7:0]" });
  });

  // The page rect, text labels and the highlight overlay carry no keys, and a click on them is a
  // click on nothing rather than on whatever happens to be behind them.
  it("returns null for an unkeyed element", () => {
    expect(selectionFromElement(el({ fill: "#fff" }))).toBeNull();
    expect(selectionFromElement(null)).toBeNull();
  });

  it("refuses a keyed element that is missing its identity", () => {
    expect(selectionFromElement(el({ "data-kind": "pin", "data-ref": "U7" }))).toBeNull();
    expect(selectionFromElement(el({ "data-kind": "component" }))).toBeNull();
  });
});

describe("ranking overlapping candidates", () => {
  const pin: Selection = { kind: "pin", ref: "U7", pin: "1" };
  const comp: Selection = { kind: "component", ref: "U7" };
  const net: Selection = { kind: "net", net: "SDA" };

  // A symbol is drawn over its own pins, so topmost-wins would make a pin unclickable.
  it("prefers the most specific, not the topmost", () => {
    expect(bestOf([comp, pin, net])).toEqual(pin);
    expect(bestOf([net, comp])).toEqual(comp);
    expect(bestOf([net])).toEqual(net);
  });

  it("is null when nothing under the cursor is an entity", () => {
    expect(bestOf([null, null])).toBeNull();
  });

  // Probe order breaks a tie, so the nearer of two same-kind hits wins.
  it("keeps the first of two same-kind candidates", () => {
    const near: Selection = { kind: "net", net: "NEAR" };
    const far: Selection = { kind: "net", net: "FAR" };
    expect(bestOf([near, far])).toEqual(near);
  });
});

describe("pickAt", () => {
  // A 1px wire is a game of skill to hit exactly, so the pick samples a small ring around the
  // cursor. Here the exact point is empty and only an offset probe lands on the wire.
  it("finds an entity the cursor is merely near", () => {
    const wire = el({ "data-kind": "wire", "data-net": "GND" });
    const doc = {
      elementFromPoint: (x: number, y: number) => (x === 105 && y === 100 ? wire : null),
    } as unknown as Document;

    expect(pickAt(doc, 100, 100)).toEqual({ kind: "net", net: "GND", netId: "" });
  });

  it("is null when the ring finds nothing either", () => {
    const doc = { elementFromPoint: () => null } as unknown as Document;
    expect(pickAt(doc, 10, 10)).toBeNull();
  });
});

describe("the query a click writes", () => {
  // The generated query is the DSL lesson: it lands in the box, runs, and is editable. So it has to
  // be a query that RUNS — every relation named here exists in the catalog.
  it("asks what a pin is attached to", () => {
    const q = queryFor({ kind: "pin", ref: "U7", pin: "12" });
    expect(q).toContain(`pin.net("U7", "12", ?net)`);
    expect(q).toContain("=>");
  });

  it("asks a component for its nets and their fan-out", () => {
    expect(queryFor({ kind: "component", ref: "R1" })).toContain(`component-on-net("R1", ?net)`);
  });

  it("asks a net for what sits on it", () => {
    const q = queryFor({ kind: "net", net: "SDA" });
    expect(q).toContain(`component-on-net(?ref, "SDA")`);
    expect(q).toContain(`pin.net(?ref, ?pin, "SDA")`);
  });
});

describe("labelFor", () => {
  it("names a selection the way a person would", () => {
    expect(labelFor({ kind: "pin", ref: "U7", pin: "12" })).toBe("U7.12");
    expect(labelFor({ kind: "component", ref: "R1" })).toBe("R1");
    expect(labelFor({ kind: "net", net: "SDA" })).toBe("SDA");
  });
});
