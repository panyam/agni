import { describe, it, expect } from "vitest";
import { clone, create } from "@bufbuild/protobuf";
import { ClassSource, DesignSchema, InputDiagnosticsSchema, Role } from "./gen/agni/v1/ir/ir_pb.js";
import { PartSpecSchema } from "./gen/agni/v1/param/param_pb.js";
import { ConversionReportSchema } from "./gen/agni/v1/webapi/design_pb.js";
import type { FindingItem, FindingsState, VerdictItem } from "./findings.js";
import { type PropertiesInput, type ServedFacts, inventoryFromDesign, propertiesFor, reportLines } from "./properties.js";
import type { EntityQueryItem } from "./query.js";
import type { Selection } from "./selection.js";

// A barrel jack J1 feeding a regulator U1, with a test point on the output, which is enough to
// reach every level and every group the panel renders.
const design = create(DesignSchema, {
  libraries: [
    {
      name: "lib",
      parts: [
        { name: "JACK", pins: [{ designator: "1", name: "VIN" }, { designator: "2", name: "GND" }] },
        { name: "LDO", pins: [{ designator: "1", name: "IN" }, { designator: "2", name: "GND" }, { designator: "10", name: "OUT" }] },
        { name: "TP", pins: [{ designator: "1", name: "1" }] },
      ],
    },
  ],
  components: [
    {
      refDes: "J1",
      mpn: "PJ-002A",
      sections: [{ libraryRef: "lib", partRef: "JACK" }],
      deviceClasses: [{ class: "connector", source: ClassSource.CONVENTION }],
      attributes: { footprint: "BarrelJack" },
    },
    {
      refDes: "U1",
      mpn: "LM1117",
      value: { input: "3.3V" },
      sections: [{ libraryRef: "lib", partRef: "LDO" }],
      deviceClasses: [{ class: "regulator", source: ClassSource.DATASHEET }],
    },
    { refDes: "TP1", sections: [{ libraryRef: "lib", partRef: "TP" }], deviceClasses: [{ class: "test_point", source: ClassSource.CONVENTION }] },
  ],
  nets: [
    { name: "VIN", roles: [{ roleKind: Role.RAIL }], connections: [{ componentRef: "J1", pinRef: "1" }, { componentRef: "U1", pinRef: "1" }] },
    {
      name: "GND",
      roles: [{ roleKind: Role.GROUND }],
      connections: [{ componentRef: "J1", pinRef: "2" }, { componentRef: "U1", pinRef: "2" }],
    },
    { name: "3V3", roles: [{ roleKind: Role.RAIL }], connections: [{ componentRef: "U1", pinRef: "10" }, { componentRef: "TP1", pinRef: "1" }] },
  ],
  // A bus the reader recorded with two members, one of which the netlist names.
  inputDiagnostics: { unmodeledBuses: [{ label: "D[1:0]", kind: "bus_alias", members: ["VIN", "D0"] }] },
});

// SERVED is what the engine answers SERVED_QUERIES with on that design: pin.name off each part type,
// a name that repeats the designator included, and the one test point.
const SERVED: ServedFacts = {
  pinNames: [
    ["J1", "1", "VIN"],
    ["J1", "2", "GND"],
    ["U1", "1", "IN"],
    ["U1", "2", "GND"],
    ["U1", "10", "OUT"],
    ["TP1", "1", "1"],
  ],
  testPoints: ["TP1"],
};

const QUESTIONS: EntityQueryItem[] = [
  { kind: "component", label: "What nets does it touch?", query: "q1", teaches: "", binds: ["ref"] },
  { kind: "component", label: "Which test points reach its nets?", query: "q2", teaches: "", binds: ["ref"] },
  { kind: "net", label: "What pins are on it?", query: "q3", teaches: "", binds: ["net"] },
];

function finding(over: Partial<FindingItem>): FindingItem {
  return {
    rule: "r",
    category: "",
    profile: "",
    severity: "error",
    kind: "net",
    subject: "",
    pin: "",
    netId: "",
    busId: "",
    message: "",
    inconclusive: false,
    sheets: [],
    ...over,
  } as FindingItem;
}

