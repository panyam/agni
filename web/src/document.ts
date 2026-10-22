// The document store holds what is true of the open design and the configuration it is checked
// under, whatever the reader is doing (agni issue 390): which design, its sheets and the layouts it
// offers, the rule catalog and the active ruleset, the held check run, the expectation sidecar and the
// naming convention. ViewerPresenter holds the interaction half (selection, focus, highlights, camera,
// busy state, panel tabs) and reads this one; nothing here reads interaction state, and
// document.test.ts holds that direction, since a cycle between the two halves is what would stop the
// document half from running on the other side of a WebAssembly boundary later.
import { create } from "@bufbuild/protobuf";
import type { Client } from "@connectrpc/connect";
import { artifactUri } from "@agni/web-shared/uri.js";
import type { SheetRef } from "./gen/agni/v1/webapi/design_pb.js";
import { CheckDesignResponseSchema, CheckService, type CheckDesignResponse, type OverlayConfig } from "./gen/agni/v1/webapi/checks_pb.js";
import type { NamingConvention } from "./gen/agni/v1/config/naming_pb.js";
import { type RuleItem, defaultSelection } from "./rules.js";
import { type FindingItem, type SheetBadge, type VerdictItem, contextFromWire, outcomeWord } from "./findings.js";
import type { RuleExpectationItem } from "./expectations.js";
import { feedbackIssueURL, findingFeedback } from "./feedback.js";

type CheckClient = Client<typeof CheckService>;

export class DocumentStore {
  // mount and path name the open design, "" when none is open.
  mount = "";
  path = "";
  // sheets are the design's sheets in the server's order.
  sheets: SheetRef[] = [];
  // summary is the one-line description of the design the top bar shows.
  summary = "";
  // availableLayouts are the geometry layouts this design can be drawn in, and nativeAvailable
  // whether the native renderer can draw it.
  availableLayouts: string[] = [];
  nativeAvailable = false;
  // designContentHash is the revision this server actually read, off GetDesign, which the stale-link
  // note compares with the revision a link claimed.
  designContentHash = "";

  // catalogLoading is true while the open design's rule catalog has not arrived. A new design sets it
  // before its first sheet draws, since that sheet can draw before ListRules answers, and until then
  // selectedRules is empty or still the previous design's (agni 868). It starts true, because before
  // the first design opens there is no catalog either, and a push in that window must not offer Run.
  catalogLoading = true;
  // rules is the catalog for the open design (ListRules), and selectedRules the active ruleset the
  // user has checked, the subset CheckDesign runs. It defaults to every available rule so a design
  // opens showing all runnable checks. Group-by/filter/search are the panel's own view state.
  rules: RuleItem[] = [];
  selectedRules: string[] = [];
  // rulesByName indexes the catalog for the finding->category lookup (findings carry only the rule
  // name on the wire).
  rulesByName = new Map<string, RuleItem>();
  // run is what the reader has run on this design: the rules asked for and the engine's answers, as
  // sent (agni issue 390). It is the panel's one copy of a check, so the findings, the considered set,
  // the skipped rules, a saved report and the Tables panel all read it, and a new design or a
  // convention change drops it in one place, where four per-rule caches each had to be cleared and
  // one was missed. Run asks only for the selected rules it lacks and folds the answer in, so a
  // repeated Run costs nothing; asking for the whole selection again cost Jetson 0.3 to 0.8 s in the
  // browser for an answer already on screen. null until a run.
  run: { rules: Set<string>; resp: CheckDesignResponse } | null = null;
  // runError says why the last run failed and which rules it asked for, which it left pending
  // (agni issue 1117), null when the last run did not fail. Without it a failed run reads as a Run
  // never pressed, and a stale or lost snapshot, a worker that died, a budget the server enforced and
  // a geometry-only file with no netlist all look the same.
  runError: { message: string; rules: string[] } | null = null;
  // catalogError says why ListRules failed, "" when it answered, so an empty catalog is not read as
  // a design with no rules (agni issue 1117).
  catalogError = "";

  // expectations are the design's expectation sidecar entries, expectationFindings the findings the
  // full run produced for reconciling against them, and hasSidecar whether the design has one.
  expectations: RuleExpectationItem[] = [];
  expectationFindings: FindingItem[] = [];
  hasSidecar = false;

