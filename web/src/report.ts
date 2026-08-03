// The report panel's command-down surface, mirroring findings.ts. The presenter fetches the
// server-computed CheckReport pivot (GetCheckReport) and pushes CheckReportState; the panel
// renders it as-is. The pivot is canonical and server-side (WS3-022): sections arrive
// worst-severity first with counts, findings grouped by rule with catalog summaries, so this
// module never re-derives a group-by the CLI's report formats would then drift from.

import type { FindingItem, SheetBadge } from "./findings.js";

// ReportRuleGroup is one rule's findings inside a severity section. summary is the rule's
// catalog one-liner ("" when the rule is unknown to the catalog).
export interface ReportRuleGroup {
  rule: string;
  summary: string;
  findings: FindingItem[];
}

// ReportSection is one severity tier of the report, worst first in CheckReportData.sections;
// count is the section's finding total (server-computed, shown as the badge).
export interface ReportSection {
  severity: string; // "error" | "warning" | "info"
  count: number;
  rules: ReportRuleGroup[];
}

// CheckReportData is the view-side CheckReport: what the wire pivot says, minus proto
// machinery. rulesRun distinguishes "clean" (rules ran, nothing fired) from "skipped".
export interface CheckReportData {
  source: string;
  rulesRun: number;
  sections: ReportSection[];
}

export interface CheckReportState {
  // null when no design is open, the fetch failed (e.g. a geometry-only file), or no rules
  // are selected; the panel falls back to its empty messages.
  report: CheckReportData | null;
  // subject of the focused finding, "" when none — kept in step with the findings panel so a
  // chip and its row highlight together.
  selected: string;
  // number of rules currently selected, so the panel tells "no rules selected" from "clean".
  ruleCount: number;
}

export interface CheckReportView {
  setState: (s: CheckReportState) => void;
}

// The wire shapes reportFromWire consumes, structurally (the generated CheckReport message
// satisfies them); keeping them structural keeps this module proto-free like findings.ts.
interface WireFinding {
  rule: string;
  severity: string;
  message: string;
  subject?: { kind: string; ref: string; pin: string; netId?: string; busId?: string };
  sheets?: string[];
  locateReason?: number;
}
interface WireRuleGroup {
  rule: string;
  summary: string;
  findings: WireFinding[];
}
interface WireSection {
  severity: string;
  count: number;
  rules: WireRuleGroup[];
}
export interface WireCheckReport {
  source: string;
  rulesRun: number;
  sections: WireSection[];
}

// reportFromWire converts the CheckReport message into the view shape, preserving the
// server's section and group order. Findings become the same FindingItem the checks panel
// uses (so chips join the highlight layer identically); lookupCategory denormalizes the
// rule's category tag, which findings do not carry on the wire, and sheetBadges denormalizes
// the wire's sheet ids into display badges (the presenter holds the SheetRefs — WS9-024). It
// defaults to none so pure-logic callers need not wire it.
export function reportFromWire(
  r: WireCheckReport,
  lookupCategory: (rule: string) => string,
  sheetBadges: (ids: string[]) => SheetBadge[] = () => [],
): CheckReportData {
  return {
    source: r.source,
    rulesRun: r.rulesRun,
    sections: r.sections.map((s) => ({
      severity: s.severity,
      count: s.count,
      rules: s.rules.map((g) => ({
        rule: g.rule,
        summary: g.summary,
        findings: g.findings.map((f) => ({
          rule: f.rule,
          category: lookupCategory(f.rule),
          profile: "", // the report panel groups by rule, not interface; profile is unused here
          severity: f.severity,
          kind: f.subject?.kind ?? "",
          subject: f.subject?.ref ?? "",
          pin: f.subject?.pin ?? "",
          netId: f.subject?.netId ?? "",
          busId: f.subject?.busId ?? "",
          message: f.message,
          sheets: sheetBadges(f.sheets ?? []),
          locateReason: f.locateReason ?? 0,
        })),
      })),
    })),
  };
}
