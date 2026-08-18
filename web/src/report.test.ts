import { describe, it, expect } from "vitest";
import { reportFromWire, type WireCheckReport } from "./report.js";

const wire: WireCheckReport = {
  source: "conformance/dup_refdes.fires.kicad_sch",
  rulesRun: 7,
  sections: [
    {
      severity: "error",
      count: 2,
      rules: [
        {
          rule: "duplicate-ref-des",
          summary: "two components share a ref des",
          findings: [
            { rule: "duplicate-ref-des", severity: "error", message: "R1 used twice", subject: { kind: "component", ref: "R1", pin: "" } },
            { rule: "duplicate-ref-des", severity: "error", message: "R1 used twice", subject: { kind: "component", ref: "R1'", pin: "" } },
          ],
        },
      ],
    },
    {
      severity: "info",
      count: 1,
      rules: [
        {
          rule: "single-pin-net",
          summary: "",
          findings: [{ rule: "single-pin-net", severity: "info", message: "net N1 has one pin", subject: { kind: "net", ref: "N1", pin: "" } }],
        },
      ],
    },
  ],
};

describe("reportFromWire", () => {
  it("preserves the server's section order, counts, and group summaries verbatim", () => {
    const r = reportFromWire(wire, () => "");
    expect(r.source).toBe(wire.source);
    expect(r.rulesRun).toBe(7);
    expect(r.sections.map((s) => [s.severity, s.count])).toEqual([
      ["error", 2],
      ["info", 1],
    ]);
    expect(r.sections[0].rules[0].summary).toBe("two components share a ref des");
    expect(r.sections[1].rules[0].summary).toBe("");
  });

  it("converts findings to the checks panel's FindingItem shape with the category lookup", () => {
    const r = reportFromWire(wire, (rule) => (rule === "single-pin-net" ? "connectivity" : "identity"));
    const dup = r.sections[0].rules[0].findings[0];
    expect(dup).toEqual({
      rule: "duplicate-ref-des",
      category: "identity",
      profile: "", // the report groups by rule, not interface (WS9-041)
      severity: "error",
      kind: "component",
      subject: "R1",
      pin: "",
      netId: "",
      busId: "",
      message: "R1 used twice",
      inconclusive: false,
      // A stored report written before the context field existed carries none, and must map to an
      // empty list rather than undefined so the panel renders no chips instead of throwing.
      context: [],
      sheets: [],
      locateReason: 0,
    });
    expect(r.sections[1].rules[0].findings[0].category).toBe("connectivity");
  });

  it("maps a missing structured subject to empty kind/subject/pin instead of crashing", () => {
    const r = reportFromWire(
      { source: "x", rulesRun: 1, sections: [{ severity: "info", count: 1, rules: [{ rule: "r", summary: "", findings: [{ rule: "r", severity: "info", message: "m" }] }] }] },
      () => "",
    );
    expect(r.sections[0].rules[0].findings[0]).toMatchObject({ kind: "", subject: "", pin: "" });
  });
});
