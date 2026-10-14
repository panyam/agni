// fetchfile reads a design's file for the browser engine without hanging on a stalled transfer (agni
// issue 1002). A static host serves a large design as plain files, and a fetch whose body stops
// arriving has no timeout of its own, so one stalled file held the page's first open until a visitor
// gave up. Each attempt here is aborted when no bytes arrive for `stallMs`, and tried again.

// FetchFileOptions configures fetchFile.
export interface FetchFileOptions {
  // stallMs is how long a transfer may go without a byte before it is aborted and tried again.
  stallMs?: number;
  // attempts is how many times the file is asked for in all, the first included.
  attempts?: number;
  // onStall hears each abandoned attempt: the file, how many bytes it had and which attempt it was.
  onStall?: (s: FetchStall) => void;
  // onDone hears a completed read, for the page's timing log.
  onDone?: (d: FetchDone) => void;
}

// FetchStall is one attempt abandoned because its bytes stopped arriving.
export interface FetchStall {
  url: string;
  attempt: number;
  attempts: number;
  bytes: number;
  ms: number;
}

// FetchDone is a completed read: its size, the time from the first attempt and how many it took.
export interface FetchDone {
  url: string;
  bytes: number;
  ms: number;
  attempts: number;
}

export const STALL_MS = 30_000;
export const ATTEMPTS = 3;

// StalledError is the error fetchFile throws when every attempt stalled, naming the file, so whatever
// reports the failure can say which file never arrived.
export class StalledError extends Error {
  constructor(
    readonly url: string,
    readonly attempts: number,
    stallMs: number,
  ) {
    super(`${url}: no bytes for ${stallMs / 1000} s in each of ${attempts} attempts`);
  }
}

// fetchFile returns the bytes at url, read through fetch. A response that is not ok fails at once,
// since asking again returns the same status; only a stall is retried.
export async function fetchFile(fetch: typeof globalThis.fetch, url: URL | string, o: FetchFileOptions = {}): Promise<Uint8Array> {
  const stallMs = o.stallMs ?? STALL_MS;
  const attempts = o.attempts ?? ATTEMPTS;
  const href = String(url);
  const start = performance.now();
  for (let attempt = 1; attempt <= attempts; attempt++) {
    const got = await attemptOnce(fetch, href, stallMs);
    if ("bytes" in got) {
      o.onDone?.({ url: href, bytes: got.bytes.length, ms: performance.now() - start, attempts: attempt });
      return got.bytes;
    }
    o.onStall?.({ url: href, attempt, attempts, bytes: got.stalledAt, ms: performance.now() - start });
  }
  throw new StalledError(href, attempts, stallMs);
}

// attemptOnce reads url once, returning its bytes, or how many bytes it had when it stalled.
async function attemptOnce(fetch: typeof globalThis.fetch, url: string, stallMs: number): Promise<{ bytes: Uint8Array } | { stalledAt: number }> {
  const ctl = new AbortController();
  let received = 0;
  let stalled = false;
  let timer: ReturnType<typeof setTimeout> | undefined;
  const arm = () => {
    clearTimeout(timer);
    timer = setTimeout(() => {
      stalled = true;
      ctl.abort();
    }, stallMs);
  };
  arm();
  try {
    const res = await fetch(url, { signal: ctl.signal });
    if (!res.ok) throw new Error(`${url}: ${res.status}`);
    if (!res.body) return { bytes: new Uint8Array(await res.arrayBuffer()) };
    const chunks: Uint8Array[] = [];
    const reader = res.body.getReader();
    for (;;) {
      const { done, value } = await reader.read();
      if (done) break;
      chunks.push(value);
      received += value.length;
      arm();
    }
    const out = new Uint8Array(received);
    let at = 0;
    for (const c of chunks) {
      out.set(c, at);
      at += c.length;
    }
    return { bytes: out };
  } catch (e) {
    if (stalled) return { stalledAt: received };
    throw e;
  } finally {
    clearTimeout(timer);
  }
}
