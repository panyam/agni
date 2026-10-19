// The public demo exercise's report logic (web/browser/exercise-report.ts), tested here because
// the unit run reads src/ and its config is held identical to the workbench's (hack/fixture_copies.txt).
import { describe, expect, it } from "vitest";
import { breakdownDetail, checksBreakdown, classify, exitCode, leaks, loadDetail, loadSplit, markersFrom, pendingFiles, slowestFetch, sumSplits, table, type Row } from "../browser/exercise-report.js";

const row = (status: Row["status"]): Row => ({ step: "s", design: "d", status, detail: "" });

describe("the public demo exercise's report", () => {
  it("fails the run on a failed step and never on a missing one", () => {
    expect(exitCode([row("ok"), row("not available")])).toBe(0);
    expect(exitCode([row("ok"), row("fail"), row("not available")])).toBe(1);
  });

  it("splits requests into analysis calls, design files and page assets", () => {
    const prefix = "/agni/demo/";
    expect(classify("/agni.v1.webapi.CheckService/CheckDesign", prefix)).toBe("api");
    expect(classify("/agni/demo/raw/gateway/designs/gateway/gateway.edn", prefix)).toBe("file");
    expect(classify("/agni/demo/files/gateway.json", prefix)).toBe("file");
    expect(classify("/raw/tut/designs/gateway/gateway.edn", prefix)).toBe("file");
    expect(classify("/agni/demo/static/agni.wasm", prefix)).toBe("asset");
  });

  it("flags a request carrying a dropped design's name in its address or its body", () => {
    const markers = ["PMIC_MAIN_12V0", "CAN1_CANH"];
    const found = leaks(
      [
        { url: "http://x/agni/demo/static/agni.wasm", method: "GET", body: "" },
        { url: "http://x/collect?n=PMIC%5FMAIN%5F12V0", method: "GET", body: "" },
        { url: "http://x/agni.v1.webapi.QueryService/RunQuery", method: "POST", body: '{"q":"CAN1_CANH"}' },
      ],
      markers,
    );
    expect(found).toHaveLength(2);
    expect(found[0]).toContain("PMIC_MAIN_12V0");
    expect(found[1]).toContain("CAN1_CANH");
  });

  it("takes markers from EDIF nets and KiCad labels, skipping short names", () => {
    const edif = "(net PMIC_MAIN_12V0 (joined (net GND (joined (net (rename CAN1_CANH \"CAN1+\") (joined";
    expect(markersFrom(edif)).toEqual(["PMIC_MAIN_12V0", "CAN1_CANH"]);
    const kicad = '(label "I2C_SDA" (at 1 2)) (global_label "VBUS_IN" (shape input)) (label "EN" (at 0 0)) (label "SIGA" (at 0 0))';
    expect(markersFrom(kicad)).toEqual(["I2C_SDA", "VBUS_IN", "SIGA"]);
  });

  it("renders a markdown table with times in the unit a reader expects", () => {
    const out = table([
      { step: "1. open", design: "gateway", status: "ok", ms: 840, detail: "25 findings" },
      { step: "4. save report", design: "gateway", status: "not available", detail: "#127 | export" },
    ]);
    expect(out).toContain("| 1. open | gateway | ok | 840 ms | 25 findings |");
    expect(out).toContain("| not available |  | #127 \\| export |");
  });
  it("splits a Run checks between its requests and the page's own work", () => {
    const line = (rpc: string, t: object) => `agni timing ${rpc}: ${JSON.stringify(t)}`;
    const lines = [
      line("GetDesign", { totalMicros: "1000", page: { startMs: 50, wallMs: 10 } }),
      line("CheckDesign", { totalMicros: "6000000", stages: [{ name: "read", micros: "1000000" }, { name: "rules", micros: "5000000" }], page: { startMs: 110, wallMs: 7000 } }),
      line("GetExpectations", { page: { startMs: 7200, wallMs: 100 }, error: "no netlist" }),
      line("GetInterfaceCoverage", { totalMicros: "1500000", rules: [{ rule: "coverage/emmc", micros: "900000" }, { rule: "coverage/can", micros: "10000" }], page: { startMs: 7300, wallMs: 2000 } }),
      // Overlaps the coverage request, so the 500 ms it shares counts once.
      line("HighlightSheet", { totalMicros: "400000", page: { startMs: 8800, wallMs: 1000 } }),
      line("GetSheet", { totalMicros: "1000" }),
      "agni timing RunQuery: not json",
    ];
    const b = checksBreakdown(lines, 100, 10100);
    expect(b.requests.map((r) => r.rpc)).toEqual(["CheckDesign", "GetExpectations", "GetInterfaceCoverage", "HighlightSheet"]);
    expect(b.totalMs).toBe(10000);
    expect(b.inRequestsMs).toBe(7000 + 100 + 2000 + 500);
    expect(b.pageMs).toBe(400);
    expect(b.requests[0]).toMatchObject({ offsetMs: 10, wallMs: 7000, engineMs: 6000, widest: "rules 5000 ms" });
    expect(b.requests[1]).toMatchObject({ error: "no netlist" });
    expect(breakdownDetail(b)).toBe(
      "4 requests: CheckDesign at +10 ms took 7.0 s (engine 6.0 s, rules 5000 ms); GetExpectations at +7.1 s took 100 ms (failed: no netlist); " +
        "GetInterfaceCoverage at +7.2 s took 2.0 s (engine 1.5 s, slowest coverage/emmc 900 ms); HighlightSheet at +8.7 s took 1.0 s (engine 400 ms); in requests 9.6 s, page outside them 400 ms",
    );
  });

  it("names the design files a timed-out open was still waiting on", () => {
    const prefix = "/agni/demo/";
    const req = (path: string, startMs: number, endMs?: number) => ({ url: `https://h${path}`, method: "GET", body: "", startMs, endMs });
    const requests = [
      req("/agni/demo/raw/jetson/board/b.kicad_pcb", 1_000),
      req("/agni/demo/raw/jetson/board/b.kicad_sch", 1_000, 3_000),
      req("/agni/demo/files/jetson.json", 500, 900),
      req("/agni/demo/app.js", 0),
    ];
    expect(pendingFiles(requests, prefix, 181_000)).toBe("waiting on jetson/board/b.kicad_pcb (180 s)");
    expect(pendingFiles(requests.slice(1), prefix, 181_000)).toBe("no design file pending");
  });

  it("names the slowest file fetch the page logged", () => {
    const lines = [
      'agni fetch jetson/b.kicad_sch: {"bytes":1048576,"ms":900,"attempts":1}',
      'agni fetch jetson/b.kicad_pcb: {"bytes":88080384,"ms":31200,"attempts":2}',
      "agni timing GetDesign: {}",
    ];
    expect(slowestFetch(lines)).toBe("slowest file jetson/b.kicad_pcb 84.0 MB in 31.2 s after 2 attempts");
    expect(slowestFetch([])).toBe("");
  });
});

