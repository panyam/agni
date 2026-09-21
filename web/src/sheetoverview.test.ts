import { describe, it, expect } from "vitest";
import { sheetTiles } from "./sheetoverview.js";
import type { SheetRef } from "./gen/agni/v1/webapi/design_pb.js";
import type { FindingItem } from "./findings.js";

function sheet(id: string, name: string): SheetRef {
  return { id, name } as SheetRef;
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
      finding([]), // no geometry join (KiCad net pre-WS1-022): counts nowhere
    ];
    expect(sheetTiles([root, power, clean], findings)).toEqual([
      { id: "s1", name: "Root", count: 2, unresolved: 0 },
      { id: "s2", name: "Power", count: 1, unresolved: 0 },
      { id: "s3", name: "Clean", count: 0, unresolved: 0 },
    ]);
  });

  it("special-cases a single-sheet design to the total findings count", () => {
    // Badges are deliberately empty for one-sheet designs (WS9-024), but everything lives
    // on that sheet — a badge-joined 0 would read as clean on a design with findings.
    const findings = [finding([]), finding([])];
    expect(sheetTiles([root], findings)).toEqual([{ id: "s1", name: "Root", count: 2, unresolved: 0 }]);
  });

  it("falls back to the sheet id when the name is empty and is empty for no design", () => {
    expect(sheetTiles([sheet("s9", "")], [])).toEqual([{ id: "s9", name: "s9", count: 0, unresolved: 0 }]);
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
      { id: "s1", name: "Root", count: 1, unresolved: 1 },
      { id: "s2", name: "Power", count: 0, unresolved: 2 },
    ]);
  });

  // The single-sheet design takes the total rather than the badges, so it needs its own split or the
  // one design shape with no badges keeps the old answer.
  it("splits the single-sheet total too", () => {
    expect(sheetTiles([root], [finding([]), undecided([]), undecided([])])).toEqual([
      { id: "s1", name: "Root", count: 1, unresolved: 2 },
    ]);
  });
});
