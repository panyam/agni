// The visual diff's pure logic (WS9-005): turning a DiffDesignsResponse into the per-side
// highlight layers each canvas draws, the legend the panel shows, and the sheet pairing the
// selector offers. Everything here is data-in data-out — the DiffPresenter orchestrates the
// RPCs and the views render — so the side-filtering and coloring rules are unit-testable
// without a transport or a DOM.

import type { HighlightSpec } from "./highlights.js";
import type { DiffDesignsResponse, DiffReport } from "./gen/agni/v1/webapi/diff_pb.js";
import type { SheetRef } from "./gen/agni/v1/webapi/design_pb.js";

// DiffSide names one side of the comparison using the wire's vocabulary (DiffDesignsRequest
// a_mount/b_mount): "a" is the old revision, "b" the new.
export type DiffSide = "a" | "b";

// DIFF_COLORS is the one source of the change-class palette (the C12 discipline applied to
// view-side policy: the classes' colors live here and travel to the renderers inside the
// HighlightSpec, never as literals at the draw sites). Existence changes share a hue across
// the component and net vocabularies (added/new green, removed/deleted red); modifications
// are amber (changed/hard) and renames blue, so the two sides read as one color language.
export const DIFF_COLORS: Record<string, string> = {
  added: "#1a9850",
  new: "#1a9850",
  removed: "#d73027",
  deleted: "#d73027",
  changed: "#e08e00",
  hard: "#e08e00",
  renamed: "#4575b4",
  soft: "#8d6cb8",
};

// Which change classes each side draws (the filtering rule documented on
// DiffDesignsResponse): the old side shows what is gone or different, the new side what
// arrived or is different. "changed" components and renamed/hard/soft nets exist on both
// sides, so both draw them; a renamed net's status map carries the old AND new name, and
// each side's geometry only matches its own, so no name filtering is needed here.
const SIDE_COMPONENT: Record<DiffSide, ReadonlySet<string>> = {
  a: new Set(["removed", "changed"]),
  b: new Set(["added", "changed"]),
};
const SIDE_NET: Record<DiffSide, ReadonlySet<string>> = {
  a: new Set(["deleted", "renamed", "hard", "soft"]),
  b: new Set(["new", "renamed", "hard", "soft"]),
};

// CLASS_ORDER fixes the spec order (a later spec wins where primitives overlap, per the
// highlight layer contract): attribute-level changes first, then structural ones, with
// existence changes (added/removed) last so they always show through.
const CLASS_ORDER = ["soft", "renamed", "changed", "hard", "new", "added", "deleted", "removed"] as const;

// sideSpecs turns the response's highlight maps into one HighlightSpec per change class
// present on the given side, colored from DIFF_COLORS, in CLASS_ORDER. Names are sorted so
// the specs (and therefore the overlay requests) are deterministic. Returns [] when nothing
// on this side changed, which the caller treats as "no overlay".
export function sideSpecs(
  componentStatus: Record<string, string>,
  netStatus: Record<string, string>,
  side: DiffSide,
): HighlightSpec[] {
  const comps = new Map<string, string[]>();
  for (const [ref, cls] of Object.entries(componentStatus)) {
    if (SIDE_COMPONENT[side].has(cls)) (comps.get(cls) ?? comps.set(cls, []).get(cls)!).push(ref);
  }
  const nets = new Map<string, string[]>();
  for (const [name, cls] of Object.entries(netStatus)) {
    if (SIDE_NET[side].has(cls)) (nets.get(cls) ?? nets.set(cls, []).get(cls)!).push(name);
  }
  const specs: HighlightSpec[] = [];
  for (const cls of CLASS_ORDER) {
    const c = comps.get(cls);
    const n = nets.get(cls);
    if (!c && !n) continue;
    const spec: HighlightSpec = { color: DIFF_COLORS[cls] };
    if (c) spec.components = c.sort();
    if (n) spec.nets = n.sort();
    specs.push(spec);
  }
  return specs;
}

// LegendEntry is one row of the diff legend: the change class, its display label, its
// swatch color, and how many entities carry it (design-wide, per the ticket — the legend
// counts the whole diff, not the visible sheet).
export interface LegendEntry {
  cls: string;
  label: string;
  color: string;
  count: number;
}

