// Decode a tier-2 PackedSheet (geom_packed.proto) into a shape the WebGL renderer can
// upload directly. The proto envelope carries geometry as two opaque byte blobs:
//   - vertices: int32 LE pairs, sheet-relative: [x0,y0,x1,y1,...]
//   - primitives: fixed-width 12-byte LE records
//       kind:u8, group:u8, _pad:u16, first_vertex:u32, count:u32
// We keep the int32 vertices as an Int32Array VIEW over the decoded bytes so the same
// backing store goes straight to gl.bufferData without a copy.

import { fromBinary } from "@bufbuild/protobuf";
import { PackedSheetSchema, type PackedSheet } from "./gen/agni/v1/geom/geom_packed_pb.js";

// Primitive kinds (geom_packed.proto). GL draw modes are mapped in webgl.ts.
export const KIND_LINE_STRIP = 1;
export const KIND_LINE_LOOP = 2;
export const KIND_POINTS = 3;
// Filled areas as a triangle list (WS7-035): boards forced it in (copper at true width,
// pads, via barrels), but the kind is generic.
export const KIND_TRIANGLES = 4;

// Primitive groups. Colors are chosen per group in webgl.ts.
export const GROUP_SYMBOL = 0;
export const GROUP_WIRE = 1;
export const GROUP_PIN = 2;
// Sheet-level free graphics: junction dots, no-connect markers, notes (geom sheet.Shapes).
export const GROUP_FREE = 3;
// Synthesized worksheet furniture: page frame, zone-ruler ticks, title-block box/dividers.
export const GROUP_FRAME = 4;
// Board strata (WS7-035), mirroring render/packboard.go: the packed board reuses this
// envelope with these groups, so layer visibility is group visibility.
export const GROUP_BOARD_EDGE = 5;
export const GROUP_BOARD_COPPER_FRONT = 6;
export const GROUP_BOARD_COPPER_BACK = 7;
export const GROUP_BOARD_COPPER_INNER = 8;
export const GROUP_BOARD_THROUGH = 9;
export const GROUP_BOARD_HOLE = 10;
export const GROUP_BOARD_SILK = 11;
// Bus trunk/entry (WS7-042), mirroring render/pack.go groupBus: after the board strata in the
// shared flat group space, drawn as true-width triangle quads (GL can't stroke >1px).
export const GROUP_BUS = 12;

// Width of one packed primitive record in bytes.
export const PRIMITIVE_RECORD_SIZE = 12;

export interface Primitive {
  kind: number;
  group: number;
  firstVertex: number;
  count: number;
}

export interface PrimitiveKey {
  primitive: number;
  refDes: string;
  net: string;
  // per-instance net id (geom PrimitiveKey.net_id) for a wire primitive, so a highlight can target
  // one of two nets that share a name (WS9); empty when the solve produced no id.
  netId: string;
  // pin designator (e.g. "3") for a pin primitive, so highlights can target a single pin;
  // empty for symbol/wire primitives.
  pin: string;
  // source id (KiCad uuid, geom PrimitiveKey.bus_id) for a bus trunk/entry primitive, so a
  // bus-not-modeled finding highlights its own bus (WS7-042b); empty for every other primitive.
  busId: string;
}

// A text label in sheet-relative world coordinates (rebased like the vertex blob). The WebGL
// line pipeline draws no glyphs, so labels are rendered by the text overlay (textoverlay.ts).
export interface Label {
  x: number;
  y: number;
  text: string;
  height: number; // world units; the camera scales it to pixels
  rotationDeg: number;
  justify: string; // canonical "<h> <v>"
  color: string; // "#rrggbb"
  maxWidth: number; // world width to condense into (0 = unbounded); see textoverlay
}

// Image is a sheet raster image positioned in world coordinates (x,y = min corner, w/h = span),
// drawn by the overlay above the geometry. href is the data URI built from the packed mime+bytes.
export interface Image {
  x: number;
  y: number;
  w: number;
  h: number;
  href: string;
  rotationDeg: number;
  mirror: boolean;
}

export interface DecodedSheet {
  sheetId: string;
  layoutVersion: number;
  // Sheet min corner. Blob coordinates are relative to this; the renderer works in
  // sheet-local int32 space, so origin is only needed to map back to source units.
  originX: number;
  originY: number;
  // int32 LE pairs [x0,y0,x1,y1,...], a view over the decoded vertex bytes.
  vertices: Int32Array;
  primitives: Primitive[];
  keys: PrimitiveKey[];
  labels: Label[];
  images: Image[];
  // Default font-family for the sheet's text, chosen server-side to match the SVG backend.
  fontFamily: string;
  // Geometry colors indexed by primitive group (server-resolved from render.Style), and the
  // page background, so the WebGL renderer colors from the same palette as the SVG backend.
  groupColors: string[];
  backgroundColor: string;
}

