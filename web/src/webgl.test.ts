import { describe, it, expect } from "vitest";
import { hexToRgba } from "./webgl.js";

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
