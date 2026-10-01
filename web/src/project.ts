// The project bar's view-side types (agni issue 175). The bar says which project the open design
// resolved to, and whether the answers on screen were computed under that project's config or under
// the built-in catalog.
//
// Per-design config decides which rules run, and a findings list cannot say which config produced it.
// A rule that never ran because a project did not ask for it looks exactly like a rule that ran and
// found nothing, so the bar states whose rules are on screen. The convention bar does the same one
// level down.

// ProjectState is what the bar renders.
export interface ProjectState {
  // project is the resolved project's resource name, "" when the design belongs to none.
  project: string;
  // title is that project's human-readable label, falling back to its id.
  title: string;
  // design is the resolved design's resource name, "" when nothing resolved.
  design: string;
  // entry is the design's declared analysis entry, and namedIsEntry says whether the file the user
  // opened IS that entry. See entryNotice for the companion case.
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

// NO_PROJECT_LABEL is what the bar shows for a design that belongs to no project. It is stated rather
// than left blank because blank reads as "still resolving", and most files on a mounted folder belong
// to no project.
export const NO_PROJECT_LABEL = "no project";

// PLAIN_LABEL is what the bar shows when the built-in catalog is in effect by the user's choice. It
// produces the same findings as a design with no project, and is spelled differently because it
// means something different.
export const PLAIN_LABEL = "built-in catalog";

// projectLabel is the one-line statement of whose rules produced the answers on screen.
export function projectLabel(s: ProjectState): string {
  if (s.busy) return "resolving…";
  if (s.plain) return PLAIN_LABEL;
  if (!s.project) return NO_PROJECT_LABEL;
  return s.title || s.project;
}

// isOverridden reports whether the user chose the built-in catalog over this design's own project.
// A design with no project is not overridden. The bar styles the overridden state so it stays
// visible.
export function isOverridden(s: ProjectState): boolean {
  return s.plain;
}

// canGoPlain reports whether the built-in-catalog toggle means anything here. A design with no
// project is already running the built-in catalog.
export function canGoPlain(s: ProjectState): boolean {
  return s.project !== "";
}

// entryNotice is the line shown when the open file is NOT the design's analysis entry. The CLI swaps
// in the entry and prints a note. The viewer shows the file the user picked in the tree and states
// the entry instead, since a swap there would show a different file with nothing saying so.
export function entryNotice(s: ProjectState): string {
  if (s.busy || s.namedIsEntry || !s.entry) return "";
  return `this file is a companion view; analysis reads ${baseName(s.entry)}`;
}

function baseName(uri: string): string {
  const i = uri.lastIndexOf("/");
  return i < 0 ? uri : uri.slice(i + 1);
}
