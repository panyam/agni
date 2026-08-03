// Pure doc-IR region helpers for the extraction workbench (WS13-006), split out of regionview.tsx
// so they are testable without importing pdf.js (which regionview pulls in for rendering).
import type { Document, Page, BBox } from "./gen/agni/v1/doc/doc_pb.js";

// Region is one selectable region: a doc-IR table/figure/text block, or a "user" region the human
// drew with the marquee (WS13-006 PR 2) where doc-IR missed one. kind drives the overlay color and
// the info line; bbox is page-local points (top-left, y-down). A user region carries the page it was
// drawn on so it can be merged back onto the right rendered page.
export interface Region {
  id: string;
  kind: "table" | "figure" | "text" | "user";
  label: string;
  bbox: BBox;
  page?: number;
}

// pageRegions flattens a doc-IR page's tables, figures, and text blocks into one selectable list.
// A region with no bbox is dropped (there is nothing to place on the page).
export function pageRegions(page: Page): Region[] {
  const out: Region[] = [];
  for (const t of page.tables) if (t.bbox) out.push({ id: t.id, kind: "table", label: t.title || t.id, bbox: t.bbox });
  for (const f of page.figures) if (f.bbox) out.push({ id: f.id, kind: "figure", label: f.caption || f.id, bbox: f.bbox });
  for (const x of page.textBlocks) if (x.bbox) out.push({ id: x.id, kind: "text", label: x.kind || "text", bbox: x.bbox });
  return out;
}

// regionsForPage returns the doc-IR regions for a rendered page, matching by 1-based page number
// (doc-IR Page.number aligns with the PDF page). A page the doc-IR did not decompose has none.
export function regionsForPage(doc: Document | undefined, pageNumber: number): Region[] {
  const page = doc?.pages.find((p) => p.number === pageNumber);
  return page ? pageRegions(page) : [];
}

// RegionType is the user-assigned routing tag that decides which backend and target IR a region
// feeds (WS13-006 "region typing"). It is distinct from doc-IR `kind`: doc-IR gives table/figure/
// text and cannot tell a chart from a schematic (both are figures), so the chart/schematic/pinout
// distinction is the human's, stored in the bank per region id. Only `table` has a transcribe path
// in this PR; `schematic` -> ir.Design (WS13-002) and `chart` -> curve IR (WS13-003) are taggable
// but their extraction is deferred, so the coverage view stays honest across modalities.
export type RegionType = "table" | "schematic" | "chart" | "pinout" | "text" | "other";

// REGION_TYPES is the closed vocabulary the type picker offers, in display order.
export const REGION_TYPES: RegionType[] = ["table", "schematic", "chart", "pinout", "text", "other"];

// defaultType is the type a region starts as before the human tags it, derived from the doc-IR
// kind: a table is a table and text is text, but a figure is ambiguous (chart vs schematic vs
// pinout) and a user-drawn region has no source hint, so both default to "other" until tagged.
export function defaultType(kind: Region["kind"]): RegionType {
  switch (kind) {
    case "table":
      return "table";
    case "text":
      return "text";
    default:
      return "other";
  }
}

// pxRectToBBox converts a marquee rectangle in rendered pixels to a page-local doc-IR BBox in
// points (dividing by the page's render scale), normalizing so width/height are positive whichever
// way the drag went. The inverse of the point->pixel overlay mapping in regionview.
export function pxRectToBBox(x0: number, y0: number, x1: number, y1: number, scale: number): BBox {
  return {
    x: Math.min(x0, x1) / scale,
    y: Math.min(y0, y1) / scale,
    width: Math.abs(x1 - x0) / scale,
    height: Math.abs(y1 - y0) / scale,
  } as BBox;
}

// Coverage is the per-datasheet gap record the toolbar shows: how many regions carry at least one
// transcribed parameter vs how many are still pending, so silence never reads as coverage.
export interface Coverage {
  total: number;
  done: number;
  pending: number;
}

// coverageOf counts regions as done when isDone(region.id) reports a transcribed parameter against
// them. `regions` is the merged doc-IR + user set across every page.
export function coverageOf(regions: Region[], isDone: (id: string) => boolean): Coverage {
  let done = 0;
  for (const r of regions) if (isDone(r.id)) done++;
  return { total: regions.length, done, pending: regions.length - done };
}

// clampPage keeps a 1-based page number inside [1, total] (total>=1), for the page navigator.
export function clampPage(n: number, total: number): number {
  const top = Math.max(1, total);
  return Math.min(Math.max(1, Math.round(n)), top);
}

// MIN_REGION_PTS is the smallest a user region may be dragged to (points), so a resize cannot
// collapse a box to nothing.
export const MIN_REGION_PTS = 4;

// moveBBox translates a bbox by a delta in points (marquee move). No page-bounds clamp: a box may
// sit slightly off the page edge, which the viewport scroll reveals.
export function moveBBox(b: BBox, dxPts: number, dyPts: number): BBox {
  return { x: b.x + dxPts, y: b.y + dyPts, width: b.width, height: b.height } as BBox;
}

// Handle names the corner a resize drag moves; the opposite corner stays fixed.
export type Handle = "nw" | "ne" | "sw" | "se";

// resizeBBox returns the bbox after dragging one corner by (dxPts, dyPts). The two edges the handle
// touches move; the opposite edges stay put. The result is normalized (positive width/height) and
// floored at MIN_REGION_PTS so a drag past the opposite edge flips rather than inverts.
export function resizeBBox(b: BBox, handle: Handle, dxPts: number, dyPts: number): BBox {
  let x0 = b.x;
  let y0 = b.y;
  let x1 = b.x + b.width;
  let y1 = b.y + b.height;
  if (handle === "nw" || handle === "sw") x0 += dxPts;
  else x1 += dxPts;
  if (handle === "nw" || handle === "ne") y0 += dyPts;
  else y1 += dyPts;
  const x = Math.min(x0, x1);
  const y = Math.min(y0, y1);
  return {
    x,
    y,
    width: Math.max(MIN_REGION_PTS, Math.abs(x1 - x0)),
    height: Math.max(MIN_REGION_PTS, Math.abs(y1 - y0)),
  } as BBox;
}