// CLASS_LABELS is the display name of each change class, shared by the legend chips and the
// changes panel's group headers so the two surfaces speak one vocabulary.
export const CLASS_LABELS: Record<string, string> = {
  added: "added",
  removed: "removed",
  changed: "changed",
  new: "new net",
  deleted: "deleted net",
  renamed: "renamed net",
  hard: "hard net change",
  soft: "soft net change",
};

// legendEntries derives the legend from the report: one row per change class with a nonzero
// count, in a fixed narrative order (component classes, then net classes). A component with
// several changed fields appears once per field in the report, so "changed" counts distinct
// ref_des.
export function legendEntries(report: DiffReport | undefined): LegendEntry[] {
  if (!report) return [];
  const changedRefs = new Set(report.componentsChanged.map((c) => c.refDes));
  const netCount = (kind: string) => report.nets.filter((n) => n.kind === kind).length;
  const rows = [
    { cls: "added", count: report.componentsAdded.length },
    { cls: "removed", count: report.componentsRemoved.length },
    { cls: "changed", count: changedRefs.size },
    { cls: "new", count: netCount("new") },
    { cls: "deleted", count: netCount("deleted") },
    { cls: "renamed", count: netCount("renamed") },
    { cls: "hard", count: netCount("hard") },
    { cls: "soft", count: netCount("soft") },
  ];
  return rows.filter((r) => r.count > 0).map((r) => ({ ...r, label: CLASS_LABELS[r.cls], color: DIFF_COLORS[r.cls] }));
}

// itemPairs lists the sheet-pair indexes that show an item (either side's sheet matching),
// for the changes panel's navigation badges. Empty when the item has no sheet entries.
export function itemPairs(item: ChangedItem, pairs: SheetPair[]): number[] {
  const out: number[] = [];
  for (let i = 0; i < pairs.length; i++) {
    const p = pairs[i];
    if ((p.aId !== "" && item.aSheets.includes(p.aId)) || (p.bId !== "" && item.bSheets.includes(p.bId))) out.push(i);
  }
  return out;
}

// ChangedItem is one row of the changes panel (WS9-006): a changed entity with its class,
// its human detail line, and the sheets it lives on per side (from the response's sheet
// maps; empty when that side has no geometry or, for KiCad nets, until WS1-022). key is the
// entity's current name (a renamed net's NEW name; oldName carries the other), which is also
// the highlight join key on the b side.
export interface ChangedItem {
  kind: "component" | "net";
  cls: string;
  key: string;
  oldName: string; // renames only, else ""
  detail: string;
  aSheets: string[];
  bSheets: string[];
}

// itemId is the selection identity of an item (kinds and names can collide across the two
// vocabularies — a net and a component may share a name).
export function itemId(it: Pick<ChangedItem, "kind" | "key">): string {
  return `${it.kind}:${it.key}`;
}

// ITEM_CLASS_ORDER is the panel's group order: components first, then nets, existence
// changes before modifications within each — the order a reviewer triages in.
export const ITEM_CLASS_ORDER = ["added", "removed", "changed", "new", "deleted", "renamed", "hard", "soft"] as const;

type SheetIdsMap = DiffDesignsResponse["componentSheetsA"];

function sheetIds(m: SheetIdsMap, key: string): string[] {
  return m?.[key]?.ids ?? [];
}

