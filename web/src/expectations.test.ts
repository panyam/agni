import { describe, it, expect } from "vitest";
import { reconcile, expectationSpecs, expectationCaption, type RuleExpectationItem } from "./expectations.js";
import type { FindingItem } from "./findings.js";

function finding(rule: string, subject: string, kind = "net"): FindingItem {
  return { rule, category: "", profile: "", severity: "error", kind, subject, pin: "", netId: "", busId: "", message: "", inconclusive: false, sheets: [], locateReason: 0 };
}

describe("reconcile", () => {
  it("covers the status matrix", () => {
    const exp: RuleExpectationItem[] = [
      { rule: "matched-rule", subjects: ["A", "B"], pending: false },
      { rule: "partial-rule", subjects: ["A", "B"], pending: false },
      { rule: "missing-rule", subjects: ["A"], pending: false },
      { rule: "staged-rule", subjects: ["X"], pending: true },
    ];
    const findings = [
      finding("matched-rule", "B"),
      finding("matched-rule", "A"), // order-independent
      finding("partial-rule", "A"), // B missing -> partial
      finding("surprise-rule", "Z"), // not expected -> unexpected
    ];
    const byRule = Object.fromEntries(reconcile(exp, findings).map((r) => [r.rule, r]));

    expect(byRule["matched-rule"].status).toBe("matched");
    expect(byRule["partial-rule"].status).toBe("partial");
    expect(byRule["missing-rule"].status).toBe("missing"); // expected, nothing fired
    expect(byRule["staged-rule"].status).toBe("pending");
    expect(byRule["surprise-rule"].status).toBe("unexpected");
    expect(byRule["surprise-rule"].actual).toEqual(["Z"]);
    expect(byRule["missing-rule"].actual).toEqual([]);
  });

  it("carries the sidecar why note through to the row; unexpected rows have none", () => {
    const rows = reconcile(
      [{ rule: "r", subjects: ["A"], pending: false, why: "A has no cap; B is the control" }],
      [finding("surprise", "Z")],
    );
    const byRule = Object.fromEntries(rows.map((r) => [r.rule, r]));
    expect(byRule["r"].why).toBe("A has no cap; B is the control");
    expect(byRule["surprise"].why).toBe("");
  });

  it("stamps each row with the rule's catalog summary so a missing row reads as an assertion", () => {
    const rows = reconcile(
      [{ rule: "missing-rule", subjects: ["A"], pending: false }],
      [finding("surprise", "Z")],
      (rule) => (rule === "missing-rule" ? "nets must have caps" : rule === "surprise" ? "no floating stubs" : ""),
    );
    const byRule = Object.fromEntries(rows.map((r) => [r.rule, r]));
    expect(byRule["missing-rule"].summary).toBe("nets must have caps");
    // Unexpected rows are catalog rules too, so their summary resolves the same way.
    expect(byRule["surprise"].summary).toBe("no floating stubs");
  });

  it("joins each fired subject's sheet badges into the row (WS9-024); expected-only subjects get none", () => {
    const fired = { ...finding("r", "A"), sheets: [{ id: "s2", name: "Power" }] };
    const rows = reconcile([{ rule: "r", subjects: ["A", "B"], pending: false }], [fired]);
    expect(rows[0].subjectSheets).toEqual({ A: [{ id: "s2", name: "Power" }] });
  });

  it("a pending rule that now fires stays pending but shows its actual subjects", () => {
    const rows = reconcile([{ rule: "r", subjects: ["A"], pending: true }], [finding("r", "A")]);
    expect(rows[0].status).toBe("pending");
    expect(rows[0].actual).toEqual(["A"]);
  });

  it("keeps expectation order, unexpected rows sorted last", () => {
    const exp: RuleExpectationItem[] = [
      { rule: "b-rule", subjects: [], pending: false },
      { rule: "a-rule", subjects: [], pending: false },
    ];
    const rows = reconcile(exp, [finding("z-extra", "1"), finding("m-extra", "2")]);
    expect(rows.map((r) => r.rule)).toEqual(["b-rule", "a-rule", "m-extra", "z-extra"]);
  });
});

describe("expectationSpecs (the anchored overlay, WS9-045)", () => {
  it("colors fired subjects by reconcile status and buckets by kind", () => {
    const exp: RuleExpectationItem[] = [
      { rule: "cap-voltage", subjects: ["C1"], pending: false },
      { rule: "missing-rule", subjects: ["N1"], pending: false }, // silent -> caption, no overlay
    ];
    const findings = [finding("cap-voltage", "C1", "component"), finding("surprise", "SDA", "net")];
    const rows = reconcile(exp, findings);
    const specs = expectationSpecs(rows, findings);
    // matched (green) component C1; unexpected (amber) net SDA; missing has no finding -> no spec.
    const green = specs.find((s) => s.color === "#22c55e");
    const amber = specs.find((s) => s.color === "#f59e0b");
    expect(green?.components).toEqual(["C1"]);
    expect(amber?.nets).toEqual(["SDA"]);
    // A subject with no firing (the missing N1) never appears in the overlay.
    expect(specs.some((s) => (s.nets ?? []).includes("N1") || (s.components ?? []).includes("N1"))).toBe(false);
  });
});

describe("expectationCaption (the non-anchored verdict, WS9-045)", () => {
  it("passes when actual == expected exactly with no unexpected", () => {
    const rows = reconcile([{ rule: "r", subjects: ["A"], pending: false }], [finding("r", "A")]);
    const c = expectationCaption(rows);
    expect(c).toMatchObject({ pass: true, expected: 1, matched: 1, unexpected: 0, missing: [], silent: false });
  });

  it("fires:{} (no expectations) is a silent pass, or fails if anything fired", () => {
    expect(expectationCaption(reconcile([], []))).toMatchObject({ silent: true, pass: true, unexpected: 0 });
    const fired = expectationCaption(reconcile([], [finding("noise", "X")]));
    expect(fired).toMatchObject({ silent: true, pass: false, unexpected: 1 });
  });

  it("reports missing rules and excludes pending from the verdict", () => {
    const rows = reconcile(
      [
        { rule: "gone", subjects: ["A"], pending: false }, // expected, silent -> missing
        { rule: "staged", subjects: ["B"], pending: true }, // pending -> not counted
      ],
      [],
    );
    const c = expectationCaption(rows);
    expect(c.missing).toEqual(["gone"]);
    expect(c.pass).toBe(false);
    expect(c.expected).toBe(1); // pending excluded
  });
});
