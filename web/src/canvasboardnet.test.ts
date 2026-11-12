import { describe, it, expect } from "vitest";
import { boardNetDraws } from "./canvas.js";
import { HighlightShape } from "./highlights.js";
import { KIND_TRIANGLES, type DecodedSheet } from "./packed.js";

// A net lit on the board repaints its own copper, rather than drawing a translucent strip over it
// that a track of the same width hides (agni issue 1543). Its pours keep their wash, so lighting a
// ground tints the plane instead of painting the board solid.
describe("boardNetDraws", () => {
  const COPPER = 2, PADS = 3, ZONES = 4;
  const board: DecodedSheet = {
    sheetId: "board", layoutVersion: 2, originX: 0, originY: 0,
    vertices: Int32Array.from([0, 0, 10, 0, 0, 10, 20, 0, 30, 0, 20, 10, 40, 0, 50, 0, 40, 10]),
    primitives: [
      { kind: KIND_TRIANGLES, group: COPPER, firstVertex: 0, count: 3 }, // a track, stroked to triangles
      { kind: KIND_TRIANGLES, group: PADS, firstVertex: 3, count: 3 }, // a pad
      { kind: KIND_TRIANGLES, group: ZONES, firstVertex: 6, count: 3 }, // a pour
    ],
    keys: [], labels: [], images: [], fontFamily: "", groupColors: [], backgroundColor: "",
    groupAlphas: [0, 0, 0, 0, 0.35],
  };
  const lit = { shape: HighlightShape.PATH, color: "#ff0000", alpha: 0.4, primitives: new Set([0, 1, 2]) };

  it("repaints a net's tracks and pads opaque at the default opacity, and its pour at its wash", () => {
    const draws = boardNetDraws(board, lit, [1, 0, 1, 1]) ?? [];
    const byPrimitive = new Map(draws.flatMap((d) => [...d.primitives].map((p) => [p, d.color])));
    expect(byPrimitive.get(0)).toEqual([1, 0, 0, 1]);
    expect(byPrimitive.get(1)).toEqual([1, 0, 0, 1]);
    expect(byPrimitive.get(2)).toEqual([1, 0, 0, 0.35]);
  });

  it("fades with the reader's opacity", () => {
    const draws = boardNetDraws(board, { ...lit, alpha: 0.2 }, [1, 0, 1, 1]) ?? [];
    const byPrimitive = new Map(draws.flatMap((d) => [...d.primitives].map((p) => [p, d.color])));
    expect(byPrimitive.get(0)?.[3]).toBe(0.5);
    expect(byPrimitive.get(2)?.[3]).toBeCloseTo(0.175);
  });

  it("leaves a schematic's net to its path marker", () => {
    expect(boardNetDraws({ ...board, sheetId: "/Power" }, lit, [1, 0, 1, 1])).toBeNull();
    expect(boardNetDraws(board, { ...lit, shape: HighlightShape.BOUNDING_RECT }, [1, 0, 1, 1])).toBeNull();
  });
});
