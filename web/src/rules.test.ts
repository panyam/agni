import { describe, it, expect } from "vitest";
import {
  type RuleItem,
  tagKeys,
  groupBy,
  filterRules,
  groupSelectState,
  defaultSelection,
  toggleRule,
  toggleGroup,
} from "./rules.js";

function rule(name: string, tags: Record<string, string>, available = true): RuleItem {
  return { name, severity: "info", summary: `${name} summary`, impact: "", detail: "", reads: [], tags, available, unavailableReason: available ? "" : "needs layer" };
}

const catalog: RuleItem[] = [
  rule("single-pin-net", { category: "connectivity", tier: "P" }),
  rule("unconnected-component", { category: "connectivity", tier: "R" }),
  rule("diff-pair-naming", { category: "naming", tier: "R" }),
  rule("cap-voltage", { category: "datasheet", tier: "X" }, false),
];

describe("tagKeys", () => {
  it("returns the union of tag keys, sorted", () => {
    expect(tagKeys(catalog)).toEqual(["category", "tier"]);
  });
});

describe("groupBy", () => {
  it("buckets by a tag value in first-appearance order", () => {
    const groups = groupBy(catalog, "category");
    expect(groups.map(([v]) => v)).toEqual(["connectivity", "naming", "datasheet"]);
    expect(groups[0][1].map((r) => r.name)).toEqual(["single-pin-net", "unconnected-component"]);
  });
  it("puts rules missing the key under an empty-string bucket", () => {
    const groups = groupBy([rule("x", {})], "category");
    expect(groups).toHaveLength(1);
    expect(groups[0][0]).toBe("");
  });
});

describe("filterRules", () => {
  const empty = { tagValues: {}, availableOnly: false, search: "" };
  it("empty filter returns everything", () => {
    expect(filterRules(catalog, empty)).toHaveLength(catalog.length);
  });
  it("narrows by a tag value", () => {
    const got = filterRules(catalog, { ...empty, tagValues: { category: ["connectivity"] } });
    expect(got.map((r) => r.name)).toEqual(["single-pin-net", "unconnected-component"]);
  });
  it("intersects across keys, unions within a key", () => {
    const got = filterRules(catalog, { ...empty, tagValues: { category: ["connectivity", "naming"], tier: ["R"] } });
    expect(got.map((r) => r.name)).toEqual(["unconnected-component", "diff-pair-naming"]);
  });
  it("availableOnly drops unavailable rules", () => {
    expect(filterRules(catalog, { ...empty, availableOnly: true }).some((r) => r.name === "cap-voltage")).toBe(false);
  });
  it("search matches name or summary, case-insensitively", () => {
    expect(filterRules(catalog, { ...empty, search: "PAIR" }).map((r) => r.name)).toEqual(["diff-pair-naming"]);
  });
});

describe("selection helpers", () => {
  it("defaultSelection is every available rule", () => {
    expect(defaultSelection(catalog)).toEqual(["single-pin-net", "unconnected-component", "diff-pair-naming"]);
  });

  it("groupSelectState is all/some/none over selectable rules", () => {
    const conn = catalog.filter((r) => r.tags.category === "connectivity");
    expect(groupSelectState(conn, new Set(["single-pin-net", "unconnected-component"]))).toBe("all");
    expect(groupSelectState(conn, new Set(["single-pin-net"]))).toBe("some");
    expect(groupSelectState(conn, new Set())).toBe("none");
    // A group whose only rule is unavailable has nothing selectable -> none.
    expect(groupSelectState([catalog[3]], new Set())).toBe("none");
  });

  it("toggleRule adds/removes, but is a no-op for an unavailable rule", () => {
    expect(toggleRule(catalog[0], [])).toEqual(["single-pin-net"]);
    expect(toggleRule(catalog[0], ["single-pin-net"])).toEqual([]);
    expect(toggleRule(catalog[3], [])).toEqual([]); // cap-voltage unavailable
  });

  it("toggleGroup selects all selectable when any is off, else clears them", () => {
    const conn = catalog.filter((r) => r.tags.category === "connectivity");
    expect(new Set(toggleGroup(conn, []))).toEqual(new Set(["single-pin-net", "unconnected-component"]));
    expect(toggleGroup(conn, ["single-pin-net", "unconnected-component"])).toEqual([]);
    // Partial -> fills to all selectable.
    expect(new Set(toggleGroup(conn, ["single-pin-net"]))).toEqual(new Set(["single-pin-net", "unconnected-component"]));
  });
});
