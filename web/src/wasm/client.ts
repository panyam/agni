// The page's side of the engine worker (agni issue 178). `fetch` is what a Connect transport takes in
// place of the network's, so every generated client and every panel drives the in-browser engine
// unchanged, and a design's bytes never leave the page. The worker and its protocol are goapplib's
// wasmhost (agni issue 863); this module adds what is agni's, bringing each design's files in as the
// page first names it, and sending one the browser cannot hold to the server.
import { addFiles, startWorker, workerFetch, workerMemory, type Files } from "@panyam/tsappkit/wasmhost";

export type { Files };

// The namespace the wasm build exports under, wasmengine.Namespace on the Go side.
const namespace = "agni";

export interface WasmEngine {
  // fetch answers one request in the worker, with the network's signature.
  fetch(input: RequestInfo | URL, init?: RequestInit): Promise<Response>;
  // add puts files into a mount, keeping what it already holds. Each buffer is TRANSFERRED to the
  // worker, so it is empty here afterwards.
  add(name: string, files: Files): Promise<void>;
  // memoryBytes is the engine's wasm memory, which never shrinks, so it is the peak so far.
  memoryBytes(): Promise<number>;
}

export interface EngineAssets {
  worker: string;
  wasm: string;
  exec: string;
}

// startEngine starts the worker and resolves once the engine has loaded, or rejects with why it could
// not.
export async function startEngine(assets: EngineAssets): Promise<WasmEngine> {
  const worker = await startWorker({ ...assets, ns: namespace });
  const fetch = workerFetch(worker);
  return {
    fetch: (input, init) => fetch(input, init),
    add: (name, files) => addFiles(worker, name, files),
    memoryBytes: () => workerMemory(worker),
  };
}

// A DesignFiles lists one design's files, as WorkspaceService.ListDesignFiles answers. The type is
// structural so this module does not import the generated client.
export interface DesignFiles {
  mount: string;
  files: { path: string; sha256: string }[];
  totalSize: bigint | number;
}

// Placement says where one request's design is analysed. A design the worker cannot hold goes to the
// server engine with the reason, so the page can say why (agni issue 852).
export interface ServerFallback {
  uri: string;
  reason: string;
}

export interface DesignSource {
  // list names a design's files, given a `mount://` URI naming the design.
  list(uri: string): Promise<DesignFiles>;
  // fetchFile returns one file's bytes.
  fetchFile(mount: string, path: string): Promise<Uint8Array>;
}

// MountingOptions configures mountingFetch.
export interface MountingOptions {
  engine: WasmEngine;
  source: DesignSource;
  // network is the page's own fetch, for whatever the worker does not answer.
  network: typeof globalThis.fetch;
  // passthrough names requests that go to the network whatever they name, the calls about the
  // server's own mounts.
  passthrough?: (url: URL) => boolean;
  // browserMounts are mounts whose files exist only in the page (agni issue 854). A request naming
  // one goes to the worker, passthrough or not, and its design is never listed on the server, which
  // has never seen it.
  browserMounts?: Set<string>;
  // maxBytes is the largest design the worker takes; a bigger one goes to the server.
  maxBytes?: number;
  // onServer hears, once per design, that it went to the server and why.
  onServer?: (f: ServerFallback) => void;
}

// mountingFetch is the engine's fetch with the design's files brought in first (agni issue 853). A
// design keeps its server URL under the in-browser engine: before a request reaches the worker,
// every `mount://` URI in its body is listed through `source` and the files the worker does not
// already hold, by hash, are fetched and added under the same mount name.
//
// A design whose file set is over `maxBytes`, or whose listing fails, is analysed by the server
// instead, and `onServer` hears why once per design. Every later request naming it goes to the
// network, so one design never has its answers split between two engines.
//
// The body is read as Connect JSON, which is what the viewer's transport sends. A binary body names
// no URI this can see, so it goes to the worker as is.
export function mountingFetch(o: MountingOptions): typeof globalThis.fetch {
  const { engine, source, network } = o;
  const passthrough = o.passthrough ?? (() => false);
  const browserMounts = o.browserMounts ?? new Set<string>();
  const maxBytes = o.maxBytes ?? Number.POSITIVE_INFINITY;
  const onServer = o.onServer ?? (() => {});
  const held = new Map<string, Map<string, string>>(); // mount -> path -> sha256
  const listed = new Map<string, Promise<void>>(); // uri -> the listing that brought it in
  const onServerSide = new Set<string>(); // uris analysed by the server engine
  const toServer = (uri: string, reason: string) => {
    if (onServerSide.has(uri)) return;
    onServerSide.add(uri);
    onServer({ uri, reason });
  };

  const bring = (uri: string): Promise<void> => {
    let p = listed.get(uri);
    if (!p) {
      p = (async () => {
        let set: DesignFiles;
        try {
          set = await source.list(uri);
        } catch (err) {
          toServer(uri, `its files could not be listed for the browser (${String(err)})`);
          return;
        }
        const size = Number(set.totalSize);
        if (size > maxBytes) {
          toServer(uri, `its files are ${mb(size)}, over the ${mb(maxBytes)} the browser engine takes`);
          return;
        }
        const have = held.get(set.mount) ?? new Map<string, string>();
        const want = set.files.filter((f) => have.get(f.path) !== f.sha256);
        if (!want.length) return;
        const files: Files = {};
        await Promise.all(
          want.map(async (f) => {
            files[f.path] = await source.fetchFile(set.mount, f.path);
          }),
        );
        await engine.add(set.mount, files);
        for (const f of want) have.set(f.path, f.sha256);
        held.set(set.mount, have);
      })().catch((err: unknown) => {
        // Forget a failed listing so the next request retries it. The request itself still goes to
        // the engine, which answers not-found the way the server would.
        listed.delete(uri);
        console.warn(`could not bring ${uri} into the browser engine:`, err);
      });
      listed.set(uri, p);
    }
    return p;
  };

  return async (input, init) => {
    const req = new Request(input, init);
    const uris = new Set<string>();
    try {
      collectMountURIs(JSON.parse(await req.clone().text()), uris);
    } catch {
      // not JSON, so nothing to bring in
    }
    const inBrowser = [...uris].some((u) => browserMounts.has(mountOf(u)));
    if (inBrowser) return engine.fetch(req);
    if (passthrough(new URL(req.url))) return network(req);
    await Promise.all([...uris].map(bring));
    if ([...uris].some((u) => onServerSide.has(u))) return network(req);
    return engine.fetch(req);
  };
}

// mountOf is a `mount://` URI's mount name.
export function mountOf(uri: string): string {
  return uri.replace(/^mount:\/\//, "").split("/")[0];
}

// collectMountURIs finds every string value in a decoded JSON body that is a `mount://` URI.
function collectMountURIs(v: unknown, out: Set<string>): void {
  if (typeof v === "string") {
    if (v.startsWith("mount://")) out.add(v);
  } else if (Array.isArray(v)) {
    for (const x of v) collectMountURIs(x, out);
  } else if (v && typeof v === "object") {
    for (const x of Object.values(v)) collectMountURIs(x, out);
  }
}

// mb writes a size for a person, in KB under a megabyte.
function mb(bytes: number): string {
  if (bytes < 1 << 20) return `${Math.ceil(bytes / 1024)} KB`;
  return `${Math.round(bytes / (1 << 20))} MB`;
}
