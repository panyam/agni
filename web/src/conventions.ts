// The naming-convention bar's view-side types (WS9-128). A request may carry its own naming
// convention, which REPLACES the server's startup default for that request (WS3-124). The bar picks
// one and says which is in effect.
import type { ChecklistOption } from "./review.js";

export interface ConventionState {
  // choices are the YAML configs beside the open design, the same listing the review panel's
  // checklist picker uses. Neither picker guesses which files are its own kind; the server decides.
  choices: ChecklistOption[];
  // active is the ref currently applied, "" when the server's default is in effect.
  active: string;
  // name is the resolved convention's own `name:`, which is also the namespace its rules appear
  // under, so a finding from `acme/signal-net-naming` is readable. "" when none is applied.
  name: string;
  // busy is true while a convention is being resolved.
  busy: boolean;
  // error is a resolve failure to show inline, "" when fine.
  error: string;
}

export interface ConventionBarView {
  setState: (s: ConventionState) => void;
}

export function emptyConvention(): ConventionState {
  return { choices: [], active: "", name: "", busy: false, error: "" };
}

// SERVER_DEFAULT_LABEL is what the picker shows for "no request convention". Not "none", because a
// request carrying no convention is answered under the deployment's.
export const SERVER_DEFAULT_LABEL = "server's convention";

// activeLabel is the one-line statement of which vocabulary produced the answers on screen.
//
// A request convention REPLACES the server's, so switching can stop the deployment's rules running
// entirely. A rule that stops running produces no findings, which looks like a design that improved,
// and nothing in a findings list tells "this got fixed" from "we stopped asking".
export function activeLabel(s: ConventionState): string {
  if (s.busy) return "resolving…";
  if (!s.active) return SERVER_DEFAULT_LABEL;
  return s.name || s.active;
}

// conventionError summarizes a resolve failure for the bar, keeping the server's own message (#171).
// The picker offers every YAML beside the design, so choosing a design-intent or project descriptor
// is an EXPECTED path, and the server's answer ("field modules not found in type naming.Config", with
// the line) is the only clue which files are the right kind.
//
// Three shapes get flattened. Connect prefixes "[invalid_argument] " and the server's text then
// begins "invalid argument: " because it wraps a sentinel, and both name the status rather than the
// problem. A YAML unmarshal error's first line is only a header ("unmarshal errors:"), so the first
// detail line under it is appended. The full text stays on the title.
export function conventionError(raw: string): string {
  const msg = raw
    .replace(/^\[[a-z_]+\]\s*/i, "")
    .replace(/^invalid argument:\s*/i, "")
    .trim();
  const lines = msg
    .split("\n")
    .map((l) => l.trim())
    .filter(Boolean);
  if (lines.length === 0) return "";
  if (lines.length > 1 && lines[0].endsWith(":")) return `${lines[0]} ${lines[1]}`;
  return lines[0];
}

// isOverridden reports whether the answers on screen were computed under a request-supplied
// vocabulary rather than the deployment's. The panel styles the bar on it so the non-default state
// is visible.
export function isOverridden(s: ConventionState): boolean {
  return s.active !== "";
}
