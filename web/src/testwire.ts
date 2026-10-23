// Test fixtures for query answers, written the readable way: each row with its own cites,
// cellSheets and cellReasons. wire() normalizes one into the shape the server sends (agni issue
// 916), with each entity and citation kept once on the response and rows pointing into them, so a
// test states what a row shows rather than how the wire factors it. Used only by tests.
import { create } from "@bufbuild/protobuf";
import { RunQueryResponseSchema, type RunQueryResponse } from "./gen/agni/v1/webapi/query_pb.js";
import type { SheetBadge } from "./findings.js";
import { resultFromResponse, type QueryResult } from "./query.js";

interface FixtureRow {
  cells: string[];
  cites?: string[];
  cellSheets?: { sheetIds?: string[] }[];
  cellReasons?: number[];
  cellKinds?: string[];
  cellRefs?: string[];
}

interface Fixture {
  columns: string[];
  columnKinds?: string[];
  rows: FixtureRow[];
  [k: string]: unknown;
}

export function wire(f: Fixture): RunQueryResponse {
  const entities: { kind: string; ref: string; pin: string; sheetIds: string[]; reason: number }[] = [];
  const entityAt = new Map<string, number>();
  const sources: string[] = [];
  const sourceAt = new Map<string, number>();
  const rows = f.rows.map((r) => {
    const cellEntity = r.cells.map((cell, i) => {
      const kind = r.cellKinds?.[i] || f.columnKinds?.[i] || "";
      if (!kind) return 0;
      const ref = kind === "pin" ? (r.cellRefs?.[i] ?? "") : cell;
      const pin = kind === "pin" ? cell : "";
      const key = `${kind}\u0000${ref}\u0000${pin}`;
      let n = entityAt.get(key);
      if (n === undefined) {
        entities.push({ kind, ref, pin, sheetIds: r.cellSheets?.[i]?.sheetIds ?? [], reason: r.cellReasons?.[i] ?? 0 });
        n = entities.length;
        entityAt.set(key, n);
      }
      return n;
    });
    const citeIndex = (r.cites ?? []).map((c) => {
      let n = sourceAt.get(c);
      if (n === undefined) {
        sources.push(c);
        n = sources.length - 1;
        sourceAt.set(c, n);
      }
      return n;
    });
    return { cells: r.cells, cellKinds: r.cellKinds ?? [], cellRefs: r.cellRefs ?? [], cellEntity, citeIndex };
  });
  return { ...f, rows, entities, sources } as never;
}

// resultFromWire is resultFromResponse over a fixture written row by row.
export function resultFromWire(f: unknown, resolveSheets?: (ids: string[]) => SheetBadge[]): QueryResult {
  return resultFromResponse(wire(f as Fixture), resolveSheets);
}

// probedOneAnswer is RunQuery's answer to `component.probed_one(?r, ?p, ?u) => ?r, ?u` asked with
// witnesses (agni issue 794): one row, R1 with SENSE unprobed, witnessed by the library member's rule
// with its body facts, the test point's net fact cited, and the `not` that held as a negated leaf.
// R1 and SENSE are entities 1 and 2, VBUS 3, TP1 4, U1's pin 3 entity 5 and U1 6. The pin.net leaf
// under the test point is there to carry a pin value, not because the member reads it.
export function probedOneAnswer(): RunQueryResponse {
  return create(RunQueryResponseSchema, {
    columns: ["r", "u"],
    columnKinds: ["component", "net"],
    entities: [
      { kind: "component", ref: "R1", sheetIds: ["s1"] },
      { kind: "net", ref: "SENSE", sheetIds: ["s1"] },
      { kind: "net", ref: "VBUS", sheetIds: ["s2"] },
      { kind: "component", ref: "TP1", sheetIds: ["s2"] },
      { kind: "pin", ref: "U1", pin: "3", sheetIds: [] },
      { kind: "component", ref: "U1", sheetIds: ["s1"] },
    ],
    sources: ["x.edn:VBUS", "x.edn"],
    rows: [
      {
        cells: ["R1", "SENSE"],
        cellEntity: [1, 2],
        citeIndex: [0, 1],
        witnesses: [
          {
            relation: "component.probed_one",
            values: ["R1", "VBUS", "SENSE"],
            valueEntity: [1, 3, 2],
            rule: "component.probed_one(?r, ?probed, ?unprobed) :- component.two_terminal(?r, ?probed, ?unprobed), net.has_test_point(?probed), not net.has_test_point(?unprobed)",
            children: [
              { relation: "component.two_terminal", values: ["R1", "SENSE", "VBUS"], valueEntity: [1, 2, 3], citeIndex: [1] },
              {
                relation: "net.has_test_point",
                values: ["VBUS"],
                valueEntity: [3],
                rule: 'net.has_test_point(?n: net) :- component.net(?tp, ?n), component.class(?tp, "test_point")',
                children: [
                  { relation: "component.net", values: ["TP1", "VBUS"], valueEntity: [4, 3], citeIndex: [0] },
                  { relation: "pin.net", values: ["U1", "3", "SENSE"], valueEntity: [6, 5, 2], citeIndex: [0] },
                ],
              },
              { relation: "net.has_test_point", values: ["SENSE"], valueEntity: [2], negated: true },
            ],
          },
        ],
      },
    ],
  });
}
