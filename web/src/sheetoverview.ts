// The sheet overview's command-down surface (WS9-025), mirroring controls.ts. The presenter
// aggregates per-sheet violation counts from the findings it already holds and pushes
// OverviewState, and the panel renders tiles and emits an onSelect(sheetId) intent back up.

import type { SheetRef } from "./gen/agni/v1/webapi/design_pb.js";
import type { FindingItem } from "./findings.js";

// SheetTile is one row of the overview, a sheet and how many current findings live on it. A zero
// count draws no badge, so only a sheet with something on it carries a number (agni issue 1132).
//
// `count` is DEFECTS and `unresolved` is the inconclusive results the rule could not decide, kept
// apart because the tile's count reads as "problems on this sheet" (agni issue 350). A sheet with
// three failures and a sheet with three missing datasheets are different work.
export interface SheetTile {
  id: string;
  // name is the label the tile shows. A name two sheets share is replaced by the sheet's path, so
  // KiCad's /Power/1 and /IO/1 read as Power/1 and IO/1 rather than as two sheets called 1 (agni
  // issue 1141). A name no other sheet has is shown as the design gives it.
  name: string;
  // parentId is the sheet this one nests under, "" for a top-level sheet.
  parentId: string;
  count: number;
  unresolved: number;
  // local is how many of the defects counted here are drawn on this sheet alone. The rest are on a
  // subject drawn on several sheets, a net through the whole design, and count on each of them
  // (agni issue 1144).
  local: number;
}

// OverviewState is the whole panel's state. It holds the tiles in design order, the sheet currently
// shown, the active-rule count so the panel can tell "no rules selected" from "all clean", like
// the findings panel, and how many of those rules have run, since a count before any run is a
// claim about nothing.
export interface OverviewState {
  tiles: SheetTile[];
  activeId: string;
  ruleCount: number;
  ranCount: number;
}

// OverviewView is the command-down surface the presenter pushes OverviewState to.
export interface OverviewView {
  setState(s: OverviewState): void;
}

// sheetTiles aggregates the current findings onto the design's sheets. Multi-sheet designs join
// through each finding's sheet badges (WS9-024), and a finding whose subject has no badges (no
// geometry) counts toward no sheet, a known undercount. A SINGLE-sheet design gets the total findings
// count, because badges are deliberately empty for one-sheet designs (nowhere to navigate) and "0"
// would read as clean on a design with findings.
export function sheetTiles(sheets: SheetRef[], findings: FindingItem[]): SheetTile[] {
  if (sheets.length === 1) {
    const defects = findings.filter((f) => !f.inconclusive).length;
    const s = sheets[0];
    return [{ id: s.id, name: s.name || s.id, parentId: s.parentId ?? "", count: defects, unresolved: findings.length - defects, local: defects }];
  }
  const counts = new Map<string, number>();
  const open = new Map<string, number>();
  const local = new Map<string, number>();
  for (const f of findings) {
    const into = f.inconclusive ? open : counts;
    for (const b of f.sheets) into.set(b.id, (into.get(b.id) ?? 0) + 1);
    if (!f.inconclusive && f.sheets.length === 1) local.set(f.sheets[0].id, (local.get(f.sheets[0].id) ?? 0) + 1);
  }
  const labels = sheetLabels(sheets);
  return sheets.map((s) => ({
    id: s.id,
    name: labels.get(s.id) ?? s.id,
    parentId: s.parentId ?? "",
    count: counts.get(s.id) ?? 0,
    unresolved: open.get(s.id) ?? 0,
    local: local.get(s.id) ?? 0,
  }));
}

// sheetLabels is each sheet's label: its name, or its path when another sheet has the same name.
// The path is the id without a leading "/", which for a KiCad sheet is the chain of sheet names
// from the root.
export function sheetLabels(sheets: SheetRef[]): Map<string, string> {
  const named = new Map<string, number>();
  for (const s of sheets) {
    const n = s.name || s.id;
    named.set(n, (named.get(n) ?? 0) + 1);
  }
  return new Map(
    sheets.map((s) => {
      const n = s.name || s.id;
      const path = s.id.replace(/^\//, "");
      return [s.id, (named.get(n) ?? 0) > 1 && path ? path : n];
    }),
  );
}
