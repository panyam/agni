import { describe, it, expect } from "vitest";
import {
  type FindingItem,
  groupFindings,
  subjectsToSpecs,
  findingSpec,
  collapseInstances,
  findingKey,
  focusStack,
  sortFindings,
  severityRank,
  severitySections,
} from "./findings.js";
import { reportFromWire } from "./report.js";

function f(over: Partial<FindingItem>): FindingItem {
  return { rule: "r", category: "connectivity", profile: "", severity: "info", kind: "net", subject: "N", pin: "", netId: "", busId: "", message: "m", sheets: [], locateReason: 0, ...over };
}

const findings: FindingItem[] = [
  f({ rule: "i2c-pull-up", category: "connectivity", severity: "error", kind: "net", subject: "SCL" }),
  f({ rule: "single-pin-net", category: "connectivity", severity: "info", kind: "net", subject: "STUB" }),
  f({ rule: "unconnected-component", category: "connectivity", severity: "warning", kind: "component", subject: "C1" }),
  f({ rule: "diff-pair-naming", category: "naming", severity: "warning", kind: "net", subject: "TX_P" }),
];

describe("groupFindings", () => {
  it("by rule keeps one bucket per rule in first-appearance order", () => {
    expect(groupFindings(findings, "rule").map(([v]) => v)).toEqual([
      "i2c-pull-up",
      "single-pin-net",
      "unconnected-component",
      "diff-pair-naming",
    ]);
  });
  it("by category collapses rules sharing a category", () => {
    const g = groupFindings(findings, "category");
    expect(g.map(([v]) => v)).toEqual(["connectivity", "naming"]);
    expect(g[0][1]).toHaveLength(3);
  });
  it("by severity groups by level", () => {
    expect(groupFindings(findings, "severity").map(([v]) => v)).toEqual(["error", "info", "warning"]);
  });
  it("by kind (entity) separates net from component", () => {
    const g = groupFindings(findings, "kind");
    expect(g.map(([v]) => v)).toEqual(["net", "component"]);
    expect(g[0][1].map((x) => x.subject)).toEqual(["SCL", "STUB", "TX_P"]);
  });
  it("by interface (profile) buckets a rule's findings under its profile, non-profile rules under \"\" (WS9-041)", () => {
    const withProfiles: FindingItem[] = [
      f({ rule: "spi_nor-signal-missing", profile: "SPI_NOR", subject: "SPI_CS" }),
      f({ rule: "spi_nor-missing-pullup", profile: "SPI_NOR", subject: "SPI_CS" }),
      f({ rule: "emmc-signal-missing", profile: "eMMC", subject: "EMMC_CLK" }),
      f({ rule: "single-pin-net", profile: "", subject: "STUB" }),
    ];
    const g = groupFindings(withProfiles, "profile");
    expect(g.map(([v]) => v)).toEqual(["SPI_NOR", "eMMC", ""]);
    expect(g[0][1]).toHaveLength(2); // both SPI_NOR findings collapse together
  });
});

describe("subjectsToSpecs", () => {
  it("buckets subjects by kind into one spec", () => {
    const specs = subjectsToSpecs(findings);
    expect(specs).toHaveLength(1);
    expect(new Set(specs[0].nets)).toEqual(new Set(["SCL", "STUB", "TX_P"]));
    expect(specs[0].components).toEqual(["C1"]);
    expect(specs[0].pins).toBeUndefined();
  });
  it("dedupes repeated subjects and keeps pins as PinTargets", () => {
    const specs = subjectsToSpecs([
      f({ kind: "net", subject: "GND" }),
      f({ kind: "net", subject: "GND" }),
      f({ kind: "pin", subject: "U1", pin: "5" }),
      f({ kind: "pin", subject: "U1", pin: "5" }),
    ]);
    expect(specs[0].nets).toEqual(["GND"]);
    expect(specs[0].pins).toEqual([{ refDes: "U1", pin: "5" }]);
  });
  it("returns no spec when there is nothing to highlight", () => {
    expect(subjectsToSpecs([])).toEqual([]);
    expect(subjectsToSpecs([f({ subject: "" })])).toEqual([]);
  });
  it("buckets a bus subject by bus id, never as a net/component (WS7-042b)", () => {
    const specs = subjectsToSpecs([f({ kind: "bus", subject: "DATA[7:0]", busId: "bus-1" })]);
    expect(specs[0].busIds).toEqual(["bus-1"]);
    expect(specs[0].nets).toBeUndefined();
    expect(specs[0].components).toBeUndefined();
  });
  it("ignores a bus with no id (an undrawable bus_alias; its note is WS7-042c)", () => {
    expect(subjectsToSpecs([f({ kind: "bus", subject: "DATA" })])).toEqual([]);
  });
});

