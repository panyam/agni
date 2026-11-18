import { describe, it, expect } from "vitest";
import { Code, ConnectError } from "@connectrpc/connect";
import { create } from "@bufbuild/protobuf";
import { BlobsMissingSchema } from "@panyam/agni-client/gen/agni/v1/webapi/snapshot_pb";
import { DesignSchema } from "@panyam/agni-client/gen/agni/v1/webapi/project_pb";
import { groupBySeries, importFolder, libraryRoute, libraryUrl, manifestOf, stripCommonFolder, withUploads, type Picked } from "./libraryupload.js";

const bytes = (s: string) => new TextEncoder().encode(s);

function blobsMissing(hashes: string[]): ConnectError {
  return new ConnectError("contents missing", Code.FailedPrecondition, undefined, [{ desc: BlobsMissingSchema, value: { sha256: hashes } }]);
}

describe("stripCommonFolder", () => {
  it("drops the folder a dropped folder's files share, and leaves loose files alone", () => {
    expect(Object.keys(stripCommonFolder({ "bb/a.kicad_sch": bytes("a"), "bb/sub/b.kicad_sch": bytes("b") })).sort()).toEqual(["a.kicad_sch", "sub/b.kicad_sch"]);
    expect(Object.keys(stripCommonFolder({ "a.edn": bytes("a"), "bb/b.edn": bytes("b") })).sort()).toEqual(["a.edn", "bb/b.edn"]);
  });
});

describe("manifestOf", () => {
  it("hashes each file and leaves hidden ones out", async () => {
    const m = await manifestOf({ "a.edn": bytes("abc"), ".git/config": bytes("x"), "sub/.hidden": bytes("y") });
    expect(m.files.map((f) => f.path)).toEqual(["a.edn"]);
    // sha256("abc")
    expect(m.files[0].sha256).toBe("sha256:ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad");
    expect(m.files[0].size).toBe(3n);
    expect(m.byHash.get(m.files[0].sha256)).toEqual(bytes("abc"));
  });
});

describe("withUploads", () => {
  it("uploads the contents the server asks for, then writes again", async () => {
    const m = await manifestOf({ "a.edn": bytes("a"), "b.edn": bytes("b") });
    const asked = [m.files[1].sha256];
    const sent: Uint8Array[][] = [];
    let calls = 0;
    const got = await withUploads({ uploadBlobs: async ({ contents }) => sent.push(contents) }, m, async () => {
      calls++;
      if (calls === 1) throw blobsMissing(asked);
      return "written";
    });
    expect(got).toBe("written");
    expect(calls).toBe(2);
    expect(sent).toEqual([[bytes("b")]]);
  });

  it("passes any other error through without uploading", async () => {
    const m = await manifestOf({ "a.edn": bytes("a") });
    let uploads = 0;
    await expect(
      withUploads({ uploadBlobs: async () => uploads++ }, m, async () => {
        throw new ConnectError("no such design", Code.NotFound);
      }),
    ).rejects.toThrow("no such design");
    expect(uploads).toBe(0);
  });
});

describe("importFolder", () => {
  it("sends a dropped folder's files by their paths inside it", async () => {
    const seen: string[][] = [];
    const files: Picked = { "bb-1.1.3/bb.kicad_sch": bytes("s"), "bb-1.1.3/bb.kicad_pcb": bytes("p") };
    await importFolder(
      {
        importDesignFiles: async (req) => {
          seen.push(req.files.map((f) => f.path));
          return { artifacts: [], unread: [] } as never;
        },
        updateArtifact: async () => ({}) as never,
      },
      { uploadBlobs: async () => undefined },
      "designs/bb",
      files,
    );
    expect(seen).toEqual([["bb.kicad_pcb", "bb.kicad_sch"]]);
  });
});

describe("groupBySeries", () => {
  it("puts named series first, by name, then the designs in none", () => {
    const d = (name: string, series: string) => create(DesignSchema, { name, series });
    const groups = groupBySeries([d("designs/x", ""), d("designs/b2", "b"), d("designs/a1", "a"), d("designs/b1", "b")]);
    expect(groups.map((g) => [g.series, g.designs.map((x) => x.name)])).toEqual([
      ["a", ["designs/a1"]],
      ["b", ["designs/b2", "designs/b1"]],
      ["", ["designs/x"]],
    ]);
  });
});

describe("libraryRoute", () => {
  it("reads the list and a design from the URL, and libraryUrl writes one back", () => {
    expect(libraryRoute("/designs/")).toEqual({ kind: "list" });
    expect(libraryRoute("/designs")).toEqual({ kind: "list" });
    expect(libraryRoute("/designs/bb-1.1.3")).toEqual({ kind: "design", id: "bb-1.1.3" });
    expect(libraryRoute(libraryUrl("bb-1.1.3") + "/")).toEqual({ kind: "design", id: "bb-1.1.3" });
  });
});
