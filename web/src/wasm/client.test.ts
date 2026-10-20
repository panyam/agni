import { describe, expect, it } from "vitest";
import { readFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";
import { abortable, laneFor, mountingFetch, withheldPlaceholder, wouldShrink, SHRINK_RATIO, WITHHELD_PREFIX, type DesignFiles, type EngineLane, type LaneName, type ServerFallback, type WasmEngine } from "./client.js";

// A recording engine of its lanes, design source and network, so a test can see where each request
// went. `replace` swaps a lane's worker for an empty one, as an abort does. The prepare lane starts a
// worker per request, as the real one does; `holdPrepares` keeps each prepare unanswered until the
// returned release is called, and `released` counts the prepare workers ended without a request.
// `given` is the bytes each lane's worker holds per path. A request whose path `readsBoard` names is
// refused as the engine refuses it, while the worker holds a placeholder for a file (agni issue 992).
function harness(listing: DesignFiles | Error, readsBoard: (path: string) => boolean = () => false) {
  const engineCalls: string[] = [];
  const laneCalls: Record<LaneName, string[]> = { serve: [], jobs: [], prepare: [] };
  const laneBodies: Record<LaneName, string[]> = { serve: [], jobs: [], prepare: [] };
  const networkCalls: string[] = [];
  const mounted: Record<string, string[]> = {};
  const laneMounted: Record<LaneName, string[]> = { serve: [], jobs: [], prepare: [] };
  const fetched: string[] = [];
  const given: Record<LaneName, Record<string, Uint8Array>> = { serve: {}, jobs: {}, prepare: {} };
  const workers: Record<LaneName, object> = { serve: {}, jobs: {}, prepare: {} };
  const prepareWorkers: object[] = [];
  let released = 0;
  let hold: Promise<void> | undefined;
  const lane = (name: LaneName): EngineLane => ({
    fetch: async (input) => {
      const path = new URL((input as Request).url).pathname;
      engineCalls.push(path);
      laneCalls[name].push(path);
      laneBodies[name].push(await (input as Request).clone().text());
      if (readsBoard(path) && Object.values(given[name]).some(isPlaceholder)) return withheldError();
      return new Response("{}");
    },
    holder: async () => workers[name],
    add: async (holder, mount, files) => {
      expect(holder).toBe(workers[name]);
      Object.assign(given[name], files);
      mounted[mount] = [...(mounted[mount] ?? []), ...Object.keys(files)];
      laneMounted[name].push(...Object.keys(files));
    },
    memoryBytes: async () => 0,
    stats: async () => ({ memoryBytes: 0 }),
  });
  const prepare: EngineLane = {
    ...lane("prepare"),
    begin: async () => {
      const w = {};
      prepareWorkers.push(w);
      let ended = false;
      return {
        holder: w,
        fetch: async (input) => {
          const path = new URL((input as Request).url).pathname;
          laneCalls.prepare.push(path);
          if (hold) await hold;
          ended = true;
          return new Response("{}");
        },
        release: () => {
          if (!ended) released++;
          ended = true;
        },
      };
    },
    add: async (_holder, mount, files) => {
      Object.assign(given.prepare, files);
      mounted[mount] = [...(mounted[mount] ?? []), ...Object.keys(files)];
      laneMounted.prepare.push(...Object.keys(files));
    },
  };
  const lanes = { serve: lane("serve"), jobs: lane("jobs"), prepare };
  const engine: WasmEngine = {
    fetch: (input, init) => lanes[laneFor(new URL((input as Request).url))].fetch(input, init),
    add: async () => {},
    lanes,
    memoryBytes: async () => ({ serve: 0, jobs: 0, prepare: 0 }),
    stats: async () => ({ serve: { memoryBytes: 0 }, jobs: { memoryBytes: 0 }, prepare: { memoryBytes: 0 } }),
  };
  const holdPrepares = () => {
    let release!: () => void;
    hold = new Promise<void>((r) => (release = r));
    return () => {
      hold = undefined;
      release();
    };
  };
  const replace = (name: LaneName) => {
    workers[name] = {};
    given[name] = {};
  };
  const network = (async (input: RequestInfo | URL) => {
    networkCalls.push(new URL((input as Request).url).pathname);
    return new Response("{}");
  }) as typeof globalThis.fetch;
  let lists = 0;
  const source = {
    list: async () => {
      lists++;
      if (listing instanceof Error) throw listing;
      return listing;
    },
    fetchFile: async (_m: string, p: string) => {
      fetched.push(p);
      return new Uint8Array([1]);
    },
  };
  return { given, engine, network, source, engineCalls, laneCalls, laneBodies, networkCalls, mounted, laneMounted, fetched, replace, lists: () => lists, prepareWorkers, holdPrepares, released: () => released };
}

const isPlaceholder = (b: Uint8Array) => new TextDecoder().decode(b).startsWith(WITHHELD_PREFIX);

// withheldError is the engine's refusal of a question that read a held-back file, as Connect writes
// it (internal/wasmengine/withheld.go).
const withheldError = () =>
  new Response(JSON.stringify({ code: "failed_precondition", message: "held back", details: [{ type: "agni.v1.webapi.BlobsMissing", value: "" }] }), {
    status: 400,
    headers: { "content-type": "application/json" },
  });

const listing: DesignFiles = {
  mount: "tut",
  files: [
    { path: "designs/gateway/gateway.edn", sha256: "sha256:a" },
    { path: "project.yaml", sha256: "sha256:b" },
  ],
  totalSize: 2000n,
};

const ask = (f: typeof globalThis.fetch, rpc: string, uri: string) =>
  f(new Request(`http://h/agni.v1.webapi.${rpc}`, { method: "POST", body: JSON.stringify({ uri }) }));

// ingest is the page's Ingest of uri under id, as SnapshotRegistry sends it, and askSnap a question
// naming that snapshot (agni issue 1061).
const INGEST_PATH = "/agni.v1.webapi.SnapshotService/Ingest";
const ingest = (f: typeof globalThis.fetch, uri: string, id: string, readBoard = false) =>
  f(new Request(`http://h${INGEST_PATH}`, { method: "POST", body: JSON.stringify({ snapshot: { uri }, snapshotId: id, ...(readBoard ? { readBoard } : {}) }) }));
const askSnap = (f: typeof globalThis.fetch, rpc: string, snapshot: string) =>
  f(new Request(`http://h/agni.v1.webapi.${rpc}`, { method: "POST", body: JSON.stringify({ snapshot }) }));

describe("mountingFetch", () => {
  it("brings a design's files in once, then answers in the worker", async () => {
    const h = harness(listing);
    const f = mountingFetch({ engine: h.engine, source: h.source, network: h.network, maxBytes: 10_000 });
    await ask(f, "QueryService/RunQuery", "mount://tut/designs/gateway");
    await ask(f, "DesignService/GetSheet", "mount://tut/designs/gateway");
    expect(h.mounted).toEqual({ tut: ["designs/gateway/gateway.edn", "project.yaml"] });
    expect(h.fetched).toHaveLength(2);
    expect(h.engineCalls).toEqual(["/agni.v1.webapi.QueryService/RunQuery", "/agni.v1.webapi.DesignService/GetSheet"]);
    expect(h.networkCalls).toEqual([]);
  });

  it("sends a design over the size threshold to the server, says why once, and keeps it there", async () => {
    const h = harness(listing);
    const told: ServerFallback[] = [];
    const f = mountingFetch({ engine: h.engine, source: h.source, network: h.network, maxBytes: 1000, onServer: (x) => told.push(x) });
    await ask(f, "CheckService/GetCheckReport", "mount://tut/designs/gateway");
    await ask(f, "QueryService/RunQuery", "mount://tut/designs/gateway");
    expect(h.fetched).toEqual([]);
    expect(h.engineCalls).toEqual([]);
    expect(h.networkCalls).toHaveLength(2);
    expect(told).toHaveLength(1);
    expect(told[0].reason).toMatch(/over the/);
  });

  it("sends a design whose listing fails to the server", async () => {
    const h = harness(new Error("resource exhausted"));
    const told: ServerFallback[] = [];
    const f = mountingFetch({ engine: h.engine, source: h.source, network: h.network, maxBytes: 10_000, onServer: (x) => told.push(x) });
    await ask(f, "DesignService/GetDesign", "mount://tut/designs/gateway");
    expect(h.networkCalls).toEqual(["/agni.v1.webapi.DesignService/GetDesign"]);
    expect(told[0].reason).toMatch(/could not be listed/);
  });

  it("passes the requests it is told to straight to the network", async () => {
    const h = harness(listing);
    const f = mountingFetch({ engine: h.engine, source: h.source, network: h.network, maxBytes: 10_000, passthrough: (u) => u.pathname.includes("WorkspaceService") });
    await ask(f, "WorkspaceService/ListMounts", "mount://tut");
    expect(h.networkCalls).toEqual(["/agni.v1.webapi.WorkspaceService/ListMounts"]);
    expect(h.fetched).toEqual([]);
  });

  it("answers a browser mount in the worker, listing nothing on the server, even for workspace calls", async () => {
    const h = harness(listing);
    const f = mountingFetch({
      engine: h.engine,
      source: h.source,
      network: h.network,
      passthrough: (u) => u.pathname.includes("WorkspaceService"),
      browserMounts: new Set(["local"]),
    });
    await ask(f, "WorkspaceService/ProposeDesigns", "mount://local/abc");
    await ask(f, "CheckService/GetCheckReport", "mount://local/abc/x.edn");
    expect(h.engineCalls).toEqual(["/agni.v1.webapi.WorkspaceService/ProposeDesigns", "/agni.v1.webapi.CheckService/GetCheckReport"]);
    expect(h.networkCalls).toEqual([]);
    expect(h.fetched).toEqual([]);
  });

  it("answers checks on the jobs lane and queries on the serve lane, each bringing the files once, listing once", async () => {
    const h = harness(listing);
    const f = mountingFetch({ engine: h.engine, source: h.source, network: h.network, maxBytes: 10_000 });
    await ask(f, "CheckService/CheckDesign", "mount://tut/designs/gateway");
    await ask(f, "QueryService/RunQuery", "mount://tut/designs/gateway");
    await ask(f, "CheckService/GetCheckReport", "mount://tut/designs/gateway");
    await ask(f, "QueryService/RunQuery", "mount://tut/designs/gateway");
    expect(h.laneCalls.jobs).toEqual(["/agni.v1.webapi.CheckService/CheckDesign", "/agni.v1.webapi.CheckService/GetCheckReport"]);
    expect(h.laneCalls.serve).toEqual(["/agni.v1.webapi.QueryService/RunQuery", "/agni.v1.webapi.QueryService/RunQuery"]);
    expect(h.laneMounted.jobs).toEqual(["designs/gateway/gateway.edn", "project.yaml"]);
    expect(h.laneMounted.serve).toEqual(["designs/gateway/gateway.edn", "project.yaml"]);
    expect(h.lists()).toBe(1);
  });

  it("brings the files again into a lane's replacement worker, and only that lane's", async () => {
    const h = harness(listing);
    const f = mountingFetch({ engine: h.engine, source: h.source, network: h.network, maxBytes: 10_000 });
    await ask(f, "CheckService/CheckDesign", "mount://tut/designs/gateway");
    await ask(f, "QueryService/RunQuery", "mount://tut/designs/gateway");
    h.replace("jobs");
    await ask(f, "CheckService/CheckDesign", "mount://tut/designs/gateway");
    await ask(f, "QueryService/RunQuery", "mount://tut/designs/gateway");
    expect(h.laneMounted.jobs).toHaveLength(4);
    expect(h.laneMounted.serve).toHaveLength(2);
    expect(h.lists()).toBe(1);
  });
});

describe("the prepare lane", () => {
  it("brings the files into each prepare request's own worker, which nothing else uses", async () => {
    const h = harness(listing);
    const f = mountingFetch({ engine: h.engine, source: h.source, network: h.network, maxBytes: 10_000 });
    await ingest(f, "mount://tut/designs/gateway", "s1", true);
    await ingest(f, "mount://tut/designs/gateway", "s1", true);
    expect(h.laneCalls.prepare).toEqual([INGEST_PATH, INGEST_PATH]);
    expect(h.prepareWorkers).toHaveLength(2);
    // Each worker started empty, so each was given the design's two files.
    expect(h.laneMounted.prepare).toHaveLength(4);
    expect(h.laneCalls.jobs).toEqual([]);
    expect(h.lists()).toBe(1);
  });

  it("holds a check sent while a prepare is under way until the prepare answers", async () => {
    const h = harness(listing);
    const f = mountingFetch({ engine: h.engine, source: h.source, network: h.network, maxBytes: 10_000 });
    await ingest(f, "mount://tut/designs/gateway", "s1");
    const release = h.holdPrepares();
    const loading = ingest(f, "mount://tut/designs/gateway", "s1", true);
    const checking = askSnap(f, "CheckService/CheckDesign", "snapshots/s1");
    const querying = askSnap(f, "QueryService/RunQuery", "snapshots/s1");
    await querying;
    await new Promise((r) => setTimeout(r, 10));
    expect(h.laneCalls.serve).toEqual([INGEST_PATH, "/agni.v1.webapi.QueryService/RunQuery"]);
    expect(h.laneCalls.jobs).toEqual([]);
    release();
    await Promise.all([loading, checking]);
    // The jobs worker was never given the snapshot, so it is ingested there before the check.
    expect(h.laneCalls.jobs).toEqual([INGEST_PATH, "/agni.v1.webapi.CheckService/CheckDesign"]);
  });

  it("ends the prepare worker unused when the design goes to the server", async () => {
    const h = harness(listing);
    const f = mountingFetch({ engine: h.engine, source: h.source, network: h.network, maxBytes: 1000 });
    await ingest(f, "mount://tut/designs/gateway", "s1", true);
    expect(h.networkCalls).toEqual([INGEST_PATH]);
    expect(h.laneCalls.prepare).toEqual([]);
    expect(h.released()).toBe(1);
  });
});

describe("laneFor", () => {
  it("routes the requests that run rules or compare designs to the jobs lane, under any prefix", () => {
    const at = (p: string) => laneFor(new URL(`http://h${p}`));
    expect(at("/agni.v1.webapi.CheckService/CheckDesign")).toBe("jobs");
    expect(at("/agni/demo/agni.v1.webapi.CheckService/GetCheckReport")).toBe("jobs");
    expect(at("/agni.v1.webapi.ReviewService/CreateReview")).toBe("jobs");
    expect(at("/agni.v1.webapi.DiffService/DiffDesigns")).toBe("jobs");
    expect(at("/agni.v1.webapi.CheckService/ListRules")).toBe("serve");
    expect(at("/agni.v1.webapi.QueryService/RunQuery")).toBe("serve");
    expect(at("/agni.v1.webapi.DesignService/GetSheet")).toBe("serve");
    // An Ingest that reads the board goes to a worker of its own, and any other to the serve lane.
    expect(laneFor(new URL("http://h/agni/demo/agni.v1.webapi.SnapshotService/Ingest"), true)).toBe("prepare");
    expect(at("/agni.v1.webapi.SnapshotService/Ingest")).toBe("serve");
  });
});

// agni issue 1061: a question names a snapshot, and each worker keeps its own, so the page's ingest
// of a snapshot is what tells a worker its files and is sent to every worker before its first
// question about it.
describe("snapshots across the lanes", () => {
  it("brings the files a snapshot's ingest named into the worker a question about it reaches", async () => {
    const h = harness(listing);
    const f = mountingFetch({ engine: h.engine, source: h.source, network: h.network, maxBytes: 10_000 });
    await ingest(f, "mount://tut/designs/gateway", "s1");
    await askSnap(f, "CheckService/CheckDesign", "snapshots/s1");
    expect(h.laneMounted.jobs).toEqual(["designs/gateway/gateway.edn", "project.yaml"]);
    expect(h.laneCalls.jobs).toEqual([INGEST_PATH, "/agni.v1.webapi.CheckService/CheckDesign"]);
    expect(h.lists()).toBe(1);
  });

  it("ingests into a worker once, and again into the worker that replaces it", async () => {
    const h = harness(listing);
    const f = mountingFetch({ engine: h.engine, source: h.source, network: h.network, maxBytes: 10_000 });
    await ingest(f, "mount://tut/designs/gateway", "s1");
    await askSnap(f, "QueryService/RunQuery", "snapshots/s1");
    await askSnap(f, "DesignService/GetSheet", "snapshots/s1");
    // The serve worker took the page's own ingest, so it is asked with nothing sent ahead.
    expect(h.laneCalls.serve).toEqual([INGEST_PATH, "/agni.v1.webapi.QueryService/RunQuery", "/agni.v1.webapi.DesignService/GetSheet"]);
    h.replace("serve");
    await askSnap(f, "QueryService/RunQuery", "snapshots/s1");
    expect(h.laneCalls.serve.slice(3)).toEqual([INGEST_PATH, "/agni.v1.webapi.QueryService/RunQuery"]);
  });

  it("sends a worker the ingest without read_board, under the snapshot's own id", async () => {
    const h = harness(listing);
    const f = mountingFetch({ engine: h.engine, source: h.source, network: h.network, maxBytes: 10_000 });
    await ingest(f, "mount://tut/designs/gateway", "s1", true);
    await askSnap(f, "CheckService/CheckDesign", "snapshots/s1");
    const sent = JSON.parse(h.laneBodies.jobs[0]);
    expect(sent).toEqual({ snapshot: { uri: "mount://tut/designs/gateway" }, snapshotId: "s1" });
    expect(h.laneCalls.prepare).toEqual([INGEST_PATH]);
  });

  it("asks the server about a snapshot whose design went to the server", async () => {
    const h = harness(listing);
    const f = mountingFetch({ engine: h.engine, source: h.source, network: h.network, maxBytes: 1000 });
    await ingest(f, "mount://tut/designs/gateway", "s1");
    await askSnap(f, "CheckService/CheckDesign", "snapshots/s1");
    expect(h.networkCalls).toEqual([INGEST_PATH, "/agni.v1.webapi.CheckService/CheckDesign"]);
    expect(h.engineCalls).toEqual([]);
  });
});

const MB = 1 << 20;
const held = (memory: number, live?: number) => ({
  memoryBytes: memory * MB,
  heap: live === undefined ? undefined : { heapAlloc: live * MB, heapInuse: live * MB, totalAlloc: 0, sys: memory * MB, numGC: 1 },
});

// The serve lane replaces a worker past its limit only when wouldShrink says so (agni issue 990).
// The figures are the Jetson seed's serve worker, measured with make wasm-bench.
const [FRESH_MB, FRESH_LIVE_MB, RESTORED_MB, RESTORED_LIVE_MB] = [971, 377, 545, 386];
describe("wouldShrink", () => {
  it("replaces a worker holding a fresh read's garbage", () => {
    expect(wouldShrink(held(FRESH_MB, FRESH_LIVE_MB), SHRINK_RATIO)).toBe(true);
  });
  it("keeps a worker whose restored design is most of what it holds, as a replacement would restore the same", () => {
    expect(wouldShrink(held(RESTORED_MB, RESTORED_LIVE_MB), SHRINK_RATIO)).toBe(false);
  });
  it("replaces a worker that crept up through requests that each dropped what they made", () => {
    expect(wouldShrink(held(900, 300), SHRINK_RATIO)).toBe(true);
  });
  it("keeps a worker that reports no heap, built against goapplib before 0.7.4", () => {
    expect(wouldShrink(held(977), SHRINK_RATIO)).toBe(false);
  });
});

// agni issue 918: a request is answered only once its design's files reach the worker, and a Cancel
// pressed meanwhile must not wait for them.
describe("cancelling a request the engine has not reached yet", () => {
  it("rejects as soon as its signal fires, while its files are still on their way", async () => {
    const h = harness(listing);
    let arrive!: () => void;
    const files = new Promise<void>((r) => (arrive = r));
    const source = { ...h.source, fetchFile: async (m: string, p: string) => (await files, h.source.fetchFile(m, p)) };
    const f = mountingFetch({ engine: h.engine, source, network: h.network, maxBytes: 10_000 });
    const abort = new AbortController();
    const asked = f(new Request("http://h/agni.v1.webapi.CheckService/CheckDesign", { method: "POST", body: JSON.stringify({ uri: "mount://tut/designs/gateway" }), signal: abort.signal }));
    abort.abort();
    await expect(asked).rejects.toThrow();
    expect(h.engineCalls).toEqual([]);
    // The files go on arriving, and the next request finds them in the worker.
    arrive();
    await ask(f, "CheckService/CheckDesign", "mount://tut/designs/gateway");
    expect(h.engineCalls).toEqual(["/agni.v1.webapi.CheckService/CheckDesign"]);
  });

  it("passes an answer through untouched when nothing is cancelled", async () => {
    await expect(abortable(new AbortController().signal, Promise.resolve(7))).resolves.toBe(7);
    await expect(abortable(new AbortController().signal, Promise.reject(new Error("no netlist")))).rejects.toThrow("no netlist");
  });
});

// agni issue 992: the serve lane builds no board for a query, a coverage or a trace, so it is given a
// placeholder for the board file, and the board's bytes only when the engine says a question read it.
const BOARD = "designs/gateway/gateway.kicad_pcb";
const withBoard: DesignFiles = {
  mount: "tut",
  files: [...listing.files, { path: BOARD, sha256: "sha256:c", board: true }],
  totalSize: 3000n,
};
const GET_SHEET = "/agni.v1.webapi.DesignService/GetSheet";

describe("holding the board back from the serve lane", () => {
  it("gives the serve worker a placeholder for the board, and fetches no board, for a plain query and the open", async () => {
    const h = harness(withBoard);
    const f = mountingFetch({ engine: h.engine, source: h.source, network: h.network, maxBytes: 10_000 });
    await ingest(f, "mount://tut/designs/gateway", "s1");
    expect((await askSnap(f, "QueryService/RunQuery", "snapshots/s1")).ok).toBe(true);
    expect((await askSnap(f, "DesignService/GetDesign", "snapshots/s1")).ok).toBe(true);
    expect(h.fetched.sort()).toEqual(["designs/gateway/gateway.edn", "project.yaml"]);
    // The board's path is still in the mount, so the design resolves as it does with the file there.
    expect(new TextDecoder().decode(h.given.serve[BOARD])).toBe(`${WITHHELD_PREFIX}sha256:c\n`);
    expect(h.laneCalls.serve).toEqual([INGEST_PATH, "/agni.v1.webapi.QueryService/RunQuery", "/agni.v1.webapi.DesignService/GetDesign"]);
  });

  it("brings the board into the serve worker and asks again when the engine says a question read it", async () => {
    const h = harness(withBoard, (p) => p === GET_SHEET);
    const f = mountingFetch({ engine: h.engine, source: h.source, network: h.network, maxBytes: 10_000 });
    await ingest(f, "mount://tut/designs/gateway", "s1");
    const resp = await askSnap(f, "DesignService/GetSheet", "snapshots/s1");
    expect(resp.ok).toBe(true);
    expect(isPlaceholder(h.given.serve[BOARD])).toBe(false);
    expect(h.fetched).toContain(BOARD);
    // The files rebuilt the worker's engine, so the snapshot is ingested again before the question.
    expect(h.laneCalls.serve).toEqual([INGEST_PATH, GET_SHEET, INGEST_PATH, GET_SHEET]);
    // Once the board is in, nothing is held back and nothing is asked twice.
    await askSnap(f, "DesignService/GetSheet", "snapshots/s1");
    expect(h.laneCalls.serve.slice(4)).toEqual([GET_SHEET]);
    expect(h.fetched.filter((p) => p === BOARD)).toHaveLength(1);
  });

  it("always gives the jobs and prepare workers the board's bytes", async () => {
    const h = harness(withBoard);
    const f = mountingFetch({ engine: h.engine, source: h.source, network: h.network, maxBytes: 10_000 });
    await ingest(f, "mount://tut/designs/gateway", "s1");
    await ingest(f, "mount://tut/designs/gateway", "s1", true);
    await askSnap(f, "CheckService/CheckDesign", "snapshots/s1");
    for (const lane of ["jobs", "prepare"] as const) {
      expect(h.given[lane][BOARD], lane).toBeDefined();
      expect(isPlaceholder(h.given[lane][BOARD]), lane).toBe(false);
    }
    expect(isPlaceholder(h.given.serve[BOARD])).toBe(true);
    expect(h.fetched.filter((p) => p === BOARD)).toHaveLength(2);
  });

  it("passes a refusal that is not about a held-back file through untouched", async () => {
    const h = harness(withBoard);
    const refusing: EngineLane = { ...h.engine.lanes.serve, fetch: async () => new Response(JSON.stringify({ code: "not_found", message: "no such sheet" }), { status: 404 }) };
    const engine = { ...h.engine, lanes: { ...h.engine.lanes, serve: refusing } };
    const f = mountingFetch({ engine, source: h.source, network: h.network, maxBytes: 10_000 });
    const resp = await ask(f, "DesignService/GetSheet", "mount://tut/designs/gateway");
    expect(resp.status).toBe(404);
    expect(h.fetched).not.toContain(BOARD);
  });

  it("writes the placeholder the engine recognises, twinned through withheld_twin.json", () => {
    const here = dirname(fileURLToPath(import.meta.url));
    const twin = JSON.parse(readFileSync(join(here, "..", "..", "..", "internal", "wasmengine", "testdata", "withheld_twin.json"), "utf8")) as { sha256: string; placeholder: string };
    expect(new TextDecoder().decode(withheldPlaceholder(twin.sha256))).toBe(twin.placeholder);
  });
});
