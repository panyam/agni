import { describe, it, expect } from "vitest";
import { sheetTiles } from "./sheetoverview.js";
import type { SheetRef } from "./gen/agni/v1/webapi/design_pb.js";
import type { FindingItem } from "./findings.js";

function sheet(id: string, name: string, parentId = ""): SheetRef {
  return { id, name, parentId } as SheetRef;
}

function finding(sheets: { id: string; name: string }[]): FindingItem {
  return { rule: "r", category: "c", profile: "", severity: "info", kind: "net", subject: "N", pin: "", netId: "", busId: "", message: "m", inconclusive: false, context: [], sheets, locateReason: 0 };
}

function undecided(sheets: { id: string; name: string }[]): FindingItem {
  return { ...finding(sheets), inconclusive: true };
}

describe("sheetTiles (WS9-025)", () => {
  const root = sheet("s1", "Root");
  const power = sheet("s2", "Power");
  const clean = sheet("s3", "Clean");

  it("counts findings per sheet through their badges, keeping zero-count sheets visible", () => {
    const findings = [
      finding([{ id: "s1", name: "Root" }]),
      finding([{ id: "s1", name: "Root" }, { id: "s2", name: "Power" }]), // a spanning net counts on both
      finding([]), // no geometry join, so it counts nowhere
    ];
    expect(sheetTiles([root, power, clean], findings)).toEqual([
      { id: "s1", name: "Root", parentId: "", count: 2, unresolved: 0, local: 1 },
      { id: "s2", name: "Power", parentId: "", count: 1, unresolved: 0, local: 0 },
      { id: "s3", name: "Clean", parentId: "", count: 0, unresolved: 0, local: 0 },
    ]);
  });

  it("special-cases a single-sheet design to the total findings count", () => {
    // Badges are empty for one-sheet designs (WS9-024), but everything lives
    // on that sheet, so a badge-joined 0 would read as clean on a design with findings.
    const findings = [finding([]), finding([])];
    expect(sheetTiles([root], findings)).toEqual([{ id: "s1", name: "Root", parentId: "", count: 2, unresolved: 0, local: 2 }]);
  });

  it("falls back to the sheet id when the name is empty and is empty for no design", () => {
    expect(sheetTiles([sheet("s9", "")], [])).toEqual([{ id: "s9", name: "s9", parentId: "", count: 0, unresolved: 0, local: 0 }]);
    expect(sheetTiles([], [])).toEqual([]);
  });
});

// A tile's count is read as "problems on this sheet", so an inconclusive result in it states
// something the rule declined to state (agni issue 350).
describe("sheetTiles keeps inconclusive results out of the defect count", () => {
  const root = sheet("s1", "Root");
  const power = sheet("s2", "Power");

  it("counts them in their own column, per sheet", () => {
    const findings = [
      finding([{ id: "s1", name: "Root" }]),
      undecided([{ id: "s1", name: "Root" }, { id: "s2", name: "Power" }]),
      undecided([{ id: "s2", name: "Power" }]),
    ];
    expect(sheetTiles([root, power], findings)).toEqual([
      { id: "s1", name: "Root", parentId: "", count: 1, unresolved: 1, local: 1 },
      { id: "s2", name: "Power", parentId: "", count: 0, unresolved: 2, local: 0 },
    ]);
  });

  // The single-sheet design takes the total rather than the badges, so it needs its own split or the
  // one design shape with no badges keeps the old answer.
  it("splits the single-sheet total too", () => {
    expect(sheetTiles([root], [finding([]), undecided([]), undecided([])])).toEqual([
      { id: "s1", name: "Root", parentId: "", count: 1, unresolved: 2, local: 1 },
    ]);
  });
});

// agni issue 1141: a KiCad design whose sub-sheets under different parents share a name.
describe("sheet labels", () => {
  const sheets = [sheet("/", "Root"), sheet("/Power", "Power", "/"), sheet("/Power/1", "1", "/Power"), sheet("/IO", "IO", "/"), sheet("/IO/1", "1", "/IO")];

  it("labels a sheet whose name another sheet shares by its path, and leaves a unique name alone", () => {
    expect(sheetTiles(sheets, []).map((t) => [t.name, t.parentId])).toEqual([
      ["Root", ""],
      ["Power", "/"],
      ["Power/1", "/Power"],
      ["IO", "/"],
      ["IO/1", "/IO"],
    ]);
  });
});

// agni issue 1144: a finding counts on every sheet its subject is drawn on, and the tile also knows
// how many of its defects are drawn there alone.
describe("defects drawn on one sheet alone", () => {
  it("counts a finding on one sheet as local there and a spanning one as local nowhere", () => {
    const a = { id: "a", name: "A" };
    const b = { id: "b", name: "B" };
    const tiles = sheetTiles([sheet("a", "A"), sheet("b", "B")], [finding([a]), finding([a]), finding([a, b]), undecided([b])]);
    expect(tiles.map((t) => [t.count, t.local, t.unresolved])).toEqual([
      [3, 2, 0],
      [1, 0, 1],
    ]);
  });
});
