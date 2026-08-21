import { describe, expect, it } from "vitest";
import { outcomeWord, verdictProofStack, type VerdictItem } from "./findings.js";
import { emptyLocation, locationToUrl, parseUrl } from "./router.js";

function verdict(over: Partial<VerdictItem> = {}): VerdictItem {
  return {
    id: "i2c-pull-up:(net:SDA)",
    rule: "i2c-pull-up",
    outcome: "pass",
    subjects: [{ kind: "net", subject: "SDA", pin: "", netId: "82ddd812ce0e" }],
    statement: "SDA reaches rail +3V3 through R1",
    terms: [],
    context: [
      { kind: "component", subject: "R1", pin: "", role: "pull-up" },
      { kind: "net", subject: "+3V3", pin: "", role: "rail" },
    ],
    reason: "",
    ...over,
  };
}

describe("verdictProofStack", () => {
  // The proof's ground is the verdict's OWN entities. The findings path uses the whole findings list
  // as ground, which answers a different question ("where does this sit among the problems"), and
  // using it here would bury a two-part proof under forty unrelated subjects.
  it("draws the proof's entities as ground, not the findings list", () => {
    const specs = verdictProofStack(verdict(), [{ nets: ["SDA"] }]);
    const ground = specs.slice(0, -1);
    expect(ground.flatMap((s) => s.components ?? [])).toEqual(["R1"]);
    expect(ground.flatMap((s) => s.nets ?? [])).toEqual(["+3V3"]);
  });

  it("puts the focused subject in the last layer, so it paints over its own proof", () => {
    const specs = verdictProofStack(verdict(), [{ nets: ["SDA"] }]);
    expect(specs[specs.length - 1].nets).toEqual(["SDA"]);
  });

  // A multi-hop pull-up is the case the ordering matters for: the reader follows the path.
  it("keeps a multi-hop path in the order the walk found it", () => {
    const v = verdict({
      context: [
        { kind: "component", subject: "R1", pin: "", role: "pull-up" },
        { kind: "net", subject: "SCL_ISO", pin: "", role: "segment" },
        { kind: "component", subject: "R2", pin: "", role: "pull-up" },
        { kind: "net", subject: "+3V3", pin: "", role: "rail" },
      ],
    });
    const ground = verdictProofStack(v, [{ nets: ["SCL"] }]).slice(0, -1);
    expect(ground.flatMap((s) => s.components ?? [])).toEqual(["R1", "R2"]);
    expect(ground.flatMap((s) => s.nets ?? [])).toEqual(["SCL_ISO", "+3V3"]);
  });

  // A fail has nothing to point at: the search found no resistor and no rail. The subject must still
  // light up, or clicking a failing row would appear to do nothing.
  it("still draws the subject when the proof names no entities", () => {
    const v = verdict({ outcome: "fail", subjects: [{ kind: "net", subject: "SCL", pin: "" }], context: [] });
    const specs = verdictProofStack(v, [{ nets: ["SCL"] }]);
    expect(specs.length).toBeGreaterThan(0);
    expect(specs[specs.length - 1].nets).toEqual(["SCL"]);
  });
});

describe("outcomeWord", () => {
  it("decodes every outcome the wire can carry", () => {
    expect([1, 2, 3, 4, 5].map(outcomeWord)).toEqual([
      "pass",
      "fail",
      "no-limit",
      "not-considered",
      "inconclusive",
    ]);
  });

  // A blank outcome would read as "nothing to report" about a subject the rule did look at, which is
  // the silence this whole layer exists to remove.
  it("never renders a blank", () => {
    for (const o of [undefined, 0, 99]) expect(outcomeWord(o)).toBe("unspecified");
  });
});

describe("verdict in the URL", () => {
  it("round-trips through the address bar", () => {
    const loc = { ...emptyLocation(), mount: "m", path: "d/board.edn", verdict: "i2c-pull-up:(net:SDA)" };
    const url = locationToUrl(loc);
    expect(url).toContain("verdict=");
    const back = parseUrl(url.split("?")[0], url.split("?")[1] ?? "");
    expect(back.verdict).toBe("i2c-pull-up:(net:SDA)");
  });

  // The id's colons must survive encoding, since every id has two of them and a link that arrives
  // truncated at the first colon resolves to nothing.
  it("survives a colon-bearing id", () => {
    const loc = { ...emptyLocation(), mount: "m", path: "d/b.edn", verdict: "symbol-unresolved:(symbol:Library:Symbol)" };
    const url = locationToUrl(loc);
    const back = parseUrl(url.split("?")[0], url.split("?")[1] ?? "");
    expect(back.verdict).toBe("symbol-unresolved:(symbol:Library:Symbol)");
  });

  it("carries no verdict when none is focused", () => {
    const url = locationToUrl({ ...emptyLocation(), mount: "m", path: "d/b.edn" });
    expect(url).not.toContain("verdict=");
  });
});
