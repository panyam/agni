import { describe, it, expect } from "vitest";
import { Renderer, enableBlending, hexToRgba, strokeQuads } from "./webgl.js";
import { KIND_LINE_LOOP, KIND_LINE_STRIP, type DecodedSheet } from "./packed.js";
import { fakeGl } from "./fakegl.js";

// hexToRgba turns the server-chosen palette ("#rrggbb") into GL's 0..1 RGBA, so WebGL colors
// from the same render.Style as the SVG backend.
describe("hexToRgba", () => {
  const fb: [number, number, number, number] = [0.5, 0.5, 0.5, 1];

  it("parses #rrggbb", () => {
    expect(hexToRgba("#0a7d2c", fb)).toEqual([0x0a / 255, 0x7d / 255, 0x2c / 255, 1]);
  });
  it("parses #rgb shorthand", () => {
    expect(hexToRgba("#333", fb)).toEqual([0x33 / 255, 0x33 / 255, 0x33 / 255, 1]);
  });
  it("falls back on an unparseable value", () => {
    expect(hexToRgba("nope", fb)).toBe(fb);
    expect(hexToRgba("", fb)).toBe(fb);
  });
});

// A translucent draw must leave the canvas opaque, or the browser's premultiplied compositing adds
// the page behind it and a zone's 18% wash shows as white (agni issue 1334).
describe("enableBlending", () => {
  it("blends colour by the source alpha and keeps the canvas's alpha at one", () => {
    const calls: unknown[][] = [];
    const gl = {
      BLEND: 0x0be2,
      SRC_ALPHA: 0x0302,
      ONE_MINUS_SRC_ALPHA: 0x0303,
      ONE: 1,
      enable: (c: number) => void calls.push(["enable", c]),
      blendFuncSeparate: (...a: number[]) => void calls.push(["separate", ...a]),
    } as const;
    enableBlending(gl);
    // The alpha factors are ONE and ONE_MINUS_SRC_ALPHA, so a draw at alpha a over an opaque canvas
    // leaves a + (1 - a) = 1.
    expect(calls).toEqual([["enable", 0x0be2], ["separate", 0x0302, 0x0303, 1, 0x0303]]);
  });
});

// A sheet switch builds a new Renderer on the same context, and a context keeps every object until
// it is deleted (agni issue 1331).
describe("Renderer.dispose", () => {
  it("deletes the vertex buffer, the overlay's buffer, both VAOs and the program", () => {
    const f = fakeGl();
    const r = new Renderer(f.gl, tinySheet());
    r.setOverlays([{ color: [1, 0, 0, 1], vertices: Int32Array.from([0, 0, 10, 0, 0, 10]) }]);
    const created = (kind: string) => f.calls.filter((c) => c.name === `create${kind}`).map((c) => c.args);
    expect(created("Buffer")).toHaveLength(2);
    r.dispose();
    const deleted = (name: string) => f.calls.filter((c) => c.name === name).map((c) => c.args[0]);
    expect(deleted("deleteBuffer")).toHaveLength(2);
    expect(deleted("deleteVertexArray")).toHaveLength(2);
    expect(deleted("deleteProgram")).toHaveLength(1);
    expect(new Set([...deleted("deleteBuffer"), ...deleted("deleteVertexArray"), ...deleted("deleteProgram")]).size).toBe(5);
  });

  it("counts a frame's draw calls", () => {
    const f = fakeGl();
    const r = new Renderer(f.gl, tinySheet());
    const view = { minX: -1e9, minY: -1e9, maxX: 1e9, maxY: 1e9 };
    expect(r.render(new Float32Array(9), view)).toBe(2);
    expect(f.count("drawArrays")).toBe(2);
  });
});

// A board's courtyard and silkscreen lines carry the width their source strokes them at, and SVG
// draws them that wide (agni issue 1486). GL lines are 1px whatever the zoom.
describe("a line with a width", () => {
  // A 10 x 10 square loop, 2 wide, beside a hairline strip.
  const sheet = (): DecodedSheet => ({
    ...tinySheet(),
    vertices: Int32Array.from([0, 0, 10, 0, 10, 10, 0, 10, 0, 20, 10, 20]),
    primitives: [
      { kind: KIND_LINE_LOOP, group: 0, firstVertex: 0, count: 4, width: 2 },
      { kind: KIND_LINE_STRIP, group: 0, firstVertex: 4, count: 2 },
    ],
  });
  const view = { minX: -1e9, minY: -1e9, maxX: 1e9, maxY: 1e9 };
  const draws = (f: ReturnType<typeof fakeGl>) => f.calls.filter((c) => c.name === "drawArrays").map((c) => c.args);

  it("draws as triangles once its width spans a pixel, and as a GL line below that", () => {
    const f = fakeGl();
    const r = new Renderer(f.gl, sheet());
    r.render(new Float32Array(9), view, 2);
    // Four sides of two triangles each, after the sheet's six vertices.
    expect(draws(f)).toEqual([[f.gl.TRIANGLES, 6, 24], [f.gl.LINE_STRIP, 4, 2]]);
    f.calls.length = 0;
    r.render(new Float32Array(9), view, 2.5);
    expect(draws(f)).toEqual([[f.gl.LINE_LOOP, 0, 4], [f.gl.LINE_STRIP, 4, 2]]);
  });

  it("uploads the sheet's vertices with the stroke's triangles after them", () => {
    const f = fakeGl();
    new Renderer(f.gl, sheet());
    const upload = f.calls.find((c) => c.name === "bufferData")!.args[1] as Int32Array;
    expect(upload.length).toBe((6 + 24) * 2);
    expect(Array.from(upload.subarray(0, 12))).toEqual(Array.from(sheet().vertices));
  });

  it("runs each segment on by half the width, so the corners close", () => {
    const v = Int32Array.from([0, 0, 10, 0]);
    // One segment from (0,0) to (10,0), 2 wide, covers x from -1 to 11 and y from -1 to 1.
    expect(strokeQuads(v, { kind: KIND_LINE_STRIP, group: 0, firstVertex: 0, count: 2 }, 2)).toEqual([
      -1, 1, -1, -1, 11, 1,
      -1, -1, 11, -1, 11, 1,
    ]);
  });
});

// tinySheet is two line strips on group 0.
function tinySheet(): DecodedSheet {
  return {
    sheetId: "S", layoutVersion: 1, originX: 0, originY: 0,
    vertices: Int32Array.from([0, 0, 10, 0, 0, 10, 10, 10]),
    primitives: [
      { kind: KIND_LINE_STRIP, group: 0, firstVertex: 0, count: 2 },
      { kind: KIND_LINE_STRIP, group: 0, firstVertex: 2, count: 2 },
    ],
    keys: [], labels: [], images: [], fontFamily: "", groupColors: ["#000000"], backgroundColor: "#ffffff",
  };
}
