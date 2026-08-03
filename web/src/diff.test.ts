import { describe, it, expect } from "vitest";
import {
  DIFF_COLORS,
  changedItems,
  checkAlignment,
  focusSpecs,
  ghostSpecs,
  itemId,
  legendEntries,
  pairSheets,
  sideSpecs,
  svgFrame,
  type ChangedItem,
  type SheetPair,
} from "./diff.js";
import type { DiffDesignsResponse, DiffReport } from "./gen/agni/v1/webapi/diff_pb.js";
import type { SheetRef } from "./gen/agni/v1/webapi/design_pb.js";

// The full-taxonomy status maps the server would emit for the diffdemo fixture pair:
// R2 removed, R3 changed, R4 added; OLD deleted, NEWNET new, SIG->SIG_CLK renamed (both
// names carry "renamed"), HARD hard, STYLE soft.
const componentStatus = { R2: "removed", R3: "changed", R4: "added" };
const netStatus = {
  OLD: "deleted",
  NEWNET: "new",
  SIG: "renamed",
  SIG_CLK: "renamed",
  HARD: "hard",
  STYLE: "soft",
};

// specWithComponent / specWithNet find the (single) spec carrying an entity, so assertions
// key on content — several classes share a color (added/new, removed/deleted, changed/hard),
// so color is not a spec identity.
type Spec = { color?: string; components?: string[]; nets?: string[] };
const specWithComponent = (specs: Spec[], ref: string) => specs.find((s) => (s.components ?? []).includes(ref));
const specWithNet = (specs: Spec[], name: string) => specs.find((s) => (s.nets ?? []).includes(name));

describe("sideSpecs", () => {
  it("filters the old side to removed/changed components and deleted/renamed/hard/soft nets", () => {
    const specs = sideSpecs(componentStatus, netStatus, "a");
    expect(specWithComponent(specs, "R2")?.color).toBe(DIFF_COLORS.removed);
    expect(specWithComponent(specs, "R3")?.color).toBe(DIFF_COLORS.changed);
    expect(specWithNet(specs, "OLD")?.color).toBe(DIFF_COLORS.deleted);
    expect(specWithNet(specs, "SIG")?.nets).toEqual(["SIG", "SIG_CLK"]);
    expect(specWithNet(specs, "SIG")?.color).toBe(DIFF_COLORS.renamed);
    expect(specWithNet(specs, "STYLE")?.color).toBe(DIFF_COLORS.soft);
    expect(specWithNet(specs, "HARD")?.color).toBe(DIFF_COLORS.hard);
    // Nothing "added"/"new" leaks onto the old side.
    expect(specWithComponent(specs, "R4")).toBeUndefined();
    expect(specWithNet(specs, "NEWNET")).toBeUndefined();
  });

  it("filters the new side to added/changed components and new/renamed/hard/soft nets", () => {
    const specs = sideSpecs(componentStatus, netStatus, "b");
    expect(specWithComponent(specs, "R4")?.color).toBe(DIFF_COLORS.added);
    expect(specWithNet(specs, "NEWNET")?.color).toBe(DIFF_COLORS.new);
    expect(specWithComponent(specs, "R3")?.color).toBe(DIFF_COLORS.changed);
    expect(specWithComponent(specs, "R2")).toBeUndefined();
    expect(specWithNet(specs, "OLD")).toBeUndefined();
  });

  it("orders existence changes after modifications so they win primitive overlaps", () => {
    const specs = sideSpecs(componentStatus, netStatus, "a");
    const changedIdx = specs.findIndex((s) => (s.components ?? []).includes("R3"));
    const removedIdx = specs.findIndex((s) => (s.components ?? []).includes("R2"));
    const deletedIdx = specs.findIndex((s) => (s.nets ?? []).includes("OLD"));
    expect(changedIdx).toBeGreaterThanOrEqual(0);
    expect(removedIdx).toBeGreaterThan(changedIdx);
    expect(deletedIdx).toBeGreaterThan(changedIdx);
  });

  it("returns [] when nothing changed", () => {
    expect(sideSpecs({}, {}, "a")).toEqual([]);
    expect(sideSpecs({}, {}, "b")).toEqual([]);
  });
});

function report(over: Partial<DiffReport>): DiffReport {
  return {
    componentsAdded: [],
    componentsRemoved: [],
    componentsChanged: [],
    nets: [],
    ...over,
  } as DiffReport;
}

