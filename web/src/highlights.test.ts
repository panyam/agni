import { describe, it, expect } from "vitest";
import { DEFAULT_HIGHLIGHT_COLOR, HighlightShape, circleTriangles, entityBounds, entityFrame, normalizePathAlpha, pathQuads, rectTriangles, resolveHighlights, withFocusShape, type HighlightSpec } from "./highlights.js";
import { KIND_LINE_STRIP, type Primitive, type PrimitiveKey } from "./packed.js";

// The fixture mirrors core/render/highlight_test.go, with R1 (rect prim 2, pins 3/4), U1 (rect 5,
// pin 6), NET1 wire 0, NET2 wire 1, so the local resolution provably matches the server's
// HighlightPacked semantics.
const keys: PrimitiveKey[] = [
  { primitive: 0, refDes: "", net: "NET1", netId: "", pin: "", busId: "" },
  { primitive: 1, refDes: "", net: "NET2", netId: "", pin: "", busId: "" },
  { primitive: 2, refDes: "R1", net: "", netId: "", pin: "", busId: "" },
  { primitive: 3, refDes: "R1", net: "", netId: "", pin: "1", busId: "" },
  { primitive: 4, refDes: "R1", net: "", netId: "", pin: "2", busId: "" },
  { primitive: 5, refDes: "U1", net: "", netId: "", pin: "", busId: "" },
  { primitive: 6, refDes: "U1", net: "", netId: "", pin: "1", busId: "" },
];

describe("resolveHighlights", () => {
  it("matches components (with their pins), nets, and individual pins, one group per spec", () => {
    const specs: HighlightSpec[] = [
      { components: ["R1"], nets: ["NET2"], color: "#ff0000", alpha: 0.5 },
      { pins: [{ refDes: "U1", pin: "1" }] },
    ];
    const groups = resolveHighlights(keys, specs);
    expect(groups).toHaveLength(2);
    expect([...groups[0].primitives].sort()).toEqual([1, 2, 3, 4]);
    expect(groups[0].color).toBe("#ff0000");
    expect(groups[0].alpha).toBe(0.5);
    // U1 pin "1" alone, not U1's body, and not R1's pin "1".
    expect([...groups[1].primitives]).toEqual([6]);
  });

  it("names each matched entity once as a subject, a net matched by id by its name", () => {
    const withIds: PrimitiveKey[] = [
      ...keys,
      { primitive: 7, refDes: "", net: "NET3", netId: "n3", pin: "", busId: "" },
      { primitive: 8, refDes: "", net: "", netId: "", pin: "", busId: "bus-1" },
    ];
    const [g] = resolveHighlights(withIds, [
      { components: ["R1"], nets: ["NET2", "NOSUCH"], netIds: ["n3"], busIds: ["bus-1"], pins: [{ refDes: "U1", pin: "1" }] },
    ]);
    expect(g.subjects).toEqual(["net:NET2", "component:R1", "pin:U1.1", "net:NET3", "bus:bus-1"]);
    expect(resolveHighlights(keys, [{ nets: ["NOSUCH"] }])[0].subjects).toEqual([]);
  });

  it("yields no primitives for a subject absent from the sheet keys", () => {
    expect(resolveHighlights(keys, [{ nets: ["NET1"] }])[0].primitives.size).toBeGreaterThan(0);
    expect(resolveHighlights(keys, [{ nets: ["NOSUCH"] }])[0].primitives.size).toBe(0);
    expect(resolveHighlights(keys, [{ components: ["#PWR02"] }])[0].primitives.size).toBe(0);
  });

  it("defaults color and treats unset/out-of-range alpha as opaque", () => {
    const [g] = resolveHighlights(keys, [{ nets: ["NET1"] }]);
    expect(g.color).toBe(DEFAULT_HIGHLIGHT_COLOR);
    expect(g.alpha).toBe(1);
    expect(resolveHighlights(keys, [{ nets: ["NET1"], alpha: 0 }])[0].alpha).toBe(1);
    expect(resolveHighlights(keys, [{ nets: ["NET1"], alpha: 2 }])[0].alpha).toBe(1);
  });

  it("a spec matching nothing still yields its (empty) group, aligned with the input", () => {
    const groups = resolveHighlights(keys, [{ components: ["NOPE"] }, { nets: ["NET1"] }]);
    expect(groups[0].primitives.size).toBe(0);
    expect([...groups[1].primitives]).toEqual([0]);
  });

  it("empty selector lists match nothing (not everything)", () => {
    const [g] = resolveHighlights(keys, [{}]);
    expect(g.primitives.size).toBe(0);
  });

  it("matches a bus by bus id, grouping its segments into one entity (WS7-042b)", () => {
    const busKeys = [
      { primitive: 0, refDes: "", net: "", netId: "", pin: "", busId: "bus-1" },
      { primitive: 1, refDes: "", net: "", netId: "", pin: "", busId: "bus-1" }, // a second segment of the same bus
      { primitive: 2, refDes: "", net: "SIG", netId: "", pin: "", busId: "" },
    ];
    const [g] = resolveHighlights(busKeys, [{ busIds: ["bus-1"] }]);
    expect([...g.primitives].sort()).toEqual([0, 1]); // both bus segments, not the wire
    expect(g.entities).toEqual([[0, 1]]); // one entity so an OUTLINE recolors the whole trunk
    expect(resolveHighlights(busKeys, [{ busIds: ["nope"] }])[0].primitives.size).toBe(0);
  });
});

