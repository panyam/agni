import { describe, it, expect } from "vitest";
import { GLIDE_MS, Glide, easeInOut, lerpCamera, lerpSvgView } from "./glide.js";
import { manualClock } from "./testclock.js";

const last = (xs: number[]): number => xs[xs.length - 1];

// A view travels to a revealed subject instead of jumping (agni issue 1546).
describe("glide", () => {
  it("eases in and out, from 0 to 1", () => {
    expect(easeInOut(0)).toBe(0);
    expect(easeInOut(0.5)).toBe(0.5);
    expect(easeInOut(1)).toBe(1);
    expect(easeInOut(0.1)).toBeLessThan(0.1);
  });

  it("zooms the camera on a log scale and moves its centre in a straight line", () => {
    const a = { centerX: 0, centerY: 0, worldPerPixel: 100 };
    const b = { centerX: 10, centerY: -20, worldPerPixel: 1 };
    const mid = lerpCamera(a, b, 0.5);
    expect(mid.centerX).toBe(5);
    expect(mid.centerY).toBe(-10);
    expect(mid.worldPerPixel).toBeCloseTo(10);
  });

  it("moves an SVG view's centre point in a straight line while its scale grows on a log scale", () => {
    const w = 400, h = 300;
    const a = { tx: 0, ty: 0, scale: 1 }; // centre (200, 150)
    const b = { tx: w / 2 - 100 * 16, ty: h / 2 - 50 * 16, scale: 16 }; // centre (100, 50)
    const mid = lerpSvgView(a, b, w, h, 0.5);
    expect(mid.scale).toBeCloseTo(4);
    expect((w / 2 - mid.tx) / mid.scale).toBeCloseTo(150);
    expect((h / 2 - mid.ty) / mid.scale).toBeCloseTo(100);
    expect(lerpSvgView(a, b, w, h, 1)).toEqual(b);
  });

  it("reaches the target only when its time is up, passing through the views between", () => {
    const clock = manualClock();
    const seen: number[] = [];
    new Glide(clock, () => true).run(0, 100, (a, b, t) => a + (b - a) * t, (v) => seen.push(v));
    expect(seen).toEqual([]);
    clock.step(GLIDE_MS / 2);
    expect(last(seen)).toBe(50);
    clock.step(GLIDE_MS);
    expect(last(seen)).toBe(100);
    const n = seen.length;
    clock.step(GLIDE_MS);
    expect(seen.length).toBe(n);
  });

  it("jumps at once where motion is not allowed", () => {
    const seen: number[] = [];
    new Glide(manualClock(), () => false).run(0, 100, (a, b, t) => a + (b - a) * t, (v) => seen.push(v));
    expect(seen).toEqual([100]);
  });

  it("lets a new run replace one in flight, and stop leave the view where it is", () => {
    const clock = manualClock();
    const g = new Glide(clock, () => true);
    const seen: number[] = [];
    const lerp = (a: number, b: number, t: number) => a + (b - a) * t;
    g.run(0, 100, lerp, (v) => seen.push(v));
    clock.step(GLIDE_MS / 2);
    g.run(50, -100, lerp, (v) => seen.push(v));
    clock.step(GLIDE_MS * 2);
    expect(last(seen)).toBe(-100);
    expect(seen).not.toContain(100);
    g.run(0, 100, lerp, (v) => seen.push(v));
    clock.step(GLIDE_MS / 2);
    g.stop();
    expect(g.active).toBe(false);
    const n = seen.length;
    clock.step(GLIDE_MS);
    expect(seen.length).toBe(n);
  });
});
