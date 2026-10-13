import { describe, it, expect } from "vitest";
import { create } from "@bufbuild/protobuf";
import { TableSchema, TableEncoding, type EncodeTablesRequest, type TabulateRequest } from "./gen/agni/v1/webapi/tables_pb.js";
import { RunQueryResponseSchema } from "./gen/agni/v1/webapi/query_pb.js";
import { type SavedTable, type TableAnswer, type TableClient, type TablesState, TablesPresenter, nextOrder } from "./tables.js";

// The tests here cover agni issues 898 and 956: the panel shows Tabulate's tables for an answer the
// page holds, orders them by asking Tabulate again, and saves through Encode.

const table = (name: string, rows: string[][]) =>
  create(TableSchema, { name, columns: [{ name: "net" }], rows: rows.map((cells) => ({ cells })) });

const answer: TableAnswer = { case: "query", value: create(RunQueryResponseSchema, { columns: ["net"] }) };

// harness's has is null for a source with no answer, since undefined would take the default.
function harness(has: TableAnswer | null = answer) {
  const tabulated: Partial<TabulateRequest>[] = [];
  const encoded: Partial<EncodeTablesRequest>[] = [];
  const client = {
    tabulate: async (req: Partial<TabulateRequest>) => {
      tabulated.push(req);
      return { tables: [table("findings", [["GND"], ["VCC"]]), table("verdicts", [["GND"]])] };
    },
    encode: async (req: Partial<EncodeTablesRequest>) => {
      encoded.push(req);
      return { files: (req.tables ?? []).map((t) => ({ name: t.name, filename: `${req.stem}.${t.name}.csv`, contentType: "text/csv", content: new Uint8Array([1]) })) };
    },
  } as unknown as TableClient;
  let state: TablesState | undefined;
  const saved: SavedTable[] = [];
  const p = new TablesPresenter(
    client,
    { setState: (s) => (state = s) },
    { answer: async () => ({ answer: has ?? undefined, label: "Query", stem: "gateway", reason: "Run a query first." }) },
    (f) => saved.push(f),
  );
  return { p, tabulated, encoded, saved, state: () => state! };
}

describe("TablesPresenter", () => {
  it("shows the tables Tabulate projects from the source's answer", async () => {
    const h = harness();
    await h.p.show("query");
    expect(h.tabulated).toHaveLength(1);
    expect(h.tabulated[0].answer).toBe(answer);
    expect(h.state().tables.map((t) => t.name)).toEqual(["findings", "verdicts"]);
    expect(h.state().active).toBe(0);
    expect(h.state().error).toBe("");
  });

  it("says why when the source has no answer yet, and asks Tabulate nothing", async () => {
    const h = harness(null);
    await h.p.show("query");
    expect(h.tabulated).toHaveLength(0);
    expect(h.state().tables).toEqual([]);
    expect(h.state().error).toBe("Run a query first.");
  });

  it("orders by asking Tabulate again, ascending and then descending", async () => {
    const h = harness();
    await h.p.show("query");
    await h.p.sortBy("net");
    await h.p.sortBy("net");
    expect(h.tabulated.slice(1).map((r) => r.orderBy)).toEqual([["net"], ["-net"]]);
    expect(h.tabulated.slice(1).every((r) => r.answer === answer)).toBe(true);
    expect(h.state().orderBy).toBe("-net");
  });

  it("saves the table on screen through Encode as csv, named for the design", async () => {
    const h = harness();
    await h.p.show("query");
    h.p.select(1);
    await h.p.saveActive();
    expect(h.encoded).toHaveLength(1);
    expect(h.encoded[0].encoding).toBe(TableEncoding.CSV);
    expect(h.encoded[0].stem).toBe("gateway");
    expect(h.encoded[0].tables?.map((t) => t.name)).toEqual(["verdicts"]);
    expect(h.saved.map((f) => f.filename)).toEqual(["gateway.verdicts.csv"]);
  });

  it("saves only the named tables of an answer", async () => {
    const h = harness();
    expect(await h.p.saveAnswer(answer, "gateway", ["findings"])).toBe("");
    expect(h.encoded[0].tables?.map((t) => t.name)).toEqual(["findings"]);
  });

  it("clears a comparison's tables when the comparison closes, and keeps others", async () => {
    const h = harness();
    await h.p.show("compare");
    h.p.setCompareOpen(false);
    expect(h.state().tables).toEqual([]);
    expect(h.state().source).toBe("");
    await h.p.show("query");
    h.p.setCompareOpen(false);
    expect(h.state().tables).toHaveLength(2);
  });
});

describe("nextOrder", () => {
  it("goes ascending, then descending, then ascending on another column", () => {
    expect(nextOrder("", "net")).toBe("net");
    expect(nextOrder("net", "net")).toBe("-net");
    expect(nextOrder("-net", "net")).toBe("net");
    expect(nextOrder("net", "rule")).toBe("rule");
  });
});
