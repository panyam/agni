// Test fixtures for query answers, written the readable way: each row with its own cites,
// cellSheets and cellReasons. wire() normalizes one into the shape the server sends (agni issue
// 916), with each entity and citation kept once on the response and rows pointing into them, so a
// test states what a row shows rather than how the wire factors it. Used only by tests.
import type { RunQueryResponse } from "./gen/agni/v1/webapi/query_pb.js";
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
