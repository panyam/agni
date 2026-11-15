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

// pickThroughStack picks at device pixel (px, py). allGroups holds every group the sheet draws, and
// hidden the groups the reader hid. dpr turns the camera's world size per device pixel into one per
// CSS pixel, which the flat pick takes. A tilted plane stretches a pixel by 1 / eye height, up to 4.
export function pickThroughStack(
  camera: StackCamera,
  planes: readonly StackPickPlane[],
  allGroups: ReadonlySet<number>,
  hidden: Iterable<number>,
  px: number,
  py: number,
  dpr: number,
  pick: FlatPick,
): Selection | null {
  const eye = camera.basis().eye;
  if (Math.abs(eye.z) < GRAZING) return null;
  const perPx = (dpr * camera.worldPerPixel) / Math.max(Math.abs(eye.z), 0.25);
  const nearestFirst = [...planes].sort((a, b) => b.z * eye.z - a.z * eye.z);
  const off = [...hidden];
  for (const p of nearestFirst) {
    const at = camera.hitPlane(px, py, p.z);
    if (!at) continue;
    const skip = new Set(off);
    for (const g of allGroups) if (!p.groups.has(g)) skip.add(g);
    const sel = pick(at.x, at.y, perPx, skip);
    if (sel) return sel;
  }
  return null;
}
