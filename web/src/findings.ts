// The findings panel's command-down surface, mirroring controls.ts. The presenter owns the
// findings list and which one is focused; it pushes FindingsState and the panel renders it,
// emitting an onSelect(subject) intent back up. Group-by is the panel's own view state.

import type { HighlightSpec } from "./highlights.js";
import { LocateReason } from "./gen/agni/v1/checks/checks_pb.js";

// SheetBadge locates a finding's subject on one sheet (WS9-024): the sheet id drives navigation
// (showSheet) and the name is what the badge displays. The presenter denormalizes the wire's
// sheet ids against the design's SheetRefs, and only for multi-sheet designs — a single-sheet
// design pushes findings with no badges, so panels need no own "is this design multi-sheet" rule.
export interface SheetBadge {
  id: string;
  name: string;
}

// FindingItem is the view-side shape of a rule finding (the wire checks.Finding, minus proto
// machinery). subject is the entity ref (a net name or a ref_des) and the highlight join key;
// kind says what it is (so grouping and highlighting are exact, not string-guessed), pin is set
// only for a pin subject, and category is the finding's rule's category tag (denormalized for the
// by-category group-by). sheets are the badges for where the subject lives (one per sheet a
// spanning net touches; empty for single-sheet designs or when the server had no geometry).
export interface FindingItem {
  rule: string;
  category: string;
  // profile is the finding's rule's "profile" tag (WS9-041), the interface an interface-profile
  // rule checks (e.g. "SPI_NOR"); "" for a rule that is not part of an interface profile. Denormalized
  // from the rule catalog for the by-interface group-by, like category.
  profile: string;
  severity: string; // "error" | "warning" | "info"
  kind: string; // "net" | "component" | "pin" | "bus"
  subject: string;
  pin: string;
  // netId is the per-instance net identity (webapi Subject.net_id) for a net subject: two findings
  // on nets that share a subject name have distinct netIds, so they collapse as separate instances
  // and each locates to ITS wires (WS9). Empty for a component/pin subject or a pinless net.
  netId: string;
  // busId is the source id (webapi Subject.bus_id, a KiCad uuid) for a kind="bus" subject, so a
  // bus-not-modeled finding highlights its own drawn bus (WS7-042b) and two identically-labeled
  // buses stay distinct instances. Empty for every other subject kind.
  busId: string;
  message: string;
  sheets: SheetBadge[];
  // locateReason (checks.Finding.locate_reason) explains why clicking this finding may highlight
  // nothing, computed server-side from the geometry (WS7-042c): BUS_NOT_DRAWN for a bus with no drawn
  // wire. UNSPECIFIED (the default) means the subject is drawn and highlights.
  locateReason: LocateReason;
}

// SkippedRuleItem is one selected rule that could not evaluate, with the reason the ENGINE gave.
// The reason is passed through rather than reworded: the rule decides why it cannot run, and a
// sentence composed here would be a second opinion that drifts from the gate.
export interface SkippedRuleItem {
  rule: string;
  reason: string;
}

export interface FindingsState {
  findings: FindingItem[];
  // subject of the focused finding, "" when none (the whole selection is highlighted instead).
  selected: string;
  // number of rules currently selected, so the panel tells "no rules selected" (nothing ran) from
  // "no findings" (rules ran clean).
  ruleCount: number;
  // pending is the count of selected rules whose findings are not yet computed — checks are
  // on-demand (WS9), so a design opens (or the selection changes) with the results uncomputed. It
  // badges the Run button and distinguishes "press Run to evaluate" (pending > 0, empty list) from
  // "ran clean" (pending 0, empty list).
  pending: number;
  // running is true while a check run is in flight, so the panel disables the Run button.
  running: boolean;
  // skipped names the selected rules that could NOT run on this design, and why.
  //
  // Without it the panel cannot tell a clean board from an unanswered question. A rule whose fact
  // tier this design lacks — a board rule on a netlist, a datasheet rule with no corpus — is gated
  // before it evaluates, so it produces no findings, and an empty list reads as "nothing wrong". This
  // is the default-open panel, so that is the first thing most people see and the last thing they
  // would think to doubt.
  skipped: SkippedRuleItem[];
  // ruleSummaries maps a rule name to its catalog one-liner, shown as a group-header subtitle — the
  // per-rule description the retired report panel carried. A rule absent from the map renders none.
  ruleSummaries: Record<string, string>;
}

export interface FindingsView {
  setState: (s: FindingsState) => void;
  // setFindingLocateNote shows (or clears with "") a server-authoritative note under the checks
  // table when a clicked finding can't be located — a bus with no drawn wire (WS7-042c). Mirrors the
  // query panel's setLocateNote.
  setFindingLocateNote: (note: string) => void;
}

