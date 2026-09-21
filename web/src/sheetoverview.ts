// The sheet overview's command-down surface (WS9-025), mirroring controls.ts: the presenter
// aggregates per-sheet violation counts from the findings it already holds and pushes
// OverviewState; the panel renders tiles and emits an onSelect(sheetId) intent back up.

import type { SheetRef } from "./gen/agni/v1/webapi/design_pb.js";
import type { FindingItem } from "./findings.js";

// SheetTile is one row of the overview: a sheet and how many current findings live on it.
// A zero count is a rendered "clean" tile, not an omission (the ticket's explicit ask).
//
// `count` is DEFECTS and `unresolved` is the inconclusive results, kept apart because the tile's
// count is read as "problems on this sheet" and an inconclusive result is one the rule could not
// decide (agni issue 350). Two tiles that used to read the same now do not: a sheet with three
// failures and a sheet with three missing datasheets are different work.
export interface SheetTile {
  id: string;
  name: string;
  count: number;
  unresolved: number;
}

// OverviewState is the whole panel: the tiles in design order, the sheet currently shown,
// and the active-rule count (so the panel can tell "no rules selected" from "all clean",
// like the findings panel).
export interface OverviewState {
  tiles: SheetTile[];
  activeId: string;
  ruleCount: number;
}

// OverviewView is the command-down surface the presenter pushes OverviewState to.
export interface OverviewView {
  setState(s: OverviewState): void;
}

// sheetTiles aggregates the current findings onto the design's sheets. Multi-sheet designs
// join through each finding's sheet badges (WS9-024); a finding whose subject has no badges
// (no geometry, or a KiCad net until WS1-022) counts toward no sheet — a known undercount
// with the same root cause as the badge gap. A SINGLE-sheet design is special-cased to the
// total findings count: badges are deliberately empty for one-sheet designs (nowhere to
// navigate), but everything lives on that sheet, so "0" would read as clean on a design
// with findings.
export function sheetTiles(sheets: SheetRef[], findings: FindingItem[]): SheetTile[] {
  if (sheets.length === 1) {
    const defects = findings.filter((f) => !f.inconclusive).length;
    return [{ id: sheets[0].id, name: sheets[0].name || sheets[0].id, count: defects, unresolved: findings.length - defects }];
  }
  const counts = new Map<string, number>();
  const open = new Map<string, number>();
  for (const f of findings) {
    const into = f.inconclusive ? open : counts;
    for (const b of f.sheets) into.set(b.id, (into.get(b.id) ?? 0) + 1);
  }
  return sheets.map((s) => ({
    id: s.id,
    name: s.name || s.id,
    count: counts.get(s.id) ?? 0,
    unresolved: open.get(s.id) ?? 0,
  }));
}
