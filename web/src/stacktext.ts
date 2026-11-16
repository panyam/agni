// Text in the stack view (agni issue 1607). The text overlay bakes a sheet's labels once, in a
// Y-flipped world space, and the flat view moves them with one CSS transform. The stack camera is
// orthographic, so a plane at one height maps onto the screen by a flat transform too: one matrix per
// layer puts that layer's text on its plane at any angle. The labels draw over whatever stands in
// front of them, part blocks included; issue 1657 holds the texture approach that would hide them.

import type { StackCamera, Vec3 } from "./stackcamera.js";
import { bodyTopZ, enterPrism, type BodyFaces, type PlacedBody } from "./stackbodies.js";

// Matrix is an SVG matrix(a, b, c, d, e, f): screen = (a X + c Y + e, b X + d Y + f).
export type Matrix = [number, number, number, number, number, number];

// planeMatrix maps the overlay's baked coordinates on the plane at world z to CSS pixels. The overlay
// bakes a world point (x, y) at (x, -y) / bakeScale. project gives a world point's device pixel, and
// dpr turns that into CSS pixels.
export function planeMatrix(project: (p: Vec3) => { x: number; y: number }, z: number, bakeScale: number, dpr: number): Matrix {
  const o = project({ x: 0, y: 0, z });
  const ex = project({ x: 1, y: 0, z });
  const ey = project({ x: 0, y: 1, z });
  const k = bakeScale / dpr;
  return [(ex.x - o.x) * k, (ex.y - o.y) * k, -(ey.x - o.x) * k, -(ey.y - o.y) * k, o.x / dpr, o.y / dpr];
}

// faceShows says whether text on a face reads from where the camera sits. The top face's text reads
// from above, and the bottom face's from below. Text on no face, such as an exploded inner layer's,
// reads from either side.
export function faceShows(face: "top" | "bottom" | undefined, eyeZ: number): boolean {
  if (face === "top") return eyeZ > 0;
  if (face === "bottom") return eyeZ < 0;
  return true;
}

// LabelAnchor is where a label sits in the stack view's world: its anchor point on its plane.
export interface LabelAnchor {
  x: number;
  y: number;
  z: number;
}

// occludedLabels lists the labels whose anchor a part's block hides from the camera (agni issue
// 1657). The text overlay draws over the scene, so the stack view hides each label whose anchor lies
// behind a block, a whole label at a time. A block covers a label when the ray from the camera
// through the label's anchor enters the block before it reaches the anchor. Each block is first
// boxed on the screen, so a label tests only the blocks over its own pixel.
export function occludedLabels(camera: StackCamera, labels: readonly LabelAnchor[], bodies: readonly PlacedBody[], f: BodyFaces): Set<number> {
  const out = new Set<number>();
  if (labels.length === 0 || bodies.length === 0) return out;
  const blocks = bodies.map((b) => {
    const top = bodyTopZ(b, f);
    const face = b.back ? f.zBottom : f.zTop;
    const z0 = Math.min(face, top), z1 = Math.max(face, top);
    let x0 = Infinity, y0 = Infinity, x1 = -Infinity, y1 = -Infinity;
    for (let i = 0; i + 1 < b.ring.length; i += 2) {
      for (const z of [z0, z1]) {
        const p = camera.project({ x: b.ring[i], y: b.ring[i + 1], z });
        x0 = Math.min(x0, p.x);
        y0 = Math.min(y0, p.y);
        x1 = Math.max(x1, p.x);
        y1 = Math.max(y1, p.y);
      }
    }
    return { ring: b.ring, z0, z1, x0, y0, x1, y1 };
  });
  labels.forEach((l, i) => {
    const px = camera.project(l);
    const { o, dir } = camera.ray(px.x, px.y);
    const sLabel = (l.x - o.x) * dir.x + (l.y - o.y) * dir.y + (l.z - o.z) * dir.z;
    for (const b of blocks) {
      if (px.x < b.x0 || px.x > b.x1 || px.y < b.y0 || px.y > b.y1) continue;
      const s = enterPrism(b.ring, b.z0, b.z1, o, dir);
      if (s !== null && s < sLabel - 1e-9) {
        out.add(i);
        return;
      }
    }
  });
  return out;
}