// changedItems flattens the response into the panel's rows, grouped in ITEM_CLASS_ORDER
// (report order within a class — the diff sorts components by ref_des and groups nets by
// classification). A component's several changed fields fold into one row; a renamed net's
// a-side sheets come from its OLD name (the name a's geometry carries).
export function changedItems(resp: DiffDesignsResponse): ChangedItem[] {
  const r = resp.report;
  if (!r) return [];
  const items: ChangedItem[] = [];
  const comp = (key: string, cls: string, detail: string): ChangedItem => ({
    kind: "component",
    cls,
    key,
    oldName: "",
    detail,
    aSheets: sheetIds(resp.componentSheetsA, key),
    bSheets: sheetIds(resp.componentSheetsB, key),
  });
  for (const ref of r.componentsAdded) items.push(comp(ref, "added", ""));
  for (const ref of r.componentsRemoved) items.push(comp(ref, "removed", ""));
  const changed = new Map<string, string[]>();
  for (const c of r.componentsChanged) {
    (changed.get(c.refDes) ?? changed.set(c.refDes, []).get(c.refDes)!).push(`${c.field}: ${c.old} → ${c.new}`);
  }
  for (const [ref, fields] of changed) items.push(comp(ref, "changed", fields.join("; ")));
  const netItems: ChangedItem[] = r.nets.map((n) => ({
    kind: "net",
    cls: n.kind,
    key: n.name,
    oldName: n.oldName,
    detail:
      n.kind === "renamed"
        ? `was ${n.oldName}`
        : n.kind === "hard"
          ? [...n.added.map((c) => `+${c}`), ...n.removed.map((c) => `−${c}`)].join(" ")
          : "",
    aSheets: sheetIds(resp.netSheetsA, n.kind === "renamed" ? n.oldName : n.name),
    bSheets: sheetIds(resp.netSheetsB, n.name),
  }));
  const rank = (cls: string) => {
    const i = ITEM_CLASS_ORDER.indexOf(cls as (typeof ITEM_CLASS_ORDER)[number]);
    return i < 0 ? ITEM_CLASS_ORDER.length : i;
  };
  return [...items, ...netItems].sort((a, b) => rank(a.cls) - rank(b.cls) || 0);
}

// focusSpecs is the emphasis highlight for one selected item: sideSpecs over a singleton
// status map, so the side-filtering rule is inherited (a removed component emphasizes only
// on a, an added one only on b) and a renamed net carries both names so each side joins by
// its own.
export function focusSpecs(item: ChangedItem, side: DiffSide): HighlightSpec[] {
  const comps: Record<string, string> = {};
  const nets: Record<string, string> = {};
  if (item.kind === "component") {
    comps[item.key] = item.cls;
  } else {
    nets[item.key] = item.cls;
    if (item.oldName) nets[item.oldName] = item.cls;
  }
  return sideSpecs(comps, nets, side);
}

// ghostSpecs is the old side's contribution to the union canvas (WS9-007): ONLY what exists
// nowhere in b — removed components and deleted nets. Everything else on the union canvas is
// b's geometry (neutral) plus b's own highlight classes; drawing a's changed/renamed/hard
// entities too would double-paint them.
export function ghostSpecs(componentStatus: Record<string, string>, netStatus: Record<string, string>): HighlightSpec[] {
  const comps = Object.fromEntries(Object.entries(componentStatus).filter(([, c]) => c === "removed"));
  const nets = Object.fromEntries(Object.entries(netStatus).filter(([, c]) => c === "deleted"));
  return sideSpecs(comps, nets, "a");
}

// Frame is a sheet document's framed size in px (the SVG width/height attributes; the
// renderer frames documents with a 1:1 viewBox).
export interface Frame {
  w: number;
  h: number;
}

// svgFrame reads the frame from sheet SVG markup with a regex rather than the DOM, so the
// presenter can hold alignment evidence without touching a document (C3).
export function svgFrame(markup: string): Frame | null {
  const m = /<svg[^>]*\bwidth="([\d.]+)"[^>]*\bheight="([\d.]+)"/.exec(markup);
  if (!m) return null;
  const w = parseFloat(m[1]);
  const h = parseFloat(m[2]);
  return w > 0 && h > 0 ? { w, h } : null;
}

// AlignmentVerdict says whether superimposing the two revisions on one canvas would be
// truthful; reason (set only when not ok) is surfaced as the disabled toggle's explanation.
export interface AlignmentVerdict {
  ok: boolean;
  reason: string;
}

// Frames may differ by this fraction before the overlay is refused (renderers pad a little).
const FRAME_TOLERANCE = 0.02;
// A shared component may drift by this fraction of the shared-placement spread.
const PLACEMENT_TOLERANCE = 0.01;

type PlacementMap = DiffDesignsResponse["sharedPlacementsA"];

