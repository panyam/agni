import { create } from "@bufbuild/protobuf";
import { describe, expect, it } from "vitest";

import { NOT_COMPARED, readAgreement } from "./agreement.js";
import { DiffDesignsResponseSchema } from "./gen/agni/v1/webapi/diff_pb.js";

// The engine's answer for a netlist and its board that disagree in each way the panel lists.
const disagreeing = create(DiffDesignsResponseSchema, {
  report: {
    componentsRemoved: ["U1"],
    componentsAdded: ["H1"],
    nets: [
      { kind: "deleted", name: "EN" },
      { kind: "new", name: "Net-(C1-Pad1)" },
      { kind: "hard", name: "OUT", removed: ["C1.1"] },
      { kind: "renamed", name: "Net-(R1-Pad1)", oldName: "VIN" },
    ],
  },
  views: {
    compared: true,
    parts: { matched: 3310, schematic: 3320, board: 3321 },
    pins: { matched: 6, schematic: 7, board: 6 },
    nets: { matched: 1290, schematic: 1300, board: 1301 },
    schematicOnlyPins: ["C1.3"],
  },
});

describe("readAgreement", () => {
  it("counts each kind against the schematic", () => {
    const a = readAgreement(disagreeing);
    expect(a.summary).toBe("3,310/3,320 parts · 1,290/1,300 nets");
    expect(a.lines).toEqual(["3,310 of 3,320 parts on the board", "6 of 7 pins on a net on the board", "1,290 of 1,300 nets matched on the board"]);
  });

  it("lists what did not match, each to select in the view that draws it", () => {
    const got = readAgreement(disagreeing).sections.map((s) => [s.title, s.entries.map((e) => [e.label, e.view, e.sel.ref ?? e.sel.net, e.sel.pin ?? ""])]);
    expect(got).toEqual([
      ["Parts only on the schematic", [["U1", "schematic", "U1", ""]]],
      ["Parts only on the board", [["H1", "board", "H1", ""]]],
      ["Nets only on the schematic", [["EN", "schematic", "EN", ""]]],
      ["Nets only on the board", [["Net-(C1-Pad1)", "board", "Net-(C1-Pad1)", ""]]],
      ["Nets whose pins differ", [["OUT: the board lacks C1.1", "schematic", "OUT", ""]]],
      // The board draws a renamed net under the schematic's name, so a click selects that name.
      ["Nets named differently", [["VIN is Net-(R1-Pad1) on the board", "schematic", "VIN", ""]]],
      ["Pins on a net only on the schematic", [["C1.3", "schematic", "C1", "3"]]],
    ]);
  });

  it("says not compared, with the reason, and never a count", () => {
    const a = readAgreement(create(DiffDesignsResponseSchema, { views: { compared: false, notCompared: "the design has no board to compare its schematic with" } }));
    expect(a.compared).toBe(false);
    expect(a.summary).toBe(NOT_COMPARED);
    expect(a.lines).toEqual(["Not compared: the design has no board to compare its schematic with."]);
    expect(a.sections).toEqual([]);
  });

  it("lists nothing when the views agree", () => {
    const a = readAgreement(create(DiffDesignsResponseSchema, { views: { compared: true, parts: { matched: 2, schematic: 2, board: 2 } } }));
    expect(a.sections).toEqual([]);
    expect(a.summary).toBe("2/2 parts · 0/0 nets");
  });
});