// ran is a completed run of three rules, which is what lets a count read as a count.
function ran(findings: FindingItem[]): FindingsState {
  return { findings, verdicts: [], focusedVerdict: "", selected: "", ruleCount: 3, pending: 0, running: false, catalogLoading: false, skipped: [], ruleSummaries: {}, saveNote: "" };
}

function input(over: Partial<PropertiesInput>): PropertiesInput {
  return {
    focus: { level: "design" },
    designName: "gateway",
    summary: "gateway.edn · faithful",
    sheets: [{ id: "s1", name: "Power" }],
    report: null,
    inventory: inventoryFromDesign(design, SERVED),
    findings: null,
    datasheet: null,
    drawn: null,
    questions: QUESTIONS,
    ...over,
  };
}

const group = (s: ReturnType<typeof propertiesFor>, name: string) => s.groups.find((g) => g.name === name);
const row = (s: ReturnType<typeof propertiesFor>, label: string) => s.rows.find((r) => r.label === label)?.value;

describe("inventoryFromDesign", () => {
  it("lists a component's pins in datasheet order with their functional names and nets", () => {
    const inv = inventoryFromDesign(design, SERVED);
    // "10" after "2", which a string sort gets wrong.
    expect(inv.pinsOf.get("U1")).toEqual([
      { pin: "1", name: "IN", net: "VIN" },
      { pin: "2", name: "GND", net: "GND" },
      { pin: "10", name: "OUT", net: "3V3" },
    ]);
  });

  it("keeps no functional name that only repeats the designator", () => {
    expect(inventoryFromDesign(design, SERVED).pinsOf.get("TP1")).toEqual([{ pin: "1", name: "", net: "3V3" }]);
  });

  // agni issue 1045: the names and test points are the engine's, so with no answer from it the
  // panel shows none rather than reading the part types and class tags itself.
  it("works out no pin name or test point from the IR itself", () => {
    const inv = inventoryFromDesign(design);
    expect(inv.pinsOf.get("U1")?.map((p) => p.name)).toEqual(["", "", ""]);
    expect(inv.testPoints.size).toBe(0);
  });
});

