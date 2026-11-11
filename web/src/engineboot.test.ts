import { afterEach, describe, expect, it, vi } from "vitest";
import { artifactUri } from "@agni/web-shared/uri.js";
import { designListing, sharedListing } from "./engineboot.js";
import { reply } from "./testconnect.js";
import type { DropStore } from "./drops.js";

const LISTING = { mount: "m", files: [{ path: "d/b.kicad_pcb", sha256: "sha256:b", size: "85" }], totalSize: "85" };
const drops = {} as DropStore;
const fetchBefore = globalThis.fetch;

// listed counts the page's ListDesignFiles calls, answered with LISTING.
function listed(): { calls: () => number } {
  let calls = 0;
  globalThis.fetch = (async (input: RequestInfo | URL, init?: RequestInit) => {
    const req = input instanceof Request ? input : new Request(new URL(String(input), window.location.href), init);
    if (req.url.endsWith("/ListDesignFiles")) calls++;
    return reply(req, LISTING);
  }) as typeof globalThis.fetch;
  return { calls: () => calls };
}

afterEach(() => {
  globalThis.fetch = fetchBefore;
});

// agni issue 1527: a page lists each design once, for the Load page's record and the engine's bring.
describe("the page's design listing", () => {
  it("serves the browser engine's bring from the listing the record check took", async () => {
    const net = listed();
    const l = await designListing("m", "d", drops);
    expect(l?.files).toBe(1);
    const bring = vi.fn(async () => {
      throw new Error("listed again");
    });
    const files = await sharedListing(artifactUri("m", "d"), bring);
    expect(files.files.map((f) => f.path)).toEqual(["d/b.kicad_pcb"]);
    expect(bring).not.toHaveBeenCalled();
    expect(net.calls()).toBe(1);
  });

  it("asks again after a listing that failed", async () => {
    await expect(sharedListing("mount://m/failed", async () => Promise.reject(new Error("down")))).rejects.toThrow("down");
    const again = vi.fn(async () => ({ mount: "m", files: [], totalSize: 0 }));
    await sharedListing("mount://m/failed", again);
    expect(again).toHaveBeenCalledOnce();
  });
});
