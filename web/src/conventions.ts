// The naming-convention bar's view-side types (WS9-128). A request may carry its own naming
// convention, which REPLACES the server's startup default for that request (WS3-124), and this is the
// surface that chooses one and says which is in effect.
import type { ChecklistOption } from "./review.js";

export interface ConventionState {
  // choices are the YAML configs sitting beside the open design. The same listing feeds the review
  // panel's checklist picker; neither picker guesses which files are really its own kind, because
  // that is the server's call.
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

// SERVER_DEFAULT_LABEL is what the picker shows for "no request convention". It says whose vocabulary
// that is rather than saying "none", because there is no such thing as no vocabulary: a request that
// carries none is answered under the deployment's, and calling that "none" would suggest the engine
// was working from nothing.
export const SERVER_DEFAULT_LABEL = "server's convention";

// activeLabel is the one-line statement of which vocabulary produced the answers on screen.
//
// This exists because of what replacement does. A request convention does not add to the server's, it
// REPLACES it, so switching can make the deployment's rules stop running entirely — and a rule that
// stops running produces no findings, which looks exactly like a design that improved. Nothing in a
// findings list distinguishes "this got fixed" from "we stopped asking", so the bar has to.
export function activeLabel(s: ConventionState): string {
  if (s.busy) return "resolving…";
  if (!s.active) return SERVER_DEFAULT_LABEL;
  return s.name || s.active;
}

// isOverridden reports whether the answers on screen were computed under a request-supplied
// vocabulary rather than the deployment's. The panel styles the bar on it, so the non-default state
// is visible rather than merely available in a dropdown nobody re-reads.
export function isOverridden(s: ConventionState): boolean {
  return s.active !== "";
}
