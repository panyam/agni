import { describe, expect, it } from "vitest";
import { PRESETS, StackCamera } from "./stackcamera.js";
import { faceShows, planeMatrix } from "./stacktext.js";

describe("text on a stack plane", () => {
  const cam = new StackCamera({ x: 100, y: 50, z: 0 }, 200);
  cam.setViewport(800, 600);
  cam.setPose(PRESETS.iso);
  cam.fit();

  it("puts a baked point where the camera projects its world point, at any angle", () => {
    const bake = 1000;
    const dpr = 2;
    for (const z of [0, -30, 25]) {
      const [a, b, c, d, e, f] = planeMatrix((p) => cam.project(p), z, bake, dpr);
      for (const [x, y] of [
        [0, 0],
        [200, 0],
        [200, 100],
        [37, 81],
      ]) {
        const X = x / bake, Y = -y / bake;
        const want = cam.project({ x, y, z });
        expect(a * X + c * Y + e).toBeCloseTo(want.x / dpr, 6);
        expect(b * X + d * Y + f).toBeCloseTo(want.y / dpr, 6);
      }
    }
  });

  it("shows a face's text only from its own side", () => {
    expect([faceShows("top", 0.5), faceShows("top", -0.5), faceShows("bottom", -0.5), faceShows("bottom", 0.5)]).toEqual([true, false, true, false]);
    expect([faceShows(undefined, 0.5), faceShows(undefined, -0.5)]).toEqual([true, true]);
  });
});
