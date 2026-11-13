import { describe, it, expect } from "vitest";
import { create } from "@bufbuild/protobuf";
import { TraceOutcome, WalkGoalAnswerSchema, WalkSchema } from "./gen/agni/v1/webapi/design_pb.js";
import { HighlightShape } from "./highlights.js";
import { HOP_COLORS, WALK_CROSSED_COLOR, WALK_GOAL_COLOR, WALK_START_COLOR, goalRevealSpecs, goalSheet, parseWalkParams, splitStart, walkFromResponse, walkParams, walkSheet, walkSpecs, walkStartMarks, type WalkRequest } from "./walk.js";
import { emptyLocation, locationToUrl, parseUrl } from "./router.js";

const req: WalkRequest = { start: "R1", kind: "small-parts", hops: 2, through: [] };
const answer = () =>
  create(WalkSchema, {
    nets: [
      { hop: 1, net: { name: "SDA", sheetIds: ["s2"] }, entered: { refDes: "R1", exitPin: "1", toNet: "SDA" } },
      { hop: 1, net: { name: "+3V3", sheetIds: ["s1"] }, entered: { refDes: "R1", exitPin: "2", toNet: "+3V3" }, ended: "rail" },
      { hop: 2, net: { name: "SDA_3V3", sheetIds: ["s3"] }, entered: { refDes: "U5", enterPin: "2", exitPin: "4", fromNet: "SDA", paired: true } },
      { hop: 2, net: { name: "GND" }, entered: { refDes: "C1", enterPin: "1", exitPin: "2", fromNet: "SDA" }, ended: "ground", alsoEntered: [{ refDes: "C2" }] },
    ],
    boundary: [{ refDes: "U3", class: "ic", hop: 1, net: "SDA", reason: "8 connected pins" }],
  });

