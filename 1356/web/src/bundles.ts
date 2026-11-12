// A rule bundle is a named selection over the rule catalog. It is client-side only (no proto).
// Built-in bundles ship as data and user-saved ones live in localStorage. The panel resolves a
// bundle to rule names and fires the same onSelectionChange a checkbox would, so nothing upstream
// of the rules panel knows bundles exist.

import { type RuleItem, type RuleFilter, filterRules } from "./rules.js";

// Bundle is a named selection, either an explicit rule-name list (a user-saved snapshot) or a facet
// filter that picks up new matching rules as the catalog grows. Built-in bundles are not editable
// or deletable.
export interface Bundle {
  name: string;
  builtin?: boolean;
  rules?: string[]; // explicit rule names (user-saved snapshots)
  filter?: RuleFilter; // facet filter (the built-in defaults)
}

// BUILTIN_BUNDLES are the shipped defaults, written as filters so a new matching rule joins with no
// code change. They resolve against whatever ListRules returns, so the shareable build (no
// proprietary rules, WS3-005) yields only shareable selections.
export const BUILTIN_BUNDLES: Bundle[] = [
  { name: "Topology baseline", builtin: true, filter: { tagValues: {}, availableOnly: true, search: "" } },
  { name: "Open rules", builtin: true, filter: { tagValues: { distribution: ["open"] }, availableOnly: true, search: "" } },
];

// resolveBundle returns the rule names a bundle selects from the given catalog. Only *available*
// rules are selected, because an unavailable rule cannot run, so a saved bundle degrades gracefully
// on a design that lacks a rule's data layer.
export function resolveBundle(rules: RuleItem[], bundle: Bundle): string[] {
  if (bundle.filter) {
    return filterRules(rules, bundle.filter).filter((r) => r.available).map((r) => r.name);
  }
  const want = new Set(bundle.rules ?? []);
  return rules.filter((r) => want.has(r.name) && r.available).map((r) => r.name);
}

// upsertBundle adds or replaces a saved bundle by name, returning a new list (last write wins).
export function upsertBundle(saved: Bundle[], b: Bundle): Bundle[] {
  return [...saved.filter((x) => x.name !== b.name), b];
}

// removeBundle drops a saved bundle by name, returning a new list.
export function removeBundle(saved: Bundle[], name: string): Bundle[] {
  return saved.filter((x) => x.name !== name);
}

// allBundles is the built-ins followed by the user's saved bundles (the dropdown's contents).
export function allBundles(saved: Bundle[]): Bundle[] {
  return [...BUILTIN_BUNDLES, ...saved];
}

const STORAGE_KEY = "agni.ruleBundles";

// loadSaved reads the user's custom bundles from localStorage. A missing or corrupt store, or an
// environment with no localStorage (node tests, SSR), yields an empty list rather than throwing.
export function loadSaved(): Bundle[] {
  try {
    const raw = globalThis.localStorage?.getItem(STORAGE_KEY);
    if (!raw) return [];
    const parsed = JSON.parse(raw);
    return Array.isArray(parsed) ? (parsed as Bundle[]) : [];
  } catch {
    return [];
  }
}

// persistSaved writes the user's custom bundles to localStorage. With no localStorage or a full
// store it is a silent no-op, and the bundles do not survive a reload.
export function persistSaved(saved: Bundle[]): void {
  try {
    globalThis.localStorage?.setItem(STORAGE_KEY, JSON.stringify(saved));
  } catch {
    // storage unavailable or full
  }
}
