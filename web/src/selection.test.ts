// @vitest-environment jsdom
import { describe, it, expect } from "vitest";
import { askLabel, bestOf, fillEntityQuery, labelFor, pickAt, selectionFromCell, selectionFromElement, type Selection } from "./selection.js";

function el(attrs: Record<string, string>): Element {
  const e = document.createElementNS("http://www.w3.org/2000/svg", "polyline");
  for (const [k, v] of Object.entries(attrs)) e.setAttribute(k, v);
  return e;
}

describe("reading a keyed element", () => {
  it("reads each kind the renderer emits", () => {
    expect(selectionFromElement(el({ "data-kind": "pin", "data-ref": "U7", "data-pin": "12" }))).toEqual({ kind: "pin", ref: "U7", pin: "12" });
    expect(selectionFromElement(el({ "data-kind": "component", "data-ref": "R1" }))).toEqual({ kind: "component", ref: "R1" });
    expect(selectionFromElement(el({ "data-kind": "net", "data-net": "SDA", "data-net-id": "n1" }))).toEqual({ kind: "net", net: "SDA", netId: "n1" });
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
    const wire = el({ "data-kind": "net", "data-net": "GND" });
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

describe("filling a served preset", () => {
  // The templates live on the server, beside the relations they name, so what is tested here is the
  // substitution: the client's half of the job.
  it("substitutes a pin's ref and designator", () => {
    const q = fillEntityQuery(`pin.net("{ref}", "{pin}", ?net) => ?net`, { kind: "pin", ref: "U7", pin: "12" });
    expect(q).toBe(`pin.net("U7", "12", ?net) => ?net`);
  });

  it("substitutes every occurrence, not just the first", () => {
    const q = fillEntityQuery(`component-on-net(?r, "{net}"), pin.net(?r, ?p, "{net}") => ?r, ?p`, { kind: "net", net: "SDA" });
    expect(q).toBe(`component-on-net(?r, "SDA"), pin.net(?r, ?p, "SDA") => ?r, ?p`);
  });

  // The grammar has no escape sequence, so a quote in a designator cannot be represented. Splicing
  // one in would end the string literal early and produce a query that means something else.
  it("strips a quote rather than splicing it into a string literal", () => {
    const q = fillEntityQuery(`component-on-net("{ref}", ?n) => ?n`, { kind: "component", ref: `R"1` });
    expect(q).toBe(`component-on-net("R1", ?n) => ?n`);
  });

  it("leaves a placeholder the selection cannot fill as an empty literal", () => {
    expect(fillEntityQuery(`bus("{bus}", ?m) => ?m`, { kind: "net", net: "N" })).toBe(`bus("", ?m) => ?m`);
  });
});

describe("labelFor", () => {
  it("names a selection the way a person would", () => {
    expect(labelFor({ kind: "pin", ref: "U7", pin: "12" })).toBe("U7.12");
    expect(labelFor({ kind: "component", ref: "R1" })).toBe("R1");
    expect(labelFor({ kind: "net", net: "SDA" })).toBe("SDA");
  });
});

// A result cell is the second way a reader names an entity, and it has to produce the same value a
// click on the drawing does — otherwise the walk needs its own copy of everything downstream.
describe("reading a result cell", () => {
  it("reads the kinds the server types a column with", () => {
    expect(selectionFromCell("component", "R1")).toEqual({ kind: "component", ref: "R1" });
    expect(selectionFromCell("net", "SDA")).toEqual({ kind: "net", net: "SDA" });
  });

  // A scalar column ("") is not clickable at all, and a pin column is typed scalar today: a pin needs
  // its row's ref as well as its own cell, so it cannot be read from one cell.
  it("is null for a cell that names no entity", () => {
    expect(selectionFromCell("", "3.3")).toBeNull();
    expect(selectionFromCell("pin", "12")).toBeNull();
    expect(selectionFromCell("net", "")).toBeNull();
  });
});

describe("wording the next question", () => {
  it("asks about each kind in its own terms", () => {
    expect(askLabel({ kind: "pin", ref: "U7", pin: "12" })).toBe("What is U7.12 wired to?");
    expect(askLabel({ kind: "component", ref: "R1" })).toBe("What is R1 connected to?");
    expect(askLabel({ kind: "net", net: "SDA" })).toBe("What is on SDA?");
    expect(askLabel({ kind: "bus", busId: "D[7:0]" })).toBe("What is in D[7:0]?");
  });
});
