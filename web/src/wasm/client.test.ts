import { describe, expect, it } from "vitest";
import { laneFor, memoryVerdict, mountingFetch, watchMemory, type DesignFiles, type EngineLane, type LaneName, type ServerFallback, type WasmEngine } from "./client.js";

// A recording engine of two lanes, design source and network, so a test can see where each request
// went. `replace` swaps a lane's worker for an empty one, as an abort does.
function harness(listing: DesignFiles | Error) {
  const engineCalls: string[] = [];
  const laneCalls: Record<LaneName, string[]> = { serve: [], jobs: [] };
  const networkCalls: string[] = [];
  const mounted: Record<string, string[]> = {};
  const laneMounted: Record<LaneName, string[]> = { serve: [], jobs: [] };
  const fetched: string[] = [];
  const workers: Record<LaneName, object> = { serve: {}, jobs: {} };
  const lane = (name: LaneName): EngineLane => ({
    fetch: async (input) => {
      const path = new URL((input as Request).url).pathname;
      engineCalls.push(path);
      laneCalls[name].push(path);
      return new Response("{}");
    },
    holder: async () => workers[name],
    add: async (holder, mount, files) => {
      expect(holder).toBe(workers[name]);
      mounted[mount] = [...(mounted[mount] ?? []), ...Object.keys(files)];
      laneMounted[name].push(...Object.keys(files));
    },
    memoryBytes: async () => 0,
  });
  const lanes = { serve: lane("serve"), jobs: lane("jobs") };
  const engine: WasmEngine = {
    fetch: (input, init) => lanes[laneFor(new URL((input as Request).url))].fetch(input, init),
    add: async () => {},
    lanes,
    memoryBytes: async () => ({ serve: 0, jobs: 0 }),
  };
  const replace = (name: LaneName) => {
    workers[name] = {};
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
  return { engine, network, source, engineCalls, laneCalls, networkCalls, mounted, laneMounted, fetched, replace, lists: () => lists };
}

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
  });
});

// A lane of fake workers whose memory a test sets, recording each restart.
function memoryLane(held: number[]) {
  let worker = {} as Worker;
  const workers = [worker];
  const lane = {
    fetch: async () => new Response("{}"),
    worker: async () => worker,
    restart: async () => {
      worker = {} as Worker;
      workers.push(worker);
    },
    close: () => {},
  };
  // held[i] is what the i-th measurement reads.
  let i = 0;
  const measure = async () => held[Math.min(i++, held.length - 1)];
  return { lane, workers, measure };
}

const MB = 1 << 20;
const settle = () => new Promise((r) => setTimeout(r, 0));

describe("watchMemory", () => {
  it("replaces a worker past the limit once, then keeps a replacement whose restored design is over it", async () => {
    // Jetson's measurements in agni issue 974: 977 MB after a fresh read, 548 MB restored, each time.
    const m = memoryLane([977 * MB, 548 * MB, 548 * MB, 548 * MB]);
    const restarts: number[] = [];
    const f = watchMemory(m.lane, { limit: 512 * MB, measure: m.measure, onRestart: (b) => restarts.push(b) });
    for (let i = 0; i < 4; i++) {
      await (await f.fetch("http://h/q")).text();
      await settle();
    }
    expect(restarts).toEqual([977 * MB]);
    expect(m.workers).toHaveLength(2);
  });

  it("still replaces a raised worker that creeps past the raised limit", async () => {
    const m = memoryLane([977 * MB, 548 * MB, 600 * MB, 900 * MB]);
    const restarts: number[] = [];
    const f = watchMemory(m.lane, { limit: 512 * MB, measure: m.measure, onRestart: (b) => restarts.push(b) });
    for (let i = 0; i < 4; i++) {
      await (await f.fetch("http://h/q")).text();
      await settle();
    }
    expect(restarts).toEqual([977 * MB, 900 * MB]);
  });

  it("measures only once the body has been read", async () => {
    const m = memoryLane([977 * MB]);
    const restarts: number[] = [];
    const f = watchMemory(m.lane, { limit: 512 * MB, measure: m.measure, onRestart: (b) => restarts.push(b) });
    const res = await f.fetch("http://h/q");
    await settle();
    expect(restarts).toEqual([]);
    await res.text();
    await settle();
    expect(restarts).toEqual([977 * MB]);
  });
});

describe("memoryVerdict", () => {
  it("keeps under the limit, raises for a fresh replacement, restarts otherwise", () => {
    expect(memoryVerdict({ limit: 512, fresh: true }, 500, 256)).toEqual({ verdict: "keep", limit: 512 });
    expect(memoryVerdict({ limit: 512, fresh: true }, 548, 256)).toEqual({ verdict: "raise", limit: 804 });
    expect(memoryVerdict({ limit: 512, fresh: false }, 548, 256)).toEqual({ verdict: "restart", limit: 512 });
  });
});
