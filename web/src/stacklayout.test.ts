import { describe, expect, it } from "vitest";
import { groupDepths, layoutStack, perMm, stackZ } from "./stacklayout.js";
import { jetsonRows } from "./teststack.js";

// The tolerance on a drawn height, a hundredth of a micrometre. The sums run in floating point.
const MM_TOLERANCE = 1e-5;

describe("the stack's layout", () => {
  // The engine places every row in depth (render.BoardStack, agni issue 1635). Its tests hold the
  // rule, and these check the layout draws where it says.
  it("lays the cut's 28 rows where the engine placed them, to its stated 1.711 mm", () => {
    const rows = jetsonRows();
    const l = layoutStack(rows);
    expect(l.planes).toHaveLength(28);
    expect(Math.abs(l.totalMm - 1.711)).toBeLessThan(MM_TOLERANCE);
    l.planes.forEach((p, i) => expect([p.top, p.bottom, p.defaulted]).toEqual([rows[i].topMm, rows[i].bottomMm, rows[i].thicknessDefaulted]));
    const byName = new Map(l.planes.map((p) => [p.name, p]));
    expect(byName.get("F.SilkS")).toMatchObject({ top: 0, bottom: 0, defaulted: true });
    expect(byName.get("F.Cu")).toMatchObject({ defaulted: false });
  });

  it("draws the board's true thickness at explode 0, and keeps every gap in proportion when spread", () => {
    const l = layoutStack(jetsonRows());
    const unitsPerMm = 1e6;
    const width = 100 * unitsPerMm;
    const flat = perMm(0, l, width, unitsPerMm);
    const height = (s: number) => (stackZ(0, l, s) - stackZ(l.totalMm, l, s)) / unitsPerMm;
    expect(Math.abs(height(flat) - 1.711)).toBeLessThan(MM_TOLERANCE);
    const spread = perMm(1, l, width, unitsPerMm);
    expect(height(spread)).toBeCloseTo(60, 6);
    // dielectric 2 is 0.2 mm and dielectric 1 is 0.12 mm, at any spread.
    const gap = (s: number, i: number) => stackZ(l.planes[i].top, l, s) - stackZ(l.planes[i].bottom, l, s);
    expect(gap(spread, 6) / gap(spread, 4)).toBeCloseTo(0.2 / 0.12, 9);
    expect(gap(flat, 6) / gap(flat, 4)).toBeCloseTo(0.2 / 0.12, 9);
  });
});

describe("the depth each packed group draws at", () => {
  const l = layoutStack(jetsonRows());
  const rows = [
    { name: "F.Cu", kind: "copper", side: "front", groups: [6, 13, 255] },
    { name: "In2.Cu", kind: "copper", side: "inner", groups: [14, 15] },
    { name: "B.Cu", kind: "copper", side: "back", groups: [7, 254] },
    { name: "Edge", kind: "edge", side: "", groups: [5] },
    { name: "Through", kind: "through", side: "", groups: [9, 10] },
    { name: "Zones", kind: "zones", side: "", groups: [15, 13] },
    { name: "F.SilkS", kind: "silk", side: "front", groups: [20] },
    { name: "B.Fab", kind: "fab", side: "back", groups: [21] },
    { name: "F.Fab", kind: "fab", side: "front", groups: [22] },
  ];
  const d = groupDepths(l, rows);
  const mid = (name: string) => {
    const p = l.planes.find((q) => q.layer === name)!;
    return (p.top + p.bottom) / 2;
  };

  it("puts a copper layer's tracks, zones and pick areas at its own depth", () => {
    expect(d.depth.get(6)).toBeCloseTo(mid("F.Cu"), 9);
    expect(d.depth.get(13)).toBeCloseTo(mid("F.Cu"), 9);
    expect(d.depth.get(255)).toBeCloseTo(mid("F.Cu"), 9);
    expect(d.depth.get(15)).toBeCloseTo(mid("In2.Cu"), 9);
    expect(d.depth.get(254)).toBeCloseTo(mid("B.Cu"), 9);
  });

  it("puts a layer the stack does not name on its side's face, and the through copper in barrels", () => {
    expect(d.depth.get(22)).toBe(0);
    expect(d.depth.get(21)).toBeCloseTo(1.711, 9);
    expect(d.depth.get(5)).toBe(0);
    expect(d.depth.get(20)).toBe(0);
    expect([...d.barrels].sort()).toEqual([10, 9]);
    expect(d.depth.has(9)).toBe(false);
  });
});
