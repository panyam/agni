// Text in the stack view (agni issue 1607). The text overlay bakes a sheet's labels once, in a
// Y-flipped world space, and the flat view moves them with one CSS transform. The stack camera is
// orthographic, so a plane at one height maps onto the screen by a flat transform too: one matrix per
// layer puts that layer's text on its plane at any angle. The labels draw over whatever stands in
// front of them, part blocks included; issue 1657 holds the texture approach that would hide them.

import type { Vec3 } from "./stackcamera.js";

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
