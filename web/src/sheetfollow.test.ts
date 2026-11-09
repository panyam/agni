import { describe, it, expect } from "vitest";
import { chooseSheet, lockedPeers, type LinkedView } from "./sheetfollow.js";

describe("which sheet a view shows for a selection", () => {
  it("stays on a sheet that draws it, and offers the others", () => {
    expect(chooseSheet("s2", ["s1", "s2", "s3"])).toEqual({ open: "", others: ["s1", "s3"] });
  });

  it("switches to the one sheet that draws it", () => {
    expect(chooseSheet("s1", ["s4"])).toEqual({ open: "s4", others: [] });
  });

  it("switches to the first of several in the design's order, and offers the rest", () => {
    expect(chooseSheet("s1", ["s3", "board"])).toEqual({ open: "s3", others: ["board"] });
  });

  it("stays with nothing to offer when nothing draws it", () => {
    expect(chooseSheet("s1", [])).toEqual({ open: "", others: [] });
  });
});

describe("which views a locked camera mirrors", () => {
  const v = (id: string, sheet: string, link: LinkedView["link"]): LinkedView => ({ id, sheet, link });
  const views = [v("a", "board", "locked"), v("b", "board", "locked"), v("c", "board", "follow"), v("d", "s1", "locked")];

  it("mirrors the other locked views on the same sheet alone", () => {
    expect(lockedPeers(views, views[0]).map((p) => p.id)).toEqual(["b"]);
  });

  it("mirrors nothing from a view that is not locked", () => {
    expect(lockedPeers(views, views[2])).toEqual([]);
  });

  it("mirrors nothing across sheets, since two sheets have different frames", () => {
    expect(lockedPeers(views, views[3])).toEqual([]);
  });

  // An SVG view and a WebGL view measure their centres in different units (agni issue 1368).
  it("mirrors nothing onto a locked view drawn by the other renderer", () => {
    const mixed: LinkedView[] = [
      { id: "a", sheet: "board", link: "locked", drawnBy: "webgl" },
      { id: "b", sheet: "board", link: "locked", drawnBy: "svg" },
      { id: "c", sheet: "board", link: "locked", drawnBy: "webgl" },
    ];
    expect(lockedPeers(mixed, mixed[0]).map((p) => p.id)).toEqual(["c"]);
  });
});
