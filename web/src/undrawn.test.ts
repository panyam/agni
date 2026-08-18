import { describe, it, expect } from "vitest";
import { undrawnNote, undrawnStrip, type UndrawnPlacement } from "./undrawn.js";

const p = (over: Partial<UndrawnPlacement>): UndrawnPlacement => ({
  refDes: "U1",
  cellRef: "MCU",
  libraryRef: "Acme",
  sheetId: "s1",
  ...over,
});

describe("undrawnNote", () => {
  it("is null for a complete drawing, so the strip carries no chrome", () => {
    // The control that keeps the notice worth reading: a banner on every render is a banner nobody
    // reads, and most renders are complete.
    expect(undrawnNote([])).toBeNull();
    expect(undrawnNote(undefined)).toBeNull();
  });

  it("groups by the missing symbol, because one library costs many parts", () => {
    const note = undrawnNote([
      p({ refDes: "C1", cellRef: "CAP" }),
      p({ refDes: "C2", cellRef: "CAP" }),
      p({ refDes: "U1", cellRef: "MCU" }),
    ]);
    expect(note?.count).toBe(3);
    // Worst first: the library costing the most parts is the one to go and find.
    expect(note?.libraries).toEqual([
      { name: "Acme:CAP", count: 2 },
      { name: "Acme:MCU", count: 1 },
    ]);
  });

  it("names a library-less placement by its cell alone", () => {
    expect(undrawnNote([p({ libraryRef: "", cellRef: "R" })])?.libraries[0].name).toBe("R");
  });
});

describe("undrawnStrip", () => {
  it("shows the count, the libraries, and what it means for the reader", () => {
    const el = document.createElement("div");
    undrawnStrip(el)(undrawnNote([p({ refDes: "C1", cellRef: "CAP" }), p({ refDes: "U1" })]));
    expect(el.classList.contains("on")).toBe(true);
    expect(el.textContent).toContain("2 placements");
    expect(el.textContent).toContain("Acme:CAP");
    // The consequence is the point. A count alone does not tell a reader why the sheet they are
    // looking at will not respond to clicks.
    expect(el.textContent).toContain("cannot be clicked");
  });

  it("hides itself entirely when the drawing is complete", () => {
    const el = document.createElement("div");
    const set = undrawnStrip(el);
    set(undrawnNote([p({})]));
    set(null);
    expect(el.classList.contains("on")).toBe(false);
    expect(el.textContent).toBe("");
  });
});
