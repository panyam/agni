import { describe, it, expect } from "vitest";
import { ghostOverlays } from "./canvas.js";
import { GROUP_SYMBOL, GROUP_WIRE, KIND_LINE_STRIP, KIND_TRIANGLES, type DecodedSheet } from "./packed.js";

// A visual diff's union draws the old revision's removed parts over the new revision's sheet
// (agni issue 1335). Each sheet is packed about its own origin, so a ghost is moved by the
// difference between the two origins to sit where the old revision drew it.
describe("ghostOverlays", () => {
  const old: DecodedSheet = {
    sheetId: "a", layoutVersion: 1, originX: 100, originY: 50,
    vertices: Int32Array.from([0, 0, 10, 0, 20, 20, 30, 20, 20, 30]),
    primitives: [
      { kind: KIND_LINE_STRIP, group: GROUP_WIRE, firstVertex: 0, count: 2 },
      { kind: KIND_TRIANGLES, group: GROUP_SYMBOL, firstVertex: 2, count: 3 },
    ],
    keys: [
      { primitive: 0, refDes: "", net: "SIG", netId: "", pin: "", busId: "" },
      { primitive: 1, refDes: "R2", net: "", netId: "", pin: "", busId: "" },
    ],
    labels: [], images: [], fontFamily: "", groupColors: [], backgroundColor: "",
  };
  const onto = { originX: 40, originY: 10 };

  it("draws the removed part's fill where the old revision drew it, in the new sheet's frame", () => {
    const [g] = ghostOverlays(old, [{ components: ["R2"], color: "#ff0000" }], onto, 1, [1, 0, 1, 1]);
    // (20,20) in the old sheet is (120,70) in the design, and (80,60) in a sheet whose origin is (40,10).
    expect([...g.vertices]).toEqual([80, 60, 90, 60, 80, 70]);
    expect(g.color).toEqual([1, 0, 0, 1]);
    expect(g.subjects).toEqual(["component:R2"]);
  });

  it("draws a removed wire as a quad either side of its line", () => {
    const [g] = ghostOverlays(old, [{ nets: ["SIG"] }], onto, 2, [1, 0, 1, 1]);
    expect(g.vertices.length).toBe(12); // two triangles
    const ys = [...g.vertices].filter((_, i) => i % 2 === 1);
    expect(Math.min(...ys)).toBe(38);
    expect(Math.max(...ys)).toBe(42);
  });
});
