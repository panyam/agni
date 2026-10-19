// A fake SnapshotSource for the presenters' tests (agni issue 1061). A snapshot's name carries the
// design it names, and the convention when one rides it, so a test asserting which design a request
// asked about still can: `snapshots/mount://m/b.edn`, or `snapshots/mount://m/b.edn@acme`.
import type { OverlayConfig } from "./gen/agni/v1/webapi/checks_pb.js";
import type { Snapshot } from "./gen/agni/v1/webapi/snapshot_pb.js";
import type { SnapshotSource } from "./snapshots.js";

export function snapName(uri: string, overlay?: OverlayConfig): string {
  if (!overlay) return `snapshots/${uri}`;
  const tag = overlay.ignoreProject ? "plain" : (overlay.config?.conventions?.name ?? "overlay");
  return `snapshots/${uri}@${tag}`;
}

// fakeSnapshots names snapshots without asking anything. readBoard answers as a host that read a
// persisted board, unless board says otherwise.
export function fakeSnapshots(board: { read: boolean; persisted: boolean } = { read: true, persisted: true }): SnapshotSource & { boards: string[] } {
  const boards: string[] = [];
  return {
    boards,
    of: async (uri, overlay) => snapName(uri, overlay),
    readBoard: async (uri, overlay) => {
      boards.push(snapName(uri, overlay));
      return { name: snapName(uri, overlay), tiers: { boardRead: board.read, boardPersisted: board.persisted } } as unknown as Snapshot;
    },
  };
}
