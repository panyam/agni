// Interop check against the Go packer (render.PackSheet). Decodes the committed
// sample.pb fixture (produced by `agni render edif/testdata/sample.eds --format pack
// --sheet 0`) and asserts the decoded shape matches the ground-truth content, including the
// sheet-level free graphics (GROUP_FREE) the Go packer now emits from sheet.Shapes. Also
// decodes a hand-built PackedSheet to lock the byte-record parsing independent of the fixture.

import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { dirname, join } from "node:path";
import { describe, it, expect } from "vitest";
import { create, toBinary } from "@bufbuild/protobuf";
import { PackedSheetSchema } from "./gen/agni/v1/geom/geom_packed_pb.js";
import {
  decodePackedSheet,
  KIND_LINE_STRIP,
  KIND_LINE_LOOP,
  KIND_POINTS,
  KIND_TRIANGLES,
  GROUP_SYMBOL,
  GROUP_WIRE,
  GROUP_PIN,
  GROUP_FREE,
  GROUP_FRAME,
  GROUP_BUS,
} from "./packed.js";

const here = dirname(fileURLToPath(import.meta.url));

describe("decodePackedSheet — sample.pb fixture from the Go packer", () => {
  const bytes = readFileSync(join(here, "..", "sample.pb"));
  const sheet = decodePackedSheet(new Uint8Array(bytes));

  it("decodes the sheet header", () => {
    expect(sheet.sheetId).toBe("P1");
    expect(sheet.layoutVersion).toBe(1);
    expect(sheet.originX).toBe(1);
    expect(sheet.originY).toBe(2);
  });

  it("exposes int32 vertices as flat pairs", () => {
    expect(sheet.vertices).toBeInstanceOf(Int32Array);
    expect(sheet.vertices.length).toBe(116); // 58 vertices, x/y interleaved
    // First few vertices are the worksheet frame border's corners, verified against the blob.
    expect(Array.from(sheet.vertices.slice(0, 6))).toEqual([15, 14, 983, 14, 983, 782]);
  });

  it("parses the fixed-width primitive records", () => {
    expect(sheet.primitives.length).toBe(25);
    // The synthesized worksheet furniture is packed first (behind the schematic): the frame
    // border, then zone-ruler ticks and the title-block box/dividers, all under GROUP_FRAME.
    expect(sheet.primitives[0]).toEqual({
      kind: KIND_LINE_LOOP,
      group: GROUP_FRAME,
      firstVertex: 0,
      count: 4,
    });
    const frame = sheet.primitives.filter((p) => p.group === GROUP_FRAME).length;
    expect(frame).toBe(19); // 1 frame + 16 ruler ticks + 1 title box + 1 divider
    // The schematic content still packs after the furniture: wire, free graphic, symbol, pin.
    expect(sheet.primitives.some((p) => p.group === GROUP_WIRE)).toBe(true);
    expect(sheet.primitives.some((p) => p.group === GROUP_FREE && p.kind === KIND_LINE_LOOP)).toBe(true);
    expect(sheet.primitives.some((p) => p.group === GROUP_SYMBOL)).toBe(true);
    expect(sheet.primitives[sheet.primitives.length - 1]).toEqual({
      kind: KIND_POINTS,
      group: GROUP_PIN,
      firstVertex: 57,
      count: 1,
    });
  });

  it("carries the primitive keys (furniture and free graphics carry none)", () => {
    // 25 primitives but 5 keys: wire + 3 symbol + pin. Worksheet furniture and free graphics
    // have no ref_des/net, so they get no key.
    expect(sheet.keys.length).toBe(5);
    const keyed = new Set(sheet.keys.map((k) => k.primitive));
    const framed = sheet.primitives.flatMap((p, i) => (p.group === GROUP_FRAME ? [i] : []));
    expect(framed.some((i) => keyed.has(i))).toBe(false);
  });

  it("decodes the text labels for the overlay", () => {
    // Zone ruler (1..6 x2, A..D x2) + title-block rows + HELLO + R1 = 24 labels.
    expect(sheet.labels.length).toBe(24);
    const title = sheet.labels.find((l) => l.text === "Title: Sheet 1");
    expect(title).toBeDefined();
    expect(title?.color).toBe("#333333"); // title-block text color, matching the SVG backend
    // R1 is a ref-des field, rotated with its symbol, in the field color.
    const r1 = sheet.labels.find((l) => l.text === "R1");
    expect(r1?.color).toBe("#1560bd");
    expect(r1?.rotationDeg).toBe(90);
  });

  it("decodes the geometry palette and background from the server Style", () => {
    // Indexed by group: 0 symbol, 1 wire, 2 pin, 3 free, 4 frame (render.DefaultStyle).
    expect(sheet.groupColors).toEqual(["#222222", "#0a7d2c", "#dd1111", "#333333", "#8a6d3b"]);
    expect(sheet.backgroundColor).toBe("#fdfdfb");
  });
});

