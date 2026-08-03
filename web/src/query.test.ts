import { describe, it, expect } from "vitest";
import { resultFromResponse, reasonMessage, LocateReason } from "./query.js";

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
