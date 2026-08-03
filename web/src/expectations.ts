// The conformance-fixture expectations surface (WS9-045: was a dock panel, now an overlay + caption).
// The presenter fetches a design's expectation sidecar (GetExpectations, WS6-006) and its CheckDesign
// findings and reconciles them here; the anchored rows become status-colored highlight specs
// (expectationSpecs) on the schematic and the non-anchored verdict becomes the caption strip
// (expectationCaption). Reconcile is pure so the status matrix is unit-tested without the DOM.
//
// This is the visual half of rule-TDD: an expected rule that fires exactly is `matched` (green), one
// that is expected but silent is `missing` (red — not yet implemented, or a regression), a `pending`
// expectation is staged (author has not turned it on), and a finding no expectation covers is
// `unexpected` (amber). Reconcile is pure so the status matrix is unit-tested without the DOM.

import type { FindingItem, SheetBadge } from "./findings.js";
import { subjectsToSpecs } from "./findings.js";
import type { HighlightSpec } from "./highlights.js";

// RuleExpectationItem is the view-side shape of a wire RuleExpectation: a rule expected to fire, the
// subjects it should fire on, whether it is staged (pending) rather than asserted, and the sidecar's
// optional why narration (WS6-008: what the fixture's defect and control cases are).
export interface RuleExpectationItem {
  rule: string;
  subjects: string[];
  pending: boolean;
  why?: string;
}

export type ExpectationStatus = "matched" | "partial" | "missing" | "pending" | "unexpected";

// ExpectationRow is one reconciled rule: its status, the rule's catalog one-liner (so a row reads
// as an assertion, not a bare name — WS9-020), the subjects expected, the subjects that actually
// fired, and the sidecar's why note. For an `unexpected` row `expected` is empty and `why` is ""
// (no sidecar entry exists to narrate it); for a `missing` row `actual` is empty.
export interface ExpectationRow {
  rule: string;
  status: ExpectationStatus;
  summary: string;
  expected: string[];
  actual: string[];
  why: string;
  // Sheet badges per fired subject (WS9-024), joined from the findings: an actual chip shows
  // where its subject lives. Expected-only subjects have no finding, hence no entry.
  subjectSheets: Record<string, SheetBadge[]>;
}

// reconcile joins expectations against the actual findings into per-rule rows. Expectations keep
// their given order (the server sorts fires then pending); unexpected rules follow, sorted. A
// pending expectation stays `pending` even if its rule now fires — that is the author's cue to turn
// it into a hard expectation — but its `actual` is filled so the firing is visible. summaryFor
// resolves a rule name to its catalog one-liner (the caller holds the ListRules catalog); it
// defaults to none so pure-logic callers need not wire it.
export function reconcile(
  expectations: RuleExpectationItem[],
  findings: FindingItem[],
  summaryFor: (rule: string) => string = () => "",
): ExpectationRow[] {
  const actualByRule = new Map<string, string[]>();
  for (const f of findings) {
    const list = actualByRule.get(f.rule) ?? [];
    list.push(f.subject);
    actualByRule.set(f.rule, list);
  }
  for (const [r, subs] of actualByRule) actualByRule.set(r, [...subs].sort());

  // Per-rule subject -> sheet badges, so an actual chip can show where its subject lives.
  const sheetsByRule = new Map<string, Record<string, SheetBadge[]>>();
  for (const f of findings) {
    if (f.sheets.length === 0) continue;
    const m = sheetsByRule.get(f.rule) ?? {};
    m[f.subject] = f.sheets;
    sheetsByRule.set(f.rule, m);
  }

  const rows: ExpectationRow[] = [];
  const covered = new Set<string>();
  for (const e of expectations) {
    covered.add(e.rule);
    const actual = actualByRule.get(e.rule) ?? [];
    const expected = [...e.subjects].sort();
    let status: ExpectationStatus;
    if (e.pending) status = "pending";
    else if (actual.length === 0) status = "missing";
    else status = sameSubjects(actual, expected) ? "matched" : "partial";
    rows.push({ rule: e.rule, status, summary: summaryFor(e.rule), expected, actual, why: e.why ?? "", subjectSheets: sheetsByRule.get(e.rule) ?? {} });
  }
  for (const rule of [...actualByRule.keys()].sort()) {
    if (covered.has(rule)) continue;
    rows.push({ rule, status: "unexpected", summary: summaryFor(rule), expected: [], actual: actualByRule.get(rule)!, why: "", subjectSheets: sheetsByRule.get(rule) ?? {} });
  }
  return rows;
}