describe("findingSpec", () => {
  it("lights up a single finding exactly by kind", () => {
    expect(findingSpec(f({ kind: "component", subject: "R9" }))).toEqual([{ components: ["R9"] }]);
    expect(findingSpec(f({ kind: "net", subject: "SDA" }))).toEqual([{ nets: ["SDA"] }]);
  });
});

describe("collapseInstances", () => {
  it("folds findings identical in (rule, subject, pin, severity, message) into one row", () => {
    const dup = Array.from({ length: 7 }, () => f({ rule: "duplicate-net-name", subject: "&1V8_ETH", severity: "warning", message: "7 nets" }));
    const collapsed = collapseInstances(dup);
    expect(collapsed).toHaveLength(1);
    expect(collapsed[0].instances).toHaveLength(7);
    expect(collapsed[0].head.subject).toBe("&1V8_ETH");
  });

  it("does not fold when subject, message, or severity differ", () => {
    const items = [
      f({ subject: "A", message: "m" }),
      f({ subject: "B", message: "m" }), // different subject
      f({ subject: "A", message: "n" }), // different message
      f({ subject: "A", message: "m", severity: "error" }), // different severity
    ];
    expect(collapseInstances(items)).toHaveLength(4);
  });

  it("folds rows differing only in sheets (sheets are not part of the identity)", () => {
    const items = [
      f({ subject: "N", sheets: [{ id: "s1", name: "Root" }] }),
      f({ subject: "N", sheets: [{ id: "s2", name: "Power" }] }),
    ];
    const collapsed = collapseInstances(items);
    expect(collapsed).toHaveLength(1);
    expect(collapsed[0].instances.map((i) => i.sheets[0].id)).toEqual(["s1", "s2"]);
  });

  it("preserves first-appearance order across collapsed groups", () => {
    const items = [f({ subject: "A" }), f({ subject: "B" }), f({ subject: "A" })];
    expect(collapseInstances(items).map((c) => c.head.subject)).toEqual(["A", "B"]);
  });
});

describe("findingKey", () => {
  it("is equal for two findings that should collapse and differs when a field changes", () => {
    const a = f({ rule: "r", subject: "N", pin: "", severity: "info", message: "m" });
    expect(findingKey(a)).toBe(findingKey(f({ ...a, sheets: [{ id: "x", name: "X" }] })));
    expect(findingKey(a)).not.toBe(findingKey(f({ ...a, message: "n" })));
  });
});

describe("sortFindings", () => {
  it("sorts by severity worst-first, tiebreaking on rule then subject", () => {
    // C1(warn, unconnected-component), TX_P(warn, diff-pair-naming): same severity -> rule breaks the tie.
    expect(sortFindings(findings, "severity", 1).map((x) => x.subject)).toEqual(["SCL", "TX_P", "C1", "STUB"]);
  });
  it("reverses the whole order when dir is -1", () => {
    expect(sortFindings(findings, "severity", -1).map((x) => x.subject)).toEqual(["STUB", "C1", "TX_P", "SCL"]);
  });
  it("sorts by subject and by rule", () => {
    expect(sortFindings(findings, "subject", 1).map((x) => x.subject)).toEqual(["C1", "SCL", "STUB", "TX_P"]);
    expect(sortFindings(findings, "rule", 1).map((x) => x.rule)).toEqual(["diff-pair-naming", "i2c-pull-up", "single-pin-net", "unconnected-component"]);
  });
  it("does not mutate the input", () => {
    const before = findings.map((x) => x.subject);
    sortFindings(findings, "subject", 1);
    expect(findings.map((x) => x.subject)).toEqual(before);
  });
});