// Twin of TestHighlightEntityFraming / TestHighlightSVGBoundingShapes in
// core/render/highlight_test.go, with the same fixture geometry and the same expected constants, so
// the TS overlay and the Go SVG projection provably frame entities identically.
describe("entity grouping and framing (WS9-017)", () => {
  it("groups matched primitives per entity, a component's pins joining its entity", () => {
    const [g] = resolveHighlights(keys, [{ components: ["R1"], nets: ["NET1"] }]);
    expect(g.shape).toBe(HighlightShape.OUTLINE);
    expect(g.entities).toEqual([[0], [2, 3, 4]]);
    const [pin] = resolveHighlights(keys, [{ pins: [{ refDes: "U1", pin: "1" }] }]);
    expect(pin.entities).toEqual([[6]]);
  });

  it("defaults unset alpha to 0.3 for bounding shapes, opaque for outline", () => {
    expect(resolveHighlights(keys, [{ nets: ["NET1"] }])[0].alpha).toBe(1);
    expect(resolveHighlights(keys, [{ nets: ["NET1"], shape: HighlightShape.BOUNDING_RECT }])[0].alpha).toBe(0.3);
    expect(resolveHighlights(keys, [{ nets: ["NET1"], shape: HighlightShape.BOUNDING_CIRCLE, alpha: 0.5 }])[0].alpha).toBe(0.5);
  });

  // The world geometry of the Go fixture, tessellated as the packer would emit it.
  const vertices = new Int32Array([
    100, 100, 200, 100, // 0: NET1 wire
    100, 200, 200, 200, // 1: NET2 wire
    300, 400, 340, 400, 340, 420, 300, 420, // 2: R1 rect
    300, 410, // 3: R1 pin 1
    340, 410, // 4: R1 pin 2
    600, 400, 660, 400, 660, 460, 600, 460, // 5: U1 rect
    600, 430, // 6: U1 pin 1
  ]);
  const prims = [
    { kind: 1, group: 0, firstVertex: 0, count: 2 },
    { kind: 1, group: 0, firstVertex: 2, count: 2 },
    { kind: 2, group: 1, firstVertex: 4, count: 4 },
    { kind: 3, group: 2, firstVertex: 8, count: 1 },
    { kind: 3, group: 2, firstVertex: 9, count: 1 },
    { kind: 2, group: 1, firstVertex: 10, count: 4 },
    { kind: 3, group: 2, firstVertex: 14, count: 1 },
  ];

  it("frames entities with the twinned padded bounds and circumscribed circle", () => {
    const cases = [
      // R1 with its pins, 40x20 raw bbox, pad 8.
      { entity: [2, 3, 4], frame: [292, 392, 348, 428], circle: [320, 410, 30.360679774997898] },
      // NET1: 100x0, pad 10.
      { entity: [0], frame: [90, 90, 210, 110], circle: [150, 100, 60] },
      // Lone pin, zero-area point, floor pad 8.
      { entity: [6], frame: [592, 422, 608, 438], circle: [600, 430, 8] },
    ];
    for (const c of cases) {
      const b = entityBounds(vertices, prims, c.entity);
      expect(b).not.toBeNull();
      const f = entityFrame(b!.minX, b!.minY, b!.maxX, b!.maxY);
      expect([f.minX, f.minY, f.maxX, f.maxY]).toEqual(c.frame);
      expect([f.cx, f.cy, f.r]).toEqual(c.circle);
    }
    expect(entityBounds(vertices, prims, [])).toBeNull();
  });

  it("tessellates frames into world-space triangles", () => {
    const f = entityFrame(0, 0, 10, 10); // pad floors at 8
    const rect = rectTriangles(f);
    expect(rect).toHaveLength(12);
    expect(Math.min(...rect)).toBe(-8);
    expect(Math.max(...rect)).toBe(18);
    const circ = circleTriangles(f);
    expect(circ.length % 6).toBe(0);
    expect(circ.length).toBeGreaterThan(0);
    // Every fan triangle starts at the center, and every rim vertex sits on the radius.
    for (let t = 0; t < circ.length; t += 6) {
      expect(circ[t]).toBe(f.cx);
      expect(circ[t + 1]).toBe(f.cy);
      for (const [x, y] of [[circ[t + 2], circ[t + 3]], [circ[t + 4], circ[t + 5]]]) {
        expect(Math.sqrt((x - f.cx) ** 2 + (y - f.cy) ** 2)).toBeCloseTo(f.r, 10);
      }
    }
  });
});

