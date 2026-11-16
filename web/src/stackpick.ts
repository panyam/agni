// Picking through a board drawn as its exploded stack (agni issue 1593). A click is a ray into the
// scene. It meets each layer's plane at one point, and the nearest plane with something under that
// point answers. Each plane asks the flat board's pick index with every other plane's groups hidden,
// so a pick keys a part, pad or net exactly as a click on the flat board does.

import type { Selection } from "./selection.js";
import type { StackCamera } from "./stackcamera.js";

// StackPickPlane is one depth the stack draws groups at, with its world z.
export interface StackPickPlane {
  z: number;
  groups: ReadonlySet<number>;
}

// FlatPick is the flat board's pick at a world point, with a world size per CSS pixel and the groups
// to skip.
export type FlatPick = (x: number, y: number, perPx: number, hidden: ReadonlySet<number>) => Selection | null;

// GRAZING is the least eye height that still picks. Below it the planes are seen edge on, and a
// pixel spans most of the board.
export const GRAZING = 0.05;

// Barrel is one plated hole as the stack view draws it: a vertical cylinder of radius r about (x, y),
// from world z zTop down to zBottom, on its net.
export interface Barrel {
  x: number;
  y: number;
  r: number;
  zTop: number;
  zBottom: number;
  net: string;
}

// BarrelHit is the nearest barrel a pixel's ray meets, with how far along the ray it enters.
export interface BarrelHit {
  s: number;
  net: string;
}

// hitBarrel finds the nearest barrel the ray through device pixel (px, py) meets (agni issue 1607).
// The ray enters a barrel where it is both inside the cylinder's circle and between its ends.
export function hitBarrel(camera: StackCamera, barrels: readonly Barrel[], px: number, py: number): BarrelHit | null {
  const { o, dir } = camera.ray(px, py);
  const a = dir.x * dir.x + dir.y * dir.y;
  let best: BarrelHit | null = null;
  for (const b of barrels) {
    const ox = o.x - b.x, oy = o.y - b.y;
    let lo = -Infinity, hi = Infinity;
    const c = ox * ox + oy * oy - b.r * b.r;
    if (a < 1e-12) {
      if (c > 0) continue;
    } else {
      const half = ox * dir.x + oy * dir.y;
      const disc = half * half - a * c;
      if (disc < 0) continue;
      const root = Math.sqrt(disc);
      lo = (-half - root) / a;
      hi = (-half + root) / a;
    }
    if (Math.abs(dir.z) < 1e-12) {
      if (o.z > b.zTop || o.z < b.zBottom) continue;
    } else {
      const s1 = (b.zTop - o.z) / dir.z, s2 = (b.zBottom - o.z) / dir.z;
      lo = Math.max(lo, Math.min(s1, s2));
      hi = Math.min(hi, Math.max(s1, s2));
    }
    if (lo > hi) continue;
    if (!best || lo < best.s) best = { s: lo, net: b.net };
  }
  return best;
}

// pickThroughStack picks at device pixel (px, py). allGroups holds every group the sheet draws, and
// hidden the groups the reader hid. dpr turns the camera's world size per device pixel into one per
// CSS pixel, which the flat pick takes. A tilted plane stretches a pixel by 1 / eye height, up to 4.
// barrels are the plated holes that draw, and a barrel nearer than a plane picks its net first.
export function pickThroughStack(
  camera: StackCamera,
  planes: readonly StackPickPlane[],
  allGroups: ReadonlySet<number>,
  hidden: Iterable<number>,
  px: number,
  py: number,
  dpr: number,
  pick: FlatPick,
  barrels: readonly Barrel[] = [],
): Selection | null {
  const eye = camera.basis().eye;
  if (Math.abs(eye.z) < GRAZING) return null;
  const barrel = hitBarrel(camera, barrels, px, py);
  const barrelPick = (): Selection | null => (barrel ? { kind: "net", net: barrel.net } : null);
  const perPx = (dpr * camera.worldPerPixel) / Math.max(Math.abs(eye.z), 0.25);
  const nearestFirst = [...planes].sort((a, b) => b.z * eye.z - a.z * eye.z);
  const off = [...hidden];
  const { o, dir } = camera.ray(px, py);
  for (const p of nearestFirst) {
    if (barrel && Math.abs(dir.z) > 1e-12 && barrel.s < (p.z - o.z) / dir.z) return barrelPick();
    const at = camera.hitPlane(px, py, p.z);
    if (!at) continue;
    const skip = new Set(off);
    for (const g of allGroups) if (!p.groups.has(g)) skip.add(g);
    const sel = pick(at.x, at.y, perPx, skip);
    if (sel) return sel;
  }
  return barrelPick();
}