// FindingGroupAxis is what the checks panel groups findings by. "kind" is the entity axis
// (net/component/pin), exact via FindingItem.kind.
export type FindingGroupAxis = "rule" | "category" | "severity" | "kind" | "profile";

// groupFindings buckets findings by the chosen axis, preserving first-appearance order of values,
// as [value, items][].
export function groupFindings(findings: FindingItem[], axis: FindingGroupAxis): [string, FindingItem[]][] {
  const order: string[] = [];
  const by = new Map<string, FindingItem[]>();
  for (const f of findings) {
    const v = axis === "kind" ? f.kind : axis === "severity" ? f.severity : axis === "category" ? f.category : axis === "profile" ? f.profile : f.rule;
    if (!by.has(v)) {
      by.set(v, []);
      order.push(v);
    }
    by.get(v)!.push(f);
  }
  return order.map((v) => [v, by.get(v)!]);
}

// CollapsedFinding folds every finding sharing (rule, subject, pin, severity, message) into one
// row: head is the representative (first-seen) and instances are all the folded findings (length
// >= 1). A rule that fires once per distinct entity with the SAME display fields (duplicate-net-name
// on N same-named nets) collapses to one row with instances.length N, instead of N identical rows.
// The instances stay addressable so each can be a click-to-locate sub-row (distinct once they carry
// a per-instance identity — WS9 Phase 2; identical until then).
export interface CollapsedFinding {
  head: FindingItem;
  instances: FindingItem[];
}

// findingKey is the collapse/expand identity of a finding: the tuple that must match for two
// findings to fold into one row. Parts are joined by the NUL escape "\u0000" (written as the
// escape, never a literal NUL, so the source stays text) — it cannot appear in a
// rule/subject/pin/message, so distinct tuples never collide. The panel also keys a row's
// expand state on it. busId is included so two identically-labeled buses stay distinct rows that
// each locate their own trunk (WS7-042b); it is "" for every non-bus finding, so their collapse
// behavior is unchanged.
export function findingKey(f: FindingItem): string {
  return [f.rule, f.subject, f.pin, f.severity, f.message, f.busId].join("\u0000");
}

// collapseInstances groups adjacent-or-not findings by findingKey, preserving first-appearance
// order of the representative. Every input finding lands in exactly one CollapsedFinding.
export function collapseInstances(items: FindingItem[]): CollapsedFinding[] {
  const order: string[] = [];
  const by = new Map<string, CollapsedFinding>();
  for (const f of items) {
    const k = findingKey(f);
    const c = by.get(k);
    if (c) c.instances.push(f);
    else {
      by.set(k, { head: f, instances: [f] });
      order.push(k);
    }
  }
  return order.map((k) => by.get(k)!);
}

// FindingSortKey is the column the table sorts on; each falls back through rule then subject so the
// order is total (stable ties aside).
export type FindingSortKey = "severity" | "rule" | "subject";

const SEV_RANK: Record<string, number> = { error: 0, warning: 1, info: 2 };

// severityRank orders severities worst-first (error 0, warning 1, info 2, unknown last), matching the
// server report's section order (GetCheckReport) so the client severity view cannot reorder it.
export function severityRank(sev: string): number {
  return SEV_RANK[sev] ?? 3;
}

const cmpStr = (a: string, b: string): number => (a < b ? -1 : a > b ? 1 : 0);

// sortFindings returns a new array ordered by key then the rule/subject fallback chain; dir 1 is
// ascending (worst-first for severity), -1 reverses the whole comparison.
export function sortFindings(items: FindingItem[], key: FindingSortKey, dir: 1 | -1): FindingItem[] {
  const cmp = (a: FindingItem, b: FindingItem): number => {
    const base =
      key === "severity"
        ? severityRank(a.severity) - severityRank(b.severity) || cmpStr(a.rule, b.rule) || cmpStr(a.subject, b.subject)
        : key === "rule"
          ? cmpStr(a.rule, b.rule) || cmpStr(a.subject, b.subject)
          : cmpStr(a.subject, b.subject) || cmpStr(a.rule, b.rule);
    return base * dir;
  };
  return [...items].sort(cmp);
}

// severitySections projects findings into worst-first [severity, count] tiers, the same shape and
// order the server report's sections carry (GetCheckReport). It is the client's severity rollup and
// the parity oracle: a test pins it against reportFromWire so the two cannot drift (WS3-022).
export function severitySections(items: FindingItem[]): { severity: string; count: number }[] {
  const counts = new Map<string, number>();
  for (const f of items) counts.set(f.severity, (counts.get(f.severity) ?? 0) + 1);
  return [...counts.entries()]
    .map(([severity, count]) => ({ severity, count }))
    .sort((a, b) => severityRank(a.severity) - severityRank(b.severity));
}