describe("walk state", () => {
  it("groups the nets by hop with how each was entered", () => {
    const s = walkFromResponse(req, answer());
    expect(s.hops.map((h) => h.nets.map((n) => n.name))).toEqual([["SDA", "+3V3"], ["SDA_3V3", "GND"]]);
    const shifted = s.hops[1].nets[0];
    expect([shifted.through, shifted.enterPin, shifted.exitPin, shifted.fromNet, shifted.paired]).toEqual(["U5", "2", "4", "SDA", true]);
    expect(s.hops[1].nets[1].alsoThrough).toEqual(["C2"]);
  });

  // agni issue 1562. A ground or a rail on a board is many overlapping segments, and lighting it, however
  // faint, stacked to solid grey over the copper. The Trace tab says where the walk stopped instead.
  it("lights each hop in its shade and no net the walk stopped on, crossed parts red and the start on top", () => {
    const specs = walkSpecs(walkFromResponse(req, answer()));
    const byColor = (c: string) => specs.filter((x) => x.color === c);
    expect(byColor(HOP_COLORS[0]).flatMap((x) => x.nets ?? [])).toEqual(["SDA"]);
    expect(byColor(HOP_COLORS[1]).flatMap((x) => x.nets ?? [])).toEqual(["SDA_3V3"]);
    expect(specs.flatMap((x) => x.nets ?? [])).not.toContain("+3V3");
    expect(specs.flatMap((x) => x.nets ?? [])).not.toContain("GND");
    expect(byColor(WALK_CROSSED_COLOR).flatMap((x) => x.components ?? []).sort()).toEqual(["C1", "C2", "U5"]);
    expect(specs[specs.length - 1]).toEqual({ pins: [{ refDes: "R1", pin: "1" }, { refDes: "R1", pin: "2" }], color: WALK_START_COLOR });
    expect(specs.some((x) => x.shape === HighlightShape.BOUNDING_RECT && (x.components ?? []).includes("R1"))).toBe(false);
    expect(specs.some((x) => (x.components ?? []).includes("U3"))).toBe(true);
  });

  it("boxes a part start only when its first hop drew no net", () => {
    const bare = walkFromResponse(req, create(WalkSchema, { nets: [], boundary: [] }));
    expect(walkStartMarks(bare)).toEqual([expect.objectContaining({ components: ["R1"], shape: HighlightShape.BOUNDING_RECT })]);
  });

  it("opens on the first drawn net that the walk went on from", () => {
    expect(walkSheet(walkFromResponse(req, answer()))).toBe("s2");
  });

  it("lights nothing for a start that named nothing", () => {
    expect(walkSpecs(walkFromResponse(req, create(WalkSchema, { reason: "no component R1" })))).toEqual([]);
  });

  it("lights a goal's routes over the walk, and nothing for a goal with no route", () => {
    const goalReq: WalkRequest = { ...req, goal: { kind: "pin", at: "SDA", to: "J4.2" } };
    const routed = answer();
    routed.goal = create(WalkGoalAnswerSchema, {
      kind: "pin",
      at: "SDA",
      to: { endpoint: { refDes: "J4", pin: "2" }, pinName: "SDA", net: "SDA_J" },
      outcome: TraceOutcome.ROUTED,
      routes: [{ nets: [{ name: "SDA" }, { name: "SDA_J" }], crossings: [{ refDes: "R7", fromNet: "SDA", toNet: "SDA_J" }], endsAt: "SDA_J" }],
    });
    const s = walkFromResponse(goalReq, routed);
    expect(s.goal?.to).toBe("J4.2 (SDA)");
    const lit = walkSpecs(s).filter((x) => x.color === WALK_GOAL_COLOR);
    expect(lit.flatMap((x) => x.nets ?? [])).toEqual(["SDA", "SDA_J"]);
    expect(lit.flatMap((x) => x.components ?? [])).toEqual(["R7"]);
    expect(lit.flatMap((x) => x.pins ?? [])).toEqual([{ refDes: "J4", pin: "2" }]);
    const none = answer();
    none.goal = create(WalkGoalAnswerSchema, { kind: "pin", at: "SDA", outcome: TraceOutcome.NO_ROUTE, reason: "no series path" });
    expect(walkSpecs(walkFromResponse(goalReq, none)).filter((x) => x.color === WALK_GOAL_COLOR)).toEqual([]);
  });

  it("zooms a routed goal to the start and the parts its route crosses, and keeps the walk's sheet when it draws the route's end", () => {
    const goalReq: WalkRequest = { ...req, goal: { kind: "pull-up", at: "SDA", to: "" } };
    const routed = answer();
    routed.goal = create(WalkGoalAnswerSchema, {
      kind: "pull-up",
      at: "SDA",
      outcome: TraceOutcome.ROUTED,
      routes: [{ nets: [{ name: "SDA", sheetIds: ["s2"] }, { name: "+3V3", sheetIds: ["s1", "s3"] }], crossings: [{ refDes: "R9", fromNet: "SDA", toNet: "+3V3" }], endsAt: "+3V3" }],
    });
    const s = walkFromResponse(goalReq, routed);
    const lit = goalRevealSpecs(s);
    expect(lit.flatMap((x) => x.components ?? [])).toEqual(["R9"]);
    expect(lit.flatMap((x) => x.pins ?? [])).toEqual([{ refDes: "R1", pin: "1" }, { refDes: "R1", pin: "2" }]);
    expect(lit.flatMap((x) => x.nets ?? [])).toEqual([]);
    expect(goalSheet(s, "s2")).toBe("s1");
    expect(goalSheet(s, "s3")).toBe("s3");
    const none = answer();
    none.goal = create(WalkGoalAnswerSchema, { kind: "pull-up", at: "SDA", outcome: TraceOutcome.NO_ROUTE, reason: "no pull-up" });
    expect(goalRevealSpecs(walkFromResponse(goalReq, none))).toEqual([]);
    expect(goalSheet(walkFromResponse(goalReq, none), "s2")).toBe("");
  });

  it("opens a route whose last net is drawn nowhere on its nearest drawn net's sheet", () => {
    const routed = answer();
    routed.goal = create(WalkGoalAnswerSchema, {
      kind: "rail",
      outcome: TraceOutcome.ROUTED,
      routes: [{ nets: [{ name: "SDA", sheetIds: ["s4"] }, { name: "VBUS" }], crossings: [{ refDes: "R2" }], endsAt: "VBUS" }],
    });
    expect(goalSheet(walkFromResponse({ ...req, goal: { kind: "rail", at: "", to: "" } }, routed), "s2")).toBe("s4");
  });

  it("splits a start at its first dot", () => {
    expect(splitStart("R1")).toEqual(["R1", ""]);
    expect(splitStart("U7.3")).toEqual(["U7", "3"]);
  });

  it("round-trips through the URL", () => {
    const r: WalkRequest = { start: "U7.3", kind: "series", hops: 3, through: ["U80", "U81"] };
    const p = walkParams(r);
    expect(parseWalkParams((k) => p[k] ?? null)).toEqual(r);
    expect(walkParams({ start: "", kind: "series", hops: 2, through: [] })).toEqual({});
    const url = new URL(locationToUrl({ ...emptyLocation(), mount: "m", path: "b.edn", walk: r }), "http://x");
    expect(url.searchParams.get("walk")).toBe("U7.3");
    expect(parseUrl(url.pathname, url.search).walk).toEqual(r);
    const g: WalkRequest = { ...r, goal: { kind: "pin", at: "SDA", to: "J4.2" } };
    const gp = walkParams(g);
    expect(parseWalkParams((k) => gp[k] ?? null)).toEqual(g);
  });
});