  // convention is the resolved naming convention sent on each rule-running request, null while the
  // server's default applies. conventionRef is the ref it was resolved from, so the UI can name it.
  convention: NamingConvention | null = null;
  conventionRef = "";
  // engineVersion is the build answering, off ListRules, which a finding's report link names.
  engineVersion = "";

  // scoped holds verdicts answered for some subjects alone (agni issue 897): a selection's checks in
  // Properties, or a link that named its verdict's subjects. They are kept APART from `run`, which
  // holds whole rules, because a rule answered for one net is not that rule's answer for the design,
  // and folding it in would show the checks panel a subset as the whole. Dropped with the run.
  scoped: CheckDesignResponse["verdicts"] = [];

  // keepScoped adds a scoped answer's verdicts, each id once, the newest answer winning.
  keepScoped(vs: CheckDesignResponse["verdicts"]): void {
    const ids = new Set(vs.map((v) => v.id));
    this.scoped = [...this.scoped.filter((v) => !ids.has(v.id)), ...vs];
  }

  // dropRun forgets every answer about the design: the whole run and the scoped ones.
  dropRun(): void {
    this.run = null;
    this.runError = null;
    this.scoped = [];
  }

  // findVerdict looks one up across every rule's cached considered set, then the scoped answers.
  // Linear over a few thousand rows at worst (the sum of the considered sets, not rules times
  // subjects), too few for an index to matter.
  findVerdict(id: string): VerdictItem | undefined {
    const v = this.run?.resp.verdicts.find((x) => x.id === id) ?? this.scoped.find((x) => x.id === id);
    return v ? verdictItem(v) : undefined;
  }

  // ran reports whether the last run answered a rule, which is what separates "ran clean" from
  // "press Run".
  ran(rule: string): boolean {
    return this.run?.rules.has(rule) ?? false;
  }

  // sheetBadges denormalizes a finding's wire sheet ids into display badges (WS9-024), the id for
  // navigation and the SheetRef's name for the label (falling back to the id). A single-sheet
  // design gets no badges, since there is nowhere to navigate, so panels render badges iff present.
  sheetBadges(ids: string[]): SheetBadge[] {
    if (this.sheets.length <= 1) return [];
    return ids.map((id) => ({ id, name: this.sheets.find((s) => s.id === id)?.name || id }));
  }

  // findingItem is one wire finding as the panel shows it, with the catalog's category and profile
  // and the sheet badges of the design on screen.
  findingItem(f: CheckDesignResponse["findings"][number]): FindingItem {
    return {
      rule: f.rule,
      category: this.rulesByName.get(f.rule)?.tags.category ?? "",
      profile: this.rulesByName.get(f.rule)?.tags.profile ?? "",
      severity: f.severity,
      kind: f.subject?.kind ?? "",
      subject: f.subject?.ref ?? "",
      pin: f.subject?.pin ?? "",
      netId: f.subject?.netId ?? "",
      busId: f.subject?.busId ?? "",
      message: f.message,
      inconclusive: f.inconclusive ?? false,
      context: contextFromWire(f.context),
      sheets: this.sheetBadges(f.sheets ?? []),
      locateReason: f.locateReason ?? 0,
      reportUrl: feedbackIssueURL(findingFeedback({ rule: f.rule, severity: f.severity, inconclusive: f.inconclusive ?? false }, this.rulesByName.get(f.rule)?.shipped ?? false, this.engineVersion)),
    };
  }

  // heldRun is the selection's part of the last run as one CheckDesignResponse, rule by rule in the
  // catalog's order, or undefined until every selected rule has run. A saved report, a saved csv and the Tables panel
  // all start from it, so none of them runs a rule.
  heldRun(): { names: string[]; run: CheckDesignResponse } | undefined {
    const selected = new Set(this.selectedRules);
    // Catalog order, which is the order one run over the selection returns its findings in.
    const names = this.rules.map((r) => r.name).filter((n) => selected.has(n));
    const held = this.run;
    if (!held || names.length === 0 || names.some((n) => !held.rules.has(n))) return undefined;
    const r = held.resp;
    const run = create(CheckDesignResponseSchema, {
      findings: names.flatMap((n) => r.findings.filter((f) => f.rule === n)),
      verdicts: names.flatMap((n) => r.verdicts.filter((v) => v.rule === n)),
      skipped: r.skipped.filter((sk) => selected.has(sk.name)),
      // What the read left out is a property of the design rather than of a rule, so it travels whole
      // and a saved report states it (agni issue 717).
      unexpandedHierarchy: r.unexpandedHierarchy ?? [],
    });
    return { names, run };
  }

