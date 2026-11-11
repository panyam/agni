import { describe, expect, it } from "vitest";
import { create, fromBinary, toBinary } from "@bufbuild/protobuf";
import { IngestRequestSchema } from "./gen/agni/v1/webapi/snapshot_pb.js";
import { ListMountsResponseSchema } from "./gen/agni/v1/webapi/workspace_pb.js";
import { answer, requestJson } from "./wirecodec.js";

const INGEST = "http://h/agni.v1.webapi.SnapshotService/Ingest";
const ingest = create(IngestRequestSchema, { snapshot: { uri: "mount://m/d" }, snapshotId: "s1", readBoard: true });

// agni issue 1502: a request reads the same in either Connect codec.
describe("wirecodec", () => {
  it("reads a binary body and a JSON body to one JSON form", async () => {
    const bin = new Request(INGEST, { method: "POST", headers: { "content-type": "application/proto" }, body: toBinary(IngestRequestSchema, ingest) });
    const json = new Request(INGEST, { method: "POST", headers: { "content-type": "application/json" }, body: JSON.stringify({ snapshot: { uri: "mount://m/d" }, snapshotId: "s1", readBoard: true }) });
    const want = { snapshot: { uri: "mount://m/d" }, snapshotId: "s1", readBoard: true };
    expect(await requestJson(bin)).toEqual(want);
    expect(await requestJson(json)).toEqual(want);
  });

  it("reads nothing from a binary body whose procedure the viewer does not call", async () => {
    const req = new Request("http://h/other.Service/Do", { method: "POST", headers: { "content-type": "application/proto" }, body: new Uint8Array([10, 1, 65]) });
    expect(await requestJson(req)).toBeUndefined();
  });

  it("answers in the codec the request came in", async () => {
    const msg = create(ListMountsResponseSchema, { mounts: [{ name: "m" }], prunedMounts: 2 });
    const bin = answer(new Request("http://h/x", { method: "POST", headers: { "content-type": "application/proto" } }), ListMountsResponseSchema, msg);
    expect(bin.headers.get("content-type")).toBe("application/proto");
    expect(fromBinary(ListMountsResponseSchema, new Uint8Array(await bin.arrayBuffer())).prunedMounts).toBe(2);
    const json = answer(new Request("http://h/x", { method: "POST", headers: { "content-type": "application/json" } }), ListMountsResponseSchema, msg);
    expect(await json.json()).toEqual({ mounts: [{ name: "m" }], prunedMounts: 2 });
  });
});
