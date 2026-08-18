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
      { id: "s1", name: "Root", count: 2 },
      { id: "s2", name: "Power", count: 1 },
      { id: "s3", name: "Clean", count: 0 },
    ]);
  });

  it("special-cases a single-sheet design to the total findings count", () => {
    // Badges are deliberately empty for one-sheet designs (WS9-024), but everything lives
    // on that sheet — a badge-joined 0 would read as clean on a design with findings.
    const findings = [finding([]), finding([])];
    expect(sheetTiles([root], findings)).toEqual([{ id: "s1", name: "Root", count: 2 }]);
  });

  it("falls back to the sheet id when the name is empty and is empty for no design", () => {
    expect(sheetTiles([sheet("s9", "")], [])).toEqual([{ id: "s9", name: "s9", count: 0 }]);
    expect(sheetTiles([], [])).toEqual([]);
  });
});
