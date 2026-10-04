import { describe, expect, it } from "vitest";
import { mountingFetch, type DesignFiles, type ServerFallback, type WasmEngine } from "./client.js";
import type { Files } from "./protocol.js";

// A recording engine, design source and network, so a test can see where each request went.
function harness(listing: DesignFiles | Error) {
  const engineCalls: string[] = [];
  const networkCalls: string[] = [];
  const mounted: Record<string, string[]> = {};
  const fetched: string[] = [];
  const engine: WasmEngine = {
    fetch: async (input) => {
      engineCalls.push(new URL((input as Request).url).pathname);
      return new Response("{}");
    },
    mount: async (name: string, files: Files) => {
      mounted[name] = [...(mounted[name] ?? []), ...Object.keys(files)];
    },
    memoryBytes: async () => 0,
  };
  const network = (async (input: RequestInfo | URL) => {
    networkCalls.push(new URL((input as Request).url).pathname);
    return new Response("{}");
  }) as typeof globalThis.fetch;
  const source = {
    list: async () => {
      if (listing instanceof Error) throw listing;
      return listing;
    },
    fetchFile: async (_m: string, p: string) => {
      fetched.push(p);
      return new Uint8Array([1]);
    },
  };
  return { engine, network, source, engineCalls, networkCalls, mounted, fetched };
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
    const f = mountingFetch(h.engine, h.source, () => false, h.network, 10_000);
    await ask(f, "CheckService/GetCheckReport", "mount://tut/designs/gateway");
    await ask(f, "QueryService/RunQuery", "mount://tut/designs/gateway");
    expect(h.mounted).toEqual({ tut: ["designs/gateway/gateway.edn", "project.yaml"] });
    expect(h.fetched).toHaveLength(2);
    expect(h.engineCalls).toEqual(["/agni.v1.webapi.CheckService/GetCheckReport", "/agni.v1.webapi.QueryService/RunQuery"]);
    expect(h.networkCalls).toEqual([]);
  });

  it("sends a design over the size threshold to the server, says why once, and keeps it there", async () => {
    const h = harness(listing);
    const told: ServerFallback[] = [];
    const f = mountingFetch(h.engine, h.source, () => false, h.network, 1000, (x) => told.push(x));
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
    const f = mountingFetch(h.engine, h.source, () => false, h.network, 10_000, (x) => told.push(x));
    await ask(f, "DesignService/GetDesign", "mount://tut/designs/gateway");
    expect(h.networkCalls).toEqual(["/agni.v1.webapi.DesignService/GetDesign"]);
    expect(told[0].reason).toMatch(/could not be listed/);
  });

  it("passes the requests it is told to straight to the network", async () => {
    const h = harness(listing);
    const f = mountingFetch(h.engine, h.source, (u) => u.pathname.includes("WorkspaceService"), h.network, 10_000);
    await ask(f, "WorkspaceService/ListMounts", "mount://tut");
    expect(h.networkCalls).toEqual(["/agni.v1.webapi.WorkspaceService/ListMounts"]);
    expect(h.fetched).toEqual([]);
  });
});
