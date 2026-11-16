import { describe, expect, it } from "vitest";
import { create } from "@bufbuild/protobuf";
import { BoardDrillSpanSchema, BoardStackViewSchema } from "@panyam/agni-client/gen/agni/v1/geom/geom_packed_pb";
import { BARREL_SIDES, HOLE_FLOATS, SLAB_ALPHA, barrelSides, stackGeometry } from "./stackgl.js";
import { layoutStack } from "./stacklayout.js";
import { jetsonRows } from "./teststack.js";

// The blind-via fixture's three spans (core/render/testdata/blind-via.kicad_pcb), on Jetson's stack.
function view() {
  return create(BoardStackViewSchema, {
    unitsPerMm: 1e6,
    outlineTris: [0, 0, 20e6, 0, 20e6, 10e6],
    drills: [
      create(BoardDrillSpanSchema, { layerFrom: "F.Cu", layerTo: "B.Cu", xy: [4e6, -3e6], diameter: [0.3e6], net: ["THRU"] }),
      create(BoardDrillSpanSchema, { layerFrom: "F.Cu", layerTo: "In1.Cu", xy: [8e6, -3e6], diameter: [0.2e6], net: ["BLIND"] }),
      create(BoardDrillSpanSchema, { layerFrom: "In1.Cu", layerTo: "In2.Cu", xy: [12e6, -3e6], diameter: [0.2e6], net: ["BURIED"] }),
    ],
  });
}

describe("the stack's barrels and slabs", () => {
  const rows = jetsonRows();
  rows.forEach((r) => (r.color = r.kind === "dielectric" ? "#b59a5b" : r.kind === "mask" ? "#2f7d3b" : "#c83434"));
  const layout = layoutStack(rows);
  const g = stackGeometry(view(), layout, 1e6, -5e6);
  const hole = (net: string) => {
    const i = g.nets.indexOf(net);
    const row = Array.from(g.holes.slice(i * HOLE_FLOATS, (i + 1) * HOLE_FLOATS));
    return { x: row[0], y: row[1], r: row[2], top: row[3], bottom: row[4] };
  };
  const plane = (layer: string) => layout.planes.find((p) => p.layer === layer)!;

  it("runs a blind via's barrel across only the layers it joins", () => {
    const blind = hole("BLIND");
    expect(blind.top).toBeCloseTo(plane("F.Cu").top, 6);
    expect(blind.bottom).toBeCloseTo(plane("In1.Cu").bottom, 6);
    expect(blind.bottom).toBeLessThan(plane("In2.Cu").top);
    const buried = hole("BURIED");
    expect(buried.top).toBeCloseTo(plane("In1.Cu").top, 6);
    expect(buried.bottom).toBeCloseTo(plane("In2.Cu").bottom, 6);
    const thru = hole("THRU");
    expect(thru.top).toBeCloseTo(plane("F.Cu").top, 6);
    expect(thru.bottom).toBeCloseTo(plane("B.Cu").bottom, 6);
  });

  it("places each hole rebased to the packed sheet's origin, at its radius", () => {
    expect(g.nets).toEqual(["THRU", "BLIND", "BURIED"]);
    expect(hole("BLIND")).toMatchObject({ x: 7e6, y: 2e6, r: 0.1e6 });
  });

  it("draws every barrel from one cylinder whose sides run from the top end to the bottom end", () => {
    const sides = barrelSides();
    expect(sides.length / 3).toBe(BARREL_SIDES * 6);
    const ends = new Set<number>();
    for (let i = 0; i < sides.length; i += 3) {
      expect(Math.hypot(sides[i], sides[i + 1])).toBeCloseTo(1, 6);
      ends.add(sides[i + 2]);
    }
    expect([...ends].sort()).toEqual([0, 1]);
  });

  it("fills one slab face for each dielectric and mask, in the row's colour", () => {
    const slabs = layout.planes.filter((p) => SLAB_ALPHA[p.kind] !== undefined);
    expect(slabs).toHaveLength(14);
    expect(g.faces).toHaveLength(14);
    expect(g.faces.every((f) => f.count === 3)).toBe(true);
    expect(g.faces.map((f) => f.color)).toEqual(slabs.map((p) => p.color));
  });
});