  // verdictList is the considered set of the selected rules the last run answered, rule by rule in
  // the selection's order.
  verdictList(): VerdictItem[] {
    const vs = this.run?.resp.verdicts ?? [];
    return this.selectedRules.flatMap((name) => vs.filter((v) => v.rule === name).map(verdictItem));
  }

  // runMissing asks CheckDesign for the selected rules the run lacks and folds the answer into it.
  // snapshot is the open design's snapshot under the page's convention, asked for only when a rule
  // is missing (agni issue 1061). A failure leaves the run as it was and those rules pending, and
  // keeps the reason on runError (agni issue 1117). A geometry-only file fails this way, since the
  // engine reads a file with no netlist as an invalid argument and says so in its message.
  //
  // signal stops the request (agni issue 918). A stopped run folds in nothing, so the rules it was
  // asked for stay pending, and it is no failure, since the reader stopped it.
  async runMissing(checks: Pick<CheckClient, "checkDesign">, snapshot: () => Promise<string>, signal?: AbortSignal): Promise<void> {
    this.runError = null;
    const missing = this.selectedRules.filter((n) => !this.ran(n));
    if (missing.length === 0 || !this.mount || !this.path) return;
    try {
      const resp = await checks.checkDesign({ snapshot: await snapshot(), rules: missing }, { signal });
      // ?? [] because a hand-built response (a test stub, an older server) may omit a list, and a
      // missing field must degrade to "nothing was skipped" or "this rule stated no considered set"
      // rather than throwing mid-run and losing the findings too.
      resp.skipped ??= [];
      resp.verdicts ??= [];
      if (!this.run) {
        this.run = { rules: new Set(missing), resp };
      } else {
        for (const n of missing) this.run.rules.add(n);
        this.run.resp.findings.push(...resp.findings);
        this.run.resp.verdicts.push(...resp.verdicts);
        this.run.resp.skipped.push(...resp.skipped);
      }
    } catch (e) {
      if (!signal?.aborted) this.runError = { message: messageOf(e), rules: missing };
    }
  }

  // loadCatalog fetches the rule catalog a check of snapshot would run (ListRules) and defaults the
  // active ruleset to every available rule. A failure leaves an empty catalog and the reason on
  // catalogError, which both panels show (agni issue 1117), rather than erroring the open.
  async loadCatalog(checks: Pick<CheckClient, "listRules">, snapshot: () => Promise<string>): Promise<void> {
    try {
      const resp = await checks.listRules({ snapshot: await snapshot() });
      this.rules = resp.rules.map((r) => ({
        name: r.name,
        severity: r.severity,
        summary: r.summary,
        impact: r.impact,
        remedy: r.remedy,
        detail: r.detail,
        reads: r.reads,
        tags: r.tags,
        available: r.available,
        unavailableReason: r.unavailableReason,
        shipped: r.shipped,
      }));
      this.engineVersion = resp.engineVersion ?? "";
      this.catalogError = "";
    } catch (e) {
      this.rules = [];
      this.catalogError = messageOf(e);
    }
    this.rulesByName = new Map(this.rules.map((r) => [r.name, r]));
    this.selectedRules = defaultSelection(this.rules);
    this.catalogLoading = false;
  }
}

// messageOf pulls a human-readable message off a thrown value for inline display.
export function messageOf(e: unknown): string {
  return e instanceof Error ? e.message : String(e);
}

// verdictItem is one wire verdict as the panel shows it: its outcome as a word, its subjects, and the
// statement and terms its witness rests on.
export function verdictItem(v: CheckDesignResponse["verdicts"][number]): VerdictItem {
  return {
    id: v.id,
    rule: v.rule,
    outcome: outcomeWord(v.outcome),
    subjects: (v.subjects ?? []).map((sub) => ({
      kind: sub.kind ?? "",
      subject: sub.ref ?? "",
      pin: sub.pin ?? "",
      netId: sub.netId ?? "",
      busId: sub.busId ?? "",
    })),
    statement: v.witness?.statement ?? "",
    terms: (v.witness?.terms ?? []).map((t) => ({ label: t.label, value: t.value })),
    context: contextFromWire(v.context),
    reason: v.reason ?? "",
  };
}