describe("withFocusShape (WS9-040)", () => {
  it("marks a net focus as a PATH highlighter, not a bounding box", () => {
    expect(withFocusShape([{ nets: ["SDA"] }])).toEqual([{ nets: ["SDA"], shape: HighlightShape.PATH }]);
  });

  it("frames a component focus with a bounding rect", () => {
    expect(withFocusShape([{ components: ["R1"] }])).toEqual([{ components: ["R1"], shape: HighlightShape.BOUNDING_RECT }]);
  });

  it("frames a pin focus with a bounding rect", () => {
    const specs: HighlightSpec[] = [{ pins: [{ refDes: "U1", pin: "1" }] }];
    expect(withFocusShape(specs)).toEqual([{ pins: [{ refDes: "U1", pin: "1" }], shape: HighlightShape.BOUNDING_RECT }]);
  });

  it("treats an empty net list as non-net (bounding rect), not a path", () => {
    expect(withFocusShape([{ components: ["R1"], nets: [] }])[0].shape).toBe(HighlightShape.BOUNDING_RECT);
  });

  it("keeps a bus focus at OUTLINE — recolor the thick trunk, not a PATH marker or a box (WS7-042b)", () => {
    expect(withFocusShape([{ busIds: ["bus-1"] }])).toEqual([{ busIds: ["bus-1"], shape: HighlightShape.OUTLINE }]);
  });

  it("preserves color and alpha while stamping the shape", () => {
    const [out] = withFocusShape([{ nets: ["SDA"], color: "#123456", alpha: 0.5 }]);
    expect(out).toEqual({ nets: ["SDA"], color: "#123456", alpha: 0.5, shape: HighlightShape.PATH });
  });

  it("maps each spec independently and overwrites any pre-set shape", () => {
    const out = withFocusShape([{ nets: ["SDA"], shape: HighlightShape.BOUNDING_RECT }, { components: ["R1"] }]);
    expect(out.map((s) => s.shape)).toEqual([HighlightShape.PATH, HighlightShape.BOUNDING_RECT]);
  });
});

describe("withFocusShape with a user style (WS9-044)", () => {
  const style = { color: "#00ff00", alpha: 0.6, scale: 2 };

  it("stamps color, opacity, and width scale on a net PATH marker", () => {
    expect(withFocusShape([{ nets: ["SDA"] }], style)).toEqual([
      { nets: ["SDA"], shape: HighlightShape.PATH, color: "#00ff00", alpha: 0.6, strokeScale: 2 },
    ]);
  });

  it("stamps only the color on a component box, keeping its default opacity", () => {
    expect(withFocusShape([{ components: ["R1"] }], style)).toEqual([{ components: ["R1"], shape: HighlightShape.BOUNDING_RECT, color: "#00ff00" }]);
  });

  it("leaves specs unstyled when no style is passed (built-in look, minimal specs)", () => {
    expect(withFocusShape([{ nets: ["SDA"] }])).toEqual([{ nets: ["SDA"], shape: HighlightShape.PATH }]);
  });
});

