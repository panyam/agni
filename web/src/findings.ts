// The findings panel's command-down surface, mirroring controls.ts. The presenter owns the
// findings list and which one is focused, pushes FindingsState, and the panel renders it and emits
// an onSelect(subject) intent back up. Group-by is the panel's own view state.

import { BASE_HIGHLIGHT_ALPHA, BASE_HIGHLIGHT_COLOR, type HighlightSpec } from "./highlights.js";
import { type Selection, sameSelection } from "./selection.js";
import { LocateReason } from "./gen/agni/v1/checks/checks_pb.js";

// SheetBadge locates a finding's subject on one sheet (WS9-024). showSheet takes the id and the
// badge displays the name. The presenter fills badges only on a multi-sheet design, so a
// single-sheet design pushes none and a panel needs no "is this design multi-sheet" rule of its
// own.
export interface SheetBadge {
  id: string;
  name: string;
}

// FindingContext is one entity a finding's message names but is not about, with the part it plays.
// It satisfies HighlightSubject structurally, so it highlights through the same bucketing as a
// finding's subject.
//
// role is the rule author's word for that part ("terminal", "rail", "source"). The vocabulary is
// open, and a role is NOT unique within a finding, since "A and B both strap to address N" has two
// entities playing the same part.
export interface FindingContext {
  kind: string; // "net" | "component" | "pin" | "bus"
  subject: string;
  pin: string;
  netId?: string;
  busId?: string;
  role: string;
}

// WireContext is the shape checks.Finding.context arrives in. It is declared structurally so the
// checks panel, the report and the review share one mapper without this module importing the wire
// package.
export interface WireContext {
  subject?: { kind?: string; ref?: string; pin?: string; netId?: string; busId?: string };
  role?: string;
}

// contextFromWire maps a finding's context entities to the client shape and PRESERVES ORDER,
// because the order is the rule author's and matches the message (agni issue 349). A hand-built
// response or an older server can omit the field. A missing field maps to [], meaning the message
// names only its subject.
export function contextFromWire(cs: WireContext[] | undefined): FindingContext[] {
  return (cs ?? []).map((c) => ({
    kind: c.subject?.kind ?? "",
    subject: c.subject?.ref ?? "",
    pin: c.subject?.pin ?? "",
    netId: c.subject?.netId ?? "",
    busId: c.subject?.busId ?? "",
    role: c.role ?? "",
  }));
}

// FindingItem is the view-side shape of a rule finding (the wire checks.Finding without the proto
// machinery). subject is the entity ref (a net name or a ref_des) and the highlight join key, and kind
// says what it is, so grouping and highlighting never guess from the string. pin is set only for a pin
// subject, category is the rule's category tag (denormalized for the by-category group-by), and sheets
// holds one badge per sheet the subject touches (empty on a single-sheet design or when the server had
// no geometry).
export interface FindingItem {
  rule: string;
  category: string;
  // profile is the rule's "profile" tag (WS9-041), the interface an interface-profile rule checks
  // (e.g. "SPI_NOR"), or "" for a rule outside any profile. Denormalized from the rule catalog for the
  // by-interface group-by, like category.
  profile: string;
  severity: string; // "error" | "warning" | "info"
  kind: string; // "net" | "component" | "pin" | "bus"
  subject: string;
  pin: string;
  // netId is the per-instance net identity (webapi Subject.net_id) for a net subject. Two findings on
  // same-named nets carry distinct netIds, so they collapse as separate instances and each locates to
  // ITS wires (WS9). Empty for a component/pin subject or a pinless net.
  netId: string;
  // busId is the source id (webapi Subject.bus_id, a KiCad uuid) for a kind="bus" subject, so a
  // bus-not-modeled finding highlights its own drawn bus and two identically-labeled buses are
  // distinct instances (WS7-042b). Empty for every other subject kind.
  busId: string;
  message: string;
  // inconclusive marks a RESULT the rule could not decide rather than a defect it found (agni issue
  // 74). The rule ran with what it needed, examined this subject and could not conclude, so a
  // consumer must never count it as a failure. It is per-subject and it is not a skip. A skip is a
  // design-wide precondition, decided around the rule. The message says what could not be resolved
  // and what would resolve it. See
  // docsite/content/architecture/web-picking.md#inconclusive-results.
  inconclusive: boolean;
  // context are the entities this finding's message NAMES but is not ABOUT (agni issue 349), each
  // with the part it plays. Empty when the message names only its subject, which is most of them. The
  // panel renders them as clickable chips. They are NOT counted as findings about themselves, because
  // grouping by subject has to partition the findings (agni issue 259).
  //
  // ORDER IS THE RULE AUTHOR'S and matches the order the message names them, so chips read left to right
  // like the sentence. Never sort these. See
  // docsite/content/architecture/web-picking.md#context-entities.
  context: FindingContext[];
  sheets: SheetBadge[];
  // locateReason says why clicking this finding may highlight nothing, such as BUS_NOT_DRAWN for a
  // bus with no drawn wire. It is checks.Finding.locate_reason, computed server-side from the
  // geometry (WS7-042c). The default, UNSPECIFIED, means the subject is drawn and highlights.
  locateReason: LocateReason;
  // reportUrl opens an issue saying this finding looks wrong, built from a FindingFeedback, so it
  // names agni's rule and never anything from the design (agni issues 858, 1057). Absent before the
  // catalog has said which rules ship.
  reportUrl?: string;
}

