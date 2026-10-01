import { describe, it, expect } from "vitest";
import { bindingsInUse, cellKind, formatBinding, groupRelations, searchBindings, resultFromResponse, reasonMessage, searchPattern, LocateReason } from "./query.js";

describe("resultFromResponse", () => {
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
    const result = resultFromResponse(resp, (ids) => ids.map((id) => ({ id, name: id.toUpperCase() })));
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

  it("defaults to empty kinds and no badges when the response omits them", () => {
    const result = resultFromResponse({ columns: ["r"], rows: [{ cells: ["R1"], cites: [] }] } as never);
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
    const result = resultFromResponse(resp);
    expect(result.rows[0].cellKinds).toEqual(["component", ""]);
    expect(result.rows[1].cellKinds).toEqual(["bus", ""]);
  });

  it("pads to one entry per cell when the response omits them, so an ordinary query costs nothing", () => {
    const result = resultFromResponse({
      columns: ["r", "n"],
      columnKinds: ["component", "net"],
      rows: [{ cells: ["R1", "SDA"], cites: [] }],
    } as never);
    expect(result.rows[0].cellKinds).toEqual(["", ""]);
  });
});

describe("cellKind", () => {
  const result = (columnKinds: string[], cellKinds: string[]) =>
    resultFromResponse({
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
    expect(formatBinding("ref", 'R"1')).toBe('?ref = "R\\"1"');
  });
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
