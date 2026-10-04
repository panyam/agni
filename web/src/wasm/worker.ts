// The engine worker (agni issue 178). It loads agni.wasm, built from cmd/agni-wasm, and answers the
// page's requests through the functions the wasm exports. It runs the engine off the page's thread,
// so a check that takes seconds never freezes the viewer, and the page talks to it one message per
// RPC.
//
// It is a classic worker because Go's wasm_exec.js is a script that defines globalThis.Go, which
// importScripts loads and a module worker cannot.
import type { EngineReply, EngineRequest, Files, Ready } from "./protocol.js";

// The worker globals this file uses. The project compiles against the DOM lib, which describes a
// window, and the WebWorker lib cannot be loaded beside it, so the few worker globals are declared here.
declare function importScripts(...urls: string[]): void;
declare const self: {
  location: Location;
  postMessage(message: unknown, transfer?: Transferable[]): void;
  onmessage: ((ev: MessageEvent<EngineRequest>) => void) | null;
  Go: new () => { importObject: WebAssembly.Imports; run(i: WebAssembly.Instance): Promise<void> };
  agniReady?: () => void;
  agniMount?: (name: string, files: Files) => Error | undefined | null;
  agniHTTP?: (
    method: string,
    url: string,
    headers: Record<string, string>,
    body: Uint8Array,
  ) => Promise<{ status: number; headers: Record<string, string>; body: Uint8Array }>;
};

// The asset URLs come in the worker's own query string, so the page decides where they are served.
const params = new URLSearchParams(self.location.search);
const wasmUrl = params.get("wasm") ?? "agni.wasm";
const execUrl = params.get("exec") ?? "wasm_exec.js";

// memory is the engine's linear memory. It only grows, so its size is the engine's peak.
let memory: WebAssembly.Memory | undefined;

async function boot(): Promise<void> {
  importScripts(execUrl);
  const go = new self.Go();
  const ready = new Promise<void>((resolve) => {
    self.agniReady = resolve;
  });
  const { instance } = await WebAssembly.instantiateStreaming(fetch(wasmUrl), go.importObject);
  memory = instance.exports.mem as WebAssembly.Memory;
  void go.run(instance);
  await ready;
}

const booted = boot().then(
  () => self.postMessage({ ready: true } satisfies Ready),
  (err: unknown) => self.postMessage({ ready: false, error: String(err) } satisfies Ready),
);

self.onmessage = async (ev) => {
  await booted;
  const req = ev.data;
  try {
    if (req.kind === "stats") {
      self.postMessage({ id: req.id, ok: true, memoryBytes: memory?.buffer.byteLength ?? 0 } satisfies EngineReply);
      return;
    }
    if (req.kind === "mount") {
      const err = self.agniMount?.(req.name, req.files);
      if (err) throw err;
      self.postMessage({ id: req.id, ok: true } satisfies EngineReply);
      return;
    }
    if (!self.agniHTTP) throw new Error("the engine did not load");
    const res = await self.agniHTTP(req.method, req.url, req.headers, req.body);
    self.postMessage({ id: req.id, ok: true, status: res.status, headers: res.headers, body: res.body } satisfies EngineReply, [
      res.body.buffer,
    ]);
  } catch (err) {
    self.postMessage({ id: req.id, ok: false, error: String(err) } satisfies EngineReply);
  }
};
