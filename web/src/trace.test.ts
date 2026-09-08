import { describe, it, expect } from "vitest";
import { create } from "@bufbuild/protobuf";
import { TraceSchema, TraceOutcome } from "./gen/agni/v1/webapi/design_pb.js";
import { emptyTrace, parseEndpoint, splitTraceParam, traceFromResponse, traceSheet, traceSubjects } from "./trace.js";

// create(TraceSchema, ...) rather than an object literal: a plain literal standing in for a proto is
// invisible to `pnpm run typecheck`, which is how a fixture goes structurally wrong while the build
// stays green (web-client.md).
function routed() {
  return create(TraceSchema, {
    from: { endpoint: { refDes: "U1", pin: "3" }, pinName: "SDA", net: "SDA" },
    to: { endpoint: { refDes: "J1", pin: "1" }, pinName: "VBUS", net: "VCC" },
    outcome: TraceOutcome.ROUTED,
    radius: 6,
    crossings: [{ refDes: "R1", class: "resistor", enterPin: "2", exitPin: "1", fromNet: "SDA", toNet: "VCC" }],
    nets: [
      { name: "SDA", stubs: [{ refDes: "U2", pin: "3", class: "ic" }], stubsElided: 0, busLike: false },
      { name: "VCC", stubs: [], stubsElided: 4, busLike: true },
    ],
  });
}

describe("parseEndpoint", () => {
  it("splits at the first dot, because a ref-des has none and a pin designator might", () => {
    expect(parseEndpoint("U1.3")).toEqual({ refDes: "U1", pin: "3" });
    expect(parseEndpoint("  U1.A.1 ")).toEqual({ refDes: "U1", pin: "A.1" });
  });
  it("rejects text that is not a pin", () => {
    for (const bad of ["U1", "", ".3", "U1.", "."]) expect(parseEndpoint(bad)).toBeNull();
  });
});

describe("traceFromResponse", () => {
  it("carries every field the panel renders", () => {
    const s = traceFromResponse(routed());
    expect(s.ran).toBe(true);
    expect(s.from).toEqual({ refDes: "U1", pin: "3", pinName: "SDA", net: "SDA", sheetIds: [] });
    expect(s.crossings).toEqual([{ refDes: "R1", cls: "resistor", enterPin: "2", exitPin: "1" }]);
    expect(s.nets[1]).toEqual({ name: "VCC", stubs: [], stubsElided: 4, busLike: true, sheetIds: [] });
    expect(s.radius).toBe(6);
  });
});

// A trace answer carries where each net and endpoint is DRAWN, so the viewer can open a sheet the
// route is on rather than whichever was already showing (agni issue 657). An older server that sends
// no such field yields an empty list, not undefined, so a panel rendering badges over it is safe.
describe("sheet ids on a trace", () => {
  it("carries the server's sheets for endpoints and nets", () => {
    const t = routed();
    t.from!.sheetIds = ["s2"];
    t.nets[0].sheetIds = ["s2", "s5"];
    const s = traceFromResponse(t);
    expect(s.from.sheetIds).toEqual(["s2"]);
    expect(s.nets[0].sheetIds).toEqual(["s2", "s5"]);
  });

  it("reads a response with no sheets as drawn nowhere, not as undefined", () => {
    const s = traceFromResponse(routed());
    expect(s.from.sheetIds).toEqual([]);
    expect(s.nets[0].sheetIds).toEqual([]);
  });
});

describe("traceSheet", () => {
  it("prefers the FROM endpoint, which is the pin the reader named", () => {
    const t = routed();
    t.from!.sheetIds = ["from-sheet"];
    t.nets[0].sheetIds = ["net-sheet"];
    expect(traceSheet(traceFromResponse(t))).toBe("from-sheet");
  });

  it("falls back to the first net that is drawn", () => {
    const t = routed();
    t.nets[1].sheetIds = ["net-sheet"];
    expect(traceSheet(traceFromResponse(t))).toBe("net-sheet");
  });

  // An answer drawn nowhere must leave the view alone rather than jump somewhere arbitrary.
  it("names no sheet when nothing on the route is drawn", () => {
    expect(traceSheet(traceFromResponse(routed()))).toBe("");
  });
});

describe("traceSubjects", () => {
  it("lights the route's nets, the parts crossed, and both endpoint pins", () => {
    const subs = traceSubjects(traceFromResponse(routed()));
    expect(subs.filter((s) => s.kind === "net").map((s) => s.subject)).toEqual(["SDA", "VCC"]);
    expect(subs.filter((s) => s.kind === "component").map((s) => s.subject)).toEqual(["R1"]);
    expect(subs.filter((s) => s.kind === "pin").map((s) => `${s.subject}.${s.pin}`)).toEqual(["U1.3", "J1.1"]);
  });

  // The picture a reader goes looking for the moment they read the words.
  it("lights both nets on a no-route, and no crossings", () => {
    const t = routed();
    t.outcome = TraceOutcome.NO_ROUTE;
    const subs = traceSubjects(traceFromResponse(t));
    expect(subs.filter((s) => s.kind === "net").map((s) => s.subject)).toEqual(["SDA", "VCC"]);
    expect(subs.filter((s) => s.kind === "component")).toHaveLength(0);
  });

  // Half an answer located is more use than none, and the panel beside it says the other end named
  // nothing.
  it("lights only the end that resolved when the other did not", () => {
    const t = routed();
    t.outcome = TraceOutcome.UNRESOLVED;
    t.from!.net = "";
    const subs = traceSubjects(traceFromResponse(t));
    expect(subs.filter((s) => s.kind === "net").map((s) => s.subject)).toEqual(["VCC"]);
  });

  it("lights nothing before anything has been asked", () => {
    expect(traceSubjects(emptyTrace())).toHaveLength(0);
  });
});

describe("splitTraceParam", () => {
  it("splits a ?trace= value at the first comma", () => {
    expect(splitTraceParam("U1.3,J1.1")).toEqual(["U1.3", "J1.1"]);
    expect(splitTraceParam(" U1.3 , J1.1 ")).toEqual(["U1.3", "J1.1"]);
  });
  it("yields an empty second pin rather than throwing on a malformed value", () => {
    expect(splitTraceParam("U1.3")).toEqual(["U1.3", ""]);
  });
});
