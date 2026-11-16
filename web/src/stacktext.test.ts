import { describe, expect, it } from "vitest";
import { PRESETS, StackCamera } from "./stackcamera.js";
import { faceShows, occludedLabels, planeMatrix } from "./stacktext.js";
import { bodyGeometry } from "./stackbodies.js";
import { create } from "@bufbuild/protobuf";
import { BoardPartBodySchema } from "@panyam/agni-client/gen/agni/v1/geom/geom_packed_pb";

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

// A part's block hides the labels behind it from the camera, a whole label at a time (agni issue
// 1657). U1 stands 2.5 mm tall on the top face at z 1, on the square from (10, 10) to (20, 20).
describe("labels a block hides", () => {
  const bodies = bodyGeometry([create(BoardPartBodySchema, { refDes: "U1", side: "front", outline: [10, 10, 20, 10, 20, 20, 10, 20], heightMm: 2.5, color: "#202024" })], 0, 0).bodies;
  const faces = { zTop: 1, zBottom: -1, hScale: 1 };
  const pose = (p: keyof typeof PRESETS) => {
    const c = new StackCamera({ x: 50, y: 50, z: 0 }, 120);
    c.setViewport(800, 800);
    c.setPose(PRESETS[p]);
    c.fit();
    return c;
  };
  // From Iso the camera sits towards +x, -y and up. under lies beneath the block, behind lies on
  // the face past the block's far corner, and clear lies well away from it.
  const under = { x: 12.4, y: 18.1, z: 1 };
  const behind = { x: 17.4, y: 23.06, z: 1 };
  const clear = { x: 40, y: 40, z: 1 };

  it("hides the labels the block covers from Iso, and shows the one in the clear", () => {
    expect([...occludedLabels(pose("iso"), [under, behind, clear], bodies, faces)].sort()).toEqual([0, 1]);
  });

  it("hides only the label under the block from Top", () => {
    expect([...occludedLabels(pose("top"), [under, behind, clear], bodies, faces)]).toEqual([0]);
  });

  it("hides nothing when there are no blocks", () => {
    expect(occludedLabels(pose("iso"), [under, behind, clear], [], faces).size).toBe(0);
  });
});
