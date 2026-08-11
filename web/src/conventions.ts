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

// conventionError summarizes a resolve failure for the bar, which previously showed the fixed string
// "could not apply" and kept the real message in a title attribute nobody hovers.
//
// That mattered more than a typical error-text nit, because the picker offers every YAML sitting
// beside the design and cannot tell which are naming configs without parsing them, which is the
// server's job. Choosing a design-intent or a project descriptor by mistake is therefore an EXPECTED
// path, not an edge case, and the server answers it precisely ("field modules not found in type
// naming.Config", with the line). Replacing that with "could not apply" turned a self-explaining
// error into a dead end, and the picker gives no other clue which files are the right kind.
//
// Three shapes get flattened. TWO status prefixes stack up before the message says anything: Connect
// formats a ConnectError as "[invalid_argument] ", and the server's own error text then begins
// "invalid argument: " because it wraps a sentinel. Both name the status rather than the problem, and
// the reader can already see something failed from the chip being on screen. And a YAML unmarshal
// error's first line is only a header, so the first line ALONE would say "unmarshal errors:" and
// nothing about what was wrong; the first detail under it is the part that identifies the file as the
// wrong kind. The full text stays on the title for the remaining detail lines.
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
// vocabulary rather than the deployment's. The panel styles the bar on it, so the non-default state
// is visible rather than merely available in a dropdown nobody re-reads.
export function isOverridden(s: ConventionState): boolean {
  return s.active !== "";
}
