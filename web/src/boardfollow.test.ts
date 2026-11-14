import { describe, it, expect } from "vitest";
import { followBoard, type BoardFacts } from "./boardfollow.js";
import type { BoardLayerRow } from "./gen/agni/v1/geom/geom_packed_pb.js";

const row = (name: string, kind: string, side: string) => ({ name, kind, side, color: "", svgSelector: "", groups: [] }) as unknown as BoardLayerRow;
const rows = [
  row("F.Cu", "copper", "front"),
  row("In1.Cu", "copper", "inner"),
  row("In2.Cu", "copper", "inner"),
  row("B.Cu", "copper", "back"),
  row("Through", "through", ""),
  row("F.SilkS", "silk", "front"),
  row("B.SilkS", "silk", "back"),
  row("F.Fab", "fab", "front"),
];

const part = (side: string, layer = side === "back" ? "B.Cu" : "F.Cu"): BoardFacts => ({ part: true, side, layers: side ? [layer] : [] });
const net = (...layers: string[]): BoardFacts => ({ part: false, side: "", layers });
const sorted = (s: Set<string>) => [...s].sort();

describe("followBoard", () => {
  it("turns to the bottom for a bottom part and shows that side's copper and silkscreen", () => {
    const f = followBoard([part("back")], rows);
    expect(f.side).toBe("back");
    expect(sorted(f.show)).toEqual(["B.Cu", "B.SilkS"]);
  });

  it("turns back to the top for a top part", () => {
    const f = followBoard([part("front")], rows);
    expect(f.side).toBe("front");
    expect(sorted(f.show)).toEqual(["F.Cu", "F.SilkS"]);
  });

  it("keeps the side for a selection on both sides, and shows both", () => {
    const f = followBoard([part("front"), part("back")], rows);
    expect(f.side).toBeUndefined();
    expect(sorted(f.show)).toEqual(["B.Cu", "B.SilkS", "F.Cu", "F.SilkS"]);
  });

  it("keeps the side for a part whose board states none", () => {
    expect(followBoard([part("")], rows)).toEqual({ side: undefined, show: new Set() });
    expect(followBoard([part("front"), part("")], rows).side).toBeUndefined();
  });

  it("shows an inner-layer net's layer and keeps the side", () => {
    const f = followBoard([net("In2.Cu")], rows);
    expect(f.side).toBeUndefined();
    expect(sorted(f.show)).toEqual(["In2.Cu"]);
  });

  it("shows the union for a part and a net, and turns to the part's side", () => {
    const f = followBoard([part("back"), net("F.Cu", "In1.Cu")], rows);
    expect(f.side).toBe("back");
    expect(sorted(f.show)).toEqual(["B.Cu", "B.SilkS", "F.Cu", "In1.Cu"]);
  });

  it("ignores a layer the board view lists no copper row for", () => {
    expect(sorted(followBoard([net("F.Mask", "In9.Cu", "F.SilkS")], rows).show)).toEqual([]);
  });
});
