import { describe, it, expect } from "vitest";
import { enableBlending, hexToRgba } from "./webgl.js";

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
