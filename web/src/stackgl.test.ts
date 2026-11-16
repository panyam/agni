import { describe, expect, it } from "vitest";
import { create } from "@bufbuild/protobuf";
import { BoardDrillSpanSchema, BoardStackViewSchema } from "@panyam/agni-client/gen/agni/v1/geom/geom_packed_pb";
import { BARREL_SIDES, HOLE_FLOATS, SLAB_ALPHA, barrelSides, boundaryEdges, stackGeometry } from "./stackgl.js";
import { layoutStack } from "./stacklayout.js";
import { PRESETS, StackCamera } from "./stackcamera.js";
import { hitBarrel } from "./stackpick.js";
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

// The board look's edge wall runs round the outline and round each hole cut in it, from the top face
// to the bottom one (agni issue 1607). A 4 by 4 square with a 2 by 2 hole in its middle has eight
// edges, made of eight triangles that join the two squares.
describe("the board's edge wall", () => {
  const ring = (a: number, b: number) => [
    [a, a],
    [b, a],
    [b, b],
    [a, b],
  ];
  const outer = ring(0, 4);
  const inner = ring(1, 3);
  const tris: number[] = [];
  for (let i = 0; i < 4; i++) {
    const [o0, o1, n0, n1] = [outer[i], outer[(i + 1) % 4], inner[i], inner[(i + 1) % 4]];
    tris.push(...o0, ...o1, ...n1, ...o0, ...n1, ...n0);
  }

  it("finds the outline's and the hole's edges and no edge between two triangles", () => {
    const edges = boundaryEdges(tris);
    expect(edges.length).toBe(8);
    const onBoundary = (x: number, y: number) => x === 0 || x === 4 || y === 0 || y === 4 || x === 1 || x === 3 || y === 1 || y === 3;
    for (const [ax, ay, bx, by] of edges) expect(onBoundary((ax + bx) / 2, (ay + by) / 2) && (ax === bx || ay === by)).toBe(true);
  });

  it("stands a wall of two triangles on each edge, from depth 0 to the stack's total", () => {
    const rows = jetsonRows();
    const layout = layoutStack(rows);
    const g = stackGeometry(create(BoardStackViewSchema, { unitsPerMm: 1, outlineTris: tris }), layout, 0, 0);
    expect(g.boardWall.count).toBe(8 * 6);
    const depths = new Set<number>();
    for (let v = g.boardWall.first; v < g.boardWall.first + g.boardWall.count; v++) depths.add(g.slabs[3 * v + 2]);
    expect([...depths].sort((a, b) => a - b)).toEqual([0, layout.totalMm].sort((a, b) => a - b).map((d) => Math.fround(d)));
  });
});

// A click on a barrel of the blind-via fixture picks its net (agni issue 1607). The barrels stand at
// their spans' depths at the board's true scale, a nanometre a world unit, as x and y are.
describe("picking the blind-via fixture's barrels", () => {
  const layout = layoutStack(jetsonRows());
  const g = stackGeometry(view(), layout, 1e6, -5e6);
  const perMm = 1e6;
  const barrels = g.nets.map((net, i) => {
    const h = g.holes.slice(i * HOLE_FLOATS, (i + 1) * HOLE_FLOATS);
    return { x: h[0], y: h[1], r: h[2], zTop: -h[3] * perMm, zBottom: -h[4] * perMm, net };
  });
  const cam = new StackCamera({ x: 8e6, y: 2e6, z: 0 }, 12e6);
  cam.setViewport(1000, 800);
  cam.setPose(PRESETS.front);
  cam.fit();
  cam.zoomAt(500, 400, 40);

  it("picks each barrel's own net at its middle, seen from the front", () => {
    for (const b of barrels) {
      const at = cam.project({ x: b.x, y: b.y, z: (b.zTop + b.zBottom) / 2 });
      expect(hitBarrel(cam, barrels, at.x, at.y)?.net).toBe(b.net);
    }
  });

  it("misses below a blind via's barrel, where it ends", () => {
    const blind = barrels.find((b) => b.net === "BLIND")!;
    const below = cam.project({ x: blind.x, y: blind.y, z: blind.zBottom - (blind.zTop - blind.zBottom) });
    expect(hitBarrel(cam, [blind], below.x, below.y)).toBeNull();
  });
});