describe("decodePackedSheet — hand-built PackedSheet", () => {
  it("round-trips origin, vertices, and records", () => {
    // Build vertices [10,20, 30,40] as int32 LE.
    const vbuf = new ArrayBuffer(16);
    new Int32Array(vbuf).set([10, 20, 30, 40]);

    // One LINE_STRIP record over both vertices: kind=1, group=1, pad, first=0, count=2.
    const pbuf = new ArrayBuffer(12);
    const pv = new DataView(pbuf);
    pv.setUint8(0, KIND_LINE_STRIP);
    pv.setUint8(1, GROUP_WIRE);
    pv.setUint16(2, 0, true);
    pv.setUint32(4, 0, true);
    pv.setUint32(8, 2, true);

    const msg = create(PackedSheetSchema, {
      sheetId: "S",
      layoutVersion: 7,
      originX: 1000n,
      originY: -2000n,
      vertices: new Uint8Array(vbuf),
      primitives: new Uint8Array(pbuf),
      keys: [{ primitive: 0, refDes: "", net: "N1" }],
    });
    const encoded = toBinary(PackedSheetSchema, msg);

    const sheet = decodePackedSheet(encoded);
    expect(sheet.sheetId).toBe("S");
    expect(sheet.layoutVersion).toBe(7);
    expect(sheet.originX).toBe(1000);
    expect(sheet.originY).toBe(-2000);
    expect(Array.from(sheet.vertices)).toEqual([10, 20, 30, 40]);
    expect(sheet.primitives).toEqual([
      { kind: KIND_LINE_STRIP, group: GROUP_WIRE, firstVertex: 0, count: 2 },
    ]);
    expect(sheet.keys[0]).toEqual({ primitive: 0, refDes: "", net: "N1", netId: "", pin: "", busId: "" });
  });

  it("decodes a bus triangle primitive in GROUP_BUS (WS7-042)", () => {
    expect(GROUP_BUS).toBe(12);
    // A bus quad: kind=TRIANGLES, group=GROUP_BUS, 6 vertices (two triangles).
    const pbuf = new ArrayBuffer(12);
    const pv = new DataView(pbuf);
    pv.setUint8(0, KIND_TRIANGLES);
    pv.setUint8(1, GROUP_BUS);
    pv.setUint16(2, 0, true);
    pv.setUint32(4, 0, true);
    pv.setUint32(8, 6, true);

    const msg = create(PackedSheetSchema, {
      sheetId: "S",
      primitives: new Uint8Array(pbuf),
      // Sparse palette: only the bus slot (12) needs a color on a bus sheet.
      groupColors: ["", "", "", "", "", "", "", "", "", "", "", "", "#1a4de0"],
    });
    const sheet = decodePackedSheet(toBinary(PackedSheetSchema, msg));
    expect(sheet.primitives).toEqual([
      { kind: KIND_TRIANGLES, group: GROUP_BUS, firstVertex: 0, count: 6 },
    ]);
    expect(sheet.groupColors[GROUP_BUS]).toBe("#1a4de0");
  });
});
