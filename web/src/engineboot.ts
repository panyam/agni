// The engine and shell settings both design pages boot with: the viewer (main.ts) and the Load page
// (load.ts), which reads a design into the engine's store before the viewer opens it (agni issue
// 1072). Importing this module reads the shell's base and host into the router, so it is imported
// before anything builds or reads a URL.

import { useEngineFetch, timingMode, workspaceClient } from "./api.js";
import { startEngine, mountingFetch, mountOf, type DesignFiles, type WasmEngine } from "./wasm/client.js";
import { fetchFile } from "./wasm/fetchfile.js";
import { BROWSER_MOUNT } from "./wasm/drop.js";
import { WorkspaceService } from "./gen/agni/v1/webapi/workspace_pb.js";
import { appBase, currentLocation, setBase, setStaticHost } from "./router.js";
import { artifactUri } from "@agni/web-shared/uri.js";
import { dropOf, type DropStore } from "./drops.js";
import { filesStamp, type LoadedKey } from "./loaded.js";
import type { Listing } from "./loadflow.js";

// PageEngineOptions are what a page hears from the engine it starts.
export interface PageEngineOptions {
  // onFetched hears each design file the page fetched for the browser engine, with its size.
  onFetched?: (path: string, bytes: number) => void;
}

// engineParams are the query parameters that choose this page's engine. They ride along on every
// URL the page pushes, so a refresh or a back/forward keeps the engine the visitor asked for.
export const engineParams = (() => {
  const q = new URLSearchParams(window.location.search);
  const out = new URLSearchParams();
  for (const k of ["engine", "wasm-max-bytes"]) {
    const v = q.get(k);
    if (v !== null) out.set(k, v);
  }
  return out;
})();

export function withEngineParams(url: string): string {
  if (!engineParams.size) return url;
  const u = new URL(url, window.location.origin);
  engineParams.forEach((v, k) => u.searchParams.set(k, v));
  return u.pathname + u.search;
}

// pageEngine is the engine this page analyses its design on: the URL's `engine=`, else the one the
// server wrote into the shell (`agni serve --engine`), else the server.
export function pageEngine(): string {
  // A page served as plain files has no server to analyse anything, so it runs in the browser.
  if (onStaticHost) return "wasm";
  return engineParams.get("engine") ?? document.querySelector<HTMLElement>(".app")?.dataset.engine ?? "server";
}

// The shell says where the app is served and whether an agni server is behind it (agni issue 856).
// Both are read before anything builds or reads a URL.
export const shell = document.querySelector<HTMLElement>(".app")?.dataset ?? {};
export const onStaticHost = shell.host === "static";
setBase(shell.base || "/");
setStaticHost(onStaticHost);

// fileURL is where one of a mount's files is fetched from: the server's read-only /raw/ route, or
// the same path under the static site's raw/ folder, which `agni site` writes with that layout.
export function fileURL(mount: string, path: string): URL {
  return new URL(`${appBase()}raw/${encodeURIComponent(mount)}/${path.split("/").map(encodeURIComponent).join("/")}`, location.href);
}

// wasmMaxBytes is the largest design the page analyses in the browser: the URL's `wasm-max-bytes=`,
// else the server's `--wasm-max-bytes` from the shell (agni issue 852).
export function wasmMaxBytes(): number {
  const v = engineParams.get("wasm-max-bytes") ?? document.querySelector<HTMLElement>(".app")?.dataset.wasmMaxBytes;
  const n = Number(v);
  return v && Number.isFinite(n) && n > 0 ? n : Number.POSITIVE_INFINITY;
}

// showEngineNote says, in the top bar, that a design left the engine the page asked for.
export function showEngineNote(text: string): void {
  const el = document.getElementById("engine-note");
  if (!el) return;
  el.textContent = text;
  el.hidden = false;
}

// hideEngineNote clears the top bar's note when it is still about `about`, such as a file whose
// stalled fetch went through on a later attempt.
export function hideEngineNote(about: string): void {
  const el = document.getElementById("engine-note");
  if (el?.textContent?.includes(about)) el.hidden = true;
}

// refuseBrowserMounts answers, on the server engine, every request naming a dropped design with a
// refusal and a note rather than sending it, since the server has never seen those files (agni issue
// 854). Requests about the server's own mounts pass.
export function refuseBrowserMounts(): void {
  const reason = "A dropped design is analysed in the browser and never sent to the server, so it needs the browser engine";
  const network = globalThis.fetch.bind(globalThis);
  if (currentLocation().mount === BROWSER_MOUNT) showEngineNote(`${reason}.`);
  useEngineFetch(async (input, init) => {
    const req = new Request(input, init);
    const body = await req.clone().text();
    if (body.includes(`"mount://${BROWSER_MOUNT}/`) || body.includes(`"mount://${BROWSER_MOUNT}"`)) {
      showEngineNote(`${reason}.`);
      return new Response(JSON.stringify({ code: "failed_precondition", message: reason }), {
        status: 400,
        headers: { "Content-Type": "application/json" },
      });
    }
    return network(req);
  });
}

