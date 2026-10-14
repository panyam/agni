import { describe, expect, it } from "vitest";
import { fetchFile, StalledError, type FetchStall } from "./fetchfile.js";

// A fetch whose body sends `head` and then, on the attempts listed in `stallOn`, never sends the rest,
// as a static host's transfer did on the deployed demo (agni issue 1002). It honours the abort signal
// as a browser's fetch does.
function host(stallOn: number[], status = 200) {
  let calls = 0;
  const f = (async (_url: RequestInfo | URL, init?: RequestInit) => {
    calls++;
    const stall = stallOn.includes(calls);
    const signal = init?.signal;
    const body = new ReadableStream<Uint8Array>({
      start(c) {
        c.enqueue(new Uint8Array([1, 2]));
        if (!stall) {
          c.enqueue(new Uint8Array([3]));
          c.close();
          return;
        }
        signal?.addEventListener("abort", () => c.error(new DOMException("aborted", "AbortError")));
      },
    });
    return new Response(status === 200 ? body : null, { status });
  }) as typeof globalThis.fetch;
  return { f, calls: () => calls };
}

describe("fetchFile", () => {
  it("reads a file whole", async () => {
    const h = host([]);
    expect([...(await fetchFile(h.f, "http://h/raw/m/a.kicad_pcb", { stallMs: 50 }))]).toEqual([1, 2, 3]);
    expect(h.calls()).toBe(1);
  });

  it("abandons a transfer whose bytes stop arriving and asks again", async () => {
    const h = host([1]);
    const stalls: FetchStall[] = [];
    const got = await fetchFile(h.f, "http://h/raw/m/a.kicad_pcb", { stallMs: 30, onStall: (s) => stalls.push(s) });
    expect([...got]).toEqual([1, 2, 3]);
    expect(h.calls()).toBe(2);
    expect(stalls).toHaveLength(1);
    expect(stalls[0]).toMatchObject({ url: "http://h/raw/m/a.kicad_pcb", attempt: 1, bytes: 2 });
  });

  it("names the file when every attempt stalls", async () => {
    const h = host([1, 2, 3]);
    const err = await fetchFile(h.f, "http://h/raw/m/a.kicad_pcb", { stallMs: 20 }).catch((e: unknown) => e);
    expect(err).toBeInstanceOf(StalledError);
    expect(String(err)).toContain("a.kicad_pcb");
    expect(h.calls()).toBe(3);
  });

  it("does not ask again for a file the host refused", async () => {
    const h = host([], 404);
    await expect(fetchFile(h.f, "http://h/raw/m/gone.kicad_pcb", { stallMs: 20 })).rejects.toThrow(/404/);
    expect(h.calls()).toBe(1);
  });
});