// HighlightSubject is the minimal shape subjectsToSpecs needs: an entity ref, its kind, and (for a
// pin) its pin designator. FindingItem satisfies it structurally, and so does a bare query result
// cell (WS9-038) — so a query entity highlights through the exact same bucketing as a finding,
// without being a finding.
export interface HighlightSubject {
  kind: string; // "net" | "component" | "pin" | "bus"
  subject: string;
  pin: string;
  // netId is the per-instance net identity (optional): when present on a net subject, the spec
  // targets THAT instance by id, so two same-named nets highlight separately. A bare query cell
  // (WS9-038) has no id and joins by name.
  netId?: string;
  // busId is the source id (optional) for a kind="bus" subject: a bus has no net, so this is its
  // only highlight join key (WS7-042b).
  busId?: string;
}

// subjectsToSpecs builds a single HighlightSpec that lights up every subject at once, bucketed by
// kind (nets / components / pins) and deduped. A net subject with a netId is bucketed by id (so two
// same-named nets are distinct targets); a net without one falls back to its name. It returns []
// when there is nothing to highlight (which clears the highlight). This is the multi-subject
// highlight: the same spec drives both renderers through the presenter's setHighlights.
export function subjectsToSpecs(findings: HighlightSubject[]): HighlightSpec[] {
  const nets = new Set<string>();
  const netIds = new Set<string>();
  const busIds = new Set<string>();
  const components = new Set<string>();
  const pins: { refDes: string; pin: string }[] = [];
  const seenPin = new Set<string>();
  for (const f of findings) {
    if (f.kind === "net") {
      if (f.netId) netIds.add(f.netId);
      else if (f.subject) nets.add(f.subject);
    } else if (f.kind === "bus") {
      // A bus joins ONLY by its source id, never a net/name (WS7-042b); a bus with no id (an
      // undrawable bus_alias / EDIF array) contributes nothing and its "not drawn" note is WS7-042c.
      if (f.busId) busIds.add(f.busId);
    } else if (!f.subject) {
      continue;
    } else if (f.kind === "pin") {
      const key = `${f.subject} ${f.pin}`;
      if (!seenPin.has(key)) {
        seenPin.add(key);
        pins.push({ refDes: f.subject, pin: f.pin });
      }
    } else {
      // "component" and any unknown kind resolve as a component ref_des.
      components.add(f.subject);
    }
  }
  if (nets.size === 0 && netIds.size === 0 && busIds.size === 0 && components.size === 0 && pins.length === 0) return [];
  const spec: HighlightSpec = {};
  if (nets.size > 0) spec.nets = [...nets];
  if (netIds.size > 0) spec.netIds = [...netIds];
  if (busIds.size > 0) spec.busIds = [...busIds];
  if (components.size > 0) spec.components = [...components];
  if (pins.length > 0) spec.pins = pins;
  return [spec];
}

// findingSpec is the single-finding focus highlight — the same bucketing as subjectsToSpecs over
// one finding, so a focused net/component/pin lights up exactly.
export function findingSpec(f: FindingItem): HighlightSpec[] {
  return subjectsToSpecs([f]);
}

// entitySpecs is the focus highlight for a bare (kind, subject) that is not a finding — a query
// result cell (WS9-038). Same bucketing as findingSpec, so a located component/net paints exactly
// as the equivalent finding would.
export function entitySpecs(kind: string, subject: string, pin = ""): HighlightSpec[] {
  return subjectsToSpecs([{ kind, subject, pin }]);
}

// focusStack builds the two-layer highlight stack for a focused subject (WS9-040): the base
// findings layer with the focused NET removed, then the focus layer on top. A net's focus is a
// translucent PATH highlighter (withFocusShape), so leaving the net in the opaque base underlay
// would show through and defeat the translucency — the base drops it and the highlighter paints
// the bare wire. A focused component or pin stays in the base: its base outline plus the focus
// bounding box read as additive area emphasis, so nothing is removed for those kinds. An empty
// focus (subject not found) leaves the base untouched.
export function focusStack(findings: HighlightSubject[], kind: string, subject: string, focus: HighlightSpec[], netId = ""): HighlightSpec[] {
  // Drop the focused net from the base so the opaque underlay does not bleed through its
  // translucent PATH marker. Drop by netId when the focus carries one (only that instance leaves
  // the base; same-named siblings keep their outline), else by name.
  const isFocused = (f: HighlightSubject) =>
    f.kind === "net" && (netId !== "" ? f.netId === netId : f.subject === subject);
  const base = kind === "net" ? findings.filter((f) => !isFocused(f)) : findings;
  return [...subjectsToSpecs(base), ...focus];
}
