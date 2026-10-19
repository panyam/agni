import { describe, expect, it, vi } from "vitest";
import { create } from "@bufbuild/protobuf";
import { Code, ConnectError } from "@connectrpc/connect";
import { OverlayConfigSchema } from "./gen/agni/v1/webapi/checks_pb.js";
import { SnapshotUnavailableSchema, SnapshotUnavailable_Reason, type Snapshot } from "./gen/agni/v1/webapi/snapshot_pb.js";
import { SnapshotRegistry, recoveringInterceptor } from "./snapshots.js";

// A registry over recorded SnapshotService calls, minting ids in order.
function registry() {
  const ingests: unknown[] = [];
  const updates: unknown[] = [];
  let n = 0;
  const calls = {
    ingest: vi.fn(async (req: { snapshotId?: string }) => {
      ingests.push(req);
      return { name: `snapshots/${req.snapshotId}` } as Snapshot;
    }),
    updateSnapshot: vi.fn(async (req: unknown) => {
      updates.push(req);
      return {} as Snapshot;
    }),
  };
  return { reg: new SnapshotRegistry(calls, () => `id${++n}`), calls, ingests, updates };
}

const conv = create(OverlayConfigSchema, { config: { conventions: { name: "acme" } } });

describe("SnapshotRegistry", () => {
  it("ingests a design once per config, under an id the page minted", async () => {
    const { reg, ingests } = registry();
    expect(await reg.of("mount://m/b.edn")).toBe("snapshots/id1");
    expect(await reg.of("mount://m/b.edn")).toBe("snapshots/id1");
    expect(await reg.of("mount://m/b.edn", conv)).toBe("snapshots/id2");
    expect(await reg.of("mount://m/b.edn", create(OverlayConfigSchema, { config: { conventions: { name: "acme" } } }))).toBe("snapshots/id2");
    expect(ingests).toEqual([
      { snapshot: { uri: "mount://m/b.edn", overlay: undefined }, snapshotId: "id1" },
      { snapshot: { uri: "mount://m/b.edn", overlay: conv }, snapshotId: "id2" },
    ]);
    expect(reg.uriOf("snapshots/id2")).toBe("mount://m/b.edn");
  });

  it("forgets an ingest that failed, so the next ask tries again", async () => {
    const { reg, calls } = registry();
    calls.ingest.mockRejectedValueOnce(new Error("unreadable"));
    await expect(reg.of("mount://m/b.edn")).rejects.toThrow("unreadable");
    expect(await reg.of("mount://m/b.edn")).toBe("snapshots/id2");
  });

  it("reads the board under the snapshot it already holds", async () => {
    const { reg, ingests } = registry();
    await reg.readBoard("mount://m/b.edn");
    expect(ingests).toEqual([
      { snapshot: { uri: "mount://m/b.edn", overlay: undefined }, snapshotId: "id1" },
      { snapshot: { uri: "mount://m/b.edn", overlay: undefined }, snapshotId: "id1", readBoard: true },
    ]);
  });

  it("ingests a missing snapshot again under its id, and updates a stale one", async () => {
    const { reg, ingests, updates } = registry();
    const name = await reg.of("mount://m/b.edn", conv);
    expect(await reg.recover(name, SnapshotUnavailable_Reason.MISSING)).toBe(true);
    expect(ingests[1]).toEqual({ snapshot: { uri: "mount://m/b.edn", overlay: conv }, snapshotId: "id1" });
    expect(await reg.recover(name, SnapshotUnavailable_Reason.STALE)).toBe(true);
    expect(updates).toEqual([{ snapshot: { name, uri: "mount://m/b.edn", overlay: conv } }]);
    // control: a snapshot this page never ingested is left alone.
    expect(await reg.recover("snapshots/other", SnapshotUnavailable_Reason.MISSING)).toBe(false);
  });
});

describe("recoveringInterceptor", () => {
  const unavailable = (code: Code, name: string, reason: SnapshotUnavailable_Reason) =>
    new ConnectError("snapshot", code, undefined, [{ desc: SnapshotUnavailableSchema, value: { name, reason } }]);

  it("asks again once the registry has made the snapshot answerable", async () => {
    const { reg, ingests } = registry();
    const name = await reg.of("mount://m/b.edn");
    const next = vi.fn().mockRejectedValueOnce(unavailable(Code.NotFound, name, SnapshotUnavailable_Reason.MISSING)).mockResolvedValueOnce("answer");
    const call = recoveringInterceptor(() => reg)(next as never);
    expect(await call({} as never)).toBe("answer");
    expect(next).toHaveBeenCalledTimes(2);
    expect(ingests).toHaveLength(2);
  });

  it("passes on an error that names no snapshot, and one about a missing file", async () => {
    const { reg } = registry();
    const notFound = new ConnectError("no such file", Code.NotFound);
    const next = vi.fn().mockRejectedValue(notFound);
    await expect(recoveringInterceptor(() => reg)(next as never)({} as never)).rejects.toBe(notFound);
    expect(next).toHaveBeenCalledTimes(1);
  });

  it("asks only once more, so a snapshot that stays missing is reported", async () => {
    const { reg } = registry();
    const name = await reg.of("mount://m/b.edn");
    const err = unavailable(Code.NotFound, name, SnapshotUnavailable_Reason.MISSING);
    const next = vi.fn().mockRejectedValue(err);
    await expect(recoveringInterceptor(() => reg)(next as never)({} as never)).rejects.toBe(err);
    expect(next).toHaveBeenCalledTimes(2);
  });
});
