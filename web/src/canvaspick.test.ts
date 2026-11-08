import { describe, it, expect } from "vitest";
import { PickIndex, PIN_PICK_R, SHEET_MAX_PX, WIRE_PICK_WIDTH } from "./canvaspick.js";
import {
  GROUP_BOARD_COPPER_BACK,
  GROUP_BOARD_COPPER_FRONT,
  GROUP_BOARD_PICK_BACK,
  GROUP_BOARD_PICK_FRONT,
  GROUP_BOARD_SILK,
  GROUP_BUS,
  GROUP_PIN,
  GROUP_SYMBOL,
  GROUP_WIRE,
  KIND_LINE_LOOP,
  KIND_LINE_STRIP,
  KIND_POINTS,
  KIND_TRIANGLES,
  type DecodedSheet,
  type PrimitiveKey,
} from "./packed.js";

type Key = Partial<Omit<PrimitiveKey, "primitive">>;
interface Prim {
  kind: number;
  group: number;
  pts: [number, number][];
  key?: Key;
}

// sheetOf packs primitives the way PackSheet and PackBoard lay them out. The first primitive is an
// unkeyed frame from (0,0) to (1600,1600), so one document pixel is one world unit.
function sheetOf(sheetId: string, prims: Prim[]): DecodedSheet {
  const all: Prim[] = [{ kind: KIND_LINE_LOOP, group: 4, pts: [[0, 0], [SHEET_MAX_PX, 0], [SHEET_MAX_PX, SHEET_MAX_PX], [0, SHEET_MAX_PX]] }, ...prims];
  const vertices: number[] = [];
  const primitives = all.map((p) => {
    const firstVertex = vertices.length / 2;
    for (const [x, y] of p.pts) vertices.push(x, y);
    return { kind: p.kind, group: p.group, firstVertex, count: p.pts.length };
  });
  const keys = all.flatMap((p, i) =>
    p.key ? [{ primitive: i, refDes: "", net: "", netId: "", pin: "", busId: "", ...p.key }] : [],
  );
  return {
    sheetId, layoutVersion: 1, originX: 0, originY: 0, vertices: Int32Array.from(vertices), primitives, keys,
    labels: [], images: [], fontFamily: "", groupColors: [], backgroundColor: "",
  };
}

// quad is a filled rectangle as two triangles.
function quad(x0: number, y0: number, x1: number, y1: number): [number, number][] {
  return [[x0, y0], [x1, y0], [x1, y1], [x0, y0], [x1, y1], [x0, y1]];
}

const schematic = sheetOf("/", [
  { kind: KIND_LINE_STRIP, group: GROUP_WIRE, pts: [[100, 500], [900, 500]], key: { net: "SDA", netId: "n-sda" } },
  { kind: KIND_LINE_LOOP, group: GROUP_SYMBOL, pts: [[300, 300], [500, 300], [500, 400], [300, 400]], key: { refDes: "R7" } },
  { kind: KIND_POINTS, group: GROUP_PIN, pts: [[300, 350]], key: { refDes: "R7", pin: "1" } },
  { kind: KIND_LINE_STRIP, group: GROUP_SYMBOL, pts: [[1200, 1200], [1260, 1200]], key: { net: "GND" } },
  { kind: KIND_TRIANGLES, group: GROUP_BUS, pts: quad(100, 800, 900, 810), key: { busId: "DATA" } },
]);

