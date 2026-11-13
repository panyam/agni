import { describe, it, expect, vi } from "vitest";
import { CHUNK_MIN, GRID, buildBatches, wideCount } from "./glbatch.js";
import { Renderer } from "./webgl.js";
import { KIND_LINE_LOOP, KIND_LINE_STRIP, KIND_POINTS, KIND_TRIANGLES, type DecodedSheet, type Primitive } from "./packed.js";
import type { Bounds } from "./camera.js";
import { fakeGl } from "./fakegl.js";

// A frame draws a run of one group in a few calls, rather than one call per primitive (agni issue
// 1484). Two runs keep their order, since one group may cover another.

const boxAt = (x: number, y: number): Bounds => ({ minX: x, minY: y, maxX: x + 1, maxY: y + 1 });
const tri = (group: number, firstVertex: number): Primitive => ({ kind: KIND_TRIANGLES, group, firstVertex, count: 3 });

describe("buildBatches", () => {
  // The last primitive sits on the group-2 one drawn before it, so it has to draw after it.
  it("keeps a primitive that comes back over another group in a run of its own", () => {
    const prims = [tri(1, 0), tri(1, 3), tri(2, 6), tri(1, 9)];
    const bounds = [boxAt(0, 0), boxAt(100, 0), boxAt(500, 500), boxAt(500, 500)];
    const b = buildBatches(prims, bounds, [], null);
    expect(b.runs.map((r) => r.group)).toEqual([1, 2, 1]);
    expect(b.runs.map((r) => r.chunks.flatMap((c) => c.triList))).toEqual([[0, 1], [2], [3]]);
  });

  // A via's barrel and then its hole, twice. The second barrel touches nothing drawn since the
  // first, so it joins the first barrel's run, and the holes share a run too.
  it("joins a primitive to its group's earlier run when nothing drawn since overlaps it", () => {
    const prims = [tri(1, 0), tri(2, 3), tri(1, 6), tri(2, 9)];
    const bounds = [boxAt(0, 0), boxAt(0, 0), boxAt(900, 900), boxAt(900, 900)];
    const b = buildBatches(prims, bounds, [], null);
    expect(b.runs.map((r) => r.group)).toEqual([1, 2]);
    expect(b.runs.map((r) => r.chunks.flatMap((c) => c.triList))).toEqual([[0, 2], [1, 3]]);
  });

  it("follows the draw order it is given", () => {
    const prims = [tri(1, 0), tri(2, 3), tri(1, 6)];
    const b = buildBatches(prims, prims.map((_, i) => boxAt(i, 0)), [], Int32Array.from([1, 0, 2]));
    expect(b.runs.map((r) => r.group)).toEqual([2, 1]);
    expect(b.runs[1].chunks[0].triList).toEqual([0, 2]);
  });

  it("turns a strip and a loop into segment pairs, and leaves out a pick area and an empty primitive", () => {
    const prims: Primitive[] = [
      { kind: KIND_LINE_STRIP, group: 0, firstVertex: 0, count: 3 },
      { kind: KIND_LINE_LOOP, group: 0, firstVertex: 3, count: 3 },
      { kind: KIND_POINTS, group: 0, firstVertex: 6, count: 1 },
      { kind: KIND_TRIANGLES, group: 254, firstVertex: 7, count: 3 },
      { kind: KIND_TRIANGLES, group: 0, firstVertex: 10, count: 0 },
    ];
    const b = buildBatches(prims, prims.map(() => boxAt(0, 0)), [], null);
    expect(Array.from(b.indices)).toEqual([0, 1, 1, 2, 3, 4, 4, 5, 5, 3, 6]);
    expect(Array.from(b.chunkOf)).toEqual([0, 0, 0, -1, -1]);
  });

  it("splits a large run by a grid over the sheet, so a view culls a cell at a time", () => {
    const side = Math.ceil(Math.sqrt(CHUNK_MIN * 2));
    const prims: Primitive[] = [];
    const bounds: Bounds[] = [];
    for (let y = 0; y < side; y++) for (let x = 0; x < side; x++) {
      prims.push(tri(0, prims.length * 3));
      bounds.push(boxAt(x * 10, y * 10));
    }
    const b = buildBatches(prims, bounds, [], null);
    expect(b.runs).toHaveLength(1);
    expect(b.runs[0].chunks).toHaveLength(GRID * GRID);
    expect(b.runs[0].chunks.reduce((n, c) => n + c.triList.length, 0)).toBe(prims.length);
  });

  it("counts the lines wide enough to draw as triangles, widest first", () => {
    const c = { strokeWidths: [8, 4, 2] } as Parameters<typeof wideCount>[0];
    expect([wideCount(c, 1), wideCount(c, 3), wideCount(c, 4), wideCount(c, 9)]).toEqual([3, 2, 2, 0]);
  });
});

