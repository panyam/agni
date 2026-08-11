// The project bar's view-side types (agni issue 175): which project the open design resolved to, and
// whether the answers on screen were computed under that project's config or under the built-in
// catalog.
//
// This exists for the same reason the convention bar does, one level up. Per-design config decides
// which rules run, and a findings list cannot express which config produced it: a rule that never ran
// because a project did not ask for it looks exactly like a rule that ran and found nothing. Once
// config resolves silently from the design's project, "whose rules are these" stops being answerable
// from the screen unless something says so.

// ProjectState is what the bar renders.
export interface ProjectState {
  // project is the resolved project's resource name, "" when the design belongs to none.
  project: string;
  // title is that project's human-readable label, falling back to its id.
  title: string;
  // design is the resolved design's resource name, "" when nothing resolved.
  design: string;
  // entry is the design's declared analysis entry, and namedIsEntry says whether the file the user
  // opened IS that entry. A companion opened directly is the case worth surfacing: the served viewer
  // shows the file it was asked for, and the entry is what an analysis would read.
  entry: string;
  namedIsEntry: boolean;
  // plain is true when the user asked to see this design under the built-in catalog only.
  plain: boolean;
  // busy is true while resolution is in flight.
  busy: boolean;
  // error is a resolution failure to show inline, "" when fine.
  error: string;
}

export interface ProjectBarView {
  setState: (s: ProjectState) => void;
}

export function emptyProject(): ProjectState {
  return { project: "", title: "", design: "", entry: "", namedIsEntry: true, plain: false, busy: false, error: "" };
}

// NO_PROJECT_LABEL is what the bar shows for a design that belongs to no project.
//
// It is stated rather than left blank, because blank reads as "not checked yet" and this is a real,
// ordinary answer: most files on a mounted folder belong to no project. A reviewer who cannot tell
// "no project" from "still resolving" cannot tell whether the findings in front of them are the
// engine's opinion or a team's.
export const NO_PROJECT_LABEL = "no project";

// PLAIN_LABEL is what the bar shows when the built-in catalog is in effect by the user's choice, as
// distinct from a design that simply has no project. The two produce the same findings and mean
// different things, so they are not spelled the same.
export const PLAIN_LABEL = "built-in catalog";

// projectLabel is the one-line statement of whose rules produced the answers on screen.
export function projectLabel(s: ProjectState): string {
  if (s.busy) return "resolving…";
  if (s.plain) return PLAIN_LABEL;
  if (!s.project) return NO_PROJECT_LABEL;
  return s.title || s.project;
}

// isOverridden reports whether the screen is showing something other than this design's own default:
// either the built-in catalog by choice, or a design that resolved to no project at all. The bar
// styles the non-default state so it is visible rather than merely available in a control nobody
// re-reads.
export function isOverridden(s: ProjectState): boolean {
  return s.plain;
}

// canGoPlain reports whether the built-in-catalog toggle means anything here. A design with no
// project is already running the built-in catalog, so offering to switch to it would imply a
// difference that does not exist.
export function canGoPlain(s: ProjectState): boolean {
  return s.project !== "";
}

// entryNotice is the line shown when the open file is NOT the design's analysis entry.
//
// The served viewer deliberately shows the file it was asked for rather than silently swapping in the
// entry, which is what the CLI does with a printed note. A silent swap has no equivalent in a browser:
// the user picked a file in a tree and would be looking at a different one with nothing to say so. So
// the resolution is surfaced instead, and acting on it stays the user's move.
export function entryNotice(s: ProjectState): string {
  if (s.busy || s.namedIsEntry || !s.entry) return "";
  return `this file is a companion view; analysis reads ${baseName(s.entry)}`;
}

function baseName(uri: string): string {
  const i = uri.lastIndexOf("/");
  return i < 0 ? uri : uri.slice(i + 1);
}
