import { describe, expect, it } from "vitest";
import { boardLookPlan, padPrimitives, type LookRowIn } from "./stacklook.js";

// A four-layer board's rows as render.BoardLayers lists them. F.Cu names its zone group 20 and the
// pick group 255, In1.Cu names 14 and its zone group 21, and the Zones row lists both zone groups.
const ROWS: LookRowIn[] = [
  { kind: "copper", side: "front", groups: [6, 20, 255] },
  { kind: "copper", side: "inner", groups: [14, 21] },
  { kind: "copper", side: "back", groups: [7, 254] },
  { kind: "edge", side: "", groups: [5] },
  { kind: "through", side: "", groups: [9, 10] },
  { kind: "zones", side: "", groups: [20, 21] },
  { kind: "silk", side: "front", groups: [15] },
  { kind: "silk", side: "back", groups: [16] },
  { kind: "fab", side: "front", groups: [17] },
  { kind: "courtyard", side: "front", groups: [18] },
];

describe("the board look", () => {
  const plan = boardLookPlan(ROWS);

  it("puts the outer copper under the mask on its own face", () => {
    for (const g of [6, 20]) expect([plan.face.get(g), plan.color.get(g)]).toEqual(["top", "maskOverCopper"]);
    expect([plan.face.get(7), plan.color.get(7)]).toEqual(["bottom", "maskOverCopper"]);
  });

  it("draws the silkscreen white on its side's face", () => {
    expect([plan.face.get(15), plan.color.get(15)]).toEqual(["top", "silk"]);
    expect([plan.face.get(16), plan.color.get(16)]).toEqual(["bottom", "silk"]);
  });

  it("hides the inner copper and its zones, and the drawing aids", () => {
    expect([...plan.hidden].sort((a, b) => a - b)).toEqual([14, 17, 18, 21]);
  });

  it("keeps a zone an outer copper row names with that copper, not with the hidden zones", () => {
    expect(plan.hidden.has(20)).toBe(false);
  });

  it("draws the through copper on the top face under the mask, and the edge on the top face in its own colour", () => {
    expect([plan.face.get(9), plan.color.get(9)]).toEqual(["top", "maskOverCopper"]);
    expect([plan.face.get(5), plan.color.get(5)]).toEqual(["top", undefined]);
  });

  it("finishes the pads on copper under the mask, and no track or silkscreen", () => {
    const group = new Map([
      [0, 6],
      [1, 6],
      [2, 9],
      [3, 15],
      [4, 14],
    ]);
    const keys = [
      { primitive: 0, pin: "1" },
      { primitive: 1, pin: "" },
      { primitive: 2, pin: "2" },
      { primitive: 3, pin: "3" },
      { primitive: 4, pin: "4" },
    ];
    expect(padPrimitives(keys, (p) => group.get(p)!, plan)).toEqual([0, 2]);
  });

  // An Altium or Allegro layer gives a pour and the zone it filled (agni issue 1643). The board look
  // draws the pour, and hides the zone's outline, which would cover the pour's clearances.
  it("hides a zone outline on a layer whose pour it draws, and keeps a zone with no pour beside it", () => {
    const withPours: LookRowIn[] = [
      { kind: "copper", side: "front", groups: [6, 13, 14, 255] },
      { kind: "copper", side: "back", groups: [7, 27, 254] },
      { kind: "pours", side: "", groups: [14] },
      { kind: "zones", side: "", groups: [13, 27] },
    ];
    const p = boardLookPlan(withPours);
    expect(p.hidden.has(13)).toBe(true);
    expect([p.face.get(14), p.color.get(14)]).toEqual(["top", "maskOverCopper"]);
    expect(p.hidden.has(27)).toBe(false);
    expect([p.face.get(27), p.color.get(27)]).toEqual(["bottom", "maskOverCopper"]);
  });
});