describe("Renderer, batched", () => {
  const all = { minX: -1e9, minY: -1e9, maxX: 1e9, maxY: 1e9 };
  // Three triangles in group 0, side by side, then one in group 1.
  const sheet = (): DecodedSheet => ({
    sheetId: "S", layoutVersion: 1, originX: 0, originY: 0,
    vertices: Int32Array.from([0, 0, 1, 0, 0, 1, 10, 0, 11, 0, 10, 1, 20, 0, 21, 0, 20, 1, 30, 0, 31, 0, 30, 1]),
    primitives: [tri(0, 0), tri(0, 3), tri(0, 6), tri(1, 9)],
    keys: [], labels: [], images: [], fontFamily: "", groupColors: ["#000000", "#ff0000"], backgroundColor: "#ffffff",
  });
  const draws = (f: ReturnType<typeof fakeGl>) => f.calls.filter((c) => c.name === "drawElements").map((c) => [c.args[1], (c.args[3] as number) / 4]);

  it("draws each run in one call, in order", () => {
    const f = fakeGl();
    expect(new Renderer(f.gl, sheet()).render(new Float32Array(9), all)).toBe(2);
    expect(draws(f)).toEqual([[9, 0], [3, 9]]);
  });

  it("draws a highlighted primitive alone in its colour, in its place, and the rest around it", () => {
    const f = fakeGl();
    const r = new Renderer(f.gl, sheet());
    r.setHighlights([{ color: [0, 1, 0, 1], primitives: new Set([1]) }]);
    f.calls.length = 0;
    r.render(new Float32Array(9), all);
    expect(draws(f)).toEqual([[3, 0], [3, 3], [3, 6], [3, 9]]);
    const colours = f.calls.filter((c) => c.name === "uniform4f").map((c) => c.args.slice(1));
    expect(colours).toEqual([[0, 0, 0, 1], [0, 1, 0, 1], [0, 0, 0, 1], [1, 0, 0, 1]]);
  });

  it("culls a run off the view, and still draws its highlighted primitive", () => {
    const f = fakeGl();
    const r = new Renderer(f.gl, sheet());
    const right = { minX: 25, minY: -5, maxX: 40, maxY: 5 };
    r.render(new Float32Array(9), right);
    expect(draws(f)).toEqual([[3, 9]]);
    r.setHighlights([{ color: [0, 1, 0, 1], primitives: new Set([0]) }]);
    f.calls.length = 0;
    r.render(new Float32Array(9), right);
    expect(draws(f)).toEqual([[3, 0], [3, 9]]);
  });

  it("skips a hidden group, and still draws its highlighted primitive", () => {
    const f = fakeGl();
    const r = new Renderer(f.gl, sheet());
    r.setHiddenGroups([0]);
    r.setHighlights([{ color: [0, 1, 0, 1], primitives: new Set([2]) }]);
    f.calls.length = 0;
    r.render(new Float32Array(9), all);
    expect(draws(f)).toEqual([[3, 6], [3, 9]]);
  });

  // Laying out Jetson's board costs about 30 ms, which the board-open mission would count.
  it("draws a deferred renderer's first frame one primitive at a time, and batches after it", () => {
    vi.useFakeTimers();
    try {
      const f = fakeGl();
      const r = new Renderer(f.gl, sheet(), { defer: true });
      expect(f.calls.some((c) => c.name === "bufferData" && c.args[0] === f.gl.ELEMENT_ARRAY_BUFFER)).toBe(false);
      expect(r.render(new Float32Array(9), all)).toBe(4);
      expect(f.count("drawArrays")).toBe(4);
      vi.runAllTimers();
      f.calls.length = 0;
      expect(r.render(new Float32Array(9), all)).toBe(2);
      expect(draws(f)).toEqual([[9, 0], [3, 9]]);
    } finally {
      vi.useRealTimers();
    }
  });

  it("re-lays the index buffer for a new draw order", () => {
    const f = fakeGl();
    const r = new Renderer(f.gl, sheet());
    r.setOrder(Int32Array.from([3, 0, 1, 2]));
    f.calls.length = 0;
    r.render(new Float32Array(9), all);
    // Group 1 first now, then group 0's three.
    expect(draws(f)).toEqual([[3, 0], [9, 3]]);
  });
});
