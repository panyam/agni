// The store's pages beyond the design list (agni issue 1697): which page a URL addresses, how the
// design list filters, how a report reads as current or stale, and how a pack goes up from a dropped
// project folder. The panels in librarypanels.tsx and storepanels.tsx render these.
import type { Design } from "@panyam/agni-client/gen/agni/v1/webapi/project_pb";
import type { Report } from "@panyam/agni-client/gen/agni/v1/webapi/designs_pb";
import type { Pack } from "@panyam/agni-client/gen/agni/v1/webapi/packs_pb";
import type { SnapshotFile } from "@panyam/agni-client/gen/agni/v1/webapi/snapshot_pb";
import { uriMount, uriPath } from "@agni/web-shared/uri.js";
import { emptyLocation, locationToUrl } from "./router.js";
import { manifestOf, missingHashes, stripCommonFolder, uploadMissing, type Picked, type UploadsClient } from "./libraryupload.js";

// StoreRoute is the page a URL under /designs/, /projects/ or /packs/ addresses.
export type StoreRoute =
  | { kind: "designs" }
  | { kind: "design"; id: string }
  | { kind: "projects" }
  | { kind: "project"; id: string }
  | { kind: "packs" }
  | { kind: "pack"; id: string };

// PROJECTS_PREFIX and PACKS_PREFIX mirror projectsPrefix and packsPrefix in cmd/agni/webpage.go.
export const PROJECTS_PREFIX = "/projects/";
export const PACKS_PREFIX = "/packs/";
const DESIGNS = "/designs/";

// storeRoute reads a pathname. A path under none of the three prefixes is the design list.
export function storeRoute(pathname: string): StoreRoute {
  for (const [prefix, list, one] of [
    [DESIGNS, "designs", "design"],
    [PROJECTS_PREFIX, "projects", "project"],
    [PACKS_PREFIX, "packs", "pack"],
  ] as const) {
    if (!pathname.startsWith(prefix)) continue;
    const rest = pathname.slice(prefix.length).replace(/\/+$/, "");
    return rest === "" ? { kind: list } : { kind: one, id: decodeURIComponent(rest) };
  }
  return { kind: "designs" };
}

// projectUrl and packUrl are one project's and one pack's pages.
export function projectUrl(id: string): string {
  return PROJECTS_PREFIX + encodeURIComponent(id);
}

export function packUrl(id: string): string {
  return PACKS_PREFIX + encodeURIComponent(id);
}

// lastSegment is the id at the end of a resource name, "packs/jetson" giving "jetson".
export function lastSegment(name: string): string {
  return name.slice(name.lastIndexOf("/") + 1);
}

// KEPT_HERE is the design list's filter value for the designs this server stores.
export const KEPT_HERE = "";

// whereOf is where a design is kept: KEPT_HERE for a stored one, or the mount its folder is in.
export function whereOf(d: Design): string {
  if (!d.readOnly) return KEPT_HERE;
  return uriMount(d.uri || d.entryUri);
}

// wheres lists the mounts the designs come from, sorted, for the list's filter.
export function wheres(designs: Design[]): string[] {
  return [...new Set(designs.filter((d) => d.readOnly).map(whereOf))].sort();
}

// DesignFilter narrows the design list. text matches the id, title or series, ignoring case. where
// is KEPT_HERE, a mount's name, or undefined for every design.
export interface DesignFilter {
  text: string;
  where?: string;
}

// filterDesigns keeps the designs f matches, in the order given.
export function filterDesigns(designs: Design[], f: DesignFilter): Design[] {
  const needle = f.text.trim().toLowerCase();
  return designs.filter((d) => {
    if (f.where !== undefined && whereOf(d) !== f.where) return false;
    if (!needle) return true;
    return [d.name.replace(/^designs\//, ""), d.title, d.series].some((s) => s.toLowerCase().includes(needle));
  });
}

// reportState says whether a report ran on the design and packs as they are now.
export function reportState(r: Report): string {
  const said: string[] = [];
  if (r.designChanged) said.push("run on earlier design files");
  for (const p of r.packsChanged) said.push(`${p} changed since`);
  return said.length ? said.join("; ") : "current";
}

// diffUrl opens the viewer on a, compared against b, or "" when either has no file a reader opens.
export function diffUrl(a: Design, b: Design): string {
  if (!a.entryUri || !b.entryUri) return "";
  return locationToUrl({ ...emptyLocation(), mount: uriMount(a.entryUri), path: uriPath(a.entryUri), against: `${uriMount(b.entryUri)}/${uriPath(b.entryUri)}` });
}

// designViewUrl opens the viewer on a stored design read by name under packs, in order (agni issue
// 1697). The server redirects it to the design's current entry.
export function designViewUrl(design: string, packs: string[]): string {
  const q = new URLSearchParams();
  for (const k of packs) q.append("pack", lastSegment(k));
  const qs = q.toString();
  return `${DESIGNS}${encodeURIComponent(design.replace(/^designs\//, ""))}/view${qs ? `?${qs}` : ""}`;
}

// PacksClient is the part of PackService the packs page's upload calls.
export interface PacksClient {
  importPackFiles(req: { packId: string; title: string; files: SnapshotFile[] }): Promise<Pack>;
}

// importRounds bounds how many times an import asks again. The server names project.yaml's contents,
// then the files it names, so two rounds of uploads and a third call are the most one needs.
const importRounds = 4;

// importPack keeps the pack a dropped project folder makes. The server reads its project.yaml and
// asks only for the files the pack holds, so a folder's designs are never sent.
export async function importPack(packs: PacksClient, uploads: UploadsClient, req: { packId: string; title: string }, files: Picked): Promise<Pack> {
  const m = await manifestOf(stripCommonFolder(files));
  if (m.files.length === 0) throw new Error("nothing to send: every file picked is hidden");
  const sent = new Set<string>();
  for (let i = 0; ; i++) {
    try {
      return await packs.importPackFiles({ ...req, files: m.files });
    } catch (err) {
      const missing = missingHashes(err).filter((h) => !sent.has(h));
      if (missing.length === 0 || i + 1 >= importRounds) throw err;
      await uploadMissing(uploads, m, missing);
      for (const h of missing) sent.add(h);
    }
  }
}
