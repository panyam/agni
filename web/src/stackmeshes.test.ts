import { describe, expect, it } from "vitest";
import { create } from "@bufbuild/protobuf";
import { BoardPartBodySchema, BodyModelSchema } from "@panyam/agni-client/gen/agni/v1/geom/geom_packed_pb";
import { MeshPartSchema, ModelMeshSchema, UnreadModelSchema } from "@panyam/agni-client/gen/agni/v1/geom/geom_board_pb";
import { GetBoardMeshesResponseSchema } from "@panyam/agni-client/gen/agni/v1/webapi/design_view_pb";
import { defaultQuality, fetchMeshes, MESH_FLOATS, meshGeometry, meshNote, type MeshSet } from "./stackmeshes.js";

// A one-triangle mesh in red, half transparent.
function tri(key: string) {
  return create(ModelMeshSchema, {
    key,
    sourceTriangles: 1,
    parts: [create(MeshPartSchema, { positions: [0, 0, 0, 1, 0, 0, 0, 1, 0], indices: [0, 1, 2], rgba: 0xff000080 })],
  });
}

// A transform that moves a mesh point by (dx, dy) board units and leaves z alone.
function moved(dx: number, dy: number): number[] {
  return [1, 0, 0, dx, 0, 1, 0, dy, 0, 0, 1, 0];
}

function body(ref: string, side: string, ...keys: string[]) {
  return create(BoardPartBodySchema, {
    refDes: ref,
    side,
    outline: [0, 0, 1, 0, 1, 1],
    models: keys.map((k, i) => create(BodyModelSchema, { mesh: k, transform: moved(100 + i, 200) })),
  });
}

function set(...keys: string[]): MeshSet {
  return { meshes: keys.map(tri), simplified: [], blocks: [], unread: [], triangles: keys.length };
}

describe("the part models", () => {
  it("draws each mesh once, with an instance per part that shows it", () => {
    const g = meshGeometry([body("C1", "front", "a.wrl"), body("C2", "back", "a.wrl"), body("U1", "front", "b.wrl")], set("a.wrl", "b.wrl"), 10, 20);
    expect(g.draws.map((d) => d.key)).toEqual(["a.wrl", "b.wrl"]);
    const a = g.draws[0];
    expect(a.refs).toEqual(["C1", "C2"]);
    expect(a.instances.length).toBe(2 * MESH_FLOATS);
    // The translation rebases to the packed sheet's origin, as the bodies' outlines do.
    expect(a.instances[3]).toBe(90);
    expect(a.instances[7]).toBe(180);
    // The 13th float says the part sits on the back.
    expect([a.instances[12], a.instances[MESH_FLOATS + 12]]).toEqual([0, 1]);
    expect(Array.from(a.colors.slice(0, 4))).toEqual([255, 0, 0, 128]);
    expect(g.meshed).toEqual(new Set(["C1", "C2", "U1"]));
    expect(g.triangles).toBe(3);
    expect(g.instances).toBe(3);
  });

  it("leaves a part whose mesh did not come as a block", () => {
    const g = meshGeometry([body("C1", "front", "a.wrl"), body("J1", "front", "gone.wrl")], set("a.wrl"), 0, 0);
    expect(g.meshed.has("J1")).toBe(false);
    expect(g.draws.map((d) => d.key)).toEqual(["a.wrl"]);
  });

  it("joins a mesh's parts, each index moved past the parts before it", () => {
    const m = create(ModelMeshSchema, {
      key: "two.wrl",
      parts: [
        create(MeshPartSchema, { positions: [0, 0, 0, 1, 0, 0, 0, 1, 0], indices: [0, 1, 2], rgba: 0x000000ff }),
        create(MeshPartSchema, { positions: [0, 0, 1, 1, 0, 1, 0, 1, 1], indices: [2, 1, 0], rgba: 0xffffffff }),
      ],
    });
    const g = meshGeometry([body("U1", "front", "two.wrl")], { ...set(), meshes: [m] }, 0, 0);
    expect(Array.from(g.draws[0].indices)).toEqual([0, 1, 2, 5, 4, 3]);
    expect(Array.from(g.draws[0].colors.slice(12, 16))).toEqual([255, 255, 255, 255]);
  });
});

describe("the models' budget", () => {
  it("picks a quality from the device's memory", () => {
    expect(defaultQuality(1)).toBe("low");
    expect(defaultQuality(2)).toBe("low");
    expect(defaultQuality(4)).toBe("medium");
    expect(defaultQuality(8)).toBe("high");
    expect(defaultQuality(undefined)).toBe("medium");
  });

  it("says what the server left out", () => {
    expect(meshNote(set("a.wrl"))).toBe("");
    const s: MeshSet = { ...set("a.wrl"), simplified: ["big.wrl"], blocks: ["x.wrl", "y.wrl"], unread: [{ path: "z.step", reason: "not found", parts: 3 }] };
    expect(meshNote(s)).toBe("(models: 1 simplified, 2 as blocks, 3 with no mesh)");
  });

  it("pages through GetBoardMeshes, taking the summary from the first page", async () => {
    const pages = [
      create(GetBoardMeshesResponseSchema, {
        meshes: [tri("a.wrl")],
        simplified: ["a.wrl"],
        unread: [create(UnreadModelSchema, { path: "z.step", reason: "not found", parts: 2 })],
        triangles: 2n,
        nextPageToken: "p2",
      }),
      create(GetBoardMeshesResponseSchema, { meshes: [tri("b.wrl")] }),
    ];
    const asked: string[] = [];
    const got = await fetchMeshes(async (token) => {
      asked.push(token);
      return pages[asked.length - 1];
    });
    expect(asked).toEqual(["", "p2"]);
    expect(got.meshes.map((m) => m.key)).toEqual(["a.wrl", "b.wrl"]);
    expect(got.simplified).toEqual(["a.wrl"]);
    expect(got.unread).toEqual([{ path: "z.step", reason: "not found", parts: 2 }]);
    expect(got.triangles).toBe(2);
  });
});
