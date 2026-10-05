// The page's side of the engine workers (agni issue 178). `fetch` is what a Connect transport takes in
// place of the network's, so every generated client and every panel drives the in-browser engine
// unchanged, and a design's bytes never leave the page. The workers and their protocol are goapplib's
// wasmhost (agni issue 863); this module adds what is agni's, bringing each design's files in as the
// page first names it, and sending one the browser cannot hold to the server.
//
// The engine runs as two lanes (agni issue 911), each its own worker over its own copy of the design.
// Go's wasm build runs one thread per worker, so a request that runs the rule catalog holds its worker
// for seconds on a large board, and on one worker a query asked meanwhile waited for all of it (#900).
// The jobs lane takes the requests that run rules or compare designs, and the serve lane everything
// else. Aborting a request on a lane ends that lane's worker and starts a fresh one, which is the only
// way to stop a Go job that never yields.
import { startLane, workerMemory, addFiles, type Files, type Lane } from "@panyam/tsappkit/wasmhost";

export type { Files };

// The namespace the wasm build exports under, wasmengine.Namespace on the Go side.
const namespace = "agni";

// A LaneName names one of the engine's workers.
export type LaneName = "serve" | "jobs";

// JOB_RPCS are the requests the jobs lane answers: each runs rules over a whole design or compares
// two, so each can hold a worker for seconds. Everything else is quick once the design is read.
export const JOB_RPCS: ReadonlySet<string> = new Set([
  "agni.v1.webapi.CheckService/CheckDesign",
  "agni.v1.webapi.CheckService/GetCheckReport",
  "agni.v1.webapi.CheckService/GetInterfaceCoverage",
  "agni.v1.webapi.ReviewService/CreateReview",
  "agni.v1.webapi.DiffService/DiffDesigns",
]);

// laneFor is the lane a request's URL goes to, by its Connect procedure (the last two path segments,
// so a page served under a prefix routes the same).
export function laneFor(url: URL): LaneName {
  const procedure = url.pathname.split("/").slice(-2).join("/");
  return JOB_RPCS.has(procedure) ? "jobs" : "serve";
}

// An EngineLane is one worker of the engine, replaced after an abort. Files reach the worker that is
// current when they are added, and a replacement starts without them, so a caller tracking what a
// lane holds keys it on `holder`.
export interface EngineLane {
  fetch(input: RequestInfo | URL, init?: RequestInit): Promise<Response>;
  // holder resolves to the lane's current worker, as an opaque identity.
  holder(): Promise<object>;
  // add puts files into a mount of `holder`'s worker, keeping what it already holds. Each buffer is
  // TRANSFERRED, so it is empty here afterwards.
  add(holder: object, name: string, files: Files): Promise<void>;
  memoryBytes(): Promise<number>;
}

export interface WasmEngine {
  // fetch answers one request on its lane (laneFor), with the network's signature.
  fetch(input: RequestInfo | URL, init?: RequestInit): Promise<Response>;
  // add KEEPS files in a mount for every lane: they are copied into each lane's worker, and into any
  // worker that replaces one, because nothing else can bring them back. It is for files that exist
  // only in the page (agni issue 854); a design on the server comes in per lane, through
  // mountingFetch. The buffers stay usable here.
  add(name: string, files: Files): Promise<void>;
  lanes: Record<LaneName, EngineLane>;
  // memoryBytes is each lane's wasm memory, which never shrinks, so it is the peak so far.
  memoryBytes(): Promise<Record<LaneName, number>>;
}

export interface EngineAssets {
  worker: string;
  wasm: string;
  exec: string;
}

export interface EngineOptions {
  // oneLane runs both kinds of request on one worker, as the engine did before #911. For measuring
  // what the lanes buy; no page sets it.
  oneLane?: boolean;
}

