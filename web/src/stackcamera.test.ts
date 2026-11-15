import { describe, expect, it } from "vitest";
import { PRESETS, StackCamera, basisOf } from "./stackcamera.js";

describe("the stack camera", () => {
  it("looks straight down from the top preset, and at the front edge from the front preset", () => {
    const top = basisOf(PRESETS.top);
    expect(top.eye.z).toBeCloseTo(1, 9);
    expect(top.up.y).toBeCloseTo(1, 9);
    expect(top.right.x).toBeCloseTo(1, 9);
    const front = basisOf(PRESETS.front);
    expect(front.eye.y).toBeLessThan(-0.9);
    expect(front.up.z).toBeGreaterThan(0.9);
    const side = basisOf(PRESETS.side);
    expect(side.eye.x).toBeLessThan(-0.9);
  });

  it("finds the world point it drew at a pixel, on any plane, from any preset", () => {
    for (const pose of Object.values(PRESETS)) {
      const cam = new StackCamera({ x: 500, y: 300, z: 0 }, 1000);
      cam.setViewport(800, 600);
      cam.setPose(pose);
      cam.fit();
      for (const p of [{ x: 520, y: 310, z: 40 }, { x: 100, y: 700, z: -35 }]) {
        const px = cam.project(p);
        const hit = cam.hitPlane(px.x, px.y, p.z)!;
        expect(hit.x).toBeCloseTo(p.x, 6);
        expect(hit.y).toBeCloseTo(p.y, 6);
      }
    }
  });

  it("maps a world point into clip space where project puts it on the screen", () => {
    const cam = new StackCamera({ x: 0, y: 0, z: 0 }, 100);
    cam.setViewport(400, 200);
    cam.setPose(PRESETS.iso);
    cam.fit();
    const m = cam.matrix();
    const p = { x: 30, y: -20, z: 12 };
    const clip = [0, 1, 2].map((r) => m[r] * p.x + m[4 + r] * p.y + m[8 + r] * p.z + m[12 + r]);
    const px = cam.project(p);
    expect((clip[0] + 1) * 200).toBeCloseTo(px.x, 3);
    expect((1 - clip[1]) * 100).toBeCloseTo(px.y, 3);
    // A point nearer the eye has a smaller depth.
    const eye = cam.basis().eye;
    const near = { x: p.x + eye.x * 10, y: p.y + eye.y * 10, z: p.z + eye.z * 10 };
    const zNear = m[2] * near.x + m[6] * near.y + m[10] * near.z + m[14];
    expect(zNear).toBeLessThan(clip[2]);
  });

  it("keeps the point under the cursor in place when it zooms", () => {
    const cam = new StackCamera({ x: 0, y: 0, z: 0 }, 100);
    cam.setViewport(400, 400);
    cam.setPose(PRESETS.top);
    cam.fit();
    const before = cam.hitPlane(300, 120, 0)!;
    cam.zoomAt(300, 120, 2);
    const after = cam.hitPlane(300, 120, 0)!;
    expect(after.x).toBeCloseTo(before.x, 6);
    expect(after.y).toBeCloseTo(before.y, 6);
  });
});
