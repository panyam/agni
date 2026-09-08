import { describe, it, expect } from "vitest";
import { create } from "@bufbuild/protobuf";
import { TraceSchema, TraceOutcome } from "./gen/agni/v1/webapi/design_pb.js";
import { emptyTrace, parseEndpoint, traceFromResponse, traceSubjects } from "./trace.js";

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
    expect(s.from).toEqual({ refDes: "U1", pin: "3", pinName: "SDA", net: "SDA" });
    expect(s.crossings).toEqual([{ refDes: "R1", cls: "resistor", enterPin: "2", exitPin: "1" }]);
    expect(s.nets[1]).toEqual({ name: "VCC", stubs: [], stubsElided: 4, busLike: true });
    expect(s.radius).toBe(6);
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