// agni issue 1060: a timed request split into loading the design and answering it.
describe("splitting a request between loading and answering", () => {
  const line = (rpc: string, t: object) => `agni timing ${rpc}: ${JSON.stringify(t)}`;
  // A Jetson-shaped Run checks: build.model holds read.board, which holds build.board, then the rules
  // and a locate pass, with the page waiting far longer than the engine took.
  const check = line("CheckDesign", {
    totalMicros: 2_600_000,
    stages: [
      { name: "resolve.overlay", startMicros: 0, micros: 5_000 },
      { name: "build.model", startMicros: 10_000, micros: 1_513_000 },
      { name: "read.board", startMicros: 20_000, micros: 1_440_000 },
      { name: "build.board", startMicros: 30_000, micros: 1_268_000 },
      { name: "rules", startMicros: 1_530_000, micros: 810_000 },
      { name: "locate", startMicros: 2_350_000, micros: 200_000 },
    ],
    lookups: [
      { layer: "model", source: "built" },
      { layer: "board", source: "built" },
    ],
    page: { startMs: 817, wallMs: 3_100 },
  });

  it("counts a nested stage once, inside its outermost parent", () => {
    const s = loadSplit(check)!;
    expect(s.loadMs).toBe(1_518);
    expect(s.load).toEqual(["build.model 1513 ms", "resolve.overlay 5 ms"]);
    expect(s.rulesMs).toBe(810);
    expect(s.otherMs).toBe(272);
  });

  it("puts the page's wait outside the engine apart from the engine's own loading", () => {
    const s = loadSplit(check)!;
    expect(s.outsideMs).toBe(500);
    expect(s.startMs).toBe(817);
    expect(loadDetail(s)).toBe("500 ms outside the engine, 1.5 s loading (build.model 1513 ms, resolve.overlay 5 ms), 272 ms other engine work; cache model=built board=built");
  });

  it("reads a request that ran no rules as loading alone", () => {
    const s = loadSplit(line("GetDesign", { totalMicros: 300_000, stages: [{ name: "restore.design", startMicros: 0, micros: 250_000 }] }))!;
    expect(s.rulesMs).toBe(0);
    expect(s.loadMs).toBe(250);
    expect(s.outsideMs).toBeUndefined();
  });

  // agni issue 1061: an overlay with no identity derives its design from the kept parse on every
  // request, outside any build stage, and that is loading too.
  it("counts a derive outside any build as loading", () => {
    const s = loadSplit(line("CheckDesign", { totalMicros: 300_000, stages: [{ name: "derive.design", startMicros: 0, micros: 40_000 }, { name: "rules", startMicros: 40_000, micros: 200_000 }] }))!;
    expect(s.loadMs).toBe(40);
    expect(s.rulesMs).toBe(200);
  });

  it("returns nothing for a line that is not a timing or carries no engine total", () => {
    expect(loadSplit("agni fetch a.kicad_pcb: {}")).toBeUndefined();
    expect(loadSplit(line("CheckDesign", { page: { startMs: 1, wallMs: 2 }, error: "aborted" }))).toBeUndefined();
    expect(loadSplit("agni timing CheckDesign: {not json")).toBeUndefined();
  });

  it("adds an open's requests into one load, merging their stages by name", () => {
    const a = loadSplit(line("GetDesign", { totalMicros: 300_000, stages: [{ name: "restore.design", startMicros: 0, micros: 250_000 }], lookups: [{ layer: "design", source: "store" }], page: { startMs: 0, wallMs: 400 } }))!;
    const b = loadSplit(line("ListRules", { totalMicros: 100_000, stages: [{ name: "restore.design", startMicros: 0, micros: 50_000 }], lookups: [{ layer: "design", source: "store" }], page: { startMs: 10, wallMs: 150 } }))!;
    const s = sumSplits([a, b]);
    expect(s.loadMs).toBe(300);
    expect(s.outsideMs).toBeUndefined();
    expect(s.load).toEqual(["restore.design 300 ms"]);
    expect(s.cache).toBe("design=store");
  });
});