describe("PickIndex on a schematic", () => {
  const idx = new PickIndex(schematic);

  it("picks a wire as its net, with its id", () => {
    expect(idx.pick(500, 500, 0)).toEqual({ kind: "net", net: "SDA", netId: "n-sda" });
  });

  it("gives a wire SVG's companion stroke and no more", () => {
    expect(idx.pick(500, 500 + WIRE_PICK_WIDTH / 2 - 0.1, 0)?.kind).toBe("net");
    expect(idx.pick(500, 500 + WIRE_PICK_WIDTH / 2 + 0.1, 0)).toBeNull();
  });

  it("finds a target under a probe of SVG's ring", () => {
    // At 4 world units a CSS pixel, the probe 5 px below the click lands 20 units lower.
    expect(idx.pick(500, 520, 0)).toBeNull();
    expect(idx.pick(500, 520, 4)?.kind).toBe("net");
  });

  it("misses a small target that lies between two probes, as SVG's probes do", () => {
    // A pin 4 px from the click at 22.5 degrees sits inside the ring's disc and under no probe.
    const between = new PickIndex(sheetOf("/", [
      { kind: KIND_POINTS, group: GROUP_PIN, pts: [[800, 800]], key: { refDes: "R1", pin: "1" } },
    ]));
    const a = (22.5 * Math.PI) / 180;
    const x = 800 - 40 * Math.cos(a), y = 800 - 40 * Math.sin(a);
    // At 10 world units a CSS pixel the pin's target is 6 units across, under a probe 50 out.
    expect(between.pick(x, y, 10)).toBeNull();
    expect(between.pick(800 - 50, 800, 10)).toEqual({ kind: "pin", ref: "R1", pin: "1" });
  });

  it("picks a pin over the part drawn over it", () => {
    expect(idx.pick(301, 351, 0)).toEqual({ kind: "pin", ref: "R7", pin: "1" });
  });

  it("gives a pin SVG's invisible target", () => {
    expect(idx.pick(300 - PIN_PICK_R + 0.5, 350, 0)?.kind).toBe("pin");
  });

  it("picks a part on its outline and not inside it, as SVG's hollow outline does", () => {
    expect(idx.pick(400, 300, 0)).toEqual({ kind: "component", ref: "R7" });
    expect(idx.pick(400, 350, 0)).toBeNull();
  });

  it("picks a glyph that names a net as that net (agni issue 764)", () => {
    expect(idx.pick(1230, 1200, 0)).toEqual({ kind: "net", net: "GND", netId: "" });
  });

  it("picks a bus by its id", () => {
    expect(idx.pick(500, 805, 0)).toEqual({ kind: "bus", busId: "DATA" });
  });

  it("picks nothing on bare paper or past the sheet", () => {
    expect(idx.pick(700, 1100, 5)).toBeNull();
    expect(idx.pick(-500, -500, 5)).toBeNull();
  });
});

// rect is a pick area's outline, as PackBoard packs it.
function rect(x0: number, y0: number, x1: number, y1: number): [number, number][] {
  return [[x0, y0], [x1, y0], [x1, y1], [x0, y1]];
}

const board = sheetOf("board", [
  // A ground pour over most of the board, on the front.
  { kind: KIND_TRIANGLES, group: GROUP_BOARD_COPPER_FRONT, pts: quad(50, 50, 1500, 1500), key: { net: "GND", netId: "n-gnd" } },
  // U1's two pads, its silkscreen outline and its pick area, which reaches past its pads as
  // BoardSVG's body rect does.
  { kind: KIND_TRIANGLES, group: GROUP_BOARD_COPPER_FRONT, pts: quad(400, 400, 440, 440), key: { refDes: "U1", pin: "1", net: "SDA", netId: "n-sda" } },
  { kind: KIND_TRIANGLES, group: GROUP_BOARD_COPPER_FRONT, pts: quad(560, 400, 600, 440), key: { refDes: "U1", pin: "2", net: "GND", netId: "n-gnd" } },
  { kind: KIND_LINE_LOOP, group: GROUP_BOARD_SILK, pts: rect(360, 360, 640, 480), key: { refDes: "U1" } },
  { kind: KIND_LINE_LOOP, group: GROUP_BOARD_PICK_FRONT, pts: rect(390, 390, 610, 450), key: { refDes: "U1" } },
  // A track on the pour.
  { kind: KIND_TRIANGLES, group: GROUP_BOARD_COPPER_FRONT, pts: quad(700, 1000, 1200, 1010), key: { net: "SDA", netId: "n-sda" } },
  // U2 on the back, with a pad that has no number.
  { kind: KIND_TRIANGLES, group: GROUP_BOARD_COPPER_BACK, pts: quad(1000, 200, 1040, 240), key: { refDes: "U2", pin: "1", net: "VCC" } },
  { kind: KIND_TRIANGLES, group: GROUP_BOARD_COPPER_BACK, pts: quad(1100, 200, 1140, 240), key: { refDes: "U2" } },
  { kind: KIND_LINE_LOOP, group: GROUP_BOARD_PICK_BACK, pts: rect(1000, 200, 1140, 240), key: { refDes: "U2" } },
]);

