// Picking on the WebGL canvas (agni issue 1332). The SVG view asks the browser what lies under a
// click. The canvas has no elements to ask, so it indexes the packed sheet's keyed primitives in a
// grid and tests each one near the click against its own shape. Both views rank what they find with
// selection.ts's bestOf, so a click names one Selection in either. The contract is on
// docsite/content/architecture/web-picking.md#picking-the-webgl-canvas.
//
// The picker samples the click and the ring around it at SVG's PROBES, so a target the SVG view's
// probes miss is missed here too. Each target reaches as far as SVG's element for it does.
//   - The probes sit PROBE_R CSS pixels out at any zoom.
//   - A schematic pin also has SVG's invisible target, PIN_PICK_R document pixels.
//   - A wire has SVG's wide companion stroke, WIRE_PICK_WIDTH document pixels.
//   - A symbol's outline has SVG's stroke width, SYMBOL_STROKE document pixels.
// A document pixel grows with the zoom, as an SVG drawing's does. The SVG frame fits the sheet's
// longer side to SHEET_MAX_PX of them.

import {
  GROUP_PIN,
  GROUP_SYMBOL,
  GROUP_WIRE,
  KIND_LINE_LOOP,
  KIND_LINE_STRIP,
  KIND_POINTS,
  KIND_TRIANGLES,
  pickOnly,
  type DecodedSheet,
  type PrimitiveKey,
} from "./packed.js";
import { boundsOfVertices, type Bounds } from "./camera.js";
import { PROBES, PROBE_R, bestOf, type Selection } from "./selection.js";
import type { Members } from "./area.js";

// These mirror core/render/svg.go (sheetMaxPx, pinPickRPx, wirePickWidthPx, strokePx). Change them
// only alongside the Go constants.
export const SHEET_MAX_PX = 1600;
export const PIN_PICK_R = 6;
export const WIRE_PICK_WIDTH = 7;
export const SYMBOL_STROKE = 0.8;

// GRID is how many cells the index lays along the sheet's longer side. A primitive spanning more
// than BIG_CELLS cells, such as a pour or a frame line, goes on a list tested on every click
// rather than into each cell.
const GRID = 256;
const BIG_CELLS = 256;

// A Target is one keyed primitive the picker can return, with the selection it names and the slack
// its kind gets beyond its drawn shape, in world units. A filled target covers the inside of its
// outline. On a board a keyed loop is filled, as SVG fills a zone's polygon and a footprint's body
// rect, though the packer sends only the outline (a zone) or draws nothing (a pick area).
interface Target {
  primitive: number;
  sel: Selection;
  slack: number;
  bounds: Bounds;
  filled: boolean;
}

// PickIndex answers which keyed thing lies under a world point on one packed sheet.
export class PickIndex {
  private readonly targets: Target[] = [];
  private readonly cells = new Map<number, number[]>();
  private readonly big: number[] = [];
  // parts lists each part's targets, since an area holds a part only when all of them lie inside.
  private readonly parts = new Map<string, number[]>();
  private readonly cell: number;
  private readonly minX: number;
  private readonly minY: number;

  constructor(private readonly sheet: DecodedSheet) {
    const b = boundsOfVertices(sheet.vertices);
    const span = Math.max(1, b.maxX - b.minX, b.maxY - b.minY);
    this.cell = Math.max(1, span / GRID);
    this.minX = b.minX;
    this.minY = b.minY;
    const docPx = span / SHEET_MAX_PX;
    const board = sheet.sheetId === "board";
    for (const k of sheet.keys) {
      const p = sheet.primitives[k.primitive];
      if (!p) continue;
      const sel = selectionOf(k, board, p.kind, p.group);
      if (!sel) continue;
      const slack = board ? 0 : slackOf(p.group, p.kind, docPx);
      this.add({ primitive: k.primitive, sel, slack, bounds: this.primBounds(k.primitive), filled: board && p.kind === KIND_LINE_LOOP });
    }
  }

