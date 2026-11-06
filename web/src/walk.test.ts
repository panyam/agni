import { describe, it, expect } from "vitest";
import { create } from "@bufbuild/protobuf";
import { WalkSchema } from "./gen/agni/v1/webapi/design_pb.js";
import { HighlightShape } from "./highlights.js";
import { HOP_COLORS, WALK_CROSSED_COLOR, WALK_ENDED_COLOR, parseWalkParams, splitStart, walkFromResponse, walkParams, walkSheet, walkSpecs, type WalkRequest } from "./walk.js";
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

  it("lights each hop in its shade, ended nets faint, crossed parts red and the start on top", () => {
    const specs = walkSpecs(walkFromResponse(req, answer()));
    const byColor = (c: string) => specs.filter((x) => x.color === c);
    expect(byColor(HOP_COLORS[0]).flatMap((x) => x.nets ?? [])).toEqual(["SDA"]);
    expect(byColor(HOP_COLORS[1]).flatMap((x) => x.nets ?? [])).toEqual(["SDA_3V3"]);
    expect(byColor(WALK_ENDED_COLOR).flatMap((x) => x.nets ?? [])).toEqual(["+3V3", "GND"]);
    expect(byColor(WALK_CROSSED_COLOR).flatMap((x) => x.components ?? []).sort()).toEqual(["C1", "C2", "U5"]);
    expect(specs[specs.length - 1]).toEqual(expect.objectContaining({ components: ["R1"], shape: HighlightShape.BOUNDING_RECT }));
    expect(specs.some((x) => (x.components ?? []).includes("U3"))).toBe(true);
  });

  it("opens on the first drawn net that the walk went on from", () => {
    expect(walkSheet(walkFromResponse(req, answer()))).toBe("s2");
  });

  it("lights nothing for a start that named nothing", () => {
    expect(walkSpecs(walkFromResponse(req, create(WalkSchema, { reason: "no component R1" })))).toEqual([]);
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
  });
});
