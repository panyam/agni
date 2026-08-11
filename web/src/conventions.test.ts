import { describe, it, expect } from "vitest";
import {
  activeLabel,
  conventionError,
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

describe("conventionError", () => {
  // These are what the CLIENT sees, not what the server sends, and the difference is the bug this
  // nearly shipped with. Connect formats a ConnectError's message as "[code] " + the server's text,
  // and the server's text already begins "invalid argument: ", so TWO status prefixes stack before
  // the message says anything. A fixture written from the wire response alone would miss the first
  // one and the stripping would silently not fire in the browser.
  //
  // EXPECT_YAML is reachable from a stock checkout: the conformance fixtures ship .expect.yaml
  // siblings, so every entry the picker offers beside one of those designs is a file that cannot
  // resolve. INTENT_YAML is the shape an overlay hits, where a design-intent file sits beside the
  // design it describes.
  const EXPECT_YAML =
    "[invalid_argument] invalid argument: naming config: yaml: unmarshal errors:\n" +
    "  line 1: field fires not found in type naming.Config";
  const INTENT_YAML =
    "[invalid_argument] invalid argument: naming config: yaml: unmarshal errors:\n" +
    "  line 18: field modules not found in type naming.Config\n" +
    "  line 40: field voltage_domains not found in type naming.Config\n" +
    "  line 54: field subsystems not found in type naming.Config";
  const DESIGN_YAML =
    "[invalid_argument] invalid argument: naming config: yaml: unmarshal errors:\n" +
    "  line 2: field entry not found in type naming.Config";

  // The bug this replaced: the bar rendered the fixed string "could not apply" and put the server's
  // message in a title attribute. The summary has to carry the part that identifies the file as the
  // wrong kind, because the picker offers no other clue which files are naming configs.
  it("names the offending field, not just that something failed", () => {
    expect(conventionError(INTENT_YAML)).toContain("field modules not found");
    expect(conventionError(DESIGN_YAML)).toContain("field entry not found");
  });

  // A YAML unmarshal error's first line is a header. Taking it alone would report "unmarshal
  // errors:" and say nothing about what was wrong, which is the same dead end as "could not apply".
  it("reaches past a header line to the first detail", () => {
    expect(conventionError(INTENT_YAML)).toBe(
      "naming config: yaml: unmarshal errors: line 18: field modules not found in type naming.Config",
    );
  });

  // Both status prefixes name the code rather than the problem, and the reader can see something
  // failed from the chip being on screen at all. Asserting on BOTH is the point: stripping only the
  // server's half left "[invalid_argument] " in front of every message in the real UI, which is what
  // the browser capture caught and the wire-only fixture did not.
  it("drops both status prefixes, Connect's and the server's", () => {
    const got = conventionError(EXPECT_YAML);
    expect(got.startsWith("naming config")).toBe(true);
    expect(got).not.toContain("[invalid_argument]");
    expect(got).not.toContain("invalid argument:");
  });

  it("passes a single-line message through unchanged", () => {
    expect(conventionError("[invalid_argument] invalid argument: GetNamingConvention needs a ref")).toBe(
      "GetNamingConvention needs a ref",
    );
  });

  // The bar renders this only when state.error is set, but an empty or whitespace-only message must
  // not produce a red chip with nothing in it.
  it("is empty for no error", () => {
    expect(conventionError("")).toBe("");
    expect(conventionError("   \n  ")).toBe("");
  });
});
