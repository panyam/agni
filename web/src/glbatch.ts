// glbatch turns a sheet's primitives into a few indexed draws (agni issue 1484). The renderer used to
// make one gl.drawArrays per primitive, about 41k a frame on Jetson's board.
//
// A run is a set of primitives in one group. Every primitive in a run draws in the group's colour, so
// the run's primitives can draw in any order without changing a pixel. Runs draw in order, since one
// group may cover another. A primitive joins its group's latest run unless something drawn after that
// run overlaps it, so it moves earlier only past what it doesn't touch. A board packs each via's
// barrel and then its hole, and without this every via would start two runs. A run splits into
// chunks by a coarse grid over the sheet, so a view culls a chunk rather than each primitive.
//
// A chunk lays its indices out in one stretch of the index buffer, as four regions: the triangles of
// its wide lines, then its lines as segment pairs, then its triangles, then its points. Its lines
// sort by width, widest first, so the lines wide enough to draw as triangles at a zoom are always
// the first k of them, and a frame draws each region as one call.

import { KIND_LINE_LOOP, KIND_LINE_STRIP, KIND_POINTS, KIND_TRIANGLES, pickOnly, type Primitive } from "./packed.js";
import type { Bounds } from "./camera.js";

// Stroke is where a wide line's triangles sit in the vertex buffer, after the sheet's own vertices.
export interface Stroke {
  first: number;
  count: number;
  width: number;
}

// Region is a stretch of the index buffer, in elements.
export interface Region {
  start: number;
  end: number;
}

// Chunk is one cell of one run. lineList, triList and pointList are its primitives in index order.
// The lines run widest first, and the first strokeWidths.length of them have a stroke, whose widths
// strokeWidths holds. strokeEnds[k] is the index offset where the stroke region ends after the first
// k + 1 strokes.
export interface Chunk {
  id: number;
  bounds: Bounds;
  lineList: number[];
  triList: number[];
  pointList: number[];
  strokes: Region;
  strokeWidths: number[];
  strokeEnds: number[];
  lines: Region;
  triangles: Region;
  points: Region;
}

export interface Run {
  group: number;
  chunks: Chunk[];
}

// Batches is the index buffer to upload, the runs in draw order, and where each primitive sits. A
// primitive's own range is at[i] (count[i] elements) in its main region, and strokeAt[i] (strokeCount
// [i]) in the stroke region when it has a stroke. chunkOf[i] is the chunk holding it, or -1 for one
// the renderer never draws.
export interface Batches {
  indices: Uint32Array;
  runs: Run[];
  chunks: Chunk[];
  at: Int32Array;
  count: Int32Array;
  strokeAt: Int32Array;
  strokeCount: Int32Array;
  chunkOf: Int32Array;
}

// GRID is the cells per side of the chunk grid. A run smaller than CHUNK_MIN stays one chunk, since
// splitting it saves no culling worth a draw call. OVERLAP_GRID is the cells per side of the finer grid
// that decides whether two primitives may overlap. Two in one cell count as overlapping, which is
// the safe side.
export const GRID = 8;
export const CHUNK_MIN = 512;
export const OVERLAP_GRID = 128;

const EMPTY: Region = { start: 0, end: 0 };

// buildBatches lays out the index buffer for primitives drawn in `order` (pack order when null). A
// primitive with no vertices or in a pick-only group is left out, since the renderer never draws it.
export function buildBatches(
  primitives: Primitive[],
  bounds: Bounds[],
  strokes: (Stroke | undefined)[],
  order: ArrayLike<number> | null,
): Batches {
  const n = primitives.length;
  const at = new Int32Array(n).fill(-1);
  const count = new Int32Array(n);
  const strokeAt = new Int32Array(n).fill(-1);
  const strokeCount = new Int32Array(n);
  const chunkOf = new Int32Array(n).fill(-1);

  const drawn: number[] = [];
  for (let k = 0; k < n; k++) {
    const i = order ? order[k] : k;
    const p = primitives[i];
    if (p.count > 0 && !pickOnly(p.group)) drawn.push(i);
  }
  const sheet = unionOf(bounds, drawn);
  const runPrims = joinRuns(primitives, bounds, drawn, sheet);

  const out: Out = { buf: new Uint32Array(indexCount(primitives, strokes, drawn)), n: 0 };
  const runs: Run[] = [];
  const chunks: Chunk[] = [];
  for (const r of runPrims) {
    const run: Run = { group: r.group, chunks: [] };
    for (const cell of cells(r.prims, bounds, sheet)) {
      const chunk = layOut(cell, primitives, strokes, out, at, count, strokeAt, strokeCount);
      chunk.bounds = unionOf(bounds, cell);
      chunk.id = chunks.length;
      for (const i of cell) chunkOf[i] = chunks.length;
      chunks.push(chunk);
      run.chunks.push(chunk);
    }
    runs.push(run);
  }
  return { indices: out.buf, runs, chunks, at, count, strokeAt, strokeCount, chunkOf };
}