describe("propertiesFor", () => {
  it("describes the design, with a findings count that says the checks have not run", () => {
    const s = propertiesFor(input({}));
    expect(s.crumbs.map((c) => c.label)).toEqual(["gateway"]);
    expect(row(s, "components")).toBe("3");
    expect(row(s, "nets")).toBe("3");
    expect(row(s, "findings")).toBe("not checked yet");
    expect(s.ask).toBeUndefined();
    // A flat read lists no blocks it left out, rather than an empty group.
    expect(group(s, "not extracted")).toBeUndefined();
  });

  it("lists the hierarchical blocks the read left out, largest first, with what each holds", () => {
    // agni issue 717: the counts above cover the top level alone, so the design level names what
    // they leave out.
    const hier = clone(DesignSchema, design);
    hier.inputDiagnostics = create(InputDiagnosticsSchema, {
      unexpandedHierarchy: [
        { name: "SUB", kind: "edif_cell", instanceCount: 1 },
        { name: "PSU", kind: "edif_cell", instanceCount: 212 },
      ],
    });
    const s = propertiesFor(input({ inventory: inventoryFromDesign(hier, SERVED) }));
    expect(group(s, "not extracted")?.refs).toEqual([
      { label: "PSU", detail: "212 instances" },
      { label: "SUB", detail: "1 instance" },
    ]);
  });

  it("counts findings only once the run is complete, and keeps unresolved apart", () => {
    const s = propertiesFor(input({ findings: ran([finding({}), finding({ inconclusive: true })]) }));
    expect(row(s, "findings")).toBe("1 finding, 1 unresolved");
  });

  it("describes a sheet from what its drawing holds", () => {
    const s = propertiesFor(input({ focus: { level: "sheet", sheetId: "s1" }, drawn: { refs: ["U1", "J1"], nets: ["VIN"] } }));
    expect(s.crumbs.map((c) => c.label)).toEqual(["gateway", "Power"]);
    expect(group(s, "components")?.refs.map((r) => r.label)).toEqual(["J1", "U1"]);
    expect(group(s, "nets")?.refs[0].target).toEqual({ kind: "net", net: "VIN" });
  });

  it("describes a component with its pins, nets, classes and evidence tiers, and offers Ask", () => {
    const s = propertiesFor(input({ focus: { level: "entity", sel: { kind: "component", ref: "U1" }, sheetId: "s1" } }));
    expect(s.crumbs.map((c) => c.label)).toEqual(["gateway", "Power", "U1"]);
    expect(row(s, "mpn")).toBe("LM1117");
    expect(row(s, "value")).toBe("3.3V");
    expect(row(s, "classes")).toBe("regulator (datasheet)");
    expect(group(s, "pins")?.refs.map((r) => `${r.label} ${r.detail}`)).toEqual(["1 IN → VIN", "2 GND → GND", "10 OUT → 3V3"]);
    expect(group(s, "nets")?.refs.map((r) => r.label)).toEqual(["3V3", "GND", "VIN"]);
    expect(s.ask?.sel).toEqual({ kind: "component", ref: "U1" });
  });

  it("offers every served question for the selection's kind, in the server's order, and none for a kind with none", () => {
    const comp = propertiesFor(input({ focus: { level: "entity", sel: { kind: "component", ref: "U1" } } }));
    expect(comp.ask?.questions.map((q) => q.label)).toEqual(["What nets does it touch?", "Which test points reach its nets?"]);
    expect(propertiesFor(input({ focus: { level: "entity", sel: { kind: "pin", ref: "U1", pin: "1" } } })).ask).toBeUndefined();
  });

  it("lists the sheets a part is drawn on as sheet references, and says when that is not known yet", () => {
    const at = (drawnOn: string[] | null | undefined) =>
      group(propertiesFor(input({ focus: { level: "entity", sel: { kind: "component", ref: "U1" } }, drawnOn })), "sheets");
    expect(at(["s1"])?.refs).toEqual([{ label: "Power", sheet: "s1" }]);
    expect(at(null)?.empty).toBe("locating…");
    expect(at([])?.empty).toBe("drawn on no sheet");
    // A host with no engine to ask lists no sheets group at all, rather than one stuck locating.
    expect(at(undefined)).toBeUndefined();
  });

  it("lists a bus's members, walking to the ones the netlist names", () => {
    const s = propertiesFor(input({ focus: { level: "entity", sel: { kind: "bus", busId: "D[1:0]" } } }));
    expect(row(s, "drawn as")).toBe("bus_alias");
    expect(group(s, "members")?.refs).toEqual([
      { label: "D0", detail: "no net by this name" },
      { label: "VIN", target: { kind: "net", net: "VIN" } },
    ]);
    expect(s.note).toBeUndefined();
  });

  it("says when the read records no bus by the name the drawing gave", () => {
    const s = propertiesFor(input({ focus: { level: "entity", sel: { kind: "bus", busId: "ADDR[7:0]" } } }));
    expect(group(s, "members")?.refs).toEqual([]);
    expect(s.note).toContain("records no bus by this name");
  });

  it("lists a net's pins, components and test points", () => {
    const s = propertiesFor(input({ focus: { level: "entity", sel: { kind: "net", net: "3V3" } } }));
    expect(row(s, "roles")).toBe("rail");
    expect(group(s, "pins")?.refs.map((r) => r.label)).toEqual(["TP1.1", "U1.10"]);
    expect(group(s, "test points")?.refs.map((r) => r.label)).toEqual(["TP1"]);
  });

  it("gives a pin its functional name and puts its component in the breadcrumb", () => {
    const s = propertiesFor(input({ focus: { level: "entity", sel: { kind: "pin", ref: "U1", pin: "10" }, sheetId: "s1" } }));
    expect(s.crumbs.map((c) => c.label)).toEqual(["gateway", "Power", "U1", "pin 10"]);
    expect(row(s, "name")).toBe("OUT");
    expect(row(s, "net")).toBe("3V3");
  });

  it("shows only the findings about the selection, and says when none were looked for", () => {
    const f = [finding({ kind: "net", subject: "VIN", rule: "bulk-cap" }), finding({ kind: "net", subject: "GND", rule: "other" })];
    const sel = { level: "entity" as const, sel: { kind: "net" as const, net: "VIN" } };
    expect(group(propertiesFor(input({ focus: sel, findings: ran(f) })), "findings")?.refs.map((r) => r.label)).toEqual(["bulk-cap"]);
    expect(group(propertiesFor(input({ focus: sel })), "findings")?.empty).toBe("not checked yet");
  });

  it("keeps the datasheet's outcomes apart: no corpus, no part number, no spec, and a spec", () => {
    const at = (ref: string, corpusConnected: boolean, parts = [] as { refDes: string; mpn: string; spec: ReturnType<typeof create<typeof PartSpecSchema>> | undefined }[]) =>
      propertiesFor(input({ focus: { level: "entity", sel: { kind: "component", ref } }, datasheet: { corpusConnected, parts } })).datasheet;
    expect(at("U1", false)?.kind).toBe("no-corpus");
    expect(at("TP1", true)?.kind).toBe("no-mpn");
    expect(at("U1", true)).toEqual({ kind: "no-spec", mpn: "LM1117" });
    const spec = create(PartSpecSchema, { mpn: "LM1117", parameters: [{ name: "VIN abs max" }] });
    expect(at("U1", true, [{ refDes: "U1", mpn: "LM1117", spec }])?.kind).toBe("spec");
  });

  it("says when the netlist has no such entity rather than showing an empty part", () => {
    const s = propertiesFor(input({ focus: { level: "entity", sel: { kind: "component", ref: "ZZ9" } } }));
    expect(s.note).toContain("no component by that name");
  });

  it("says the design is still being read before the inventory arrives", () => {
    const s = propertiesFor(input({ inventory: null, focus: { level: "entity", sel: { kind: "net", net: "VIN" } } }));
    expect(s.note).toBe("Reading the design…");
  });
});

