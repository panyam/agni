// The rules panel's command-down surface, mirroring findings.ts. The presenter owns the rule
// catalog and the active selection (which rules run); it pushes RulesState and the panel renders
// it, emitting an onSelectionChange(names) intent back up. Group-by, filtering, and search are
// ephemeral view state the panel holds locally; only the selection (which drives CheckDesign)
// lives in the presenter, so it survives view-knob changes and is keyed by rule name.

// RuleItem is the view-side shape of a catalog rule (the wire webapi.RuleInfo, minus proto
// machinery). tags is the open classification the panel groups and filters by; available gates
// whether the rule can run (an unavailable rule is shown greyed and cannot be selected).
// summary/impact/remedy/detail are the rule's prose (WS9-020): the one-liner shown per row, what
// goes wrong on violation, what to do about it, and the long-form detail markdown behind the row's
// expand affordance.
export interface RuleItem {
  name: string;
  severity: string; // "error" | "warning" | "info"
  summary: string;
  impact: string;
  remedy: string;
  detail: string; // markdown
  reads: string[];
  tags: Record<string, string>;
  available: boolean;
  unavailableReason: string;
}

export interface RulesState {
  rules: RuleItem[];
  // selected rule names — the active ruleset that drives which checks run.
  selected: string[];
  // fired[name] is how many findings that rule produced this run (0 when it ran clean; absent when
  // it has not run). The panel shows it in the per-group fired/selected/available badge.
  fired: Record<string, number>;
}

export interface RulesView {
  setState: (s: RulesState) => void;
}

// RuleFilter narrows the visible catalog. tagValues maps a tag key to the values allowed for it;
// keys intersect (a rule must match every constrained key) and values within a key union. search
// is a case-insensitive substring matched against name and summary.
export interface RuleFilter {
  tagValues: Record<string, string[]>;
  availableOnly: boolean;
  search: string;
}

// TriState is a group checkbox's state given the current selection: every selectable rule
// selected, some, or none.
export type TriState = "all" | "some" | "none";

// tagKeys returns the tag keys present across the catalog, sorted, as the group-by options. A new
// provider tag key appears here automatically (the group-by is data-driven, not a fixed list).
export function tagKeys(rules: RuleItem[]): string[] {
  const keys = new Set<string>();
  for (const r of rules) for (const k of Object.keys(r.tags)) keys.add(k);
  return [...keys].sort();
}

// groupBy buckets rules by the value of a tag key, preserving first-appearance order of values
// (matching the server's TreeBy). A rule missing the key falls under "" so no rule is dropped.
export function groupBy(rules: RuleItem[], key: string): [string, RuleItem[]][] {
  const order: string[] = [];
  const byValue = new Map<string, RuleItem[]>();
  for (const r of rules) {
    const v = r.tags[key] ?? "";
    if (!byValue.has(v)) {
      byValue.set(v, []);
      order.push(v);
    }
    byValue.get(v)!.push(r);
  }
  return order.map((v) => [v, byValue.get(v)!]);
}

// filterRules applies a RuleFilter. An empty filter returns every rule; each constrained axis
// narrows, and the axes intersect.
export function filterRules(rules: RuleItem[], f: RuleFilter): RuleItem[] {
  const needle = f.search.trim().toLowerCase();
  return rules.filter((r) => {
    if (f.availableOnly && !r.available) return false;
    if (needle && !r.name.toLowerCase().includes(needle) && !r.summary.toLowerCase().includes(needle)) return false;
    for (const [key, values] of Object.entries(f.tagValues)) {
      if (values.length > 0 && !values.includes(r.tags[key] ?? "")) return false;
    }
    return true;
  });
}

// groupSelectState reports a group's tri-state over the selectable (available) rules only, since
// unavailable rules cannot be selected. A group with no selectable rules reads as "none".
export function groupSelectState(group: RuleItem[], selected: Set<string>): TriState {
  const selectable = group.filter((r) => r.available);
  if (selectable.length === 0) return "none";
  const on = selectable.filter((r) => selected.has(r.name)).length;
  if (on === 0) return "none";
  if (on === selectable.length) return "all";
  return "some";
}

// defaultSelection is every available rule — the active ruleset a design opens with, so the first
// run matches the whole (runnable) catalog.
export function defaultSelection(rules: RuleItem[]): string[] {
  return rules.filter((r) => r.available).map((r) => r.name);
}

// toggleRule adds or removes one rule from the selection; an unavailable rule is a no-op (it
// cannot be selected). Returns a new array.
export function toggleRule(rule: RuleItem, selected: string[]): string[] {
  if (!rule.available) return selected;
  return selected.includes(rule.name) ? selected.filter((n) => n !== rule.name) : [...selected, rule.name];
}

// toggleGroup flips a whole group: if any selectable rule in the group is unselected it selects all
// selectable rules in the group, otherwise it deselects them all (the tri-state click target).
export function toggleGroup(group: RuleItem[], selected: string[]): string[] {
  const selectable = group.filter((r) => r.available).map((r) => r.name);
  const sel = new Set(selected);
  const allOn = selectable.length > 0 && selectable.every((n) => sel.has(n));
  if (allOn) {
    for (const n of selectable) sel.delete(n);
  } else {
    for (const n of selectable) sel.add(n);
  }
  return [...sel];
}