  // pick returns the Selection a click at world (x, y) names, or null when nothing keyed lies under
  // it or under any probe around it. perPx is the world size of one CSS pixel, which places the
  // probes. hidden holds the primitive groups the view hides, and a hidden group picks nothing, as a
  // hidden SVG layer picks nothing.
  pick(x: number, y: number, perPx: number, hidden: ReadonlySet<number> = new Set()): Selection | null {
    const near = [...this.near(x, y, PROBE_R * Math.SQRT2 * perPx)];
    const found: Selection[] = [];
    for (const [dx, dy] of PROBES) {
      // Screen Y runs down and world Y up, so a probe below the click is lower in the world.
      const px = x + dx * perPx, py = y - dy * perPx;
      const here: { sel: Selection; z: number }[] = [];
      for (const i of near) {
        const t = this.targets[i];
        if (hidden.has(this.sheet.primitives[t.primitive].group)) continue;
        const b = t.bounds;
        if (px < b.minX - t.slack || px > b.maxX + t.slack || py < b.minY - t.slack || py > b.maxY + t.slack) continue;
        if ((t.filled && this.inside(t.primitive, px, py)) || this.distance(t.primitive, px, py) <= t.slack) here.push({ sel: t.sel, z: t.primitive });
      }
      // Under one probe the topmost comes first, as elementFromPoint answers. A sheet packs in
      // paint order, so a later primitive is drawn over an earlier one, and a front track beats a
      // back one and the pour under it (PackBoard).
      here.sort((a, b) => b.z - a.z);
      for (const h of here) found.push(h.sel);
    }
    // In probe order, so a tie between two of one kind goes to the nearer probe, as SVG's does.
    return bestOf(found);
  }

  // membersIn returns the area a Shift-drag over world rectangle r selects, by area.ts's membersIn
  // rules. A part counts when every shape keyed to it lies inside, and a pin of a part not wholly
  // inside counts on its own. A net counts when any one of its primitives lies inside, since each
  // board track and schematic wire packs as a primitive of its own. A shape counts by its bounds
  // with no stroke, as getBoundingClientRect measures an SVG element. A schematic pin also reaches
  // as far as SVG's pick circle, the element SVG keys it on. A shape in a hidden group lies
  // nowhere, as a display: none element measures as an empty rect.
  membersIn(r: Bounds, hidden: ReadonlySet<number> = new Set()): Members {
    const within = (i: number): boolean => {
      const t = this.targets[i];
      if (hidden.has(this.sheet.primitives[t.primitive].group)) return false;
      const reach = t.sel.kind === "pin" ? t.slack : 0;
      const b = t.bounds;
      return b.minX - reach >= r.minX && b.maxX + reach <= r.maxX && b.minY - reach >= r.minY && b.maxY + reach <= r.maxY;
    };
    const refs = new Set<string>();
    const pins: { ref: string; pin: string }[] = [];
    const wires = new Set<string>();
    const judged = new Set<string>();
    for (const i of this.overlapping(r)) {
      const sel = this.targets[i].sel;
      if (sel.kind === "component" && sel.ref && !judged.has(sel.ref)) {
        judged.add(sel.ref);
        if ((this.parts.get(sel.ref) ?? []).every(within)) refs.add(sel.ref);
      } else if (sel.kind === "pin" && sel.ref && sel.pin && within(i)) {
        pins.push({ ref: sel.ref, pin: sel.pin });
      } else if (sel.kind === "net" && sel.net && within(i)) {
        wires.add(sel.net);
      }
    }
    const loose = new Set(pins.filter((p) => !refs.has(p.ref)).map((p) => `${p.ref}.${p.pin}`));
    return { refs: [...refs].sort(), pins: [...loose].sort(), wires: [...wires].sort() };
  }