describe("reportLines", () => {
  it("counts glyphs by class and calls out unmapped and unresolved parts", () => {
    const r = create(ConversionReportSchema, {
      components: [
        { refDes: "R1", kind: "glyph", deviceClass: "resistor", label: "resistor" },
        { refDes: "U1", kind: "box", label: "box" },
        { refDes: "U2", kind: "unresolved", label: "unresolved" },
      ],
    });
    expect(reportLines(r)).toEqual({ counts: "box 1 · resistor 1 · unresolved 1", box: ["U1"], unresolved: ["U2"] });
  });

  it("is nothing for a faithful drawing", () => {
    expect(reportLines(null)).toBeNull();
  });
});

// agni issue 897: a selected net, part or pin shows the selected rules' answer about it alone, which
// the presenter asks CheckDesign for.
describe("a selection's own checks", () => {
  const verdict = (rule: string, outcome: string, statement = ""): VerdictItem => ({
    id: `${rule}:(net:VIN)`,
    rule,
    outcome,
    subjects: [{ kind: "net", subject: "VIN", pin: "", netId: "", busId: "" }],
    statement,
    terms: [],
    context: [],
    reason: "",
    sheets: [],
  });
  const net = { level: "entity" as const, sel: { kind: "net" as const, net: "VIN" } };

  it("lists the failing and undecided verdicts worst first, each showing its proof, and counts every outcome", () => {
    const s = propertiesFor(
      input({
        focus: net,
        checks: {
          state: "answered",
          verdicts: [verdict("b-rule", "pass"), verdict("z-rule", "inconclusive", "no datasheet"), verdict("a-rule", "fail", "no pull-up"), verdict("c-rule", "not-considered")],
          notAsked: [],
        },
      }),
    );
    expect(group(s, "checks")?.refs).toEqual([
      { label: "a-rule", detail: "no pull-up", verdict: "a-rule:(net:VIN)" },
      { label: "z-rule", detail: "no datasheet", verdict: "z-rule:(net:VIN)" },
    ]);
    expect(row(s, "checks")).toBe("1 fail, 1 inconclusive, 1 pass, 1 not considered");
    // The run's findings group gives way to it, rather than listing the same answers twice.
    expect(group(s, "findings")).toBeUndefined();
  });

  it("names the rules it did not ask, with the engine's reason, so their absence does not read as a pass", () => {
    const why = "concludes about the design as a whole, so it is not asked of a net or part";
    const s = propertiesFor(
      input({ focus: net, checks: { state: "answered", verdicts: [verdict("a-rule", "pass")], notAsked: [{ name: "symbol-unresolved", reason: why }, { name: "bus-not-modeled", reason: why }] } }),
    );
    expect(row(s, "not asked")).toBe(`bus-not-modeled, symbol-unresolved: ${why}`);
    expect(group(s, "checks")?.empty).toBe("nothing failing");
  });

  it("says when it is asking, when no rule is selected, when it could not ask, and when nothing considers the selection", () => {
    const say = (checks: Parameters<typeof input>[0]["checks"]) => {
      const s = propertiesFor(input({ focus: net, checks }));
      return [row(s, "checks"), group(s, "checks")?.empty];
    };
    expect(say({ state: "asking" })).toEqual(["checking…", "checking…"]);
    expect(say({ state: "no-rules" })).toEqual(["no rules selected", "no rules selected"]);
    expect(say({ state: "failed", error: "no netlist" })).toEqual(["could not check: no netlist", "could not check: no netlist"]);
    expect(say({ state: "answered", verdicts: [], notAsked: [] })[0]).toBe("no selected rule considers it");
  });

  it("falls back to the run's findings where nothing asks", () => {
    const s = propertiesFor(input({ focus: net }));
    expect(group(s, "findings")).toBeDefined();
    expect(group(s, "checks")).toBeUndefined();
    expect(row(s, "checks")).toBeUndefined();
  });
});