describe("PATH highlighter alpha (WS9-040)", () => {
  it("defaults an unset/out-of-range PATH alpha to translucent 0.4", () => {
    expect(normalizePathAlpha(undefined)).toBe(0.4);
    expect(normalizePathAlpha(0)).toBe(0.4);
    expect(normalizePathAlpha(1)).toBe(0.4);
    expect(normalizePathAlpha(0.6)).toBe(0.6);
  });

  it("resolves a PATH spec as a translucent recolor of the net's primitives", () => {
    // NET1 is wire primitive 0 in the fixture. PATH recolors it at the translucent default, as
    // OUTLINE does, since the GL backend cannot widen 1px lines.
    const [g] = resolveHighlights(keys, [{ nets: ["NET1"], shape: HighlightShape.PATH }]);
    expect(g.shape).toBe(HighlightShape.PATH);
    expect(g.alpha).toBe(0.4);
    expect([...g.primitives]).toEqual([0]);
  });

  it("surfaces a spec's strokeScale, defaulting to 1 (WS9-044)", () => {
    expect(resolveHighlights(keys, [{ nets: ["NET1"], shape: HighlightShape.PATH, strokeScale: 2 }])[0].strokeScale).toBe(2);
    expect(resolveHighlights(keys, [{ nets: ["NET1"] }])[0].strokeScale).toBe(1);
  });
});

describe("pathQuads (WS9-043)", () => {
  const strip = (firstVertex: number, count: number): Primitive => ({ kind: KIND_LINE_STRIP, group: 0, firstVertex, count });

  it("tessellates a straight wire segment into a quad of the given half-width", () => {
    const verts = Int32Array.from([0, 0, 10, 0]); // horizontal segment -> perpendicular is ±y
    expect(pathQuads(verts, [strip(0, 2)], [0], 2)).toEqual([0, 2, 0, -2, 10, 2, 0, -2, 10, -2, 10, 2]);
  });

  it("emits one quad (12 numbers) per segment of a bent wire", () => {
    const verts = Int32Array.from([0, 0, 10, 0, 10, 10]); // two segments
    expect(pathQuads(verts, [strip(0, 3)], [0], 1)).toHaveLength(24);
  });

  it("skips a zero-length segment and an out-of-range primitive index", () => {
    const verts = Int32Array.from([5, 5, 5, 5]); // degenerate segment
    expect(pathQuads(verts, [strip(0, 2)], [0], 2)).toEqual([]);
    expect(pathQuads(verts, [strip(0, 2)], [99], 2)).toEqual([]);
  });

  it("tessellates every primitive of a multi-wire net entity", () => {
    const verts = Int32Array.from([0, 0, 10, 0, 0, 5, 10, 5]); // two separate 1-segment wires
    expect(pathQuads(verts, [strip(0, 2), strip(2, 2)], [0, 1], 2)).toHaveLength(24);
  });
});

describe("per-instance net id matching (WS9)", () => {
  // Two wire primitives share the net NAME "PWR_A" but have distinct per-instance ids.
  const dupKeys: PrimitiveKey[] = [
    { primitive: 0, refDes: "", net: "PWR_A", netId: "aaa", pin: "", busId: "" },
    { primitive: 1, refDes: "", net: "PWR_A", netId: "bbb", pin: "", busId: "" },
  ];

  it("a netIds spec matches only that instance, not its same-named sibling", () => {
    const [g] = resolveHighlights(dupKeys, [{ netIds: ["aaa"] }]);
    expect([...g.primitives]).toEqual([0]);
  });

  it("a name spec still matches every same-named wire (the whole-selection highlight)", () => {
    const [g] = resolveHighlights(dupKeys, [{ nets: ["PWR_A"] }]);
    expect([...g.primitives].sort()).toEqual([0, 1]);
  });

  it("groups same-named instances as separate entities so each frames on its own", () => {
    const [g] = resolveHighlights(dupKeys, [{ netIds: ["aaa", "bbb"] }]);
    expect(g.entities).toHaveLength(2);
  });
});