// SERVE_LANE_MAX_BYTES is the serve worker's memory limit (agni issues 911, 974, 990). A worker that has
// crept past it is replaced, when its live heap says a replacement would come back smaller
// (wouldShrink in wasm/client.ts), by one that restores the open design from the browser's cache. A
// worker that is past it only because the open design is that large is kept.
export const SERVE_LANE_MAX_BYTES = 512 << 20;

// startPageEngine puts the page on the in-browser engine when it runs on wasm (agni issue 853): the engine
// loads in a worker, and every client the page builds afterwards talks to it, bringing in each
// design's files from the server the first time a request names it. The design keeps the URL it has
// under the server engine. Listings and file reads go to the server, which owns the mount namespace.
// It must finish before the app root builds its clients. The assets sit beside this bundle, so the
// page works under any path prefix.
export async function startPageEngine(opts: PageEngineOptions = {}): Promise<WasmEngine | undefined> {
  if (pageEngine() !== "wasm") {
    refuseBrowserMounts();
    return undefined;
  }
  const asset = (name: string) => new URL(name, import.meta.url).href;
  const engine = await startEngine(
    { worker: asset("agni-worker.js"), wasm: asset("agni.wasm"), exec: asset("wasm_exec.js") },
    {
      serveMaxBytes: SERVE_LANE_MAX_BYTES,
      onRestart: (lane, s) =>
        console.info(
          `agni: the ${lane} worker held ${Math.round(s.memoryBytes / (1 << 20))} MB, ${Math.round((s.heap?.heapAlloc ?? 0) / (1 << 20))} MB of it live, and was replaced`,
        ),
    },
  );
  const network = globalThis.fetch.bind(globalThis);
  const timed = timingMode() !== "";
  const files = workspaceClient();
  const workspacePath = `/${WorkspaceService.typeName}/`;
  useEngineFetch(
    mountingFetch({
      engine,
      source: {
        // A static site lists each mount whole, in files/<mount>.json beside its raw/ folder, since a
        // seeded mount is one design and there is no server to ask.
        list: onStaticHost
          ? async (uri) => {
              const res = await network(new URL(`${appBase()}files/${encodeURIComponent(mountOf(uri))}.json`, location.href));
              if (!res.ok) throw new Error(`${mountOf(uri)}: ${res.status}`);
              return (await res.json()) as DesignFiles;
            }
          : (uri) => files.listDesignFiles({ uri }),
        // A transfer that stops sending is abandoned and asked for again, and the top bar says which
        // file is slow, rather than the page waiting on it forever (agni issue 1002).
        fetchFile: (mount, path) =>
          fetchFile(network, fileURL(mount, path), {
            onStall: (s) =>
              showEngineNote(
                `Fetching ${path} stalled after ${Math.round(s.bytes / (1 << 20))} MB; ${s.attempt < s.attempts ? `asking again (${s.attempt + 1} of ${s.attempts})` : "giving up"}.`,
              ),
            onDone: (d) => {
              opts.onFetched?.(path, d.bytes);
              if (d.attempts > 1) hideEngineNote(path);
              if (timed) console.info(`agni fetch ${mount}/${path}: ${JSON.stringify({ bytes: d.bytes, ms: Math.round(d.ms), attempts: d.attempts })}`);
            },
          }),
      },
      network,
      // With no server, workspace calls are the worker's too.
      passthrough: (url) => !onStaticHost && url.pathname.startsWith(workspacePath),
      browserMounts: new Set([BROWSER_MOUNT]),
      maxBytes: wasmMaxBytes(),
      onServer: (f) => showEngineNote(`Analysed on the server: ${f.reason}.`),
    }),
  );
  return engine;
}

// shellBuild is the build that wrote the page, which a record of a load is keyed on, since another
// build may read a file differently and keeps a store of its own.
export function shellBuild(): string {
  return shell.build ?? "";
}

// loadedKey names a load of a design on this page's engine and build.
export function loadedKey(mount: string, path: string): LoadedKey {
  return { engine: pageEngine(), build: shellBuild(), uri: artifactUri(mount, path) };
}

// designListing is what a design's files hold now: the stamp a record of its load must match, and
// how many files and bytes there are. A dropped design is named by its drop's last change, since its
// files are the page's own; it is undefined for a drop this browser no longer keeps. A design on a
// static host is named by its mount's listing, which `agni site` writes whole.
export async function designListing(mount: string, path: string, drops: DropStore): Promise<Listing | undefined> {
  if (mount === BROWSER_MOUNT) {
    const d = await drops.get(dropOf(path));
    return d ? { stamp: `drop ${d.id} ${d.updatedAt}`, files: d.files, bytes: d.bytes } : undefined;
  }
  if (onStaticHost) {
    const res = await globalThis.fetch(new URL(`${appBase()}files/${encodeURIComponent(mount)}.json`, location.href));
    if (!res.ok) throw new Error(`the listing of ${mount} answered ${res.status}`);
    const l = (await res.json()) as DesignFiles;
    return { stamp: filesStamp(l.files ?? []), files: l.files?.length ?? 0, bytes: Number(l.totalSize ?? 0) };
  }
  const resp = await workspaceClient().listDesignFiles({ uri: artifactUri(mount, path) });
  return { stamp: filesStamp(resp.files), files: resp.files.length, bytes: Number(resp.totalSize) };
}
