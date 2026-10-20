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
//
// A third lane, prepare, loads a design ahead of its checks (agni issue 1061). Each request there gets
// a worker of its own that ends once it has answered, so the board's parse leaves its peak memory in
// no worker that lives on, and the jobs worker restores the parsed board from the shared store. Jobs
// requests wait while a prepare is under way, so the two never parse one board side by side.
import { startLane, startWorker, workerFetch, workerMemory, workerStats, addFiles, type Files, type Lane, type WorkerStats } from "@panyam/tsappkit/wasmhost";

export type { Files, WorkerStats };

// The namespace the wasm build exports under, wasmengine.Namespace on the Go side.
const namespace = "agni";

// A LaneName names one of the engine's lanes.
export type LaneName = "serve" | "jobs" | "prepare";

// JOB_RPCS are the requests the jobs lane answers: each runs rules over a whole design or compares
// two, so each can hold a worker for seconds. Everything else is quick once the design is read.
export const JOB_RPCS: ReadonlySet<string> = new Set([
  "agni.v1.webapi.CheckService/CheckDesign",
  "agni.v1.webapi.CheckService/GetCheckReport",
  "agni.v1.webapi.CheckService/GetInterfaceCoverage",
  "agni.v1.webapi.ReviewService/CreateReview",
  "agni.v1.webapi.DiffService/DiffDesigns",
]);

// INGEST is the procedure that loads a design into a worker (agni issue 1061). One that reads the
// board goes to the prepare lane, on a worker that ends after it, and any other to the serve lane.
export const INGEST = "agni.v1.webapi.SnapshotService/Ingest";

// laneFor is the lane a request's URL goes to, by its Connect procedure (the last two path segments,
// so a page served under a prefix routes the same). readBoard is an Ingest body's read_board.
export function laneFor(url: URL, readBoard = false): LaneName {
  const procedure = procedureOf(url);
  if (procedure === INGEST && readBoard) return "prepare";
  return JOB_RPCS.has(procedure) ? "jobs" : "serve";
}

function procedureOf(url: URL): string {
  return url.pathname.split("/").slice(-2).join("/");
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
  // stats is the worker's wasm memory and its live Go heap, collecting garbage first, which pauses
  // the worker. For measuring; the lane's own limit asks the same at idle moments.
  stats(): Promise<WorkerStats>;
  // begin is set on a lane that starts a worker per request (prepare). It resolves to that worker and
  // the fetch to send the request on, which ends the worker once it has answered; release ends it
  // for a request that is sent elsewhere instead. A caller bringing files in first uses this rather
  // than holder, since the worker it fills is the one the request must reach.
  begin?(): Promise<LaneRun>;
}

// A LaneRun is one request's own worker on a lane that keeps none (EngineLane.begin).
export interface LaneRun {
  holder: object;
  fetch(input: RequestInfo | URL, init?: RequestInit): Promise<Response>;
  release(): void;
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
  // stats is each lane's memory beside its live heap (EngineLane.stats).
  stats(): Promise<Record<LaneName, WorkerStats>>;
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
  // serveMaxBytes is the serve lane's memory limit. Wasm memory never shrinks, so a worker that read
  // a large design keeps the read's peak for the rest of the visit. Past the limit, once idle, the
  // lane replaces the worker only when a fresh one would come back smaller (wouldShrink), and the
  // fresh one restores the design from the browser's cache on its next request rather than reading
  // it (agni issue 911). A worker whose live heap is most of its memory is kept however far past the
  // limit it is, since its replacement would restore the same state, which is what made the Jetson
  // seed replace its worker after every request (agni issue 974). Unset means no limit.
  serveMaxBytes?: number;
  // serveShrinkRatio is wouldShrink's ratio for the serve lane, SHRINK_RATIO when unset. For measuring.
  serveShrinkRatio?: number;
  // onRestart hears that a lane replaced its worker for passing its limit, and what it held.
  onRestart?: (lane: LaneName, stats: WorkerStats) => void;
  // onServeStats hears every measurement the serve lane takes of an idle worker past its limit, and
  // whether it replaces the worker. For measuring: a caller asking the worker itself at the same
  // moment races the lane, which may end the worker under it.
  onServeStats?: (stats: WorkerStats, replaced: boolean) => void;
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
    stats: async () => workerStats(await lane.worker(), { gc: true }),
  });

  const ratio = opts.serveShrinkRatio ?? SHRINK_RATIO;
  const serve = engineLane(
    startLane(
      start,
      opts.serveMaxBytes === undefined
        ? undefined
        : {
            maxMemoryBytes: opts.serveMaxBytes,
            restartIf: (s) => {
              const replace = wouldShrink(s, ratio);
              opts.onServeStats?.(s, replace);
              return replace;
            },
            onRestart: (s) => opts.onRestart?.("serve", s),
          },
    ),
  );
  const jobs = opts.oneLane ? serve : engineLane(startLane(start));

  // The prepare lane's worker answers one request and ends. Its answer is read whole before the
  // worker goes, which suits LoadDesign's few bytes.
  const begin = async (): Promise<LaneRun> => {
    let ended = false;
    let w: Worker | undefined;
    const end = () => {
      if (ended) return;
      ended = true;
      w?.terminate();
    };
    try {
      w = await startWorker(start);
      await ensureKept(w);
    } catch (err) {
      end();
      throw err;
    }
    const worker = w;
    return {
      holder: worker,
      fetch: async (input, init) => {
        try {
          const res = await workerFetch(worker)(input, init);
          const body = await res.arrayBuffer();
          return new Response(body, { status: res.status, statusText: res.statusText, headers: res.headers });
        } finally {
          end();
        }
      },
      release: end,
    };
  };
  const prepare: EngineLane = opts.oneLane
    ? serve
    : {
        begin,
        fetch: async (input, init) => (await begin()).fetch(input, init),
        holder: async () => {
          throw new Error("the prepare lane keeps no worker between requests");
        },
        add: (holder, name, files) => addFiles(holder as Worker, name, files),
        memoryBytes: async () => 0,
        stats: async () => ({ memoryBytes: 0 }),
      };
  const lanes: Record<LaneName, EngineLane> = { serve, jobs, prepare };
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
      return { serve: s, jobs: j, prepare: 0 };
    },
    stats: async () => {
      const s = await serve.stats();
      return { serve: s, jobs: jobs === serve ? s : await jobs.stats(), prepare: { memoryBytes: 0 } };
    },
  };
}