// SkippedRuleItem is one selected rule that could not evaluate, with the reason the ENGINE gave.
// The reason passes through unreworded, since a sentence composed here would drift from the gate.
export interface SkippedRuleItem {
  rule: string;
  reason: string;
}

export interface FindingsState {
  findings: FindingItem[];
  // verdicts is the CONSIDERED SET for the same run, what each converted rule concluded about every
  // subject it looked at, passes included. Empty means no SELECTED rule reports coverage, so the panel
  // must not render it as "this design was not checked".
  verdicts: VerdictItem[];
  // focusedVerdict is the id of the verdict currently drawn as a proof, "" when none.
  focusedVerdict: string;
  // subject of the focused finding, "" when none (the whole selection is highlighted instead).
  selected: string;
  // number of rules currently selected, so the panel tells "no rules selected" (nothing ran) from
  // "no findings" (rules ran clean).
  ruleCount: number;
  // pending counts the selected rules whose findings are not computed yet. Checks run on demand (WS9),
  // so a design opens, or the selection changes, with the results uncomputed. It badges the Run button
  // and tells "press Run to evaluate" (pending > 0, empty list) from "ran clean" (pending 0, empty list).
  pending: number;
  // running is true while a check run is in flight, so the panel turns the Run button into Cancel.
  running: boolean;
  // cancelled is true when the last run was stopped before it answered (agni issue 918), so the rules
  // it left pending read as not run rather than as clean. Absent means not cancelled.
  cancelled?: boolean;
  // runError says why the last run failed and names the selected rules it left pending (agni issue
  // 1117), so they read as not run rather than as a Run never pressed. Absent when it did not fail.
  runError?: { message: string; rules: string[] };
  // catalogLoading is true from the moment a design starts opening until its rule catalog
  // (ListRules) answers. The first sheet can draw before then, and a Run pressed in that window has
  // no ruleset to run, so the panel disables the button and says why (agni 868).
  catalogLoading: boolean;
  // catalogError says why the rule catalog did not load, so an empty selection does not read as
  // "No rules selected." (agni issue 1117). Absent or "" when it loaded.
  catalogError?: string;
  // skipped names the selected rules that could NOT run on this design, and why. Some rules need a
  // fact tier the design lacks, such as a board rule on a netlist or a datasheet rule with no
  // corpus. Such a rule is gated before it evaluates and produces no findings. Without this list
  // the unanswered question would read as a clean board, and this panel opens by default, so most
  // people would see that misreading first.
  skipped: SkippedRuleItem[];
  // ruleSummaries maps a rule name to its catalog one-liner, shown as a group-header subtitle. A rule
  // absent from the map renders none.
  ruleSummaries: Record<string, string>;
  // ruleDocs maps a rule name to the published page explaining it, linked from the rule's group
  // header. A rule absent from the map gets no link.
  ruleDocs?: Record<string, string>;
  // saveNote says why the last report save failed, "" when it did not (agni issue 127).
  saveNote: string;
}

