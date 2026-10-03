// The page's side of the engine worker (agni issue 178). `fetch` is what a Connect transport takes in
// place of the network's, so every generated client and every panel drives the in-browser engine
// unchanged, and a design's bytes never leave the page.
import type { EngineReply, EngineRequest, Files, Ready } from "./protocol.js";

export interface WasmEngine {
  // fetch answers one request in the worker, with the network's signature.
  fetch(input: RequestInfo | URL, init?: RequestInit): Promise<Response>;
  // mount replaces one mount's files. Each buffer is TRANSFERRED to the worker, so it is empty here
  // afterwards.
  mount(name: string, files: Files): Promise<void>;
}

export interface EngineAssets {
  worker: string;
  wasm: string;
  exec: string;
}

// startEngine starts the worker and resolves once the engine has loaded, or rejects with why it could
// not.
export function startEngine(assets: EngineAssets): Promise<WasmEngine> {
  const url = `${assets.worker}?${new URLSearchParams({ wasm: assets.wasm, exec: assets.exec })}`;
  const worker = new Worker(url);
  const pending = new Map<number, (r: EngineReply) => void>();
  let next = 1;

  // Unsent is a request before it has an id. Omit over a union keeps only the shared keys, so it is
  // applied per member.
  type Unsent = EngineRequest extends infer R ? (R extends EngineRequest ? Omit<R, "id"> : never) : never;
  const call = (req: Unsent, transfer: Transferable[]): Promise<EngineReply> =>
    new Promise((resolve) => {
      const id = next++;
      pending.set(id, resolve);
      worker.postMessage({ ...req, id }, transfer);
    });

  const engine: WasmEngine = {
    async fetch(input, init) {
      const req = new Request(input, init);
      const body = new Uint8Array(await req.arrayBuffer());
      const headers: Record<string, string> = {};
      req.headers.forEach((v, k) => (headers[k] = v));
      const u = new URL(req.url);
      const reply = await call({ kind: "http", method: req.method, url: u.pathname + u.search, headers, body }, [
        body.buffer,
      ]);
      if (!reply.ok) throw new Error(reply.error);
      return new Response(reply.body ? new Uint8Array(reply.body) : null, { status: reply.status, headers: reply.headers });
    },
    async mount(name, files) {
      const reply = await call(
        { kind: "mount", name, files },
        Object.values(files).map((b) => b.buffer),
      );
      if (!reply.ok) throw new Error(reply.error);
    },
  };

  return new Promise((resolve, reject) => {
    worker.onmessage = (ev: MessageEvent<Ready | EngineReply>) => {
      const m = ev.data;
      if ("ready" in m) {
        if (m.ready) resolve(engine);
        else reject(new Error(m.error));
        return;
      }
      pending.get(m.id)?.(m);
      pending.delete(m.id);
    };
    worker.onerror = (ev) => reject(new Error(ev.message));
  });
}

// SeedManifest names one mount's files under a base URL. It is the stand-in for the loaders #853
// designs (dropped files, a static host), and only fetches what it lists.
export interface SeedManifest {
  mount: string;
  base: string;
  files: string[];
}

// mountSeed fetches a manifest's files and mounts them.
export async function mountSeed(engine: WasmEngine, manifestUrl: string): Promise<void> {
  const res = await fetch(manifestUrl);
  if (!res.ok) throw new Error(`seed ${manifestUrl}: ${res.status}`);
  const m = (await res.json()) as SeedManifest;
  const base = new URL(m.base, new URL(manifestUrl, location.href));
  const files: Files = {};
  await Promise.all(
    m.files.map(async (p) => {
      const r = await fetch(new URL(p, base));
      if (!r.ok) throw new Error(`seed ${p}: ${r.status}`);
      files[p] = new Uint8Array(await r.arrayBuffer());
    }),
  );
  await engine.mount(m.mount, files);
}