// Out is the index buffer being written, sized up front, since pushing a million numbers onto an
// array and copying them cost a board tens of milliseconds.
interface Out {
  buf: Uint32Array;
  n: number;
}

// indexCount is how many indices the drawn primitives take, strokes included.
function indexCount(primitives: Primitive[], strokes: (Stroke | undefined)[], drawn: number[]): number {
  let total = 0;
  for (const i of drawn) {
    const p = primitives[i];
    if (p.kind === KIND_LINE_STRIP || p.kind === KIND_LINE_LOOP) {
      total += 2 * Math.max(p.count - 1, 0) + (p.kind === KIND_LINE_LOOP && p.count > 2 ? 2 : 0);
      total += strokes[i]?.count ?? 0;
    } else total += p.count;
  }
  return total;
}

// joinRuns puts each primitive, in draw order, into a run of its group. It joins the group's latest
// run when no primitive in a later run touches its cells, and starts a new run otherwise. top[cell]
// holds the latest run with a primitive in that cell.
function joinRuns(primitives: Primitive[], bounds: Bounds[], drawn: number[], sheet: Bounds): { group: number; prims: number[] }[] {
  const runs: { group: number; prims: number[] }[] = [];
  const latest = new Map<number, number>();
  const top = new Int32Array(OVERLAP_GRID * OVERLAP_GRID).fill(-1);
  const w = Math.max(sheet.maxX - sheet.minX, 1) / OVERLAP_GRID;
  const h = Math.max(sheet.maxY - sheet.minY, 1) / OVERLAP_GRID;
  const cell = (v: number, min: number, size: number) => Math.min(OVERLAP_GRID - 1, Math.max(0, Math.floor((v - min) / size)));
  for (const i of drawn) {
    const b = bounds[i];
    const x0 = cell(b.minX, sheet.minX, w), x1 = cell(b.maxX, sheet.minX, w);
    const y0 = cell(b.minY, sheet.minY, h), y1 = cell(b.maxY, sheet.minY, h);
    let under = -1;
    for (let y = y0; y <= y1; y++) {
      for (let x = y * OVERLAP_GRID + x0, end = y * OVERLAP_GRID + x1; x <= end; x++) if (top[x] > under) under = top[x];
    }
    const g = primitives[i].group;
    let r = latest.get(g) ?? -1;
    if (r < 0 || r < under) {
      r = runs.length;
      runs.push({ group: g, prims: [] });
      latest.set(g, r);
    }
    runs[r].prims.push(i);
    for (let y = y0; y <= y1; y++) {
      for (let x = y * OVERLAP_GRID + x0, end = y * OVERLAP_GRID + x1; x <= end; x++) if (top[x] < r) top[x] = r;
    }
  }
  return runs;
}

// cells splits a run's primitives by the grid cell their centre falls in, keeping their order within
// each cell. A small run is one cell.
function cells(prims: number[], bounds: Bounds[], sheet: Bounds): number[][] {
  if (prims.length < CHUNK_MIN) return [prims];
  const w = Math.max(sheet.maxX - sheet.minX, 1);
  const h = Math.max(sheet.maxY - sheet.minY, 1);
  const byCell = new Map<number, number[]>();
  for (const i of prims) {
    const b = bounds[i];
    const cx = Math.min(GRID - 1, Math.max(0, Math.floor((((b.minX + b.maxX) / 2 - sheet.minX) / w) * GRID)));
    const cy = Math.min(GRID - 1, Math.max(0, Math.floor((((b.minY + b.maxY) / 2 - sheet.minY) / h) * GRID)));
    const key = cy * GRID + cx;
    const list = byCell.get(key);
    if (list) list.push(i);
    else byCell.set(key, [i]);
  }
  return [...byCell.keys()].sort((a, b) => a - b).map((k) => byCell.get(k)!);
}