// VerdictItem is the view-side shape of one verdict (the wire checks.Verdict without the proto
// machinery). It records what a rule concluded about ONE subject. A rule also records a verdict for
// a subject it could not judge, and unlike a FindingItem a verdict exists for a PASS. Its subjects
// and context are HighlightSubjects, so the same bucketing lights them up without a second code
// path.
export interface VerdictItem {
  // id is the derived name, "<rule>:(<kind>:<ref>,...)". A CLI row or a filed link addresses it as
  // the click target. It does not change when the ANSWER changes, so a link filed while a check
  // passed still resolves once it starts failing.
  id: string;
  rule: string;
  // outcome is the lower-case vocabulary word ("pass" | "fail" | "no-limit" | "not-considered" |
  // "inconclusive"), decoded from the wire enum by outcomeWord.
  outcome: string;
  // subjects is the TUPLE this verdict is about, in the rule's order. Most rules carry one entity. A
  // relation rule carries two or three and all of them are the subject, so drawing only one as the
  // figure would show half of a clearance violation.
  subjects: HighlightSubject[];
  // statement is the one-line proof, present on pass, fail and inconclusive. It is empty on
  // no-limit and not-considered, since those verdicts have nothing to prove.
  statement: string;
  // terms are the labelled VALUES behind the statement. They are not entities and are not
  // clickable, so they are kept apart from context.
  terms: { label: string; value: string }[];
  // context lists the entities named in the proof, typed, ordered and clickable. The hops of a
  // pull-up path arrive here, so the drawing can show a proof rather than a subject.
  context: FindingContext[];
  // reason is why a not-considered verdict could not be decided, in the rule author's words.
  reason: string;
  // sheets are the ids of the sheets the subjects are drawn on, in the server's order (agni issue
  // 1151), empty when the design has no resolvable geometry. Locating the verdict opens the first
  // when the shown sheet is not among them.
  sheets: string[];
}

// verdictSubjectLabel is a verdict's tuple as one cell, each entity as ref (or ref.pin for a pin)
// joined with a plus. A relation verdict names two or three entities, and a row showing only the first
// would leave the reader guessing the rest.
export function verdictSubjectLabel(v: VerdictItem): string {
  return v.subjects.map((e) => (e.pin ? `${e.subject}.${e.pin}` : e.subject)).join(" + ");
}

// outcomeWord decodes the wire enum to the vocabulary word. An unrecognised value becomes
// "unspecified" rather than "", since a blank outcome would read as "nothing to report" about a
// subject the rule did look at. It spells each word as service.OutcomeWord does, and
// service/testdata/outcome_twin.json holds the two together (agni issue 1045).
export function outcomeWord(o: number | undefined): string {
  switch (o) {
    case 1:
      return "pass";
    case 2:
      return "fail";
    case 3:
      return "no-limit";
    case 4:
      return "not-considered";
    case 5:
      return "inconclusive";
    default:
      return "unspecified";
  }
}

// verdictProofStack builds the highlight layers for one verdict, with its CONTEXT as the ground and
// its SUBJECTS as the figure on top. It is focusStack with a different base. The findings path uses the
// whole findings list as ground ("where does this sit among the problems"), and a proof uses only its
// own entities ("what holds this up"). Both go through the same builder, so there is one drawing path.
export function verdictProofStack(v: VerdictItem, focus: HighlightSpec[]): HighlightSpec[] {
  return focusStack(v.context, v.subjects, focus);
}

export interface FindingsView {
  setState: (s: FindingsState) => void;
  // setFindingLocateNote shows a server-authoritative note under the checks table when a clicked
  // finding can't be located, such as a bus with no drawn wire (WS7-042c). An empty note clears it.
  // It mirrors the query panel's setLocateNote.
  setFindingLocateNote: (note: string) => void;
}

// FindingGroupAxis is what the checks panel groups findings by. "kind" is the entity axis
// (net/component/pin), exact via FindingItem.kind.
export type FindingGroupAxis = "rule" | "category" | "severity" | "kind" | "profile";

// UNRESOLVED_GROUP is the severity-axis group for an inconclusive finding, used in place of the
// severity it carries. The rule declined to decide, so filing it under "error" would group it with
// defects (agni issue 350). Every other axis is unaffected, because an inconclusive result still
// has a rule, a category, an entity kind and a profile.
export const UNRESOLVED_GROUP = "unresolved";

