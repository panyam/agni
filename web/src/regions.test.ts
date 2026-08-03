import { describe, it, expect } from "vitest";
import { pageRegions, regionsForPage, defaultType, pxRectToBBox, coverageOf, clampPage, moveBBox, resizeBBox, MIN_REGION_PTS, type Region } from "./regions.js";
import type { Document, Page, BBox } from "./gen/agni/v1/doc/doc_pb.js";

const bbox = (x: number, y: number, w: number, h: number) => ({ x, y, width: w, height: h });

// A doc-IR page with one table, one figure, and two text blocks (one with no bbox, which is dropped).
function fakePage(): Page {
  return {
    number: 2,
    tables: [{ id: "p2.t1", title: "Abs Max", bbox: bbox(10, 20, 100, 50) }],
    figures: [{ id: "p2.f1", caption: "Fig 1", bbox: bbox(5, 5, 30, 30) }],
    textBlocks: [
      { id: "p2.x1", kind: "heading", text: "hi", bbox: bbox(0, 0, 10, 10) },
      { id: "p2.x2", kind: "para", text: "no bbox" },
    ],
  } as unknown as Page;
}

describe("regions", () => {
  it("flattens tables, figures, and text into one list, dropping bbox-less regions", () => {
    const rs = pageRegions(fakePage());
    expect(rs.map((r) => [r.kind, r.id])).toEqual([
      ["table", "p2.t1"],
      ["figure", "p2.f1"],
      ["text", "p2.x1"],
    ]);
    expect(rs[0].label).toBe("Abs Max");
  });

  it("matches doc-IR regions to a rendered page by 1-based number", () => {
    const doc = { pages: [fakePage()] } as unknown as Document;
    expect(regionsForPage(doc, 2)).toHaveLength(3);
    expect(regionsForPage(doc, 1)).toEqual([]); // a page the doc-IR did not decompose
    expect(regionsForPage(undefined, 2)).toEqual([]);
  });

  it("defaultType maps kind to a routing type, leaving figures/user regions untyped", () => {
    expect(defaultType("table")).toBe("table");
    expect(defaultType("text")).toBe("text");
    expect(defaultType("figure")).toBe("other"); // chart vs schematic is the human's to tag
    expect(defaultType("user")).toBe("other");
  });

  it("pxRectToBBox converts a marquee rect to points, normalizing drag direction", () => {
    // scale 2: 100px wide -> 50pt. Dragging bottom-right to top-left gives the same box.
    expect(pxRectToBBox(20, 40, 120, 140, 2)).toEqual({ x: 10, y: 20, width: 50, height: 50 });
    expect(pxRectToBBox(120, 140, 20, 40, 2)).toEqual({ x: 10, y: 20, width: 50, height: 50 });
  });

  it("coverageOf counts regions with a transcribed parameter as done", () => {
    const rs: Region[] = [
      { id: "p1.t1", kind: "table", label: "a", bbox: {} as BBox },
      { id: "p1.t2", kind: "table", label: "b", bbox: {} as BBox },
      { id: "p1.f1", kind: "figure", label: "c", bbox: {} as BBox },
    ];
    const done = new Set(["p1.t1"]);
    expect(coverageOf(rs, (id) => done.has(id))).toEqual({ total: 3, done: 1, pending: 2 });
  });

  it("clampPage keeps a page in [1, total] and rounds", () => {
    expect(clampPage(0, 22)).toBe(1);
    expect(clampPage(5.4, 22)).toBe(5);
    expect(clampPage(99, 22)).toBe(22);
    expect(clampPage(3, 0)).toBe(1); // total 0 -> at least page 1
  });

  const box = (x: number, y: number, w: number, h: number): BBox => ({ x, y, width: w, height: h }) as BBox;

  it("moveBBox translates by a points delta, size unchanged", () => {
    expect(moveBBox(box(10, 20, 30, 40), 5, -3)).toEqual({ x: 15, y: 17, width: 30, height: 40 });
  });

  it("resizeBBox moves the handle's edges, keeps the opposite corner, floors at min size", () => {
    // se: grow width/height from the top-left anchor.
    expect(resizeBBox(box(10, 20, 30, 40), "se", 10, 5)).toEqual({ x: 10, y: 20, width: 40, height: 45 });
    // nw: move the top-left in; bottom-right (40,60) stays fixed.
    expect(resizeBBox(box(10, 20, 30, 40), "nw", 5, 5)).toEqual({ x: 15, y: 25, width: 25, height: 35 });
    // dragging the SE corner onto the NW corner (10,20) collapses width/height, floored at min.
    const collapsed = resizeBBox(box(10, 20, 30, 40), "se", -30, -40);
    expect(collapsed).toEqual({ x: 10, y: 20, width: MIN_REGION_PTS, height: MIN_REGION_PTS });
    // dragging a handle far past the opposite corner flips the box rather than inverting it.
    const flipped = resizeBBox(box(10, 20, 30, 40), "se", -100, 0);
    expect(flipped.x).toBe(-60);
    expect(flipped.width).toBe(70);
  });
});