// layOut appends one chunk's indices, region by region, and records where each primitive landed.
function layOut(
  cell: number[],
  primitives: Primitive[],
  strokes: (Stroke | undefined)[],
  out: Out,
  at: Int32Array,
  count: Int32Array,
  strokeAt: Int32Array,
  strokeCount: Int32Array,
): Chunk {
  const isLine = (k: number) => k === KIND_LINE_STRIP || k === KIND_LINE_LOOP;
  const lines = cell.filter((i) => isLine(primitives[i].kind));
  // Widest first, and a line with no stroke after every line with one.
  lines.sort((a, b) => (strokes[b]?.width ?? 0) - (strokes[a]?.width ?? 0));
  const triangles = cell.filter((i) => primitives[i].kind === KIND_TRIANGLES);
  const points = cell.filter((i) => !isLine(primitives[i].kind) && primitives[i].kind !== KIND_TRIANGLES);

  const stroked = lines.filter((i) => strokes[i]);
  const strokeStart = out.n;
  const strokeEnds: number[] = [];
  for (const i of stroked) {
    const s = strokes[i]!;
    strokeAt[i] = out.n;
    for (let v = s.first; v < s.first + s.count; v++) out.buf[out.n++] = v;
    strokeCount[i] = s.count;
    strokeEnds.push(out.n);
  }
  const strokeRegion = { start: strokeStart, end: out.n };

  const linesStart = out.n;
  for (const i of lines) {
    const p = primitives[i];
    at[i] = out.n;
    for (let v = p.firstVertex; v + 1 < p.firstVertex + p.count; v++) {
      out.buf[out.n++] = v;
      out.buf[out.n++] = v + 1;
    }
    if (p.kind === KIND_LINE_LOOP && p.count > 2) {
      out.buf[out.n++] = p.firstVertex + p.count - 1;
      out.buf[out.n++] = p.firstVertex;
    }
    count[i] = out.n - at[i];
  }
  const lineRegion = { start: linesStart, end: out.n };

  const triStart = out.n;
  for (const i of triangles) {
    const p = primitives[i];
    at[i] = out.n;
    for (let v = p.firstVertex; v < p.firstVertex + p.count; v++) out.buf[out.n++] = v;
    count[i] = p.count;
  }
  const triRegion = { start: triStart, end: out.n };

  const pointStart = out.n;
  for (const i of points) {
    const p = primitives[i];
    at[i] = out.n;
    for (let v = p.firstVertex; v < p.firstVertex + p.count; v++) out.buf[out.n++] = v;
    count[i] = p.count;
  }
  const pointRegion = { start: pointStart, end: out.n };

  return {
    id: -1,
    bounds: { minX: 0, minY: 0, maxX: 0, maxY: 0 },
    lineList: lines,
    triList: triangles,
    pointList: points,
    strokes: stroked.length > 0 ? strokeRegion : EMPTY,
    strokeWidths: stroked.map((i) => strokes[i]!.width),
    strokeEnds,
    lines: lineRegion,
    triangles: triRegion,
    points: pointRegion,
  };
}

// wideCount is how many of a chunk's stroked lines draw as triangles when one canvas pixel spans
// perPx world units, which is those at least that wide. strokeWidths runs widest first.
export function wideCount(c: Chunk, perPx: number): number {
  let lo = 0;
  let hi = c.strokeWidths.length;
  while (lo < hi) {
    const mid = (lo + hi) >> 1;
    if (c.strokeWidths[mid] >= perPx) lo = mid + 1;
    else hi = mid;
  }
  return lo;
}

function unionOf(bounds: Bounds[], prims: number[]): Bounds {
  let minX = Infinity, minY = Infinity, maxX = -Infinity, maxY = -Infinity;
  for (const i of prims) {
    const b = bounds[i];
    if (b.minX < minX) minX = b.minX;
    if (b.minY < minY) minY = b.minY;
    if (b.maxX > maxX) maxX = b.maxX;
    if (b.maxY > maxY) maxY = b.maxY;
  }
  return { minX, minY, maxX, maxY };
}