describe("legendEntries", () => {
  it("counts distinct changed ref_des and per-kind nets, omitting zero classes", () => {
    const r = report({
      componentsAdded: ["R4"],
      componentsChanged: [
        { refDes: "R3", field: "Value", old: "10k", new: "22k" },
        { refDes: "R3", field: "parts", old: "a", new: "b" },
      ] as DiffReport["componentsChanged"],
      nets: [
        { kind: "renamed", name: "SIG_CLK", oldName: "SIG", added: [], removed: [] },
        { kind: "hard", name: "HARD", oldName: "", added: ["R4.1"], removed: [] },
      ] as unknown as DiffReport["nets"],
    });
    const legend = legendEntries(r);
    expect(legend.map((e) => [e.cls, e.count])).toEqual([
      ["added", 1],
      ["changed", 1], // two changed fields on R3 = one changed component
      ["renamed", 1],
      ["hard", 1],
    ]);
    for (const e of legend) expect(e.color).toBe(DIFF_COLORS[e.cls]);
  });

  it("is empty without a report or without changes", () => {
    expect(legendEntries(undefined)).toEqual([]);
    expect(legendEntries(report({}))).toEqual([]);
  });
});

describe("changedItems (WS9-006)", () => {
  const resp = {
    report: {
      componentsAdded: ["R4"],
      componentsRemoved: ["R2"],
      componentsChanged: [
        { refDes: "R3", field: "Value", old: "10k", new: "22k" },
        { refDes: "R3", field: "parts", old: "a", new: "b" },
      ],
      nets: [
        { kind: "hard", name: "HARD", oldName: "", added: ["R4.1"], removed: ["R2.2"] },
        { kind: "renamed", name: "SIG_CLK", oldName: "SIG", added: [], removed: [] },
        { kind: "new", name: "NEWNET", oldName: "", added: [], removed: [] },
      ],
    },
    componentStatus: { R2: "removed", R3: "changed", R4: "added" },
    netStatus: { HARD: "hard", SIG: "renamed", SIG_CLK: "renamed", NEWNET: "new" },
    componentSheetsA: { R2: { ids: ["a1", "a2"] } },
    componentSheetsB: { R4: { ids: ["b1"] } },
    netSheetsA: { SIG: { ids: ["a1"] } },
    netSheetsB: { SIG_CLK: { ids: ["b1"] } },
  } as unknown as DiffDesignsResponse;

  it("flattens to class-ordered rows with folded fields and per-side sheets", () => {
    const items = changedItems(resp);
    expect(items.map((i) => `${i.cls}:${i.key}`)).toEqual([
      "added:R4",
      "removed:R2",
      "changed:R3",
      "new:NEWNET",
      "renamed:SIG_CLK",
      "hard:HARD",
    ]);
    const by = new Map(items.map((i) => [itemId(i), i]));
    expect(by.get("component:R3")?.detail).toBe("Value: 10k → 22k; parts: a → b");
    expect(by.get("component:R2")?.aSheets).toEqual(["a1", "a2"]);
    expect(by.get("component:R2")?.bSheets).toEqual([]);
    expect(by.get("net:HARD")?.detail).toBe("+R4.1 −R2.2");
    expect(by.get("net:SIG_CLK")?.detail).toBe("was SIG");
    // The renamed net's a-side sheets come from its OLD name (a's geometry only knows SIG).
    expect(by.get("net:SIG_CLK")?.aSheets).toEqual(["a1"]);
    expect(by.get("net:SIG_CLK")?.bSheets).toEqual(["b1"]);
  });

  it("is empty without a report", () => {
    expect(changedItems({} as DiffDesignsResponse)).toEqual([]);
  });
});

describe("focusSpecs (WS9-006)", () => {
  const item = (over: Partial<ChangedItem>): ChangedItem => ({
    kind: "component",
    cls: "removed",
    key: "R2",
    oldName: "",
    detail: "",
    aSheets: [],
    bSheets: [],
    ...over,
  });

  it("inherits the side filter: a removed component emphasizes only on the old side", () => {
    const a = focusSpecs(item({}), "a");
    expect(a).toHaveLength(1);
    expect(a[0].components).toEqual(["R2"]);
    expect(a[0].color).toBe(DIFF_COLORS.removed);
    expect(focusSpecs(item({}), "b")).toEqual([]);
  });

  it("carries both names of a renamed net so each side joins by its own", () => {
    const renamed = item({ kind: "net", cls: "renamed", key: "SIG_CLK", oldName: "SIG" });
    for (const side of ["a", "b"] as const) {
      const specs = focusSpecs(renamed, side);
      expect(specs).toHaveLength(1);
      expect(specs[0].nets).toEqual(["SIG", "SIG_CLK"]);
    }
  });
});

describe("ghostSpecs (WS9-007)", () => {
  it("keeps only removed components and deleted nets — b draws everything else", () => {
    const specs = ghostSpecs(componentStatus, netStatus);
    expect(specs.flatMap((s) => s.components ?? [])).toEqual(["R2"]);
    expect(specs.flatMap((s) => s.nets ?? [])).toEqual(["OLD"]);
    expect(ghostSpecs({ R3: "changed" }, { SIG: "renamed" })).toEqual([]);
  });
});