// SHRINK_RATIO is how small a worker's live heap must be beside its memory before replacing it pays.
// On the Jetson seed the serve worker holds 377 MB live in 971 MB after a fresh read (0.39), and 386 MB
// in 545 MB once restored (0.71), so this sits between the two (make wasm-bench, agni issue 990).
export const SHRINK_RATIO = 0.55;

// wouldShrink reports whether a fresh worker would come back smaller than this one: its live heap is
// what a replacement restores, and its wasm memory is what it holds now, which never shrinks. A
// worker that reports no heap (a wasm built against goapplib before 0.7.4) is kept, since nothing
// says a replacement would be smaller (agni issue 990).
export function wouldShrink(s: WorkerStats, ratio: number): boolean {
  return s.heap !== undefined && s.heap.heapAlloc < s.memoryBytes * ratio;
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

// mountingFetch is the engine's fetch with the design's files brought in first (agni issue 853), and
// with each jobs request held while a prepare request is under way (agni issue 1061), so a Run checks
// sent mid-load restores the board the prepare worker is about to store instead of parsing it too. A
// design keeps its server URL under the in-browser engine: before a request reaches a lane, every
// `mount://` URI in its body is listed through `source`, once per design, and the files that lane's
// worker does not already hold, by hash, are fetched and added under the same mount name. Each lane
// brings its own, and a lane's replacement worker brings them again, since it starts empty (#911).
//
// A question names a snapshot rather than files (agni issue 1061). The page's Ingest of each snapshot
// passes through here, so its files are the ones that Ingest named, and the Ingest is sent again to
// any worker that is asked about the snapshot without holding it: each worker keeps its own, and a
// replacement starts with none.
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
  const preparing = new Set<Promise<void>>(); // prepare requests under way, settled when each answers
  const afterPrepares = async () => {
    while (preparing.size) await Promise.allSettled([...preparing]);
  };
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

  const bring = async (uri: string, lane: EngineLane, into?: object): Promise<void> => {
    const set = await list(uri);
    if (!set) return;
    const holder = into ?? (await lane.holder());
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

  // Every worker keeps its own snapshots (agni issue 1061), so the page's ingest of each is kept and
  // sent to a worker before the first question that worker is asked about it.
  const ingests = new Map<string, { url: string; body: string; uris: Set<string> }>(); // snapshot -> the page's ingest of it
  const ingested = new WeakMap<object, Set<string>>(); // worker -> the snapshots it holds
  const holds = (worker: object): Set<string> => {
    let set = ingested.get(worker);
    if (!set) ingested.set(worker, (set = new Set()));
    return set;
  };

  // send answers req on its lane, bringing the design's files into the worker first unless they
  // already live in the page (a browser mount, which every worker is given at start), and ingesting
  // each snapshot it names that the worker does not hold. ingestOf is the snapshot req itself ingests.
  const send = async (req: Request, uris: Set<string>, bringFiles: boolean, name: LaneName, snaps: string[], ingestOf?: string): Promise<Response> => {
    const lane = engine.lanes[name];
    if (name === "jobs") await afterPrepares();
    const run = lane.begin ? await lane.begin() : undefined;
    if (bringFiles) {
      try {
        await Promise.all([...uris].map((u) => bring(u, lane, run?.holder)));
      } catch (err) {
        run?.release();
        throw err;
      }
      if ([...uris].some((u) => onServerSide.has(u))) {
        run?.release();
        return network(req);
      }
    }
    // A request cancelled while its files were on their way stops here, before any worker sees it,
    // and the files it brought stay for the next one (agni issue 918).
    if (req.signal.aborted) {
      run?.release();
      throw req.signal.reason ?? new DOMException("aborted", "AbortError");
    }
    const on = (r: Request) => (run ? run.fetch(r) : lane.fetch(r));
    const worker = run?.holder ?? (await lane.holder());
    for (const snap of snaps) {
      const page = ingests.get(snap);
      if (!page || holds(worker).has(snap)) continue;
      const r = await on(new Request(page.url, { method: "POST", headers: { "content-type": "application/json", "connect-protocol-version": "1" }, body: page.body }));
      // A worker that cannot ingest it answers the question with NOT_FOUND, which the page's
      // transport recovers from or reports.
      if (r.ok) holds(worker).add(snap);
    }
    const resp = await on(req);
    if (ingestOf && resp.ok) holds(worker).add(ingestOf);
    return resp;
  };
  const tracked = (req: Request, uris: Set<string>, bringFiles: boolean, name: LaneName, snaps: string[], ingestOf?: string): Promise<Response> => {
    const p = send(req, uris, bringFiles, name, snaps, ingestOf);
    if (name === "prepare") {
      const settled = p.then(
        () => {},
        () => {},
      );
      preparing.add(settled);
      void settled.then(() => preparing.delete(settled));
    }
    return p;
  };

  return async (input, init) => {
    const req = new Request(input, init);
    const url = new URL(req.url);
    const uris = new Set<string>();
    const snaps = new Set<string>();
    let body: Record<string, unknown> | undefined;
    try {
      body = JSON.parse(await req.clone().text());
      collectMountURIs(body, uris);
      collectSnapshots(body, snaps);
    } catch {
      // not JSON, so nothing to bring in
    }
    let ingestOf: string | undefined;
    let readBoard = false;
    if (procedureOf(url) === INGEST && body) {
      readBoard = body.readBoard === true;
      if (typeof body.snapshotId === "string" && body.snapshotId) {
        ingestOf = `snapshots/${body.snapshotId}`;
        // Kept without read_board, so a worker given it later reads the netlist alone.
        if (!ingests.has(ingestOf)) ingests.set(ingestOf, { url: req.url, body: JSON.stringify({ ...body, readBoard: undefined }), uris: new Set(uris) });
      }
    }
    // A question names its snapshot rather than files, so its files are the ones its ingest named.
    for (const snap of snaps) for (const u of ingests.get(snap)?.uris ?? []) uris.add(u);
    const name = laneFor(url, readBoard);
    const inBrowser = [...uris].some((u) => browserMounts.has(mountOf(u)));
    if (inBrowser) return abortable(req.signal, tracked(req, uris, false, name, [...snaps], ingestOf));
    if (passthrough(url)) return network(req);
    return abortable(req.signal, tracked(req, uris, true, name, [...snaps], ingestOf));
  };
}

// abortable rejects as soon as signal fires, whatever stage the request is in (agni issue 918). A
// request waits on its design's files reaching the worker before the lane sees it, and only the lane
// stops on the signal, so without this a Cancel pressed during that wait did nothing until every file
// had arrived, 85 MB on the Jetson board. The work it leaves behind finishes harmlessly: the files
// land in the worker for the next request, and the lane drops a request whose signal has fired.
export function abortable<T>(signal: AbortSignal, work: Promise<T>): Promise<T> {
  if (signal.aborted) {
    work.catch(() => undefined);
    return Promise.reject(signal.reason ?? new DOMException("aborted", "AbortError"));
  }
  return new Promise<T>((resolve, reject) => {
    const stop = () => reject(signal.reason ?? new DOMException("aborted", "AbortError"));
    signal.addEventListener("abort", stop, { once: true });
    work.then(
      (v) => {
        signal.removeEventListener("abort", stop);
        resolve(v);
      },
      (e) => {
        signal.removeEventListener("abort", stop);
        reject(e);
      },
    );
  });
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

// collectSnapshots finds every string value in a decoded JSON body that names a snapshot.
function collectSnapshots(v: unknown, out: Set<string>): void {
  if (typeof v === "string") {
    if (v.startsWith("snapshots/")) out.add(v);
  } else if (Array.isArray(v)) {
    for (const x of v) collectSnapshots(x, out);
  } else if (v && typeof v === "object") {
    for (const x of Object.values(v)) collectSnapshots(x, out);
  }
}

// mb writes a size for a person, in KB under a megabyte.
function mb(bytes: number): string {
  if (bytes < 1 << 20) return `${Math.ceil(bytes / 1024)} KB`;
  return `${Math.round(bytes / (1 << 20))} MB`;
}