// Interpret a bytes blob as an Int32Array. The proto decoder does not guarantee the
// Uint8Array is 4-byte aligned into its ArrayBuffer, so copy into a fresh buffer when the
// offset is not aligned (an Int32Array view requires a multiple-of-4 byteOffset).
function asInt32Array(bytes: Uint8Array): Int32Array {
  if (bytes.byteLength % 4 !== 0) {
    throw new Error(`vertices blob length ${bytes.byteLength} is not a multiple of 4`);
  }
  if (bytes.byteOffset % 4 === 0) {
    return new Int32Array(bytes.buffer, bytes.byteOffset, bytes.byteLength / 4);
  }
  const copy = bytes.slice(); // fresh, 0-offset buffer
  return new Int32Array(copy.buffer, 0, copy.byteLength / 4);
}

// Parse the fixed-width primitive records with a DataView (little-endian).
function parsePrimitives(bytes: Uint8Array): Primitive[] {
  if (bytes.byteLength % PRIMITIVE_RECORD_SIZE !== 0) {
    throw new Error(
      `primitives blob length ${bytes.byteLength} is not a multiple of ${PRIMITIVE_RECORD_SIZE}`,
    );
  }
  const view = new DataView(bytes.buffer, bytes.byteOffset, bytes.byteLength);
  const n = bytes.byteLength / PRIMITIVE_RECORD_SIZE;
  const out: Primitive[] = new Array(n);
  for (let i = 0; i < n; i++) {
    const off = i * PRIMITIVE_RECORD_SIZE;
    out[i] = {
      kind: view.getUint8(off + 0),
      group: view.getUint8(off + 1),
      // bytes 2..3 are _pad (u16), skipped.
      firstVertex: view.getUint32(off + 4, true),
      count: view.getUint32(off + 8, true),
    };
  }
  return out;
}

// toRenderable maps a decoded PackedSheet message into the renderer's upload-ready shape.
// The RPC path (GetSheet) already holds a PackedSheet message, so it calls this directly;
// decodePackedSheet is the binary-blob entry point (used by the decode tests) on top of it.
export function toRenderable(msg: PackedSheet): DecodedSheet {
  return {
    sheetId: msg.sheetId,
    layoutVersion: msg.layoutVersion,
    originX: Number(msg.originX),
    originY: Number(msg.originY),
    vertices: asInt32Array(msg.vertices),
    primitives: parsePrimitives(msg.primitives),
    keys: msg.keys.map((k) => ({ primitive: k.primitive, refDes: k.refDes, net: k.net, netId: k.netId ?? "", pin: k.pin, busId: k.busId ?? "" })),
    labels: msg.labels.map((l) => ({
      x: l.x,
      y: l.y,
      text: l.text,
      height: l.height,
      rotationDeg: l.rotationDeg,
      justify: l.justify,
      color: l.color,
      maxWidth: l.maxWidth,
    })),
    images: msg.images.map((im) => ({
      x: im.x,
      y: im.y,
      w: im.w,
      h: im.h,
      href: `data:${im.mime || "image/png"};base64,${bytesToBase64(im.data)}`,
      rotationDeg: im.rotationDeg,
      mirror: im.mirror,
    })),
    fontFamily: msg.fontFamily || "monospace",
    groupColors: msg.groupColors,
    backgroundColor: msg.backgroundColor,
  };
}

// bytesToBase64 encodes raw image bytes for a data URI. Chunked so a large image does not blow
// the argument limit of String.fromCharCode(...spread). btoa is available in browsers and Node.
function bytesToBase64(bytes: Uint8Array): string {
  let binary = "";
  const chunk = 0x8000;
  for (let i = 0; i < bytes.length; i += chunk) {
    binary += String.fromCharCode(...bytes.subarray(i, i + chunk));
  }
  return btoa(binary);
}

export function decodePackedSheet(buf: Uint8Array | ArrayBuffer): DecodedSheet {
  const bytes = buf instanceof ArrayBuffer ? new Uint8Array(buf) : buf;
  return toRenderable(fromBinary(PackedSheetSchema, bytes));
}
