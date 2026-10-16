import { readFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";
import { describe, it, expect } from "vitest";
import { create } from "@bufbuild/protobuf";
import { QuerySetSchema, RunQueryResponseSchema } from "./gen/agni/v1/webapi/query_pb.js";
import { RELATION_KINDS, bindingsInUse, compareNatural, cellKind, drawnSheets, tabsFromSet, formatBinding, groupRelations, searchBindings, resultFromResponse, reasonMessage, searchPattern, LocateReason } from "./query.js";
import { resultFromWire } from "./testwire.js";

describe("resultFromResponse", () => {
  // agni issue 896: total survives only when the limit actually left rows out.
  it("keeps the answer's total when a limit cut it, and none when the answer is whole", () => {
    const resp = (total: number) => ({ columns: ["r"], columnKinds: [""], rows: [{ cells: ["R1"] }], total }) as never;
    expect(resultFromResponse(resp(1229)).total).toBe(1229);
    expect(resultFromResponse(resp(1)).total).toBe(0);
    expect(resultFromResponse(resp(0)).total).toBe(0);
  });

  it("carries column kinds and resolves each navigable cell's sheet badges via the resolver", () => {
    const resp = {
      columns: ["r", "n", "v"],
      columnKinds: ["component", "net", ""],
      rows: [
        {
          cells: ["R1", "SDA", "3.3"],
          cites: [],
          cellSheets: [{ sheetIds: ["s2"] }, { sheetIds: ["s1", "s2"] }, { sheetIds: [] }],
        },
      ],
    } as never;
    const result = resultFromWire(resp, (ids) => ids.map((id) => ({ id, name: id.toUpperCase() })));
    expect(result.columnKinds).toEqual(["component", "net", ""]);
    const row = result.rows[0];
    expect(row.cells).toEqual(["R1", "SDA", "3.3"]);
    expect(row.cellSheets[0]).toEqual([{ id: "s2", name: "S2" }]);
    expect(row.cellSheets[1]).toEqual([
      { id: "s1", name: "S1" },
      { id: "s2", name: "S2" },
    ]);
    expect(row.cellSheets[2]).toEqual([]); // a scalar cell resolves to no badges
  });

  it("reads locations and citations from the response's tables (agni 916)", () => {
    const resolve = (ids: string[]) => ids.map((id) => ({ id, name: id.toUpperCase() }));
    const perRow = {
      columns: ["r", "n"],
      columnKinds: ["component", "net"],
      rows: [
        { cells: ["R1", "SDA"], cites: ["a.edn"], cellSheets: [{ sheetIds: ["s2"] }, { sheetIds: ["s1", "s2"] }], cellReasons: [0, 0] },
        { cells: ["R2", "SDA"], cites: ["a.edn"], cellSheets: [{ sheetIds: [] }, { sheetIds: ["s1", "s2"] }], cellReasons: [LocateReason.NO_GEOMETRY, 0] },
      ],
    } as never;
    const tables = {
      columns: ["r", "n"],
      columnKinds: ["component", "net"],
      entities: [
        { kind: "component", ref: "R1", pin: "", sheetIds: ["s2"], reason: 0 },
        { kind: "net", ref: "SDA", pin: "", sheetIds: ["s1", "s2"], reason: 0 },
        { kind: "component", ref: "R2", pin: "", sheetIds: [], reason: LocateReason.NO_GEOMETRY },
      ],
      sources: ["a.edn"],
      rows: [
        { cells: ["R1", "SDA"], cites: [], citeIndex: [0], cellEntity: [1, 2] },
        { cells: ["R2", "SDA"], cites: [], citeIndex: [0], cellEntity: [3, 2] },
      ],
    } as never;
    expect(resultFromResponse(tables, resolve).rows).toEqual(resultFromWire(perRow, resolve).rows);
  });

  it("defaults to empty kinds and no badges when the response omits them", () => {
    const result = resultFromWire({ columns: ["r"], rows: [{ cells: ["R1"], cites: [] }] } as never);
    expect(result.columnKinds).toEqual([]);
    expect(result.rows[0].cellSheets).toEqual([[]]);
  });
});

describe("reasonMessage (WS7-042c bus)", () => {
  it("explains a bus that is not drawn", () => {
    const msg = reasonMessage(LocateReason.BUS_NOT_DRAWN, "bus", "DATA[7:0]");
    expect(msg).toContain("DATA[7:0]");
    expect(msg).toContain("no drawn wire");
  });
})

// A search result is the one answer set whose rows are not all the same shape, so its cells carry
// their own kinds (agni issue 338). resultFromResponse must keep them aligned with cells, including
// for the rows a server sent none for.
describe("resultFromResponse cell kinds", () => {
  it("carries a per-row cell kind alongside the cells", () => {
    const resp = {
      columns: ["name", "kind"],
      columnKinds: ["", ""],
      rows: [
        { cells: ["U1", "component"], cites: [], cellKinds: ["component", ""] },
        { cells: ["DATA[1:0]", "bus"], cites: [], cellKinds: ["bus", ""] },
      ],
    } as never;
    const result = resultFromWire(resp);
    expect(result.rows[0].cellKinds).toEqual(["component", ""]);
    expect(result.rows[1].cellKinds).toEqual(["bus", ""]);
  });

  it("pads to one entry per cell when the response omits them, so an ordinary query costs nothing", () => {
    const result = resultFromWire({
      columns: ["r", "n"],
      columnKinds: ["component", "net"],
      rows: [{ cells: ["R1", "SDA"], cites: [] }],
    } as never);
    expect(result.rows[0].cellKinds).toEqual(["", ""]);
  });
});

describe("cellKind", () => {
  const result = (columnKinds: string[], cellKinds: string[]) =>
    resultFromWire({
      columns: columnKinds.map((_, i) => `c${i}`),
      columnKinds,
      rows: [{ cells: cellKinds.map((_, i) => `v${i}`), cites: [], cellKinds }],
    } as never);

  it("prefers the row's own kind, which is the whole point of having one", () => {
    const r = result(["", ""], ["net", ""]);
    expect(cellKind(r, r.rows[0], 0)).toBe("net");
  });

  it("falls back to the column kind, so every existing query keeps working unchanged", () => {
    const r = result(["component", "net"], ["", ""]);
    expect(cellKind(r, r.rows[0], 0)).toBe("component");
    expect(cellKind(r, r.rows[0], 1)).toBe("net");
  });

  it("is empty for a scalar cell under either source", () => {
    const r = result(["", ""], ["", ""]);
    expect(cellKind(r, r.rows[0], 1)).toBe("");
  });
});

// A design is full of names the regex engine would otherwise read as syntax. A reader typing one of
// them means the characters, and a search that silently matched something else would be worse than
// one that found nothing.
describe("searchPattern", () => {
  it("leaves an ordinary name alone, so the query stays readable in the box", () => {
    expect(searchPattern("CAN")).toBe("CAN");
    expect(searchPattern("USB_D-")).toBe("USB_D-");
  });

  it("escapes the metacharacters real net and part names actually contain", () => {
    expect(searchPattern("VDD+")).toBe("VDD\\+");
    expect(searchPattern("DATA[7:0]")).toBe("DATA\\[7:0\\]");
    expect(searchPattern("VREF(A)")).toBe("VREF\\(A\\)");
    expect(searchPattern("3.3V")).toBe("3\\.3V");
  });

  // The pattern is bound as a value (agni issue 793), so a quote no longer ends a string literal and
  // a name carrying one can be searched for.
  it("keeps a double quote, since the pattern is bound rather than spliced into the query", () => {
    expect(searchPattern('A"B')).toBe('A"B');
  });
});

describe("searchBindings", () => {
  const search = { query: "entity(?name, ?kind), str.match(?name, ?pattern) => ?name, ?kind", teaches: "", bind: "pattern", pattern: "(?i){term}" };

  it("binds the served pattern with the escaped term in place of {term}", () => {
    expect(searchBindings(search, "CAN")).toEqual({ pattern: "(?i)CAN" });
  });

  it("escapes before filling, so a typed metacharacter cannot reach the regex as syntax", () => {
    expect(searchBindings(search, "VDD+")).toEqual({ pattern: "(?i)VDD\\+" });
  });

  // A string replacement reads $& as "the match", which would put {term} itself into the pattern.
  it("fills a term containing $& literally", () => {
    expect(searchBindings(search, "A$&B")).toEqual({ pattern: "(?i)A\\$&B" });
  });
});

describe("bindingsInUse", () => {
  it("keeps only the bindings whose variable the query still names", () => {
    const b = { ref: "U1", net: "GND" };
    expect(bindingsInUse("component.net(?ref, ?n) => ?n", b)).toEqual({ ref: "U1" });
    expect(bindingsInUse("component.net(?r, ?n) => ?n", b)).toEqual({});
  });

  it("matches the whole variable name, not a prefix of a longer one", () => {
    expect(bindingsInUse("component.net(?refdes, ?n)", { ref: "U1" })).toEqual({});
  });
});

describe("formatBinding", () => {
  it("spells the value as the query would write the constant", () => {
    expect(formatBinding("ref", "R1")).toBe('?ref = "R1"');
  });
  it("keeps a pattern's backslash as written, since a query string takes no escapes", () => {
    expect(formatBinding("p", "^U1\\.")).toBe('?p = "^U1\\."');
  });
  // agni issue 1045: one fixture with core/query's FormatBindings.
  const here = dirname(fileURLToPath(import.meta.url));
  const twin = JSON.parse(readFileSync(join(here, "..", "..", "core", "query", "testdata", "binding_twin.json"), "utf8")) as {
    cases: { name: string; text?: string; number?: number; want: string }[];
  };
  it("has binding cases to check", () => expect(twin.cases.length).toBeGreaterThanOrEqual(4));
  for (const c of twin.cases) {
    it(`spells as FormatBindings does: ${c.want}`, () => {
      const isNum = c.number !== undefined;
      expect(formatBinding(c.name, isNum ? String(c.number) : c.text!, isNum)).toBe(c.want);
    });
  }
});

describe("groupRelations", () => {
  it("groups the shipped library's derived members under Library, after the base tiers and before the predicates", () => {
    const item = (name: string, kind: string) => ({ name, kind, args: [], summary: "", detail: "" });
    const groups = groupRelations([
      item("str.contains", "predicate"),
      item("net.has_test_point", "derived"),
      item("component.net", "netlist"),
    ]);
    expect(groups.map((g) => g.label)).toEqual(["Netlist", "Library", "Predicates"]);
    expect(groups[1].items.map((r) => r.name)).toEqual(["net.has_test_point"]);
  });
});

// Properties reads where a selected part or net is drawn off the find-by-name answer for its whole
// name (agni issue 902).
describe("drawnSheets", () => {
  // A part and a net sharing a name, as a find-by-name answer types them, row by row.
  const resp = create(RunQueryResponseSchema, {
    columns: ["name", "kind"],
    columnKinds: ["", ""],
    rows: [
      { cells: ["VBUS", "component"], cellKinds: ["component", ""], cellEntity: [1, 0] },
      { cells: ["VBUS", "net"], cellKinds: ["net", ""], cellEntity: [2, 0] },
    ],
    entities: [
      { kind: "component", ref: "VBUS", sheetIds: ["s1"] },
      { kind: "net", ref: "VBUS", sheetIds: ["s2", "s3"] },
    ],
  });

  it("reads the sheets off the row of the kind asked about", () => {
    expect(drawnSheets(resp, "net", "VBUS")).toEqual(["s2", "s3"]);
    expect(drawnSheets(resp, "component", "VBUS")).toEqual(["s1"]);
  });

  it("is none when the answer does not name it", () => {
    expect(drawnSheets(resp, "net", "GND")).toEqual([]);
  });
});

// agni issue 745: a set opens as one tab per query.
describe("tabsFromSet", () => {
  it("names each tab for its query, joins the preamble to it, and keeps the set's order and bindings", () => {
    const set = create(QuerySetSchema, {
      title: "Audit",
      preamble: "res(?r) :- component.class(?r, \"resistor\");",
      queries: [
        { name: "Resistors", query: "res(?r) => ?r" },
        { name: "Busy nets", query: "net.pin_count(?n, ?c), ?c >= ?min => ?n", bindings: { min: { kind: { case: "number", value: 3 } }, n: { kind: { case: "text", value: "GND" } } } },
      ],
    });
    expect(tabsFromSet(set)).toEqual([
      { title: "Resistors", text: 'res(?r) :- component.class(?r, "resistor");\nres(?r) => ?r', bindings: {}, numbers: [] },
      { title: "Busy nets", text: 'res(?r) :- component.class(?r, "resistor");\nnet.pin_count(?n, ?c), ?c >= ?min => ?n', bindings: { min: "3", n: "GND" }, numbers: ["min"] },
    ]);
  });

  it("holds the query alone when the set has no preamble", () => {
    const set = create(QuerySetSchema, { queries: [{ name: "Rails", query: " net.rail(?n) => ?n\n" }] });
    expect(tabsFromSet(set)[0].text).toBe("net.rail(?n) => ?n");
  });
});

// agni issue 1045: the picker's groups are the catalog's kinds, held to the fixture core/facts reads.
describe("RELATION_KINDS, twinned with facts.KindOrder via kind_order.json", () => {
  it("names every kind the catalog serves, in its order", () => {
    const here = dirname(fileURLToPath(import.meta.url));
    const twin = JSON.parse(readFileSync(join(here, "..", "..", "core", "facts", "testdata", "kind_order.json"), "utf8")) as { kinds: string[] };
    expect(twin.kinds.length).toBeGreaterThan(4);
    expect(RELATION_KINDS.map((k) => k.key)).toEqual(twin.kinds);
  });
});

describe("compareNatural, twinned with service via natural_twin.json", () => {
  const here = dirname(fileURLToPath(import.meta.url));
  const twin = JSON.parse(readFileSync(join(here, "..", "..", "service", "testdata", "natural_twin.json"), "utf8")) as { lists: string[][] };
  it("has lists to check", () => expect(twin.lists.length).toBeGreaterThanOrEqual(5));
  for (const want of twin.lists) {
    it(`orders ${want.join(" < ")}`, () => {
      expect([...want].reverse().sort(compareNatural)).toEqual(want);
    });
  }
});