describe("severityRank", () => {
  it("orders error < warning < info < unknown", () => {
    expect([severityRank("error"), severityRank("warning"), severityRank("info"), severityRank("bogus")]).toEqual([0, 1, 2, 3]);
  });
});

describe("per-instance net id (WS9)", () => {
  it("subjectsToSpecs buckets a net with a netId by id, not name, so instances are distinct targets", () => {
    const specs = subjectsToSpecs([
      f({ kind: "net", subject: "PWR_A", netId: "aaa" }),
      f({ kind: "net", subject: "PWR_A", netId: "bbb" }),
      f({ kind: "net", subject: "GND", netId: "" }), // no id -> falls back to name
    ]);
    expect(specs).toHaveLength(1);
    expect(new Set(specs[0].netIds)).toEqual(new Set(["aaa", "bbb"]));
    expect(specs[0].nets).toEqual(["GND"]);
  });

  it("findingSpec targets one instance by id", () => {
    expect(findingSpec(f({ kind: "net", subject: "PWR_A", netId: "aaa" }))).toEqual([{ netIds: ["aaa"] }]);
  });

  it("collapse folds same-key findings that differ only in netId, keeping each instance addressable", () => {
    const items = [f({ subject: "PWR_A", netId: "aaa" }), f({ subject: "PWR_A", netId: "bbb" })];
    const collapsed = collapseInstances(items);
    expect(collapsed).toHaveLength(1);
    expect(collapsed[0].instances.map((i) => i.netId)).toEqual(["aaa", "bbb"]);
  });

  it("focusStack drops only the focused instance from the base, keeping same-named siblings lit", () => {
    const all = [f({ kind: "net", subject: "PWR_A", netId: "aaa" }), f({ kind: "net", subject: "PWR_A", netId: "bbb" })];
    const stack = focusStack(all, "net", "PWR_A", [{ netIds: ["aaa"] }], "aaa");
    // Base is the sibling (bbb) only; the focus layer for aaa sits on top.
    expect(stack).toEqual([{ netIds: ["bbb"] }, { netIds: ["aaa"] }]);
  });
});

describe("severitySections parity with the server report (WS3-022)", () => {
  // The client's severity rollup must land on the same worst-first order and per-severity counts the
  // server report (GetCheckReport) emits, so the merged panel's severity grouping cannot drift from the
  // CLI report. This pins severitySections against reportFromWire over the SAME finding set.
  const mixed: FindingItem[] = [
    f({ severity: "info", subject: "I1" }),
    f({ severity: "error", subject: "E1" }),
    f({ severity: "warning", subject: "W1" }),
    f({ severity: "info", subject: "I2" }),
    f({ severity: "error", subject: "E2" }),
    f({ severity: "info", subject: "I3" }),
  ];

  it("matches the report's section order and counts", () => {
    const client = severitySections(mixed).map((s) => [s.severity, s.count]);
    // The server emits sections worst-first with per-severity counts; reportFromWire preserves them.
    const wire = {
      source: "x",
      rulesRun: 1,
      sections: [
        { severity: "error", count: 2, rules: [] },
        { severity: "warning", count: 1, rules: [] },
        { severity: "info", count: 3, rules: [] },
      ],
    };
    const report = reportFromWire(wire, () => "").sections.map((s) => [s.severity, s.count]);
    expect(client).toEqual(report);
  });
});