  private add(t: Target): void {
    const i = this.targets.length;
    this.targets.push(t);
    if (t.sel.kind === "component" && t.sel.ref) {
      const list = this.parts.get(t.sel.ref);
      if (list) list.push(i);
      else this.parts.set(t.sel.ref, [i]);
    }
    const b = t.bounds;
    const x0 = this.cellX(b.minX - t.slack), x1 = this.cellX(b.maxX + t.slack);
    const y0 = this.cellY(b.minY - t.slack), y1 = this.cellY(b.maxY + t.slack);
    if ((x1 - x0 + 1) * (y1 - y0 + 1) > BIG_CELLS) {
      this.big.push(i);
      return;
    }
    for (let cx = x0; cx <= x1; cx++) {
      for (let cy = y0; cy <= y1; cy++) {
        const key = cx * 65536 + cy;
        const list = this.cells.get(key);
        if (list) list.push(i);
        else this.cells.set(key, [i]);
      }
    }
  }

  // near lists the targets whose cells lie within r of (x, y), each once, and every big target.
  private near(x: number, y: number, r: number): Set<number> {
    const out = new Set<number>(this.big);
    const x0 = this.cellX(x - r), x1 = this.cellX(x + r);
    const y0 = this.cellY(y - r), y1 = this.cellY(y + r);
    for (let cx = x0; cx <= x1; cx++) {
      for (let cy = y0; cy <= y1; cy++) {
        for (const i of this.cells.get(cx * 65536 + cy) ?? []) out.add(i);
      }
    }
    return out;
  }

  // overlapping lists the targets in the cells that rectangle r covers, each once, and every big
  // target.
  private overlapping(r: Bounds): Set<number> {
    const out = new Set<number>(this.big);
    const x0 = this.cellX(r.minX), x1 = this.cellX(r.maxX);
    const y0 = this.cellY(r.minY), y1 = this.cellY(r.maxY);
    for (let cx = x0; cx <= x1; cx++) {
      for (let cy = y0; cy <= y1; cy++) {
        for (const i of this.cells.get(cx * 65536 + cy) ?? []) out.add(i);
      }
    }
    return out;
  }

  // A world coordinate's cell, clamped to the grid, since a click can land past the sheet.
  private cellX(x: number): number {
    return Math.min(GRID, Math.max(0, Math.floor((x - this.minX) / this.cell)));
  }
  private cellY(y: number): number {
    return Math.min(GRID, Math.max(0, Math.floor((y - this.minY) / this.cell)));
  }

  private primBounds(i: number): Bounds {
    const p = this.sheet.primitives[i];
    const v = this.sheet.vertices;
    return boundsOfVertices(v.subarray(p.firstVertex * 2, (p.firstVertex + p.count) * 2));
  }

  // inside reports whether (x, y) lies inside a loop primitive's polygon, by the even-odd rule.
  private inside(i: number, x: number, y: number): boolean {
    const p = this.sheet.primitives[i];
    const v = this.sheet.vertices;
    let in_ = false;
    for (let j = 0, k = p.count - 1; j < p.count; k = j++) {
      const xj = v[(p.firstVertex + j) * 2], yj = v[(p.firstVertex + j) * 2 + 1];
      const xk = v[(p.firstVertex + k) * 2], yk = v[(p.firstVertex + k) * 2 + 1];
      if (yj > y !== yk > y && x < ((xk - xj) * (y - yj)) / (yk - yj) + xj) in_ = !in_;
    }
    return in_;
  }

  // distance is how far (x, y) lies from a primitive's drawn shape: 0 inside a filled one, and the
  // distance to the nearest point, segment or triangle edge otherwise.
  private distance(i: number, x: number, y: number): number {
    const p = this.sheet.primitives[i];
    const v = this.sheet.vertices;
    const at = (j: number): [number, number] => [v[(p.firstVertex + j) * 2], v[(p.firstVertex + j) * 2 + 1]];
    let best = Infinity;
    switch (p.kind) {
      case KIND_POINTS:
        for (let j = 0; j < p.count; j++) {
          const [px, py] = at(j);
          best = Math.min(best, Math.hypot(x - px, y - py));
        }
        return best;
      case KIND_LINE_STRIP:
      case KIND_LINE_LOOP: {
        const n = p.kind === KIND_LINE_LOOP ? p.count : p.count - 1;
        if (p.count === 1) return Math.hypot(x - at(0)[0], y - at(0)[1]);
        for (let j = 0; j < n; j++) best = Math.min(best, segmentDistance(x, y, at(j), at((j + 1) % p.count)));
        return best;
      }
      case KIND_TRIANGLES:
        for (let j = 0; j + 2 < p.count; j += 3) {
          const a = at(j), b = at(j + 1), c = at(j + 2);
          if (inTriangle(x, y, a, b, c)) return 0;
          best = Math.min(best, segmentDistance(x, y, a, b), segmentDistance(x, y, b, c), segmentDistance(x, y, c, a));
        }
        return best;
      default:
        return Infinity;
    }
  }
}

