// The review panel's view-side types (WS9-052). A review RUN is a resource (WS9-053): the presenter
// lists the runs stored for the open design, creates new ones, and pushes a ReviewState the panel
// renders. Clicking a finding emits onLocate, reusing the same locate path a query cell or a finding
// already uses.
import type { CheckResults } from "./gen/agni/v1/checks/checks_pb.js";
import type { Review } from "./gen/agni/v1/webapi/review_pb.js";
import { contextFromWire, type FindingItem } from "./findings.js";
import { LocateReason } from "./gen/agni/v1/checks/checks_pb.js";

// OUTCOMES is the review vocabulary, in the order a report reads: the two decided verdicts, then the
// ways a question went unanswered, then the ones awaiting data.
//
// It is a string union rather than an enum because the wire field is a string, deliberately, so a new
// honest verdict is a review-layer change rather than a schema migration. The panel must therefore
// cope with an outcome it has never heard of, and it does: an unknown value renders under its own
// name with the neutral "unknown" style, never silently as a pass.
export const OUTCOME_PASS = "pass";
export const OUTCOME_FAIL = "fail";
export const OUTCOME_NOT_APPLICABLE = "not-applicable";
export const OUTCOME_NOT_AUTOMATED = "not-automated";
export const OUTCOME_PROVISIONAL = "provisional";
export const OUTCOME_NEEDS_DESIGN_INTENT = "needs-design-intent";
export const OUTCOME_COMPUTED_NA = "computed-n/a";
export const OUTCOME_NEEDS_DATA = "needs-data";
export const OUTCOME_INCONCLUSIVE = "inconclusive";

// Tally counts item outcomes, mirroring core/review's Tally field for field. Both surfaces DERIVE it
// from the item outcomes rather than reading it off the wire, and core/review/testdata/tally_twin.json
// is the shared fixture that keeps them from drifting.
export interface Tally {
  pass: number;
  fail: number;
  notApplicable: number;
  notAutomated: number;
  provisional: number;
  needsDesignIntent: number;
  computedNA: number;
  needsData: number;
  inconclusive: number;
  total: number;
}

export function emptyTally(): Tally {
  return {
    pass: 0, fail: 0, notApplicable: 0, notAutomated: 0,
    provisional: 0, needsDesignIntent: 0, computedNA: 0,
    needsData: 0, inconclusive: 0, total: 0,
  };
}

// tally counts a list of outcome strings. An outcome this build does not know still counts toward
// `total`, exactly as the Go side does: the count of things asked must not shrink because the client
// is older than the engine that answered them.
export function tally(outcomes: string[]): Tally {
  const t = emptyTally();
  for (const o of outcomes) {
    t.total++;
    switch (o) {
      case OUTCOME_PASS: t.pass++; break;
      case OUTCOME_FAIL: t.fail++; break;
      case OUTCOME_NOT_APPLICABLE: t.notApplicable++; break;
      case OUTCOME_NOT_AUTOMATED: t.notAutomated++; break;
      case OUTCOME_PROVISIONAL: t.provisional++; break;
      case OUTCOME_NEEDS_DESIGN_INTENT: t.needsDesignIntent++; break;
      case OUTCOME_COMPUTED_NA: t.computedNA++; break;
      case OUTCOME_NEEDS_DATA: t.needsData++; break;
      case OUTCOME_INCONCLUSIVE: t.inconclusive++; break;
    }
  }
  return t;
}

// covered is the coverage axis: how many items a mechanism exists for, which is everything except
// not-automated. It is the number a team reads as "how much of our checklist is mechanised", so it
// deliberately counts an item awaiting a datasheet value or a design-intent declaration as COVERED —
// those have a mechanism that ran and reported honestly, unlike an item nothing answers at all.
export function covered(t: Tally): number {
  return t.total - t.notAutomated;
}

// ReviewItemView is one checklist item's outcome plus the findings that produced it.
export interface ReviewItemView {
  id: string;
  title: string;
  outcome: string;
  note: string;
  findings: FindingItem[];
}

export interface ReviewAreaView {
  name: string;
  items: ReviewItemView[];
}

// ReviewRunView is one stored run as the panel renders it. createdAt and producerVersion come from
// the document's meta rather than being reconstructed, so an archived run says which build produced
// it; design is what the run was about, and designHash is the revision identity that tells a reader
// whether two runs describe the same bytes.
export interface ReviewRunView {
  name: string;
  design: string;
  designHash: string;
  createdAt: string;
  manifest: string;
  producerVersion: string;
  areas: ReviewAreaView[];
}

// ChecklistOption is one manifest a user can run, discovered by listing the design's directory.
export interface ChecklistOption {
  ref: string;
  label: string;
}

export interface ReviewState {
  // runs are the stored runs for the open design, newest first. Only identity and headline numbers
  // are needed to render the picker, but a run carries its whole document, so the selected one needs
  // no second fetch.
  runs: ReviewRunView[];
  // selected is the run being shown, "" when none. A fresh design load selects the newest run if one
  // exists, so opening a reviewed board shows its latest verdict rather than an empty panel.
  selected: string;
  // checklists are the manifests found beside the design, and checklist is the chosen one.
  checklists: ChecklistOption[];
  checklist: string;
  // running is true while a create is in flight, so the panel disables its button.
  running: boolean;
  // storeConfigured is false when the server was started without --review-store. It is a distinct
  // state from "no runs yet" on purpose: one is a deployment that cannot keep runs at all, the other
  // is a design nobody has reviewed. Collapsing them would send a user hunting for a missing button.
  storeConfigured: boolean;
  // error is a message to show instead of results, "" when fine.
  error: string;
}

