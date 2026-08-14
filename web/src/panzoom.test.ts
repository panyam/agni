import { describe, it, expect } from "vitest";
import {
  wheelZoomFactor,
  zoomAbout,
  zoomAboutClamped,
  clampScale,
  panBy,
  contentPointAt,
  fitInto,
} from "./panzoom.js";

describe("wheelZoomFactor", () => {
  it("zooms in on a negative delta and out on a positive one", () => {
    expect(wheelZoomFactor(-100)).toBeGreaterThan(1);
    expect(wheelZoomFactor(100)).toBeLessThan(1);
    expect(wheelZoomFactor(0)).toBe(1);
  });

  it("is scale-free: equal and opposite deltas cancel exactly", () => {
    // The whole reason the curve is exponential rather than a fixed step per notch. A user who
    // overshoots and comes back the same distance lands where they started, at any zoom level.
    expect(wheelZoomFactor(-120) * wheelZoomFactor(120)).toBeCloseTo(1, 12);
  });
});

describe("zoomAbout", () => {
  it("holds the content point under the cursor fixed", () => {
    const v = { tx: 40, ty: -30, scale: 2 };
    const [cx, cy] = [250, 180];
    const before = contentPointAt(v, cx, cy);
    const after = contentPointAt(zoomAbout(v, cx, cy, 1.7), cx, cy);
    expect(after.x).toBeCloseTo(before.x, 10);
    expect(after.y).toBeCloseTo(before.y, 10);
  });

  it("scales by the factor", () => {
    expect(zoomAbout({ tx: 0, ty: 0, scale: 1.5 }, 10, 10, 2).scale).toBe(3);
  });
});

describe("zoomAboutClamped", () => {
  it("stops at the limit instead of sliding the content", () => {
    // Clamping AFTER anchoring would keep translating every frame the wheel is held at the limit,
    // which reads as the page crawling away on its own.
    const atMax = { tx: 12, ty: 34, scale: 8 };
    const next = zoomAboutClamped(atMax, 300, 200, 1.4, 0.25, 8);
    expect(next).toEqual(atMax);
  });

  it("applies only the part of the factor that fits, still anchored", () => {
    const v = { tx: 0, ty: 0, scale: 6 };
    const next = zoomAboutClamped(v, 100, 100, 4, 0.25, 8);
    expect(next.scale).toBe(8);
    const before = contentPointAt(v, 100, 100);
    const after = contentPointAt(next, 100, 100);
    expect(after.x).toBeCloseTo(before.x, 10);
    expect(after.y).toBeCloseTo(before.y, 10);
  });
});

describe("panBy", () => {
  it("translates without touching the zoom", () => {
    expect(panBy({ tx: 5, ty: 7, scale: 3 }, -20, 12)).toEqual({ tx: -15, ty: 19, scale: 3 });
  });

  it("keeps the grabbed content point under the pointer", () => {
    const v = { tx: 5, ty: 7, scale: 3 };
    const grabbed = contentPointAt(v, 200, 150);
    const moved = contentPointAt(panBy(v, 40, -25), 240, 125);
    expect(moved.x).toBeCloseTo(grabbed.x, 10);
    expect(moved.y).toBeCloseTo(grabbed.y, 10);
  });
});

describe("clampScale", () => {
  it("bounds on both sides and passes an in-range scale through", () => {
    expect(clampScale(0.1, 0.5, 4)).toBe(0.5);
    expect(clampScale(9, 0.5, 4)).toBe(4);
    expect(clampScale(2, 0.5, 4)).toBe(2);
  });
});

describe("fitInto", () => {
  it("contains the content and centers it", () => {
    // 600x800 content in a 900x400 host: height is the tighter axis.
    const v = fitInto(600, 800, 900, 400, { margin: 0 });
    expect(v.scale).toBeCloseTo(0.5, 10);
    expect(v.tx).toBeCloseTo((900 - 300) / 2, 10);
    expect(v.ty).toBeCloseTo(0, 10);
  });

  it("leaves the requested margin", () => {
    expect(fitInto(100, 100, 200, 200, { margin: 0.1 }).scale).toBeCloseTo(1.8, 10);
  });

  it("centers on the scale it actually got when a limit bites", () => {
    // A clamp applied after centering would leave the content off-center at the limit.
    const v = fitInto(100, 100, 1000, 1000, { margin: 0, maxScale: 2 });
    expect(v.scale).toBe(2);
    expect(v.tx).toBeCloseTo((1000 - 200) / 2, 10);
    expect(v.ty).toBeCloseTo((1000 - 200) / 2, 10);
  });

  it("falls back to scale 1 on a degenerate size rather than 0 or Infinity", () => {
    expect(fitInto(0, 0, 0, 0).scale).toBeGreaterThan(0);
    expect(Number.isFinite(fitInto(0, 0, 0, 0).scale)).toBe(true);
  });
});
