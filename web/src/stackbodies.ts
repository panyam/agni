// Part bodies on a board's stack view (agni issue 1352). Each placed part stands on its side's face
// of the board as its footprint's hull, raised to the height the file states. A part whose file
// states no height stands at TYPICAL_BODY_MM and draws apart, so a reader can tell the file never
// said it.
//
// A height draws at the board's true scale whatever the explode control says. The explode control
// spreads the layers to show the stack, and a part would tower over the board if it spread too.

import type { BoardPartBody, BoardStackView } from "./gen/agni/v1/geom/geom_packed_pb.js";
import type { Selection } from "./selection.js";
import type { StackCamera } from "./stackcamera.js";

// TYPICAL_BODY_MM is the height a part whose file states none draws at.
export const TYPICAL_BODY_MM = 1;

// defaultedNote says how many of a stack's parts stand at TYPICAL_BODY_MM because their file states
// no height, as the stack bar shows it beside its Parts toggle. It is "" when every part states one.
export function defaultedNote(view: Pick<BoardStackView, "bodies">): string {
  const n = view.bodies.filter((b) => b.heightMm === undefined || b.heightMm <= 0).length;
  if (n === 0) return "";
  const of = n === view.bodies.length ? "all" : `${n} of ${view.bodies.length}`;
  return ` (${of} at a default ${TYPICAL_BODY_MM} mm)`;
}

// BODY_FLOATS is the length of one vertex: x, y, the height in millimetres, and a code holding the
// shade and the side. The code is the shade, from 0 to 1, plus 2 for a part on the back.
export const BODY_FLOATS = 4;

// LIGHT is the direction the walls take their shade from, in the board's plane. A wall facing it
// draws brightest, and the top face draws at full shade.
const LIGHT: [number, number] = [-0.6, 0.8];

// BodyRun is the run of vertices one part draws.
export interface BodyRun {
  ref: string;
  first: number;
  count: number;
}

// PlacedBody is one part as the picker reads it: its ring rebased to the packed sheet's origin, its
// height in millimetres, and its side.
export interface PlacedBody {
  ref: string;
  ring: Float32Array;
  heightMm: number;
  back: boolean;
  defaulted: boolean;
}

// BodyGeometry is what the stack view draws for the parts. The stated parts come first and the
// defaulted ones after, so each set draws in one call. stated is the vertex count of the first set.
export interface BodyGeometry {
  verts: Float32Array;
  stated: number;
  runs: Map<string, BodyRun>;
  bodies: PlacedBody[];
}

// bodyGeometry builds the bodies' triangles. originX and originY are the packed sheet's origin, which
// the outlines' board coordinates rebase to. A body with fewer than three points draws nothing.
export function bodyGeometry(bodies: readonly BoardPartBody[], originX: number, originY: number): BodyGeometry {
  const placed: PlacedBody[] = [];
  for (const b of bodies) {
    const n = Math.floor(b.outline.length / 2);
    if (n < 3) continue;
    const ring = new Float32Array(2 * n);
    for (let i = 0; i < n; i++) {
      ring[2 * i] = b.outline[2 * i] - originX;
      ring[2 * i + 1] = b.outline[2 * i + 1] - originY;
    }
    const stated = b.heightMm !== undefined && b.heightMm > 0;
    placed.push({ ref: b.refDes, ring, heightMm: stated ? b.heightMm! : TYPICAL_BODY_MM, back: b.side === "back", defaulted: !stated });
  }
  placed.sort((a, b) => Number(a.defaulted) - Number(b.defaulted));
  const out: number[] = [];
  const runs = new Map<string, BodyRun>();
  let stated = 0;
  for (const p of placed) {
    const first = out.length / BODY_FLOATS;
    pushBody(out, p);
    runs.set(p.ref, { ref: p.ref, first, count: out.length / BODY_FLOATS - first });
    if (!p.defaulted) stated = out.length / BODY_FLOATS;
  }
  return { verts: Float32Array.from(out), stated, runs, bodies: placed };
}

