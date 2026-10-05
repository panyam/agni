// The public demo exercise's report logic (web/browser/exercise-report.ts), tested here because
// the unit run reads src/ and its config is held identical to the workbench's (hack/fixture_copies.txt).
import { describe, expect, it } from "vitest";
import { classify, exitCode, leaks, markersFrom, table, type Row } from "../browser/exercise-report.js";

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
});