// groupFindings buckets findings by the chosen axis, preserving first-appearance order of values,
// as [value, items][].
export function groupFindings(findings: FindingItem[], axis: FindingGroupAxis): [string, FindingItem[]][] {
  const order: string[] = [];
  const by = new Map<string, FindingItem[]>();
  for (const f of findings) {
    const v =
      axis === "kind"
        ? f.kind
        : axis === "severity"
          ? f.inconclusive
            ? UNRESOLVED_GROUP
            : f.severity
          : axis === "category"
            ? f.category
            : axis === "profile"
              ? f.profile
              : f.rule;
    if (!by.has(v)) {
      by.set(v, []);
      order.push(v);
    }
    by.get(v)!.push(f);
  }
  return order.map((v) => [v, by.get(v)!]);
}

// CollapsedFinding folds every finding sharing findingKey into one row. head is the first-seen
// representative and instances holds every folded finding (length >= 1). A rule that fires once per
// entity with the SAME display fields (duplicate-net-name on N same-named nets) collapses to one
// row with instances.length N. Each instance is still addressable as a click-to-locate sub-row,
// told apart by its netId.
export interface CollapsedFinding {
  head: FindingItem;
  instances: FindingItem[];
}

// findingKey is the collapse identity of a finding, and the panel also keys a row's expand state on
// it. Parts join with the NUL escape "\u0000". The source writes the escape and never a literal
// NUL, so the file is plain text. NUL cannot appear in a rule, subject, pin or message, so distinct
// tuples never collide. busId is included so two identically-labeled buses are distinct rows and
// each locates its own trunk (WS7-042b). It is "" for every non-bus finding.
export function findingKey(f: FindingItem): string {
  return [f.rule, f.subject, f.pin, f.severity, f.message, f.busId].join("\u0000");
}

// collapseInstances groups adjacent-or-not findings by findingKey, preserving first-appearance
// order of the representative. Every input finding goes into exactly one CollapsedFinding.
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

// severityRank orders severities worst-first (unknown -1, error 0, warning 1, info 2), as the
// engine's report does (core/results.SeverityRank). An unrecognised severity comes first. A
// provider's custom level may be worse than the defects, so it is never buried below them (agni
// issue 1045). core/results/testdata/severity_twin.json holds the two to one order.
export function severityRank(sev: string): number {
  return SEV_RANK[sev] ?? -1;
}

// findingRank orders one finding for the severity column, as severityRank plus what a severity
// string cannot say. An inconclusive result carries the severity the rule WOULD have reported, so
// ranking by that string would sort it among real defects (agni issue 350). It ranks after every
// severity instead, including an unrecognized one, so the defects are together at the top.
export function findingRank(f: FindingItem): number {
  return f.inconclusive ? 4 : severityRank(f.severity);
}

const cmpStr = (a: string, b: string): number => (a < b ? -1 : a > b ? 1 : 0);

// sortFindings returns a new array ordered by key then the rule/subject fallback chain; dir 1 is
// ascending (worst-first for severity), -1 reverses the whole comparison.
export function sortFindings(items: FindingItem[], key: FindingSortKey, dir: 1 | -1): FindingItem[] {
  const cmp = (a: FindingItem, b: FindingItem): number => {
    const base =
      key === "severity"
        ? findingRank(a) - findingRank(b) || cmpStr(a.rule, b.rule) || cmpStr(a.subject, b.subject)
        : key === "rule"
          ? cmpStr(a.rule, b.rule) || cmpStr(a.subject, b.subject)
          : cmpStr(a.subject, b.subject) || cmpStr(a.rule, b.rule);
    return base * dir;
  };
  return [...items].sort(cmp);
}

// severitySections projects findings into worst-first [severity, count] tiers, in the shape and
// order of the engine report's sections (core/results.Pivot). Severities sort by severityRank and
// ties by name, and UNRESOLVED_GROUP comes last with every inconclusive finding. findings.test.ts
// and core/results read one fixture, testdata/severity_twin.json, so the panel's ranking and the
// report's cannot drift (WS3-022, agni issue 1045).
export function severitySections(items: FindingItem[]): { severity: string; count: number }[] {
  const counts = new Map<string, number>();
  let unresolved = 0;
  for (const f of items) {
    if (f.inconclusive) unresolved++;
    else counts.set(f.severity, (counts.get(f.severity) ?? 0) + 1);
  }
  const out = [...counts.entries()]
    .map(([severity, count]) => ({ severity, count }))
    .sort((a, b) => severityRank(a.severity) - severityRank(b.severity) || cmpStr(a.severity, b.severity));
  if (unresolved > 0) out.push({ severity: UNRESOLVED_GROUP, count: unresolved });
  return out;
}

