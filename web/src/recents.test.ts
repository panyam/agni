// @vitest-environment jsdom
import { describe, it, expect, beforeEach } from "vitest";
import { clearRecents, loadRecents, noteOpen, LIMIT, type Recent } from "./recents.js";

const design = (path: string, at: number): Recent => ({ kind: "design", mount: "m", path, label: path, at });

beforeEach(() => localStorage.clear());

describe("recents", () => {
  it("returns nothing before anything has been opened", () => {
    expect(loadRecents()).toEqual([]);
  });

  it("orders most-recent first and moves a re-open to the front instead of duplicating it", () => {
    noteOpen({ kind: "design", mount: "m", path: "a.edn", label: "a.edn" }, 100);
    noteOpen({ kind: "design", mount: "m", path: "b.edn", label: "b.edn" }, 200);
    noteOpen({ kind: "design", mount: "m", path: "a.edn", label: "a.edn" }, 300);

    const got = loadRecents();
    expect(got.map((r) => r.path)).toEqual(["a.edn", "b.edn"]);
    expect(got[0].at).toBe(300);
  });

  // The same path under two kinds stays two entries: they reopen on different pages, so collapsing
  // them would send one of the two to a page that cannot show it.
  it("keeps a design and a datasheet at the same path apart", () => {
    noteOpen({ kind: "design", mount: "m", path: "x", label: "x" }, 100);
    noteOpen({ kind: "datasheet", mount: "m", path: "x", label: "x" }, 200);
    expect(loadRecents()).toHaveLength(2);
  });

  it("keeps the newest LIMIT entries", () => {
    for (let i = 0; i < LIMIT + 5; i++) {
      noteOpen({ kind: "design", mount: "m", path: `d${i}.edn`, label: `d${i}` }, i);
    }
    const got = loadRecents();
    expect(got).toHaveLength(LIMIT);
    expect(got[0].path).toBe(`d${LIMIT + 4}.edn`);
    expect(got.some((r) => r.path === "d0.edn")).toBe(false);
  });

  // The store is user-writable and outlives any one version of this module, so a row that lost a
  // field is dropped rather than rendered as a link with an undefined path.
  it("drops unreadable rows and survives unreadable JSON", () => {
    localStorage.setItem("agni.recents", JSON.stringify([design("good.edn", 1), { kind: "design", mount: "m" }, null]));
    expect(loadRecents().map((r) => r.path)).toEqual(["good.edn"]);

    localStorage.setItem("agni.recents", "{not json");
    expect(loadRecents()).toEqual([]);
  });

  it("clears", () => {
    noteOpen({ kind: "design", mount: "m", path: "a.edn", label: "a" }, 1);
    clearRecents();
    expect(loadRecents()).toEqual([]);
  });
});
