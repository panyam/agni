import { describe, it, expect } from "vitest";
import { create } from "@bufbuild/protobuf";
import { PackedSheetSchema } from "./gen/agni/v1/geom/geom_packed_pb.js";
import { toRenderable } from "./packed.js";

// toRenderable maps a decoded PackedSheet message (as the GetSheet RPC returns) into the
// renderer's shape: int32 vertex pairs and parsed fixed-width primitive records.
describe("toRenderable", () => {
  it("views vertices as int32 pairs and parses primitive records", () => {
    const vertices = new Uint8Array([0, 0, 0, 0, 100, 0, 0, 0]); // int32 LE: 0, 100
    // one 12-byte record: kind=1, group=1, pad=0, firstVertex=0, count=1
    const primitives = new Uint8Array([1, 1, 0, 0, 0, 0, 0, 0, 1, 0, 0, 0]);
    const msg = create(PackedSheetSchema, { sheetId: "P1", layoutVersion: 1, vertices, primitives });

    const r = toRenderable(msg);
    expect(r.sheetId).toBe("P1");
    expect(Array.from(r.vertices)).toEqual([0, 100]);
    expect(r.primitives).toHaveLength(1);
    expect(r.primitives[0]).toEqual({ kind: 1, group: 1, firstVertex: 0, count: 1 });
  });
});
