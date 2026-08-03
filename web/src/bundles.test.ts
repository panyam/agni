import { describe, it, expect } from "vitest";
import { type RuleItem } from "./rules.js";
import { type Bundle, BUILTIN_BUNDLES, resolveBundle, upsertBundle, removeBundle, allBundles } from "./bundles.js";

function rule(name: string, tags: Record<string, string>, available = true): RuleItem {
  return { name, severity: "info", summary: "", impact: "", detail: "", reads: [], tags, available, unavailableReason: "" };
}

const catalog: RuleItem[] = [
  rule("single-pin-net", { category: "connectivity", distribution: "open" }),
  rule("unconnected-component", { category: "connectivity", distribution: "open" }),
  rule("i2c-pull-up", { category: "connectivity", distribution: "public-reference" }),
  rule("cap-voltage", { category: "datasheet", distribution: "public-reference" }, false),
];

describe("resolveBundle", () => {
  it("a filter bundle selects the matching, available rules", () => {
    const open = BUILTIN_BUNDLES.find((b) => b.name === "Open rules")!;
    expect(resolveBundle(catalog, open)).toEqual(["single-pin-net", "unconnected-component"]);
  });

  it("the topology-baseline filter selects every available rule (excludes unavailable)", () => {
    const base = BUILTIN_BUNDLES.find((b) => b.name === "Topology baseline")!;
    expect(resolveBundle(catalog, base)).toEqual(["single-pin-net", "unconnected-component", "i2c-pull-up"]);
    expect(resolveBundle(catalog, base)).not.toContain("cap-voltage"); // unavailable
  });

  it("an explicit-list bundle selects its listed rules, dropping unknown or unavailable ones", () => {
    const b: Bundle = { name: "mine", rules: ["i2c-pull-up", "cap-voltage", "ghost"] };
    expect(resolveBundle(catalog, b)).toEqual(["i2c-pull-up"]); // cap-voltage unavailable, ghost absent
  });
});

describe("saved-bundle list", () => {
  it("upsert adds then replaces by name; remove drops by name", () => {
    let saved: Bundle[] = [];
    saved = upsertBundle(saved, { name: "a", rules: ["x"] });
    saved = upsertBundle(saved, { name: "b", rules: ["y"] });
    expect(saved.map((b) => b.name)).toEqual(["a", "b"]);
    saved = upsertBundle(saved, { name: "a", rules: ["z"] }); // replace
    expect(saved.find((b) => b.name === "a")!.rules).toEqual(["z"]);
    expect(saved).toHaveLength(2);
    saved = removeBundle(saved, "a");
    expect(saved.map((b) => b.name)).toEqual(["b"]);
  });

  it("allBundles lists built-ins before saved", () => {
    const names = allBundles([{ name: "custom", rules: [] }]).map((b) => b.name);
    expect(names.slice(0, BUILTIN_BUNDLES.length)).toEqual(BUILTIN_BUNDLES.map((b) => b.name));
    expect(names[names.length - 1]).toBe("custom");
  });
});
