import { describe, it, expect } from "vitest";
import { UPRIGHT, flip, flipped, fromBelow, orientBox, orientPoint, orientedSize, rotateBy, unorientPoint, type Orientation, type Rotation } from "./orient.js";

const W = 200;
const H = 100;
const ALL: Orientation[] = ([0, 90, 180, 270] as Rotation[]).flatMap((rot) => [false, true].map((flipH) => ({ rot, flipH, flipV: false })));

describe("an orientation of a 200 by 100 drawing", () => {
  it("swaps width and height on a quarter turn", () => {
    expect(orientedSize({ ...UPRIGHT, rot: 90 }, W, H)).toEqual({ w: 100, h: 200 });
    expect(orientedSize({ ...UPRIGHT, rot: 180 }, W, H)).toEqual({ w: 200, h: 100 });
  });

  it("maps the drawing's corner to where each orientation puts it", () => {
    const tl = { x: 0, y: 0 };
    expect(orientPoint(UPRIGHT, W, H, tl)).toEqual({ x: 0, y: 0 });
    // A quarter turn clockwise on screen takes the top-left corner to the top-right.
    expect(orientPoint({ ...UPRIGHT, rot: 90 }, W, H, tl)).toEqual({ x: 100, y: 0 });
    expect(orientPoint({ ...UPRIGHT, rot: 180 }, W, H, tl)).toEqual({ x: 200, y: 100 });
    expect(orientPoint({ ...UPRIGHT, rot: 270 }, W, H, tl)).toEqual({ x: 0, y: 200 });
    expect(orientPoint({ ...UPRIGHT, flipH: true }, W, H, tl)).toEqual({ x: 200, y: 0 });
  });

  it("keeps every point inside the oriented drawing, and undoes itself, in all eight orientations", () => {
    expect(ALL.length).toBe(8);
    for (const o of ALL) {
      const s = orientedSize(o, W, H);
      for (const p of [{ x: 0, y: 0 }, { x: 200, y: 100 }, { x: 37, y: 81 }]) {
        const q = orientPoint(o, W, H, p);
        expect(q.x).toBeGreaterThanOrEqual(0);
        expect(q.y).toBeGreaterThanOrEqual(0);
        expect(q.x).toBeLessThanOrEqual(s.w);
        expect(q.y).toBeLessThanOrEqual(s.h);
        const back = unorientPoint(o, W, H, q);
        expect(back.x).toBeCloseTo(p.x);
        expect(back.y).toBeCloseTo(p.y);
      }
    }
  });

  it("draws a vertical flip as a horizontal flip plus a half turn", () => {
    const v = { ...UPRIGHT, flipV: true };
    const h180 = { rot: 180 as Rotation, flipH: true, flipV: false };
    for (const p of [{ x: 0, y: 0 }, { x: 37, y: 81 }]) expect(orientPoint(v, W, H, p)).toEqual(orientPoint(h180, W, H, p));
  });

  it("maps a box to the box around its turned corners", () => {
    expect(orientBox({ ...UPRIGHT, rot: 90 }, W, H, { x: 10, y: 20, w: 30, h: 5 })).toEqual({ x: 75, y: 10, w: 5, h: 30 });
  });

  it("flips as the reader sees it, whatever the turn", () => {
    const turned = { ...UPRIGHT, rot: 90 as Rotation };
    // On a quarter-turned drawing, a flip left to right on screen is the drawing's own vertical flip.
    const flipped = flip(turned, "h");
    expect(flipped).toEqual({ rot: 90, flipH: false, flipV: true });
    const a = orientPoint(turned, W, H, { x: 37, y: 81 });
    const b = orientPoint(flipped, W, H, { x: 37, y: 81 });
    expect(b.x).toBeCloseTo(100 - a.x);
    expect(b.y).toBeCloseTo(a.y);
  });

  it("says a flip is on along the screen axis whose flip turned it on, whatever the turn", () => {
    for (const rot of [0, 90, 180, 270] as Rotation[]) {
      const o = { ...UPRIGHT, rot };
      expect([flipped(flip(o, "h"), "h"), flipped(flip(o, "h"), "v")], `rot ${rot}`).toEqual([true, false]);
      expect([flipped(flip(o, "v"), "h"), flipped(flip(o, "v"), "v")], `rot ${rot}`).toEqual([false, true]);
    }
  });

  it("sees the board from below after an odd number of flips", () => {
    let o = UPRIGHT;
    expect(fromBelow(o)).toBe(false);
    o = flip(o, "h");
    expect(fromBelow(o)).toBe(true);
    o = flip(rotateBy(o), "h");
    expect(fromBelow(o)).toBe(false);
    o = flip(o, "v");
    expect(fromBelow(o)).toBe(true);
  });

  it("turns a quarter at a time and comes back round", () => {
    let o = UPRIGHT;
    for (let i = 0; i < 4; i++) o = rotateBy(o);
    expect(o).toEqual(UPRIGHT);
    expect(rotateBy(UPRIGHT, -1).rot).toBe(270);
  });
});