// HighlightSubject holds the minimum subjectsToSpecs needs, an entity ref, its kind, and a pin's
// designator. FindingItem satisfies it structurally, and so does a bare query result cell
// (WS9-038), so a query entity highlights through the same bucketing as a finding.
export interface HighlightSubject {
  kind: string; // "net" | "component" | "pin" | "bus"
  subject: string;
  pin: string;
  // netId is the optional per-instance net identity. On a net subject the spec targets THAT
  // instance, so two same-named nets highlight separately. A bare query cell (WS9-038) has none and joins
  // by name.
  netId?: string;
  // busId is the optional source id for a kind="bus" subject. A bus has no net, so this is its only
  // highlight join key (WS7-042b).
  busId?: string;
}

// subjectsToSpecs builds one HighlightSpec that lights up every subject at once, bucketed by kind
// (nets / components / pins) and deduped. A net with a netId buckets by id, so same-named nets are
// distinct targets, and one without falls back to its name. Returns [] when there is nothing to
// highlight, which clears the highlight. The presenter's setHighlights drives both renderers with it.
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
      // A bus joins ONLY by its source id (WS7-042b). One with no id (an undrawable bus_alias or EDIF
      // array) contributes nothing, and its "not drawn" note is WS7-042c.
      if (f.busId) busIds.add(f.busId);
    } else if (!f.subject) {
      continue;
    } else if (f.kind === "pin") {
      const key = `${f.subject} ${f.pin}`;
      if (!seenPin.has(key)) {
        seenPin.add(key);
        pins.push({ refDes: f.subject, pin: f.pin });
      }
    } else if (f.kind === "component" || f.kind === "") {
      components.add(f.subject);
    }
    // Any other kind contributes nothing. This renderer has no geometry join for it, and treating it
    // as a ref-des would highlight whatever shared the string (a symbol reference, an endpoint's "x,y").
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

// findingSpec is the focus highlight for one finding, the same bucketing as subjectsToSpecs.
export function findingSpec(f: FindingItem): HighlightSpec[] {
  return subjectsToSpecs([f]);
}

// entitySpecs is the focus highlight for a bare (kind, subject) that is not a finding, such as a
// query result cell (WS9-038). It buckets like findingSpec, so a located entity paints as the
// equivalent finding would.
export function entitySpecs(kind: string, subject: string, pin = ""): HighlightSpec[] {
  return subjectsToSpecs([{ kind, subject, pin }]);
}

// focusStack builds the two-layer highlight stack for a focused subject (WS9-040), the base
// findings layer with each focused NET removed and the focus layer on top. A net's focus is a
// translucent PATH highlighter (withFocusShape), and the net left in the opaque base would show
// through it. A focused component or pin is kept in the base, since its outline plus the focus
// bounding box read as added emphasis. An empty focus (subject not found) leaves the base
// untouched.
//
// `figures` is a LIST because a verdict's subject is a tuple (a clearance violation is about two nets).
// A findings caller passes its one subject.
export function focusStack(findings: HighlightSubject[], figures: HighlightSubject[], focus: HighlightSpec[]): HighlightSpec[] {
  // Drop by netId when the figure carries one, so same-named siblings keep their outline, else by
  // name.
  const focusedNets = figures.filter((f) => f.kind === "net");
  const isFocused = (f: HighlightSubject) =>
    f.kind === "net" &&
    focusedNets.some((g) => ((g.netId ?? "") !== "" ? f.netId === g.netId : f.subject === g.subject));
  const base = focusedNets.length > 0 ? findings.filter((f) => !isFocused(f)) : findings;
  const baseSpecs = subjectsToSpecs(base);
  // With no figure the base is the whole message, and muting it would dim the only layer on the sheet.
  if (focus.length === 0) return baseSpecs;
  // Otherwise stamp the base as CONTEXT, so it is a different hue from the focus (agni issue 348).
  // The focus layer keeps the default color, so the reader's style from the Highlight menu still
  // wins.
  return [
    ...baseSpecs.map((s) => ({ ...s, color: BASE_HIGHLIGHT_COLOR, alpha: BASE_HIGHLIGHT_ALPHA })),
    ...focus,
  ];
}


// The checks shown for a selection are a projection of the findings already computed (agni issue
// 259). They are never a scoped re-run. A re-run could disagree with the report beside it (C25),
// and it would redo net solving per click. Every Finding carries exactly one Subject, so grouping
// by subject PARTITIONS the findings. See
// docsite/content/architecture/web-picking.md#what-is-already-known-about-a-selection.

