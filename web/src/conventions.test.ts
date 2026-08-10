import { describe, it, expect } from "vitest";
import {
  activeLabel,
  emptyConvention,
  isOverridden,
  SERVER_DEFAULT_LABEL,
  type ConventionState,
} from "./conventions.js";

function state(over: Partial<ConventionState> = {}): ConventionState {
  return { ...emptyConvention(), ...over };
}

describe("activeLabel", () => {
  // The default state must NAME whose vocabulary is in effect rather than say "none". A request that
  // carries no convention is still answered under the deployment's, and calling that "none" would
  // suggest the engine was working from no vocabulary at all.
  it("names the server's convention rather than calling it none", () => {
    expect(activeLabel(state())).toBe(SERVER_DEFAULT_LABEL);
    expect(activeLabel(state()).toLowerCase()).not.toContain("none");
  });

  it("prefers the convention's own name over its filename", () => {
    expect(activeLabel(state({ active: "proj/house.yaml", name: "house" }))).toBe("house");
  });

  // The name is also the namespace its rules appear under, so a finding reading `acme/...` is only
  // interpretable if the bar says `acme`. Falling back to the ref keeps it interpretable when the
  // resolve has not landed yet.
  it("falls back to the ref when the name is not known yet", () => {
    expect(activeLabel(state({ active: "proj/house.yaml" }))).toBe("proj/house.yaml");
  });

  it("says it is working while resolving", () => {
    expect(activeLabel(state({ active: "x.yaml", name: "x", busy: true }))).toBe("resolving…");
  });
});

describe("isOverridden", () => {
  // This drives the bar's styling, and the styling is the point: a request convention REPLACES the
  // server's, so rules can stop running, and a rule that stops running produces no findings. In a
  // findings list that is indistinguishable from a design that got fixed.
  it("is false under the server's convention and true under a request's", () => {
    expect(isOverridden(state())).toBe(false);
    expect(isOverridden(state({ active: "proj/house.yaml", name: "house" }))).toBe(true);
  });

  // A resolve that failed left the previous vocabulary in effect, so the indicator must reflect what
  // the findings were actually computed under, not what the user last clicked.
  it("reflects what is applied, not what was attempted", () => {
    expect(isOverridden(state({ active: "", error: "could not parse" }))).toBe(false);
  });
});