// pushBody adds one body's top face as a fan, since the ring is convex, and a quad for each wall.
function pushBody(out: number[], p: PlacedBody): void {
  const r = p.ring;
  const n = r.length / 2;
  const side = p.back ? 2 : 0;
  const h = p.heightMm;
  const v = (i: number, height: number, shade: number) => out.push(r[2 * i], r[2 * i + 1], height, shade + side);
  for (let i = 1; i + 1 < n; i++) {
    v(0, h, 1);
    v(i, h, 1);
    v(i + 1, h, 1);
  }
  const len = Math.hypot(LIGHT[0], LIGHT[1]);
  for (let i = 0; i < n; i++) {
    const j = (i + 1) % n;
    const dx = r[2 * j] - r[2 * i];
    const dy = r[2 * j + 1] - r[2 * i + 1];
    const d = Math.hypot(dx, dy) || 1;
    // A counter-clockwise ring's outward normal is the edge turned clockwise.
    const facing = ((dy / d) * LIGHT[0] - (dx / d) * LIGHT[1]) / len;
    const shade = 0.55 + 0.3 * Math.max(0, facing);
    v(i, 0, shade);
    v(j, 0, shade);
    v(j, h, shade);
    v(i, 0, shade);
    v(j, h, shade);
    v(i, h, shade);
  }
}

// BodyFaces is where the board's two faces draw in world z, and how many world units one millimetre
// of a part's height spans.
export interface BodyFaces {
  zTop: number;
  zBottom: number;
  hScale: number;
}

// bodyTopZ is the world z of a body's top face. A part on the back hangs below the bottom face.
export function bodyTopZ(b: PlacedBody, f: BodyFaces): number {
  return b.back ? f.zBottom - b.heightMm * f.hScale : f.zTop + b.heightMm * f.hScale;
}

// pickBody is the part whose top face the ray through device pixel (px, py) meets nearest the eye,
// among the parts on one side, or null. A body's walls do not pick, so a body seen edge on picks
// through to the layers.
export function pickBody(camera: StackCamera, bodies: readonly PlacedBody[], f: BodyFaces, back: boolean, px: number, py: number): string | null {
  const eyeZ = camera.basis().eye.z;
  let best: string | null = null;
  let bestNear = -Infinity;
  for (const b of bodies) {
    if (b.back !== back) continue;
    const z = bodyTopZ(b, f);
    const at = camera.hitPlane(px, py, z);
    if (!at || !insideConvex(b.ring, at.x, at.y)) continue;
    if (z * eyeZ > bestNear) {
      bestNear = z * eyeZ;
      best = b.ref;
    }
  }
  return best;
}

// pickWithBodies picks at device pixel (px, py) in the order the stack draws from the eye. The parts
// on the side facing the camera stand nearest, then come the layers, which pickLayers picks, then the
// parts hanging off the far face. A body picks as its part, as a click on its pads does.
export function pickWithBodies(
  camera: StackCamera,
  bodies: readonly PlacedBody[],
  f: BodyFaces,
  px: number,
  py: number,
  pickLayers: () => Selection | null,
): Selection | null {
  const fromBelow = camera.basis().eye.z < 0;
  const part = (ref: string | null): Selection | null => (ref ? { kind: "component", ref } : null);
  return part(pickBody(camera, bodies, f, fromBelow, px, py)) ?? pickLayers() ?? part(pickBody(camera, bodies, f, !fromBelow, px, py));
}

// insideConvex says whether (x, y) sits in a counter-clockwise convex ring, its edges included.
export function insideConvex(ring: Float32Array, x: number, y: number): boolean {
  const n = ring.length / 2;
  for (let i = 0; i < n; i++) {
    const j = (i + 1) % n;
    const ax = ring[2 * i], ay = ring[2 * i + 1];
    if ((ring[2 * j] - ax) * (y - ay) - (ring[2 * j + 1] - ay) * (x - ax) < 0) return false;
  }
  return n >= 3;
}
