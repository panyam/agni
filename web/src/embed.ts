// embed.ts is a LIBRARY bundle: it mounts the REAL web-app panels as standalone
// islands into a static host page (the docs site), driven by a recorded backend
// instead of a live server.
//
// This is Phase 1 of the docs-embed plan (WS14-006): the actual ChecksPanel
// island runs, so the docs show the real component, not a screenshot. The only
// substitute is the DATA source — a JSON bundle captured from `agni serve` over
// a seeded design (ListRules + CheckDesign responses). Phase 2 swaps that
// recorded source for a wasm engine behind the same mount function, with no
// panel change.
//
// The page opts in with a tag:
//   <agni-checks src="/agni/static/designs/showcase.checks.json"></agni-checks>
// This module auto-hydrates every such tag on DOMContentLoaded.

import { findingsPanelIsland } from "./findingspanel.js";
import type { FindingItem, FindingsState } from "./findings.js";
import type { LocateReason } from "./gen/agni/v1/webapi/query_pb.js";

// The shape of the recorded bundle (a capture of the two CheckService responses
// in Connect JSON encoding). Only the fields the panel reads are typed.
interface RecordedRule {
  name: string;
  summary?: string;
  tags?: { category?: string; profile?: string };
}
interface RecordedSubject {
  kind?: string;
  ref?: string;
  pin?: string;
  netId?: string;
  busId?: string;
}
interface RecordedFinding {
  rule: string;
  severity: string;
  subject?: RecordedSubject;
  message: string;
  locateReason?: number;
}
interface RecordedBundle {
  design?: string;
  listRules: { rules?: RecordedRule[] };
  checkDesign: { findings?: RecordedFinding[] };
}

// toFindingItems mirrors ViewerPresenter.runSelection (viewer.ts): each wire
// Finding becomes a FindingItem, with category/profile denormalized from the
// rule catalog. Kept deliberately close to the presenter so the recorded path
// and the live path produce the same view state.
function toFindingItems(bundle: RecordedBundle): FindingItem[] {
  const rulesByName = new Map<string, RecordedRule>();
  for (const r of bundle.listRules.rules ?? []) rulesByName.set(r.name, r);
  return (bundle.checkDesign.findings ?? [])
    .map((f) => ({
      rule: f.rule,
      category: rulesByName.get(f.rule)?.tags?.category ?? "",
      profile: rulesByName.get(f.rule)?.tags?.profile ?? "",
      severity: f.severity,
      kind: f.subject?.kind ?? "",
      subject: f.subject?.ref ?? "",
      pin: f.subject?.pin ?? "",
      netId: f.subject?.netId ?? "",
      busId: f.subject?.busId ?? "",
      message: f.message,
      sheets: [],
      locateReason: (f.locateReason ?? 0) as LocateReason,
    }))
    .sort((a, b) => (a.rule !== b.rule ? (a.rule < b.rule ? -1 : 1) : a.subject < b.subject ? -1 : a.subject > b.subject ? 1 : 0));
}

function ruleSummaries(bundle: RecordedBundle): Record<string, string> {
  const out: Record<string, string> = {};
  for (const r of bundle.listRules.rules ?? []) if (r.summary) out[r.name] = r.summary;
  return out;
}

/**
 * mountChecksPanel mounts the real findings/checks island into `el`, driven by
 * a recorded CheckService bundle. Returns a disposer.
 *
 * The panel is on-demand exactly like the live app: it opens showing the rule
 * count with results pending, and the Run button reveals the recorded findings.
 */
export function mountChecksPanel(el: HTMLElement, bundle: RecordedBundle): () => void {
  const items = toFindingItems(bundle);
  const ruleCount = (bundle.listRules.rules ?? []).length;
  const summaries = ruleSummaries(bundle);

  // onSelect would highlight the subject in the schematic — no canvas in this
  // slice (the viewer embed is the next one), so it is a no-op here.
  const { island, view } = findingsPanelIsland(el, null, {
    onSelect: () => {},
    onRun: () => {
      view.setState({ findings: [], selected: "", ruleCount, pending: ruleCount, running: true, ruleSummaries: summaries });
      // A microtask later, reveal the recorded results (mirrors the live
      // round-trip: running flips off, pending clears).
      queueMicrotask(() =>
        view.setState({ findings: items, selected: "", ruleCount, pending: 0, running: false, ruleSummaries: summaries }),
      );
    },
  });
  island.activate();

  // Initial state: rules loaded, results not yet run (the live "press Run" state).
  const initial: FindingsState = { findings: [], selected: "", ruleCount, pending: ruleCount, running: false, ruleSummaries: summaries };
  view.setState(initial);

  return () => island.deactivate();
}

/** Hydrate every <agni-checks src="..."> on the page. */
async function hydrate(): Promise<void> {
  const tags = [...document.querySelectorAll<HTMLElement>("agni-checks")];
  await Promise.all(
    tags.map(async (el) => {
      if (el.getAttribute("data-agni-mounted") === "true") return;
      el.setAttribute("data-agni-mounted", "true");
      const src = el.getAttribute("src");
      if (!src) {
        el.textContent = "agni-checks: missing src";
        return;
      }
      try {
        const bundle = (await (await fetch(src)).json()) as RecordedBundle;
        mountChecksPanel(el, bundle);
      } catch (e) {
        el.textContent = `agni-checks: could not load ${src} (${e instanceof Error ? e.message : String(e)})`;
      }
    }),
  );
}

if (document.readyState === "loading") {
  window.addEventListener("DOMContentLoaded", () => void hydrate());
} else {
  void hydrate();
}

export {};
