import { describe, it, expect } from "vitest";
import { Code, ConnectError } from "@connectrpc/connect";
import { create } from "@bufbuild/protobuf";
import { BlobsMissingSchema, type SnapshotFile } from "@panyam/agni-client/gen/agni/v1/webapi/snapshot_pb";
import { DesignSchema } from "@panyam/agni-client/gen/agni/v1/webapi/project_pb";
import { ReportSchema } from "@panyam/agni-client/gen/agni/v1/webapi/designs_pb";
import { PackSchema } from "@panyam/agni-client/gen/agni/v1/webapi/packs_pb";
import { diffUrl, importPack, packUrl, projectUrl, reportState, storeRoute, whereOf } from "./storepages.js";
import { moveUp } from "./storepanels.jsx";

const bytes = (s: string) => new TextEncoder().encode(s);

describe("storeRoute", () => {
  it("reads the list or one resource under each of the three prefixes", () => {
    expect(storeRoute("/designs/")).toEqual({ kind: "designs" });
    expect(storeRoute("/designs/bb-1.1.3")).toEqual({ kind: "design", id: "bb-1.1.3" });
    expect(storeRoute("/projects/")).toEqual({ kind: "projects" });
    expect(storeRoute(projectUrl("bench") + "/")).toEqual({ kind: "project", id: "bench" });
    expect(storeRoute("/packs")).toEqual({ kind: "designs" });
    expect(storeRoute(packUrl("jetson"))).toEqual({ kind: "pack", id: "jetson" });
  });
});

describe("whereOf", () => {
  it("names the mount a read-only design is in, and nothing for a stored one", () => {
    expect(whereOf(create(DesignSchema, { uri: "mount://uploads/bb/r", readOnly: false }))).toBe("");
    expect(whereOf(create(DesignSchema, { uri: "mount://tut/designs/g", readOnly: true }))).toBe("tut");
  });
});

describe("reportState", () => {
  it("says current, or what changed since the report ran", () => {
    expect(reportState(create(ReportSchema, {}))).toBe("current");
    expect(reportState(create(ReportSchema, { designChanged: true, packsChanged: ["packs/a"] }))).toBe("run on earlier design files; packs/a changed since");
  });
});

describe("diffUrl", () => {
  it("opens the first design's entry against the second's", () => {
    const a = create(DesignSchema, { entryUri: "mount://uploads/bb-1.1.3/r3/schematic/bb.kicad_sch" });
    const b = create(DesignSchema, { entryUri: "mount://uploads/bb-1.1.2/r2/schematic/bb.kicad_sch" });
    expect(diffUrl(a, b)).toBe("/mounts/uploads/bb-1.1.3/r3/schematic/bb.kicad_sch/view?against=uploads%2Fbb-1.1.2%2Fr2%2Fschematic%2Fbb.kicad_sch");
    expect(diffUrl(a, create(DesignSchema, {}))).toBe("");
  });
});

describe("importPack", () => {
  it("uploads what each answer asks for until the server keeps the pack", async () => {
    const files = { "proj/project.yaml": bytes("name: house\n"), "proj/profiles/can.yaml": bytes("name: can\n"), "proj/designs/g/g.edn": bytes("(edif g)") };
    const sent: SnapshotFile[][] = [];
    const uploaded: string[] = [];
    let calls = 0;
    const packs = {
      importPackFiles: async (req: { files: SnapshotFile[] }) => {
        sent.push(req.files);
        calls++;
        const byPath = new Map(req.files.map((f) => [f.path, f.sha256]));
        if (calls === 1) throw new ConnectError("missing", Code.FailedPrecondition, undefined, [{ desc: BlobsMissingSchema, value: { sha256: [byPath.get("project.yaml")!] } }]);
        if (calls === 2) throw new ConnectError("missing", Code.FailedPrecondition, undefined, [{ desc: BlobsMissingSchema, value: { sha256: [byPath.get("profiles/can.yaml")!] } }]);
        return create(PackSchema, { name: "packs/house" });
      },
    };
    const uploads = {
      uploadBlobs: async (req: { contents: Uint8Array[] }) => {
        uploaded.push(...req.contents.map((b) => new TextDecoder().decode(b)));
      },
    };
    const made = await importPack(packs, uploads, { packId: "", title: "" }, files);
    expect(made.name).toBe("packs/house");
    expect(calls).toBe(3);
    expect(uploaded).toEqual(["name: house\n", "name: can\n"]);
    expect(sent[0].map((f) => f.path)).toEqual(["designs/g/g.edn", "profiles/can.yaml", "project.yaml"]);
  });

  it("passes an error that names no contents on", async () => {
    const packs = {
      importPackFiles: async () => {
        throw new ConnectError("no project.yaml", Code.InvalidArgument);
      },
    };
    await expect(importPack(packs, { uploadBlobs: async () => undefined }, { packId: "", title: "" }, { "a.txt": bytes("a") })).rejects.toThrow(/no project.yaml/);
  });
});

describe("moveUp", () => {
  it("swaps an item with the one before it", () => {
    expect(moveUp(["a", "b", "c"], 2)).toEqual(["a", "c", "b"]);
  });
});
