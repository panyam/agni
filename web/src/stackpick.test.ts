import { describe, expect, it } from "vitest";
import { PickIndex } from "./canvaspick.js";
import { KIND_TRIANGLES, type DecodedSheet } from "./packed.js";
import { PRESETS, StackCamera } from "./stackcamera.js";
import { pickThroughStack } from "./stackpick.js";

// Two tracks over one another, a front one in group 6 and an inner one in group 14, both a
// 100 by 10 filled strip, with the inner one longer so it reaches past the front one.
function sheet(): DecodedSheet {
  const strip = (x0: number, x1: number) => [x0, 0, x1, 0, x1, 10, x0, 0, x1, 10, x0, 10];
  const vertices = Int32Array.from([...strip(0, 100), ...strip(0, 200)]);
  return {
    sheetId: "board",
    layoutVersion: 1,
    originX: 0,
    originY: 0,
    vertices,
    primitives: [
      { kind: KIND_TRIANGLES, group: 6, firstVertex: 0, count: 6 },
      { kind: KIND_TRIANGLES, group: 14, firstVertex: 6, count: 6 },
    ],
    keys: [
      { primitive: 0, refDes: "", net: "TOP", netId: "", pin: "", busId: "", placement: 0 },
      { primitive: 1, refDes: "", net: "INNER", netId: "", pin: "", busId: "", placement: 0 },
    ],
    labels: [],
    images: [],
    fontFamily: "",
  } as unknown as DecodedSheet;
}

describe("picking through the stack", () => {
  const planes = [
    { z: 50, groups: new Set([6]) },
    { z: -20, groups: new Set([14]) },
  ];
  const all = new Set([6, 14]);
  const setup = (pose: keyof typeof PRESETS) => {
    const s = sheet();
    const picks = new PickIndex(s);
    const cam = new StackCamera({ x: 100, y: 5, z: 0 }, 220);
    cam.setViewport(800, 800);
    cam.setPose(PRESETS[pose]);
    cam.fit();
    const pick = (hidden: number[], at: { x: number; y: number; z: number }) => {
      const px = cam.project(at);
      return pickThroughStack(cam, planes, all, hidden, px.x, px.y, 1, (x, y, perPx, h) => picks.pick(x, y, perPx, h));
    };
    return { pick };
  };

  it("picks an inner track where nothing nearer covers it, by the same keys as the flat board", () => {
    const { pick } = setup("iso");
    expect(pick([], { x: 160, y: 5, z: -20 })).toMatchObject({ kind: "net", net: "INNER" });
  });

  it("picks the nearest plane first, and skips a layer the reader hid", () => {
    const { pick } = setup("top");
    expect(pick([], { x: 50, y: 5, z: 50 })).toMatchObject({ kind: "net", net: "TOP" });
    expect(pick([6], { x: 50, y: 5, z: 50 })).toMatchObject({ kind: "net", net: "INNER" });
  });

  it("picks the other side's layer first from below", () => {
    const s = sheet();
    const picks = new PickIndex(s);
    const cam = new StackCamera({ x: 100, y: 5, z: 0 }, 220);
    cam.setViewport(800, 800);
    cam.setPose({ yaw: 0, pitch: -90 });
    cam.fit();
    const px = cam.project({ x: 50, y: 5, z: -20 });
    expect(pickThroughStack(cam, planes, all, [], px.x, px.y, 1, (x, y, perPx, h) => picks.pick(x, y, perPx, h))).toMatchObject({ net: "INNER" });
  });

  it("picks nothing when the planes are seen edge on", () => {
    const s = sheet();
    const picks = new PickIndex(s);
    const cam = new StackCamera({ x: 100, y: 5, z: 0 }, 220);
    cam.setViewport(800, 800);
    cam.setPose({ yaw: 0, pitch: 0 });
    cam.fit();
    expect(pickThroughStack(cam, planes, all, [], 400, 400, 1, (x, y, perPx, h) => picks.pick(x, y, perPx, h))).toBeNull();
  });
});