describe("svgFrame (WS9-007)", () => {
  it("parses the renderer's frame and rejects unframed markup", () => {
    expect(svgFrame('<svg xmlns="http://www.w3.org/2000/svg" width="1680.0" height="924.8" viewBox="0 0 1680.0 924.8">')).toEqual({
      w: 1680,
      h: 924.8,
    });
    expect(svgFrame("<svg><rect/></svg>")).toBeNull();
    expect(svgFrame("")).toBeNull();
  });
});

describe("checkAlignment (WS9-007)", () => {
  const pair: SheetPair = { name: "Top", aId: "a1", bId: "b1" };
  const frame = { w: 1000, h: 800 };
  type Placements = DiffDesignsResponse["sharedPlacementsA"];
  // Two shared anchors spread across the sheet; positions identical between sides.
  const aligned: { a: Placements; b: Placements } = {
    a: { R1: { sheet: "a1", x: 0, y: 0 }, C1: { sheet: "a1", x: 900, y: 700 } } as unknown as Placements,
    b: { R1: { sheet: "b1", x: 0, y: 0 }, C1: { sheet: "b1", x: 900, y: 700 } } as unknown as Placements,
  };

  it("passes identical frames with agreeing placements", () => {
    expect(checkAlignment(pair, aligned.a, aligned.b, frame, frame)).toEqual({ ok: true, reason: "" });
  });

  it("refuses one-sided pairs, unknown frames, and diverging page sizes", () => {
    expect(checkAlignment({ name: "X", aId: "a1", bId: "" }, aligned.a, aligned.b, frame, frame).ok).toBe(false);
    expect(checkAlignment(pair, aligned.a, aligned.b, null, frame).ok).toBe(false);
    const verdict = checkAlignment(pair, aligned.a, aligned.b, frame, { w: 1200, h: 800 });
    expect(verdict.ok).toBe(false);
    expect(verdict.reason).toContain("page sizes");
  });

  it("refuses when a shared component moved beyond tolerance, naming it", () => {
    const moved = {
      ...aligned.b,
      C1: { sheet: "b1", x: 700, y: 500 },
    } as unknown as Placements;
    const verdict = checkAlignment(pair, aligned.a, moved, frame, frame);
    expect(verdict.ok).toBe(false);
    expect(verdict.reason).toContain("C1");
  });

  it("tolerates sub-threshold drift and falls back to frames on thin evidence", () => {
    const drifted = {
      ...aligned.b,
      C1: { sheet: "b1", x: 903, y: 700 }, // 3 units of a 900 spread — under 1%
    } as unknown as Placements;
    expect(checkAlignment(pair, aligned.a, drifted, frame, frame).ok).toBe(true);
    // No shared placements on THIS pair (they live on other sheets) -> frames decide.
    const elsewhere = { R1: { sheet: "a9", x: 0, y: 0 } } as unknown as Placements;
    expect(checkAlignment(pair, elsewhere, elsewhere, frame, frame).ok).toBe(true);
    // Empty maps (netlist-only side) -> frames decide.
    expect(checkAlignment(pair, {} as unknown as Placements, {} as unknown as Placements, frame, frame).ok).toBe(true);
  });
});

function sheet(id: string, name: string): SheetRef {
  return { id, name } as SheetRef;
}

describe("pairSheets", () => {
  it("matches by name in A's order and appends B-only sheets", () => {
    const a = [sheet("a1", "Top"), sheet("a2", "Power"), sheet("a3", "Legacy")];
    const b = [sheet("b0", "Intro"), sheet("b1", "Top"), sheet("b2", "Power")];
    expect(pairSheets(a, b)).toEqual([
      { name: "Top", aId: "a1", bId: "b1" },
      { name: "Power", aId: "a2", bId: "b2" },
      { name: "Legacy", aId: "a3", bId: "" },
      { name: "Intro", aId: "", bId: "b0" },
    ]);
  });

  it("pairs positionally when no names match (revisions renaming their pages)", () => {
    // The kicad diffdemo pair: one sheet each, titled "… revision A" / "… revision B". A
    // strict by-name pairing would offer only one-sided views.
    const a = [sheet("a1", "Demo rev A")];
    const b = [sheet("b1", "Demo rev B"), sheet("b2", "Extra")];
    expect(pairSheets(a, b)).toEqual([
      { name: "Demo rev A / Demo rev B", aId: "a1", bId: "b1" },
      { name: "Extra", aId: "", bId: "b2" },
    ]);
  });

  it("consumes each B sheet once for duplicate names and falls back to ids for empty names", () => {
    const a = [sheet("a1", "S"), sheet("a2", "S"), sheet("a3", "")];
    const b = [sheet("b1", "S")];
    expect(pairSheets(a, b)).toEqual([
      { name: "S", aId: "a1", bId: "b1" },
      { name: "S", aId: "a2", bId: "" },
      { name: "a3", aId: "a3", bId: "" },
    ]);
  });
});
