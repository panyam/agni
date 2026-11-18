// Sending files to a server's design store from the designs page (agni issue 1719). A design's
// files go up the way the CLI's `agni designs push` sends them: a manifest of paths and hashes
// first, then only the contents the server says it lacks, then the same request again. The server
// groups a folder into artifacts (ImportDesignFiles), so this page never decides which file is the
// schematic and which the board.
import { ConnectError } from "@connectrpc/connect";
import { create } from "@bufbuild/protobuf";
import type { Design } from "@panyam/agni-client/gen/agni/v1/webapi/project_pb";
import { SnapshotFileSchema, BlobsMissingSchema, type SnapshotFile } from "@panyam/agni-client/gen/agni/v1/webapi/snapshot_pb";
import type { Artifact, ImportDesignFilesResponse } from "@panyam/agni-client/gen/agni/v1/webapi/designs_pb";

// Picked is files read from a drop or a file input, by their path in what was picked.
export type Picked = Record<string, Uint8Array>;

// Manifest is a picked set as the server is sent it, and the contents by hash for when it asks.
export interface Manifest {
  files: SnapshotFile[];
  byHash: Map<string, Uint8Array>;
}

// UploadsClient is the part of SnapshotService an upload needs.
export interface UploadsClient {
  uploadBlobs(req: { contents: Uint8Array[] }): Promise<unknown>;
}

// uploadBatchBytes bounds one UploadBlobs request, as the CLI's does, so a large board goes in
// several.
export const uploadBatchBytes = 32 << 20;

// stripCommonFolder drops the folder every path shares, so a dropped folder "bb-1.1.3/" sends
// "bb.kicad_sch" rather than "bb-1.1.3/bb.kicad_sch". A loose set of files is left as it is.
export function stripCommonFolder(files: Picked): Picked {
  const paths = Object.keys(files);
  if (paths.length === 0) return files;
  const top = paths[0].split("/")[0];
  if (!paths.every((p) => p.startsWith(top + "/"))) return files;
  const out: Picked = {};
  for (const [p, b] of Object.entries(files)) out[p.slice(top.length + 1)] = b;
  return out;
}

// hidden reports a path the server refuses, a dot-file or one under a dot-folder.
function hidden(path: string): boolean {
  return path.split("/").some((seg) => seg.startsWith("."));
}

async function sha256(b: Uint8Array): Promise<string> {
  const d = new Uint8Array(await crypto.subtle.digest("SHA-256", b as Uint8Array<ArrayBuffer>));
  return "sha256:" + Array.from(d, (x) => x.toString(16).padStart(2, "0")).join("");
}

// manifestOf hashes picked files into the manifest the server is sent. Hidden files are left out,
// since the server refuses them.
export async function manifestOf(files: Picked): Promise<Manifest> {
  const out: Manifest = { files: [], byHash: new Map() };
  for (const path of Object.keys(files).sort()) {
    if (hidden(path)) continue;
    const b = files[path];
    const h = await sha256(b);
    out.files.push(create(SnapshotFileSchema, { path, sha256: h, size: BigInt(b.length) }));
    out.byHash.set(h, b);
  }
  return out;
}

// missingHashes reads the hashes a FAILED_PRECONDITION with a BlobsMissing detail names, and none
// for any other error.
export function missingHashes(err: unknown): string[] {
  const ce = ConnectError.from(err);
  return ce.findDetails(BlobsMissingSchema).flatMap((d) => d.sha256);
}

// withUploads makes a write, and when the server answers that it lacks contents, uploads them from
// the manifest in batches and makes the write once more.
export async function withUploads<T>(uploads: UploadsClient, m: Manifest, write: () => Promise<T>): Promise<T> {
  try {
    return await write();
  } catch (err) {
    const missing = missingHashes(err);
    if (missing.length === 0) throw err;
    await uploadMissing(uploads, m, missing);
    return await write();
  }
}

// uploadMissing sends the contents of the hashes named, from the manifest, in batches.
export async function uploadMissing(uploads: UploadsClient, m: Manifest, missing: string[]): Promise<void> {
  let batch: Uint8Array[] = [];
  let size = 0;
  for (const h of missing) {
    const b = m.byHash.get(h);
    if (!b) throw new Error(`the server asked for contents this upload did not send (${h})`);
    if (size + b.length > uploadBatchBytes && batch.length) {
      await uploads.uploadBlobs({ contents: batch });
      batch = [];
      size = 0;
    }
    batch.push(b);
    size += b.length;
  }
  if (batch.length) await uploads.uploadBlobs({ contents: batch });
}

// DesignsClient is the part of DesignService the library page calls.
export interface DesignsClient {
  importDesignFiles(req: { parent: string; files: SnapshotFile[] }): Promise<ImportDesignFilesResponse>;
  updateArtifact(req: { artifact: { name: string; files: SnapshotFile[] }; allowMissing: boolean }): Promise<Artifact>;
}

// importFolder sends picked files to a design, and the server groups them into artifacts.
export async function importFolder(designs: DesignsClient, uploads: UploadsClient, design: string, files: Picked): Promise<ImportDesignFilesResponse> {
  const m = await manifestOf(stripCommonFolder(files));
  if (m.files.length === 0) throw new Error("nothing to send: every file picked is hidden");
  return withUploads(uploads, m, () => designs.importDesignFiles({ parent: design, files: m.files }));
}

// replaceArtifact replaces one artifact's files whole with picked files.
export async function replaceArtifact(designs: DesignsClient, uploads: UploadsClient, artifact: string, files: Picked): Promise<Artifact> {
  const m = await manifestOf(stripCommonFolder(files));
  if (m.files.length === 0) throw new Error("nothing to send: every file picked is hidden");
  return withUploads(uploads, m, () => designs.updateArtifact({ artifact: { name: artifact, files: m.files }, allowMissing: true }));
}

// SeriesGroup is the designs of one series, or the designs in none under series "".
export interface SeriesGroup {
  series: string;
  designs: Design[];
}

// groupBySeries groups designs by series, named series first in name order, then the designs in no
// series. Order within a group is the listing's.
export function groupBySeries(designs: Design[]): SeriesGroup[] {
  const by = new Map<string, Design[]>();
  for (const d of designs) {
    const k = d.series;
    by.set(k, [...(by.get(k) ?? []), d]);
  }
  const named = [...by.keys()].filter((k) => k !== "").sort();
  const out = named.map((series) => ({ series, designs: by.get(series)! }));
  if (by.has("")) out.push({ series: "", designs: by.get("")! });
  return out;
}

// LibraryRoute is what a /designs/ URL addresses: the list, or one design by id.
export type LibraryRoute = { kind: "list" } | { kind: "design"; id: string };

// DESIGNS_PREFIX is the designs' URL space, the mirror of designsPrefix in cmd/agni/webpage.go.
export const DESIGNS_PREFIX = "/designs/";

// libraryRoute reads a pathname under /designs/.
export function libraryRoute(pathname: string): LibraryRoute {
  const rest = pathname.startsWith(DESIGNS_PREFIX) ? pathname.slice(DESIGNS_PREFIX.length).replace(/\/+$/, "") : "";
  return rest === "" ? { kind: "list" } : { kind: "design", id: decodeURIComponent(rest) };
}

// libraryUrl is a design's page, /designs/<id>.
export function libraryUrl(id: string): string {
  return DESIGNS_PREFIX + encodeURIComponent(id);
}

// designId is the id in "designs/{design}".
export function designId(name: string): string {
  return name.replace(/^designs\//, "");
}