export interface ReviewView {
  setState: (s: ReviewState) => void;
}

export function emptyReview(): ReviewState {
  return {
    runs: [], selected: "", checklists: [], checklist: "",
    running: false, storeConfigured: true, error: "",
  };
}

// runTally counts every item across a run's areas.
export function runTally(run: ReviewRunView): Tally {
  return tally(run.areas.flatMap((a) => a.items.map((i) => i.outcome)));
}

// areaTally counts one area's items, for the per-area subtotal.
export function areaTally(area: ReviewAreaView): Tally {
  return tally(area.items.map((i) => i.outcome));
}

// reviewFromWire maps a stored Review resource into the panel's view state.
//
// It reads the DOCUMENT rather than a bespoke response shape, which is what makes an archived run and
// a just-created one render through one path: GetReview and CreateReview both return a Review, and a
// run fetched months later carries everything this needs.
export function reviewFromWire(rv: Review): ReviewRunView {
  const doc: CheckResults | undefined = rv.results;
  return {
    name: rv.name,
    design: doc?.design?.source ?? "",
    designHash: doc?.design?.contentHash ?? "",
    createdAt: doc?.meta?.createdAt ?? "",
    manifest: doc?.manifest ?? "",
    producerVersion: doc?.meta?.producerVersion ?? "",
    areas: (doc?.areas ?? []).map((a) => ({
      name: a.name,
      items: (a.items ?? []).map((i) => ({
        id: i.id,
        title: i.title,
        outcome: i.outcome,
        note: i.note,
        findings: (i.findings ?? []).map((f) => ({
          rule: f.rule,
          category: "",
          profile: "",
          severity: f.severity,
          kind: f.subject?.kind ?? "",
          subject: f.subject?.ref ?? "",
          pin: f.subject?.pin ?? "",
          netId: f.subject?.netId ?? "",
          busId: f.subject?.busId ?? "",
          message: f.message,
          inconclusive: f.inconclusive ?? false,
          context: contextFromWire(f.context),
          sheets: [],
          locateReason: f.locateReason ?? LocateReason.UNSPECIFIED,
        })),
      })),
    })),
  };
}

// checklistOptions picks the manifests out of a directory listing. It filters by extension only,
// because deciding whether a YAML file is a review manifest means parsing it, and that is the
// server's job: GetReviewManifest validates and says so. Offering a file that turns out not to be a
// checklist costs one clear error; hiding a real one costs a user their own file.
export function checklistOptions(entries: { name: string; path: string; isDir: boolean }[]): ChecklistOption[] {
  return entries
    .filter((e) => !e.isDir && (e.name.endsWith(".yaml") || e.name.endsWith(".yml")))
    .map((e) => ({ ref: e.path, label: e.name }));
}

// selectedRun returns the run the state points at, or undefined.
export function selectedRun(s: ReviewState): ReviewRunView | undefined {
  return s.runs.find((r) => r.name === s.selected);
}

// OUTCOME_LABEL is the short text an outcome chip shows. An outcome absent from this map renders
// under its own wire name, which is why a build older than the engine degrades to "unrecognised
// verdict" rather than to a wrong one.
export const OUTCOME_LABEL: Record<string, string> = {
  [OUTCOME_PASS]: "pass",
  [OUTCOME_FAIL]: "fail",
  [OUTCOME_NOT_APPLICABLE]: "n/a",
  [OUTCOME_NOT_AUTOMATED]: "not automated",
  [OUTCOME_PROVISIONAL]: "provisional",
  [OUTCOME_NEEDS_DESIGN_INTENT]: "needs intent",
  [OUTCOME_COMPUTED_NA]: "computed n/a",
  [OUTCOME_NEEDS_DATA]: "needs data",
  [OUTCOME_INCONCLUSIVE]: "inconclusive",
};

// KNOWN_OUTCOMES is the set the panel styles explicitly. outcomeClass keys off it so an unknown
// verdict gets the neutral style rather than falling through to whatever CSS matched last.
const KNOWN_OUTCOMES = new Set(Object.keys(OUTCOME_LABEL));

// outcomeClass is the chip's CSS class for an outcome.
//
// The mapping is the panel's load-bearing accessibility decision, not styling. Only `pass` may look
// like a pass. Everything else has to read as unfinished, because the entire reason this vocabulary
// exists is that a question nobody answered must not score as answered. An unknown outcome gets
// `rv-unknown`, which is styled like the unanswered group rather than the passing one: guessing
// optimistically about a verdict this build does not understand is the one guess that can mislead.
export function outcomeClass(outcome: string): string {
  if (!KNOWN_OUTCOMES.has(outcome)) return "rv-unknown";
  return "rv-" + outcome.replace(/[^a-z0-9]+/g, "-");
}
