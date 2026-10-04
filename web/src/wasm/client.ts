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

// A DesignFiles lists one design's files, as WorkspaceService.ListDesignFiles answers. The type is
// structural so this module does not import the generated client.
export interface DesignFiles {
  mount: string;
  files: { path: string; sha256: string }[];
}

export interface DesignSource {
  // list names a design's files, given a `mount://` URI naming the design.
  list(uri: string): Promise<DesignFiles>;
  // fetchFile returns one file's bytes.
  fetchFile(mount: string, path: string): Promise<Uint8Array>;
}

// mountingFetch is the engine's fetch with the design's files brought in first (agni issue 853). A
// design keeps its server URL under the in-browser engine: before a request reaches the worker,
// every `mount://` URI in its body is listed through `source` and the files the worker does not
// already hold, by hash, are fetched and mounted under the same mount name. Requests in `passthrough`
// skip the worker entirely, which is where the calls naming the server's own mounts go.
//
// The body is read as Connect JSON, which is what the viewer's transport sends. A binary body names
// no URI this can see, so it goes to the worker as is.
export function mountingFetch(
  engine: WasmEngine,
  source: DesignSource,
  passthrough: (url: URL) => boolean,
  network: typeof globalThis.fetch,
): typeof globalThis.fetch {
  const held = new Map<string, Map<string, string>>(); // mount -> path -> sha256
  const listed = new Map<string, Promise<void>>(); // uri -> the mount that brought it in

  const bring = (uri: string): Promise<void> => {
    let p = listed.get(uri);
    if (!p) {
      p = (async () => {
        const set = await source.list(uri);
        const have = held.get(set.mount) ?? new Map<string, string>();
        const want = set.files.filter((f) => have.get(f.path) !== f.sha256);
        if (!want.length) return;
        const files: Files = {};
        await Promise.all(
          want.map(async (f) => {
            files[f.path] = await source.fetchFile(set.mount, f.path);
          }),
        );
        await engine.mount(set.mount, files);
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
    if (passthrough(new URL(req.url))) return network(req);
    const text = await req.clone().text();
    const uris = new Set<string>();
    try {
      collectMountURIs(JSON.parse(text), uris);
    } catch {
      // not JSON, so nothing to bring in
    }
    await Promise.all([...uris].map(bring));
    return engine.fetch(req);
  };
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
