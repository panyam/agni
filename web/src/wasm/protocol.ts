// The messages between the page and the engine worker. Every request carries an id, and the worker
// answers each id once, so the page can have several RPCs in flight at a time.

// Files is one mount's tree, keyed by slash-separated path within the mount.
export type Files = Record<string, Uint8Array>;

export type EngineRequest =
  | { id: number; kind: "mount"; name: string; files: Files }
  | { id: number; kind: "http"; method: string; url: string; headers: Record<string, string>; body: Uint8Array }
  // stats asks how much memory the engine holds, for measurement (agni issue 852).
  | { id: number; kind: "stats" };

export type EngineReply =
  | { id: number; ok: true; status?: number; headers?: Record<string, string>; body?: Uint8Array; memoryBytes?: number }
  | { id: number; ok: false; error: string };

// Ready is the worker's first message, once the engine has composed, or the reason it could not load.
export type Ready = { ready: true } | { ready: false; error: string };