// drawnOnPacked lists the parts and nets a packed sheet draws, as svgview.ts's drawnOnSheet reads
// them off an SVG sheet. A key counts when a click on it would name a part or a net, so a board's
// silkscreen and fab outlines add nothing, as their unkeyed SVG elements add nothing.
export function drawnOnPacked(sheet: DecodedSheet): { refs: string[]; nets: string[] } {
  const board = sheet.sheetId === "board";
  const refs = new Set<string>();
  const nets = new Set<string>();
  for (const k of sheet.keys) {
    const p = sheet.primitives[k.primitive];
    if (!p) continue;
    const sel = selectionOf(k, board, p.kind, p.group);
    if (sel?.kind === "component" && sel.ref) refs.add(sel.ref);
    if (sel?.kind === "net" && sel.net) nets.add(sel.net);
  }
  return { refs: [...refs], nets: [...nets] };
}

// selectionOf reads a key as the Selection a click on it names, or null for a primitive a click
// passes through. It reads the key as selection.ts's selectionFromElement reads an SVG element.
// On a board a part picks from its pick area and from a pad with no number. Its silkscreen and fab
// outlines name it and still pass a click through, as BoardSVG's do (agni issue 1347).
function selectionOf(k: PrimitiveKey, board: boolean, kind: number, group: number): Selection | null {
  if (k.refDes !== "" && k.pin !== "") return { kind: "pin", ref: k.refDes, pin: k.pin };
  if (k.busId !== "") return { kind: "bus", busId: k.busId };
  if (k.refDes !== "") return board && kind !== KIND_TRIANGLES && !pickOnly(group) ? null : { kind: "component", ref: k.refDes };
  if (k.net !== "" || k.netId !== "") return { kind: "net", net: k.net, netId: k.netId };
  return null;
}

// slackOf is the extra reach a schematic primitive gets past its drawn shape, SVG's pick target
// for its kind, in world units.
function slackOf(group: number, kind: number, docPx: number): number {
  if (group === GROUP_PIN || kind === KIND_POINTS) return PIN_PICK_R * docPx;
  if (group === GROUP_WIRE) return (WIRE_PICK_WIDTH / 2) * docPx;
  if (group === GROUP_SYMBOL) return (SYMBOL_STROKE / 2) * docPx;
  return 0;
}

function segmentDistance(x: number, y: number, [ax, ay]: [number, number], [bx, by]: [number, number]): number {
  const dx = bx - ax, dy = by - ay;
  const len2 = dx * dx + dy * dy;
  const t = len2 === 0 ? 0 : Math.max(0, Math.min(1, ((x - ax) * dx + (y - ay) * dy) / len2));
  return Math.hypot(x - (ax + t * dx), y - (ay + t * dy));
}

function inTriangle(x: number, y: number, [ax, ay]: [number, number], [bx, by]: [number, number], [cx, cy]: [number, number]): boolean {
  const d1 = (x - bx) * (ay - by) - (ax - bx) * (y - by);
  const d2 = (x - cx) * (by - cy) - (bx - cx) * (y - cy);
  const d3 = (x - ax) * (cy - ay) - (cx - ax) * (y - ay);
  const neg = d1 < 0 || d2 < 0 || d3 < 0;
  const pos = d1 > 0 || d2 > 0 || d3 > 0;
  return !(neg && pos);
}