describe("PickIndex on a board", () => {
  const idx = new PickIndex(board);

  it("picks a pad as a pin of its part", () => {
    expect(idx.pick(420, 420, 0)).toEqual({ kind: "pin", ref: "U1", pin: "1" });
  });

  it("picks the part inside its pick area, over the pour under it", () => {
    expect(idx.pick(500, 420, 0)).toEqual({ kind: "component", ref: "U1" });
    // Past the pads and inside the pick area, as BoardSVG's body rect reaches.
    expect(idx.pick(395, 445, 0)).toEqual({ kind: "component", ref: "U1" });
  });

  it("picks a pad with no number as its part", () => {
    expect(idx.pick(1120, 220, 0)).toEqual({ kind: "component", ref: "U2" });
  });

  it("lets a click through the silkscreen", () => {
    // A point on the outline, outside the pick area, so the pour is the only other thing under it.
    expect(idx.pick(370, 480, 0)).toEqual({ kind: "net", net: "GND", netId: "n-gnd" });
  });

  it("picks a track over the pour it crosses", () => {
    expect(idx.pick(900, 1005, 0)).toEqual({ kind: "net", net: "SDA", netId: "n-sda" });
  });

  it("picks the pour where nothing else lies", () => {
    expect(idx.pick(900, 700, 0)).toEqual({ kind: "net", net: "GND", netId: "n-gnd" });
  });

  it("picks nothing on a hidden layer, and a part hides with its side's copper row", () => {
    // B.Cu's row names the back copper and the back pick areas (BoardLayers).
    const back = new Set([GROUP_BOARD_COPPER_BACK, GROUP_BOARD_PICK_BACK]);
    expect(idx.pick(1070, 220, 0)).toEqual({ kind: "component", ref: "U2" });
    expect(idx.pick(1070, 220, 0, back)).toEqual({ kind: "net", net: "GND", netId: "n-gnd" });
    expect(idx.pick(1020, 220, 0, back)?.kind).toBe("net");
    const all = new Set([...back, GROUP_BOARD_COPPER_FRONT, GROUP_BOARD_PICK_FRONT]);
    expect(idx.pick(1070, 220, 0, all)).toBeNull();
  });

  it("gives board copper no slack beyond its shape, only the probes", () => {
    expect(idx.pick(1600, 1600, 0)).toBeNull();
    expect(idx.pick(1550, 1550, 10)?.kind).toBe("net");
  });
});

describe("PickIndex paint order", () => {
  // A back track packs before the longer front track that crosses it, as PackBoard packs back to
  // front, so the front one is drawn on top and a click where they cross takes it.
  const crossing = new PickIndex(sheetOf("board", [
    { kind: KIND_TRIANGLES, group: GROUP_BOARD_COPPER_BACK, pts: quad(495, 450, 505, 550), key: { net: "BACK" } },
    { kind: KIND_TRIANGLES, group: GROUP_BOARD_COPPER_FRONT, pts: quad(100, 495, 900, 505), key: { net: "FRONT" } },
  ]));

  it("takes the topmost of two nets under a probe", () => {
    expect(crossing.pick(500, 500, 0)).toEqual({ kind: "net", net: "FRONT", netId: "" });
  });

  it("takes the one underneath where the top one is hidden", () => {
    expect(crossing.pick(500, 500, 0, new Set([GROUP_BOARD_COPPER_FRONT]))).toEqual({ kind: "net", net: "BACK", netId: "" });
  });
});

describe("PickIndex zones", () => {
  // An L-shaped zone, packed as its outline alone, as PackBoard packs a zone.
  const zone = new PickIndex(sheetOf("board", [
    { kind: KIND_LINE_LOOP, group: GROUP_BOARD_COPPER_FRONT, pts: [[100, 100], [700, 100], [700, 300], [300, 300], [300, 700], [100, 700]], key: { net: "+1V8" } },
  ]));

  it("picks a zone anywhere inside it, as SVG fills its polygon", () => {
    expect(zone.pick(200, 600, 0)).toEqual({ kind: "net", net: "+1V8", netId: "" });
    expect(zone.pick(600, 200, 0)?.kind).toBe("net");
  });

  it("picks nothing in the notch of a zone that is not convex", () => {
    expect(zone.pick(500, 500, 0)).toBeNull();
  });
});