// checkAlignment is the WS9-007 overlay gate for one sheet pair: both sides must exist,
// their frames must match within FRAME_TOLERANCE, and the shared components placed on this
// pair must sit within PLACEMENT_TOLERANCE of each other (normalized by the shared
// placements' own spread — placement coordinates are geometry units, not frame px). Sparse
// evidence degrades gracefully: no shared placements on this pair (netlist-only sides, tiny
// sheets, a spread too small to normalize by) falls back to the frame verdict alone, so a
// same-tool re-export is not refused just because the sample is thin.
export function checkAlignment(
  pair: SheetPair,
  placementsA: PlacementMap,
  placementsB: PlacementMap,
  frameA: Frame | null,
  frameB: Frame | null,
): AlignmentVerdict {
  if (!pair.aId || !pair.bId) return { ok: false, reason: "this sheet exists on one side only" };
  if (!frameA || !frameB) return { ok: false, reason: "sheet frames unknown" };
  if (
    Math.abs(frameA.w - frameB.w) > FRAME_TOLERANCE * Math.max(frameA.w, frameB.w) ||
    Math.abs(frameA.h - frameB.h) > FRAME_TOLERANCE * Math.max(frameA.h, frameB.h)
  ) {
    return { ok: false, reason: "page sizes differ between revisions" };
  }
  const shared: { ref: string; ax: number; ay: number; bx: number; by: number }[] = [];
  for (const [ref, pa] of Object.entries(placementsA ?? {})) {
    const pb = placementsB?.[ref];
    if (!pb || pa.sheet !== pair.aId || pb.sheet !== pair.bId) continue;
    shared.push({ ref, ax: pa.x, ay: pa.y, bx: pb.x, by: pb.y });
  }
  if (shared.length < 2) return { ok: true, reason: "" }; // spread needs two points; frames agreed
  const xs = shared.map((s) => s.ax);
  const ys = shared.map((s) => s.ay);
  const spread = Math.max(Math.max(...xs) - Math.min(...xs), Math.max(...ys) - Math.min(...ys));
  if (spread <= 0) return { ok: true, reason: "" };
  for (const s of shared) {
    if (Math.abs(s.ax - s.bx) > PLACEMENT_TOLERANCE * spread || Math.abs(s.ay - s.by) > PLACEMENT_TOLERANCE * spread) {
      return { ok: false, reason: `shared components moved between revisions (${s.ref})` };
    }
  }
  return { ok: true, reason: "" };
}

// SheetPair is one row of the sheet selector: the display name and the sheet id on each
// side, "" where that side has no sheet of this name (an added or removed page).
export interface SheetPair {
  name: string;
  aId: string;
  bId: string;
}

// pairSheets matches the two designs' sheet lists by name (each B sheet consumed at most
// once), in A's order, then appends B-only sheets in B's order. A sheet with an empty name
// pairs by nothing and displays as its id. When NO name matches at all, it falls back to
// pairing positionally — successive revisions routinely rename their pages (a title block
// carrying the revision), and a strict by-name pairing would then offer only one-sided
// views; index order is the best structural guess and the selector still shows both names.
export function pairSheets(a: SheetRef[], b: SheetRef[]): SheetPair[] {
  const used = new Set<string>();
  const out: SheetPair[] = a.map((s) => {
    const m = s.name ? b.find((t) => t.name === s.name && !used.has(t.id)) : undefined;
    if (m) used.add(m.id);
    return { name: s.name || s.id, aId: s.id, bId: m?.id ?? "" };
  });
  if (!out.some((p) => p.aId && p.bId)) {
    return a.map((s, i) => {
      const t = b[i];
      const name = t && t.name !== s.name ? `${s.name || s.id} / ${t.name || t.id}` : s.name || s.id;
      return { name, aId: s.id, bId: t?.id ?? "" };
    }).concat(b.slice(a.length).map((t) => ({ name: t.name || t.id, aId: "", bId: t.id })));
  }
  for (const t of b) if (!used.has(t.id)) out.push({ name: t.name || t.id, aId: "", bId: t.id });
  return out;
}