describe("the selected entity", () => {
  it("names a selected part to the panel, and nothing at the design level", () => {
    const sel = { kind: "component" as const, ref: "U1" };
    expect(propertiesFor(input({ focus: { level: "entity", sel } })).sel).toEqual(sel);
    expect(propertiesFor(input({})).sel).toBeUndefined();
  });
});

// agni issue 904: an area of U1 and TP1. 3V3 runs U1 to TP1 and is wholly inside; VIN and GND reach
// J1, which is not, so they are the block's interface.
describe("an area", () => {
  const area: Selection = { kind: "area", refs: ["U1", "TP1"], pins: [], wires: [], nets: ["3V3", "GND", "VIN"] };

  it("splits its nets into boundary and internal, and counts its parts by class", () => {
    const s = propertiesFor(input({ focus: { level: "entity", sel: area, sheetId: "s1" } }));
    expect(s.kind).toBe("area");
    expect(s.title).toBe("Area (2)");
    expect(group(s, "boundary nets")?.refs.map((r) => [r.label, r.detail])).toEqual([
      ["GND", "1 of 2 pins inside"],
      ["VIN", "1 of 2 pins inside"],
    ]);
    expect(group(s, "internal nets")?.refs.map((r) => r.label)).toEqual(["3V3"]);
    expect(group(s, "components")?.refs.map((r) => r.label)).toEqual(["TP1", "U1"]);
    expect(row(s, "parts")).toBe("2: 1 regulator, 1 test_point");
    expect(row(s, "nets")).toBe("2 boundary, 1 internal");
  });

  it("reads Design › Sheet › Area, and a member reached from it keeps the area as its parent", () => {
    const s = propertiesFor(input({ focus: { level: "entity", sel: area, sheetId: "s1" } }));
    expect(s.crumbs.map((c) => c.label)).toEqual(["gateway", "Power", "Area (2)"]);
    expect(s.within).toEqual(area);
    const member = propertiesFor(input({ focus: { level: "entity", sel: { kind: "component", ref: "U1" }, sheetId: "s1", within: area } }));
    expect(member.crumbs.map((c) => c.label)).toEqual(["gateway", "Power", "Area (2)", "U1"]);
    expect(member.crumbs[2].focus).toEqual({ level: "entity", sel: area, sheetId: "s1" });
  });

  it("lists the findings on its members and none on a part outside it", () => {
    const s = propertiesFor(
      input({ focus: { level: "entity", sel: area }, findings: ran([finding({ kind: "component", subject: "U1", rule: "on-u1" }), finding({ kind: "component", subject: "J1", rule: "on-j1" })]) }),
    );
    expect(group(s, "findings")?.refs.map((r) => r.label)).toEqual(["on-u1"]);
  });

  it("asks the area's own questions, with the whole set to bind", () => {
    const q: EntityQueryItem = { kind: "area", label: "What does this block connect to?", query: "q", teaches: "", binds: ["ref"] };
    const s = propertiesFor(input({ focus: { level: "entity", sel: area }, questions: [...QUESTIONS, q] }));
    expect(s.ask?.questions).toEqual([q]);
    expect(s.ask?.sel).toEqual(area);
  });
});
