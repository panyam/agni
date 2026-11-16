import { describe, expect, it } from "vitest";
import { create } from "@bufbuild/protobuf";
import { BoardPartBodySchema } from "./gen/agni/v1/geom/geom_packed_pb.js";
import { PRESETS, StackCamera } from "./stackcamera.js";
import { BODY_FLOATS, TYPICAL_BODY_MM, bodyGeometry, bodyTopZ, defaultedNote, insideConvex, pickWithBodies, type BodyFaces } from "./stackbodies.js";

// Three parts on a 100 by 100 board: a 2.5 mm one on the front, one on the back, and one on the front
// whose file states no height. Each footprint is a 10 by 10 square, counter-clockwise.
function square(x: number, y: number): number[] {
  return [x, y, x + 10, y, x + 10, y + 10, x, y + 10];
}

function bodies() {
  return [
    create(BoardPartBodySchema, { refDes: "U1", side: "front", outline: square(10, 10), heightMm: 2.5, color: "#202024" }),
    create(BoardPartBodySchema, { refDes: "U2", side: "back", outline: square(60, 10), heightMm: 1.5, color: "#202024" }),
    create(BoardPartBodySchema, { refDes: "U3", side: "front", outline: square(60, 60), color: "#202024" }),
  ];
}

describe("the parts' bodies", () => {
  const g = bodyGeometry(bodies(), 0, 0);
  const vertsOf = (ref: string) => {
    const run = g.runs.get(ref)!;
    const out: number[][] = [];
    for (let i = run.first; i < run.first + run.count; i++) out.push(Array.from(g.verts.slice(i * BODY_FLOATS, (i + 1) * BODY_FLOATS)));
    return out;
  };

  it("stands a part with no stated height at the typical height, after the stated parts", () => {
    expect(g.bodies.map((b) => [b.ref, b.heightMm, b.defaulted])).toEqual([
      ["U1", 2.5, false],
      ["U2", 1.5, false],
      ["U3", TYPICAL_BODY_MM, true],
    ]);
    const defaulted = g.colorRuns.filter((r) => r.defaulted);
    expect(defaulted.map((r) => [r.first, r.count])).toEqual([[g.runs.get("U3")!.first, g.runs.get("U3")!.count]]);
    expect(Math.max(...vertsOf("U3").map((v) => v[2]))).toBe(TYPICAL_BODY_MM);
  });

  it("draws the bodies of one colour in one run, and a part at the default height in a run of its own", () => {
    const mixed = bodies();
    mixed.push(create(BoardPartBodySchema, { refDes: "R1", side: "front", outline: square(30, 30), heightMm: 0.5, color: "#55565c" }));
    const m = bodyGeometry(mixed, 0, 0);
    expect(m.colorRuns.map((r) => [r.color, r.defaulted])).toEqual([
      ["#202024", false],
      ["#55565c", false],
      ["#202024", true],
    ]);
    const span = (ref: string) => m.runs.get(ref)!;
    expect(m.colorRuns[0].count).toBe(span("U1").count + span("U2").count);
    expect(m.colorRuns.reduce((n, r) => n + r.count, 0)).toBe(m.verts.length / BODY_FLOATS);
  });

  it("builds a top face and four walls for a square, each at the part's height or the board", () => {
    const v = vertsOf("U1");
    // Two triangles on top and two per wall.
    expect(v.length).toBe(2 * 3 + 4 * 6);
    expect(new Set(v.map((p) => p[2]))).toEqual(new Set([0, 2.5]));
    expect(v.every((p) => p[3] < 2)).toBe(true);
    expect(vertsOf("U2").every((p) => p[3] >= 2)).toBe(true);
  });

  it("rebases the outline to the packed sheet's origin", () => {
    const moved = bodyGeometry(bodies(), 5, -5);
    expect(Array.from(moved.bodies[0].ring.slice(0, 2))).toEqual([5, 15]);
  });

  it("raises a front part above the top face and hangs a back part below the bottom face", () => {
    const f: BodyFaces = { zTop: 1, zBottom: -1, hScale: 2 };
    expect(bodyTopZ(g.bodies[0], f)).toBe(6);
    expect(bodyTopZ(g.bodies[1], f)).toBe(-4);
  });

  it("tells a point inside a convex ring from one outside", () => {
    const ring = Float32Array.from(square(0, 0));
    expect(insideConvex(ring, 5, 5)).toBe(true);
    expect(insideConvex(ring, 11, 5)).toBe(false);
  });
});

describe("picking a part's body", () => {
  const g = bodyGeometry(bodies(), 0, 0);
  const f: BodyFaces = { zTop: 1, zBottom: -1, hScale: 1 };
  const camera = (pitch: number) => {
    const cam = new StackCamera({ x: 50, y: 50, z: 0 }, 120);
    cam.setViewport(800, 800);
    cam.setPose({ ...PRESETS.top, pitch });
    cam.fit();
    return cam;
  };
  const at = (cam: StackCamera, x: number, y: number, z: number, layers: () => null | { kind: "net"; net: string } = () => null) => {
    const px = cam.project({ x, y, z });
    return pickWithBodies(cam, g.bodies, f, px.x, px.y, layers);
  };

  it("picks a front part from above, before the layers under it", () => {
    const cam = camera(PRESETS.top.pitch);
    expect(at(cam, 15, 15, bodyTopZ(g.bodies[0], f), () => ({ kind: "net", net: "GND" }))).toEqual({ kind: "component", ref: "U1" });
  });

  it("picks the layers over a back part from above, and the back part where the layers have nothing", () => {
    const cam = camera(PRESETS.top.pitch);
    const z = bodyTopZ(g.bodies[1], f);
    expect(at(cam, 65, 15, z, () => ({ kind: "net", net: "GND" }))).toEqual({ kind: "net", net: "GND" });
    expect(at(cam, 65, 15, z)).toEqual({ kind: "component", ref: "U2" });
  });

  it("picks a back part first from below", () => {
    const cam = camera(-90);
    expect(at(cam, 65, 15, bodyTopZ(g.bodies[1], f), () => ({ kind: "net", net: "GND" }))).toEqual({ kind: "component", ref: "U2" });
  });

  it("picks the layers where no part stands", () => {
    const cam = camera(PRESETS.top.pitch);
    expect(at(cam, 40, 40, 1, () => ({ kind: "net", net: "GND" }))).toEqual({ kind: "net", net: "GND" });
  });
});

describe("the note on defaulted heights", () => {
  it("counts the parts whose file states no height, and says nothing when all state one", () => {
    expect(defaultedNote({ bodies: bodies() })).toBe(" (1 of 3 at a default 1 mm)");
    expect(defaultedNote({ bodies: bodies().slice(2) })).toBe(" (all at a default 1 mm)");
    expect(defaultedNote({ bodies: bodies().slice(0, 2) })).toBe("");
  });
});