// A drawing keys a wire by name alone when two of the design's nets share the name (agni issue
// 1443). A finding naming one of them by id then lights that name's wires, as HighlightSheet does.
describe("a net id's name joins a wire keyed by name alone (agni issue 1443)", () => {
  const nameOnly: PrimitiveKey[] = [
    { primitive: 0, refDes: "", net: "PWR_A", netId: "", pin: "", busId: "" },
    { primitive: 1, refDes: "", net: "PWR_A", netId: "", pin: "", busId: "" },
    { primitive: 2, refDes: "", net: "GND", netId: "", pin: "", busId: "" },
  ];
  const dupKeys: PrimitiveKey[] = [
    { primitive: 0, refDes: "", net: "PWR_A", netId: "aaa", pin: "", busId: "" },
    { primitive: 1, refDes: "", net: "PWR_A", netId: "bbb", pin: "", busId: "" },
  ];

  it("lights every wire of the name when the wires carry no id", () => {
    const [g] = resolveHighlights(nameOnly, [{ netIds: ["aaa"], netIdNames: ["PWR_A"] }]);
    expect([...g.primitives].sort()).toEqual([0, 1]);
    expect(g.subjects).toEqual(["net:PWR_A"]);
  });

  it("never joins by name a wire that carries an id, so the focus stays on its instance", () => {
    const [g] = resolveHighlights(dupKeys, [{ netIds: ["aaa"], netIdNames: ["PWR_A"] }]);
    expect([...g.primitives]).toEqual([0]);
  });

  it("lights nothing by id alone on a wire keyed by name, which is why a spec carries the names", () => {
    const [g] = resolveHighlights(nameOnly, [{ netIds: ["aaa"] }]);
    expect([...g.primitives]).toEqual([]);
  });
});

describe("framing a point (agni issue 1374)", () => {
  it("pads a zero-size entity by the caller's floor when it beats the 8-unit one", () => {
    const f = entityFrame(5, 5, 5, 5, 20);
    expect([f.minX, f.minY, f.maxX, f.maxY]).toEqual([-15, -15, 25, 25]);
    expect(entityFrame(5, 5, 5, 5).maxX).toBe(13);
  });
});

// A sheet that places R2 twice, far apart, mirroring twoR2 in core/render/highlight_placement_test.go.
// Placement 3 draws its rect at the origin and placement 4 at (5000, 5000), each with two pins
// (agni issue 1477).
describe("a ref-des placed twice", () => {
  const rect = (x: number, y: number) => [x, y, x + 40, y, x + 40, y + 20, x, y + 20];
  const vertices = Int32Array.from([...rect(0, 0), 0, 10, 40, 10, ...rect(5000, 5000), 5000, 5010, 5040, 5010]);
  const prims: Primitive[] = [
    { kind: 2, group: 0, firstVertex: 0, count: 4 },
    { kind: 3, group: 2, firstVertex: 4, count: 1 },
    { kind: 3, group: 2, firstVertex: 5, count: 1 },
    { kind: 2, group: 0, firstVertex: 6, count: 4 },
    { kind: 3, group: 2, firstVertex: 10, count: 1 },
    { kind: 3, group: 2, firstVertex: 11, count: 1 },
  ];
  const r2 = (primitive: number, pin: string, placement: number): PrimitiveKey => ({ primitive, refDes: "R2", net: "", netId: "", pin, busId: "", placement });
  const twice = [r2(0, "", 3), r2(1, "1", 3), r2(2, "2", 3), r2(3, "", 4), r2(4, "1", 4), r2(5, "2", 4)];
  const contains = (b: { minX: number; minY: number; maxX: number; maxY: number }, x: number, y: number) =>
    x >= b.minX && x <= b.maxX && y >= b.minY && y <= b.maxY;

  it("frames each placement on its own, and neither frame reaches the other placement", () => {
    const [g] = resolveHighlights(twice, [{ components: ["R2"], shape: HighlightShape.BOUNDING_RECT }]);
    expect(g.entities).toHaveLength(2);
    expect(g.subjects).toEqual(["component:R2"]);
    const boxes = g.entities.map((e) => entityBounds(vertices, prims, e)!);
    expect(boxes.map((b) => [b.minX, b.minY, b.maxX, b.maxY])).toEqual([
      [0, 0, 40, 20],
      [5000, 5000, 5040, 5020],
    ]);
    expect(contains(boxes[0], 5000, 5000)).toBe(false);
    expect(contains(boxes[1], 0, 0)).toBe(false);
  });

  it("lights a listed pin on each placement apart", () => {
    const [g] = resolveHighlights(twice, [{ pins: [{ refDes: "R2", pin: "1" }] }]);
    expect(g.entities).toEqual([[1], [4]]);
    expect(g.subjects).toEqual(["pin:R2.1"]);
  });

  it("keeps one frame for a part placed once", () => {
    const [g] = resolveHighlights(keys, [{ components: ["R1"], shape: HighlightShape.BOUNDING_RECT }]);
    expect(g.entities).toEqual([[2, 3, 4]]);
  });
});