// selectionFromFinding reads a finding as the thing it is about. A keyed element on the drawing and
// a result cell also produce a Selection, and this is the third producer. So one identity rule,
// sameSelection, decides when two things are the same net, for the canvas, the query table and the
// checks panel alike. It covers two nets that share a display name and differ only by netId.
export function selectionFromFinding(f: FindingItem): Selection | null {
  switch (f.kind) {
    case "pin":
      return f.subject && f.pin ? { kind: "pin", ref: f.subject, pin: f.pin } : null;
    case "component":
      return f.subject ? { kind: "component", ref: f.subject } : null;
    case "net":
      return f.subject || f.netId ? { kind: "net", net: f.subject, netId: f.netId } : null;
    case "bus":
      return f.busId ? { kind: "bus", busId: f.busId } : null;
    default:
      return null;
  }
}

// findingsFor projects the findings owning any of the given subjects, in the order the pass
// produced them. It takes a SET. A query answer, a sheet, a netclass and a diff's changed entities
// all ask this question, and a click is its one-subject case. A finding is returned ONCE however
// many subjects it matches, so a query answering R1 on five nets reports R1's one finding once.
//
// It answers OWNERSHIP, not mention. An entity named only in a finding's context (agni issue 349) does
// not get that finding here, since context is not counted as a finding about itself.
export function findingsFor(findings: FindingItem[], selections: (Selection | null)[]): FindingItem[] {
  const wanted = selections.filter((s): s is Selection => s !== null);
  if (wanted.length === 0) return [];
  return findings.filter((f) => {
    const sel = selectionFromFinding(f);
    return sel !== null && wanted.some((w) => (w.kind === "area" ? inArea(sel, w) : sameSelection(sel, w)));
  });
}

// inArea reports whether a finding's subject is a member of an area (agni issue 904). A member is
// one of the area's parts, a pin of one of its parts, one of its loose pins, or a net drawn inside
// it.
function inArea(sel: Selection, area: Selection): boolean {
  switch (sel.kind) {
    case "component":
      return (area.refs ?? []).includes(sel.ref ?? "");
    case "pin":
      return (area.refs ?? []).includes(sel.ref ?? "") || (area.pins ?? []).includes(`${sel.ref}.${sel.pin}`);
    case "net":
      return (area.wires ?? []).includes(sel.net ?? "");
    default:
      return false;
  }
}

// SeverityTally counts a projection by severity, because "3 findings" and "3 errors" are different
// news. `inconclusive` is counted apart and excluded from `total`, since an inconclusive result is
// neither a pass nor a fail. It is still shown because a reader can act on it by supplying what was
// missing.
export interface SeverityTally {
  error: number;
  warning: number;
  info: number;
  total: number;
  inconclusive: number;
}

// tallySeverities counts a finding list. An unrecognized severity still counts toward the total, so
// the total is never less than the number of defects and a new severity cannot silently vanish.
export function tallySeverities(findings: FindingItem[]): SeverityTally {
  const t: SeverityTally = { error: 0, warning: 0, info: 0, total: 0, inconclusive: 0 };
  for (const f of findings) {
    if (f.inconclusive) {
      t.inconclusive++;
      continue;
    }
    t.total++;
    if (f.severity === "error") t.error++;
    else if (f.severity === "warning") t.warning++;
    else if (f.severity === "info") t.info++;
  }
  return t;
}

// CheckedState is how much of the current ruleset has actually run, which decides whether a count
// beside an entity means anything. Without it a zero reads as "this entity is clean" when nobody has
// pressed Run.
export type CheckedState = "loading" | "no-rules" | "running" | "not-run" | "partial" | "complete";

// checkedState classifies a pushed FindingsState. `partial` gets its own name because a half-run
// ruleset gives real findings and an understated count at once, so the count has to read as a floor.
// `loading` comes before `no-rules` because a catalog that has not arrived selects nothing yet.
export function checkedState(s: { ruleCount: number; pending: number; running: boolean; catalogLoading?: boolean }): CheckedState {
  if (s.running) return "running";
  if (s.catalogLoading) return "loading";
  if (s.ruleCount === 0) return "no-rules";
  if (s.pending >= s.ruleCount) return "not-run";
  return s.pending > 0 ? "partial" : "complete";
}