function sameSubjects(a: string[], b: string[]): boolean {
  if (a.length !== b.length) return false;
  for (let i = 0; i < a.length; i++) if (a[i] !== b[i]) return false;
  return true;
}

// STATUS_COLOR maps a reconcile status to its overlay color (WS9-045). Only statuses with a subject
// that actually FIRED reach the overlay (matched/partial/unexpected) — those have a finding, hence a
// kind to highlight. missing/pending have no firing to anchor and live in the caption instead.
const STATUS_COLOR: Record<ExpectationStatus, string> = {
  matched: "#22c55e", // green: expected and fired exactly
  partial: "#f97316", // orange: fired, but not the exact expected subject set
  unexpected: "#f59e0b", // amber: fired with no expectation covering it
  missing: "#ef4444", // red (caption only)
  pending: "#9ca3af", // gray (caption only)
};

// expectationSpecs turns the ANCHORED half of the reconcile into status-colored highlight specs
// (WS9-045): one spec per status color, listing the subjects that actually fired for rows of that
// status, so the fixture's assertions read AS annotations on the drawing. Kind (net/component/pin) is
// taken from the findings — an expected-but-silent (missing) subject has no finding and no kind, so it
// is a caption row, not an overlay. Reuses subjectsToSpecs (the same bucketing findings highlighting
// uses) and only adds the color, so it rides the existing highlight rails with no new server logic.
export function expectationSpecs(rows: ExpectationRow[], findings: FindingItem[]): HighlightSpec[] {
  const statusOf = new Map<string, ExpectationStatus>();
  for (const r of rows) for (const s of r.actual) statusOf.set(`${r.rule} ${s}`, r.status);
  const byColor = new Map<string, FindingItem[]>();
  for (const f of findings) {
    const st = statusOf.get(`${f.rule} ${f.subject}`);
    if (!st) continue;
    const list = byColor.get(STATUS_COLOR[st]) ?? [];
    list.push(f);
    byColor.set(STATUS_COLOR[st], list);
  }
  const specs: HighlightSpec[] = [];
  for (const [color, fs] of byColor) {
    for (const s of subjectsToSpecs(fs)) specs.push({ ...s, color });
  }
  return specs;
}

// ExpectationCaption is the NON-anchored residue of the reconcile (WS9-045): the set-equality verdict
// with counts, plus the expected-but-silent rules (missing) that have no drawing anchor. It is the
// caption strip's whole content; a design with no sidecar produces no caption (the strip hides).
export interface ExpectationCaption {
  pass: boolean; // actual == expected exactly: nothing missing, partial, or unexpected
  expected: number; // count of asserted (non-pending) expectations
  matched: number; // of those, how many fired exactly
  unexpected: number; // findings no expectation covers
  missing: string[]; // rules expected to fire but silent (rendered red)
  silent: boolean; // the fires:{} case — no expectations at all (a "nothing may fire" assertion)
}

// expectationCaption folds the reconciled rows into the caption verdict. silent is the passes-variant
// fires:{} assertion (no expectations); pass then means nothing fired at all.
export function expectationCaption(rows: ExpectationRow[]): ExpectationCaption {
  let expected = 0,
    matched = 0,
    unexpected = 0,
    partial = 0;
  const missing: string[] = [];
  for (const r of rows) {
    if (r.status === "unexpected") unexpected++;
    else if (r.status === "pending") continue; // staged, not asserted — excluded from the verdict
    else {
      expected++;
      if (r.status === "matched") matched++;
      else if (r.status === "missing") missing.push(r.rule);
      else if (r.status === "partial") partial++;
    }
  }
  return {
    pass: missing.length === 0 && unexpected === 0 && partial === 0,
    expected,
    matched,
    unexpected,
    missing,
    silent: expected === 0,
  };
}