// startEngine starts the lanes and resolves once each has loaded, or rejects with why one could not.
export async function startEngine(assets: EngineAssets, opts: EngineOptions = {}): Promise<WasmEngine> {
  const start = { ...assets, ns: namespace };
  const kept = new Map<string, Files>(); // mount -> path -> bytes, the page's own copy
  const keptIn = new WeakMap<object, Map<string, Set<string>>>(); // worker -> mount -> paths it was given

  // ensureKept copies the kept files a worker has not been given into it.
  const ensureKept = async (w: Worker) => {
    let given = keptIn.get(w);
    if (!given) keptIn.set(w, (given = new Map()));
    for (const [mount, files] of kept) {
      const have = given.get(mount) ?? new Set<string>();
      const want: Files = {};
      for (const [path, bytes] of Object.entries(files)) if (!have.has(path)) want[path] = bytes.slice();
      const paths = Object.keys(want);
      if (!paths.length) continue;
      await addFiles(w, mount, want);
      for (const path of paths) have.add(path);
      given.set(mount, have);
    }
  };

  const engineLane = (lane: Lane): EngineLane => ({
    fetch: async (input, init) => {
      await ensureKept(await lane.worker());
      return lane.fetch(input, init);
    },
    holder: () => lane.worker(),
    add: (holder, name, files) => addFiles(holder as Worker, name, files),
    memoryBytes: async () => workerMemory(await lane.worker()),
  });

  const serve = engineLane(startLane(start));
  const jobs = opts.oneLane ? serve : engineLane(startLane(start));
  const lanes: Record<LaneName, EngineLane> = { serve, jobs };
  await Promise.all([serve.holder(), jobs.holder()]);

  return {
    fetch: (input, init) => lanes[laneFor(new URL(input instanceof Request ? input.url : String(input), location.href))].fetch(input, init),
    add: async (name, files) => {
      kept.set(name, { ...(kept.get(name) ?? {}), ...files });
      await Promise.all(
        [...new Set([serve, jobs])].map(async (l) => {
          // A rewritten path must reach a worker that already has the old one.
          const w = (await l.holder()) as Worker;
          const given = keptIn.get(w)?.get(name);
          for (const path of Object.keys(files)) given?.delete(path);
          await ensureKept(w);
        }),
      );
    },
    lanes,
    memoryBytes: async () => {
      const [s, j] = await Promise.all([serve.memoryBytes(), jobs.memoryBytes()]);
      return { serve: s, jobs: j };
    },
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
// design keeps its server URL under the in-browser engine: before a request reaches a lane, every
// `mount://` URI in its body is listed through `source`, once per design, and the files that lane's
// worker does not already hold, by hash, are fetched and added under the same mount name. Each lane
// brings its own, and a lane's replacement worker brings them again, since it starts empty (#911).
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
  const listings = new Map<string, Promise<DesignFiles | undefined>>(); // uri -> its files, or undefined when it went to the server
  const held = new WeakMap<object, Map<string, Map<string, string>>>(); // worker -> mount -> path -> sha256
  const bringing = new WeakMap<object, Map<string, Promise<void>>>(); // worker -> uri -> the bring under way
  const onServerSide = new Set<string>(); // uris analysed by the server engine
  const toServer = (uri: string, reason: string) => {
    if (onServerSide.has(uri)) return;
    onServerSide.add(uri);
    onServer({ uri, reason });
  };

  const list = (uri: string): Promise<DesignFiles | undefined> => {
    let p = listings.get(uri);
    if (!p) {
      p = (async () => {
        let set: DesignFiles;
        try {
          set = await source.list(uri);
        } catch (err) {
          toServer(uri, `its files could not be listed for the browser (${String(err)})`);
          return undefined;
        }
        const size = Number(set.totalSize);
        if (size > maxBytes) {
          toServer(uri, `its files are ${mb(size)}, over the ${mb(maxBytes)} the browser engine takes`);
          return undefined;
        }
        return set;
      })();
      listings.set(uri, p);
    }
    return p;
  };

  const bring = async (uri: string, lane: EngineLane): Promise<void> => {
    const set = await list(uri);
    if (!set) return;
    const holder = await lane.holder();
    let under = bringing.get(holder);
    if (!under) bringing.set(holder, (under = new Map()));
    let p = under.get(uri);
    if (!p) {
      p = (async () => {
        let mounts = held.get(holder);
        if (!mounts) held.set(holder, (mounts = new Map()));
        const have = mounts.get(set.mount) ?? new Map<string, string>();
        const want = set.files.filter((f) => have.get(f.path) !== f.sha256);
        if (!want.length) return;
        const files: Files = {};
        await Promise.all(
          want.map(async (f) => {
            files[f.path] = await source.fetchFile(set.mount, f.path);
          }),
        );
        await lane.add(holder, set.mount, files);
        for (const f of want) have.set(f.path, f.sha256);
        mounts.set(set.mount, have);
      })().catch((err: unknown) => {
        // Forget a failed bring so the next request retries it. The request itself still goes to
        // the engine, which answers not-found the way the server would.
        under.delete(uri);
        console.warn(`could not bring ${uri} into the browser engine:`, err);
      });
      under.set(uri, p);
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
    const lane = engine.lanes[laneFor(new URL(req.url))];
    await Promise.all([...uris].map((u) => bring(u, lane)));
    if ([...uris].some((u) => onServerSide.has(u))) return network(req);
    return lane.fetch(req);
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
