import { describe, expect, it, vi } from "vitest";
import { admit, type AdmitDeps } from "./admit.js";

function deps(over: Partial<AdmitDeps> = {}): AdmitDeps {
  return {
    pathname: "/designs/m/b.edn/view",
    search: "",
    dropMountPrefix: "/designs/local/",
    stamp: async () => "s1",
    loaded: () => false,
    ...over,
  };
}

// agni issue 1072: the viewer opens only a design the Load page read, and sends any other there with
// its address whole, so a link lands where it pointed once the design is read.
describe("whether the viewer opens its design", () => {
  it("sends a design with no record to its Load page, the query kept", async () => {
    const a = await admit(deps({ search: "?verdict=v&rule=r" }));
    expect(a).toEqual({ open: false, goTo: "/designs/m/b.edn/load?verdict=v&rule=r" });
  });

  it("opens a design whose record matches its files now", async () => {
    const loaded = vi.fn(() => true);
    expect(await admit(deps({ loaded }))).toEqual({ open: true });
    expect(loaded).toHaveBeenCalledWith("m", "b.edn", "s1");
  });

  it("sends a design whose files changed since it was read, or cannot be listed, to the Load page", async () => {
    expect((await admit(deps({ loaded: (_m, _p, stamp) => stamp === "old" }))).open).toBe(false);
    expect((await admit(deps({ stamp: async () => { throw new Error("offline"); }, loaded: () => true }))).open).toBe(false);
    expect((await admit(deps({ stamp: async () => "", loaded: () => true }))).open).toBe(false);
  });

  // The Load page may have been unable to write its record (a private window), so the marker it
  // sends is what keeps the two pages from sending a reader back and forth.
  it("opens a design the Load page has just sent, and drops the marker from the address", async () => {
    const stamp = vi.fn(async () => "s1");
    const a = await admit(deps({ search: "?sheet=s&loaded=1", stamp }));
    expect(a).toEqual({ open: true, replaceWith: "/designs/m/b.edn/view?sheet=s" });
    expect(stamp).not.toHaveBeenCalled();
  });

  it("sends the browser mount's bare address to the drop page and leaves any other bare address alone", async () => {
    expect(await admit(deps({ pathname: "/designs/local/view/" }))).toEqual({ open: false, goTo: "/designs/local/load/" });
    expect(await admit(deps({ pathname: "/designs/m/view" }))).toEqual({ open: true });
  });
});
