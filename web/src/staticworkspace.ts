// The file trees' two listings on a static host (agni issue 978). A static site has no server to list
// a mount, and the page's engine holds only the files it was given, so asking it which mounts exist
// answers none and the Compare picker opened empty. `agni site` writes what the server's
// WorkspaceService would answer instead: mounts.json, its ListMounts, and dirs/<mount>.json, its
// ListDir of every folder in the mount, one folder's entries after another in one listing. This
// answers ListMounts and ListDir from those files and passes every other request on. A mount the
// site does not carry, such as the browser mount a dropped design is read from, is still the page
// engine's to list.

import { create, fromJson, toJsonString, type JsonValue } from "@bufbuild/protobuf";
import { uriMount, uriPath } from "@agni/web-shared/uri.js";
import {
  FileKind,
  ListDirRequestSchema,
  ListDirResponseSchema,
  ListMountsRequestSchema,
  ListMountsResponseSchema,
  WorkspaceService,
  type DirEntry,
  type ListMountsResponse,
} from "./gen/agni/v1/webapi/workspace_pb.js";

// SiteRead fetches one of the site's files by its path under the site's base and returns its JSON.
// It throws when the file is missing.
export type SiteRead = (path: string) => Promise<unknown>;

// staticWorkspace answers WorkspaceService.ListMounts and ListDir from the files `agni site` wrote,
// read through read, on the terms the server's ListMounts and ListDir use: entries in the server's
// order, and with `opens` set, a folder or mount left out when nothing beneath it is a file of a
// kind the caller opens. ListMounts adds the mounts next lists that the site does not carry, and a
// ListDir in one of those goes to next, as does every other request. Each file is read once.
export function staticWorkspace(read: SiteRead, next: typeof globalThis.fetch): typeof globalThis.fetch {
  let mounts: Promise<ListMountsResponse> | undefined;
  const trees = new Map<string, Promise<DirEntry[]>>();
  const mountList = () =>
    (mounts ??= read("mounts.json").then((j) => fromJson(ListMountsResponseSchema, j as JsonValue, { ignoreUnknownFields: true })));
  const tree = (mount: string): Promise<DirEntry[]> => {
    let p = trees.get(mount);
    if (!p) {
      p = read(`dirs/${encodeURIComponent(mount)}.json`).then((j) => fromJson(ListDirResponseSchema, j as JsonValue, { ignoreUnknownFields: true }).entries);
      // A failed read is not kept, so the next listing asks again.
      p.catch(() => trees.delete(mount));
      trees.set(mount, p);
    }
    return p;
  };

  const service = `/${WorkspaceService.typeName}/`;
  return async (input, init) => {
    const req = new Request(input, init);
    const at = new URL(req.url).pathname;
    const i = at.lastIndexOf(service);
    const method = i >= 0 ? at.slice(i + service.length) : "";
    if (method !== "ListMounts" && method !== "ListDir") return next(req);
    const body = (await req.clone().text()) || "{}";
    try {
      if (method === "ListMounts") {
        const r = fromJson(ListMountsRequestSchema, JSON.parse(body) as JsonValue, { ignoreUnknownFields: true });
        const [all, engine] = await Promise.all([mountList(), engineMounts(next, req)]);
        const opens = opensSet(r.opens);
        const kept = opens ? await filter(all.mounts, async (m) => holdsOpenable(await tree(m.name), "", opens)) : all.mounts;
        const carried = new Set(all.mounts.map((m) => m.name));
        const theirs = engine.mounts.filter((m) => !carried.has(m.name));
        return answer(
          toJsonString(
            ListMountsResponseSchema,
            create(ListMountsResponseSchema, { mounts: [...kept, ...theirs], prunedMounts: all.mounts.length - kept.length + engine.prunedMounts }),
          ),
        );
      }
      const r = fromJson(ListDirRequestSchema, JSON.parse(body) as JsonValue, { ignoreUnknownFields: true });
      const mount = uriMount(r.uri);
      if (!(await mountList()).mounts.some((m) => m.name === mount)) return next(req);
      const all = await tree(mount);
      const dir = uriPath(r.uri).replace(/\/+$/, "");
      if (dir !== "" && !all.some((e) => e.isDir && uriPath(e.uri) === dir)) return refuse("not_found", `${r.uri} is not a folder`);
      const opens = opensSet(r.opens);
      const entries = all.filter((e) => parentOf(uriPath(e.uri)) === dir && (!opens || !e.isDir || holdsOpenable(all, uriPath(e.uri), opens)));
      return answer(toJsonString(ListDirResponseSchema, create(ListDirResponseSchema, { entries })));
    } catch (err) {
      return refuse("unavailable", `the site's listing could not be read (${String(err)})`);
    }
  };
}

// engineMounts is what next answers req with, the page engine's mounts, or none when it cannot say,
// since a failure there must not hide the site's own.
async function engineMounts(next: typeof globalThis.fetch, req: Request): Promise<ListMountsResponse> {
  try {
    const res = await next(req);
    if (res.ok) return fromJson(ListMountsResponseSchema, (await res.json()) as JsonValue, { ignoreUnknownFields: true });
  } catch {
    // Answered below.
  }
  return create(ListMountsResponseSchema, {});
}

// opensSet is a request's declared kinds, or undefined for no filter, with UNSPECIFIED dropped as the
// server drops it.
function opensSet(kinds: FileKind[]): Set<FileKind> | undefined {
  const set = new Set(kinds.filter((k) => k !== FileKind.UNSPECIFIED));
  return set.size ? set : undefined;
}

// holdsOpenable reports whether any file under dir ("" for the mount root) is of a kind in opens.
function holdsOpenable(all: DirEntry[], dir: string, opens: Set<FileKind>): boolean {
  const under = dir === "" ? "" : `${dir}/`;
  return all.some((e) => !e.isDir && opens.has(e.kind) && uriPath(e.uri).startsWith(under));
}

function parentOf(path: string): string {
  const slash = path.lastIndexOf("/");
  return slash < 0 ? "" : path.slice(0, slash);
}

async function filter<T>(xs: T[], keep: (x: T) => Promise<boolean>): Promise<T[]> {
  const kept = await Promise.all(xs.map(keep));
  return xs.filter((_, i) => kept[i]);
}

function answer(json: string): Response {
  return new Response(json, { status: 200, headers: { "Content-Type": "application/json" } });
}

// refuse answers with a Connect error, so the client reports it as the server's would be.
function refuse(code: "not_found" | "unavailable", message: string): Response {
  return new Response(JSON.stringify({ code, message }), {
    status: code === "not_found" ? 404 : 503,
    headers: { "Content-Type": "application/json" },
  });
}
