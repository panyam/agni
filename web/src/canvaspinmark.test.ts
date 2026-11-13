import { describe, it, expect } from "vitest";
import { pinMarkTriangles } from "./canvas.js";
import { PIN_MARK_FRACTION, pinMarkRadius, resolveHighlights } from "./highlights.js";
import { KIND_LINE_STRIP, KIND_POINTS, type DecodedSheet } from "./packed.js";

// A lit pin on a schematic draws as a dot the size the SVG overlay draws, since in WebGL the pin's
// own primitive is a point a pixel or two across. A walk from a part marks its start with these
// (PR 1575), and recolouring the point alone left the start close to invisible.
describe("pinMarkTriangles", () => {
  const sheet: DecodedSheet = {
    sheetId: "/SoM_IO", layoutVersion: 2, originX: 0, originY: 0,
    // R1's body outline, then its pins 1 and 2 as points.
    vertices: Int32Array.from([0, 0, 0, 1000, 400, 1000, 400, 0, 200, 1200, 200, -200]),
    primitives: [
      { kind: KIND_LINE_STRIP, group: 1, firstVertex: 0, count: 4 },
      { kind: KIND_POINTS, group: 2, firstVertex: 4, count: 1 },
      { kind: KIND_POINTS, group: 2, firstVertex: 5, count: 1 },
    ],
    keys: [
      { primitive: 0, refDes: "R1", net: "", netId: "", pin: "", busId: "" },
      { primitive: 1, refDes: "R1", net: "", netId: "", pin: "1", busId: "" },
      { primitive: 2, refDes: "R1", net: "", netId: "", pin: "2", busId: "" },
    ],
    labels: [], images: [], fontFamily: "", groupColors: [], backgroundColor: "",
  };
  const centre = (tris: number[], disc: number) => [tris[disc * 288], tris[disc * 288 + 1]];

  it("draws one disc per lit pin, centred on the pin, at the SVG dot's radius", () => {
    const [g] = resolveHighlights(sheet.keys, [{ pins: [{ refDes: "R1", pin: "1" }, { refDes: "R1", pin: "2" }] }]);
    expect(g.pins).toEqual([true, true]);
    const r = pinMarkRadius(10_000);
    expect(r).toBe(10_000 * PIN_MARK_FRACTION);
    const tris = pinMarkTriangles(sheet, g, r);
    // 48 triangles of three x,y points each, per disc.
    expect(tris.length).toBe(2 * 48 * 6);
    expect(centre(tris, 0)).toEqual([200, 1200]);
    expect(centre(tris, 1)).toEqual([200, -200]);
    const far = Math.max(...tris.slice(0, 288).map((v, i) => (i % 2 === 0 ? Math.abs(v - 200) : Math.abs(v - 1200))));
    expect(far).toBeCloseTo(r);
  });

  it("draws nothing for a whole part, and nothing on the board, whose pads show at full size", () => {
    const [part] = resolveHighlights(sheet.keys, [{ components: ["R1"] }]);
    expect(part.pins).toEqual([false]);
    expect(pinMarkTriangles(sheet, part, 25)).toEqual([]);
    const [pin] = resolveHighlights(sheet.keys, [{ pins: [{ refDes: "R1", pin: "1" }] }]);
    expect(pinMarkTriangles({ ...sheet, sheetId: "board" }, pin, 25)).toEqual([]);
  });
});
