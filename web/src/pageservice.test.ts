import { describe, expect, it, vi } from "vitest";
import { Code, ConnectError } from "@connectrpc/connect";
import { ViewerLocationSchema } from "./gen/agni/v1/viewer/page_pb.js";
import type { ViewerPageAPI } from "./pageapi.js";
import { isLinkOnly, locationFromLink, locationFromWire, locationToWire, pageDispatcher } from "./pageservice.js";
import { emptyLocation, type ViewerLocation } from "./router.js";

// full sets every field of a location to a value distinguishable from its zero, and every list to
// two elements, so a conversion dropping a field or all but one element cannot round-trip it (C26).
function full(): ViewerLocation {
  return {
    mount: "demo",
    path: "boards/demo-board.kicad_sch",
    isDir: false,
    sheet: "s-power",
    mode: "webgl",
    layout: "grid",
    symbols: true,
    flatten: true,
    verdict: "demo-pull-up:(net:SDA)",
    hash: "sha256:00ff",
    rule: "demo-pull-up",
    subjects: ["net:SDA", "pin:U1.3"],
    trace: "U1.3,J1.1",
    traceHops: 3,
    query: "net(?n)",
    title: "Nets",
    binds: ["n=SDA", "n=SCL"],
    set: "power",
    q: "rails",
    against: "demo/boards/demo-board-revb.kicad_sch",
    renameApprox: true,
  };
}

function fakeApi(over: Partial<ViewerPageAPI> = {}): ViewerPageAPI {
  return {
    location: () => ({ ...emptyLocation(), mount: "demo", path: "d.kicad_sch", sheet: "s-root" }),
    applyLocation: vi.fn(async () => ({ location: emptyLocation(), verdictResolved: false })),
    locateVerdict: vi.fn(async () => true),
    selectFinding: vi.fn(async () => true),
    locateEntity: vi.fn(async () => {}),
    showSheet: vi.fn(async () => {}),
    ...over,
  };
}

describe("ViewerLocation on the wire", () => {
  it("round-trips every field of the router's location (C26)", () => {
    const { isDir: _isDir, ...want } = full();
    expect(locationFromWire(locationToWire(full()))).toEqual(want);
  });

  // The census the round trip cannot be: a field added to the proto and not to the conversion reads
  // as zero on both sides of the round trip, so this compares the two field lists themselves.
  it("names the same fields as the router's location, less isDir", () => {
    const wire = ViewerLocationSchema.fields.map((f) => f.localName).sort();
    const ts = Object.keys(full()).filter((k) => k !== "isDir").sort();
    expect(wire).toEqual(ts);
  });

  it("reads an empty request field as 'leave it alone'", () => {
    expect(locationFromWire(locationToWire({ ...emptyLocation(), symbols: false, flatten: false }))).toEqual({ symbols: false, flatten: false });
    expect(locationFromWire(undefined)).toEqual({});
  });

  it("refuses a mode the viewer cannot render", () => {
    expect(() => locationFromWire(locationToWire({ ...emptyLocation(), mode: "pdf" as never }))).toThrow(/mode "pdf"/);
  });
});

describe("pageDispatcher", () => {
  it("answers a method by its bare name and by its Connect path", async () => {
    const api = fakeApi();
    const dispatch = pageDispatcher(api);
    const body = { id: "demo-pull-up:(net:SDA)", rule: "demo-pull-up", subjects: ["net:SDA"] };
    for (const name of ["LocateVerdict", "agni.v1.viewer.ViewerPageService/LocateVerdict", "/agni.v1.viewer.ViewerPageService/LocateVerdict"]) {
      const res = (await dispatch(name, body)) as { resolved: boolean; location: { sheet: string } };
      expect(res.resolved, name).toBe(true);
      expect(res.location.sheet, name).toBe("s-root");
    }
    expect(api.locateVerdict).toHaveBeenCalledWith("demo-pull-up:(net:SDA)", { rule: "demo-pull-up", subjects: ["net:SDA"] });
  });

  it("passes only the fields a caller set through ApplyLocation", async () => {
    const api = fakeApi();
    await pageDispatcher(api)("ApplyLocation", { location: { verdict: "r:(net:A)", rule: "r", subjects: ["net:A"] } });
    expect(api.applyLocation).toHaveBeenCalledWith({ verdict: "r:(net:A)", rule: "r", subjects: ["net:A"] });
  });

  it("says unimplemented for a method the service lacks", async () => {
    await expect(pageDispatcher(fakeApi())("ReloadPage", {})).rejects.toMatchObject({ code: Code.Unimplemented });
  });

  it("says invalid_argument for a body that does not parse", async () => {
    await expect(pageDispatcher(fakeApi())("ShowSheet", { sheet: 7 })).rejects.toMatchObject({ code: Code.InvalidArgument });
    await expect(pageDispatcher(fakeApi())("ShowSheet", { page: "s1" })).rejects.toMatchObject({ code: Code.InvalidArgument });
  });

  it("says invalid_argument for a request the page refuses, and keeps a ConnectError's own code", async () => {
    const refuse = fakeApi({ showSheet: async () => Promise.reject(new Error('the open design has no sheet "s9"')) });
    await expect(pageDispatcher(refuse)("ShowSheet", { sheet: "s9" })).rejects.toMatchObject({ code: Code.InvalidArgument, rawMessage: 'the open design has no sheet "s9"' });
    const busy = fakeApi({ showSheet: async () => Promise.reject(new ConnectError("busy", Code.Unavailable)) });
    await expect(pageDispatcher(busy)("ShowSheet", { sheet: "s1" })).rejects.toMatchObject({ code: Code.Unavailable });
  });
});

describe("ApplyLocation with a link", () => {
  const link = "http://localhost:8080/designs/kicad/hier_numeric_root.kicad_sch/view?verdict=duplicate-ref-des%3A%28component%3AR2%29&rule=duplicate-ref-des&subject=component%3AR2";

  it("applies the fields the link sets, read by the page's own parser", async () => {
    const api = fakeApi();
    await pageDispatcher(api)("ApplyLocation", { url: link });
    expect(api.applyLocation).toHaveBeenCalledWith({
      mount: "kicad",
      path: "hier_numeric_root.kicad_sch",
      verdict: "duplicate-ref-des:(component:R2)",
      rule: "duplicate-ref-des",
      subjects: ["component:R2"],
    });
  });

  it("applies a switch a link turns on, and leaves one it omits alone", () => {
    expect(locationFromLink(`${link}&sym=1&flatten=1`)).toMatchObject({ symbols: true, flatten: true });
    expect(locationFromLink(link)).not.toHaveProperty("symbols");
    expect(locationFromLink(link)).not.toHaveProperty("flatten");
  });

  it("reads a link on another host as the same design", () => {
    expect(locationFromLink(link.replace("localhost", "127.0.0.1"))).toEqual(locationFromLink(link));
  });

  it("refuses a link to no design, and a request naming both a link and a location", async () => {
    const dispatch = pageDispatcher(fakeApi());
    await expect(dispatch("ApplyLocation", { url: "http://localhost:8080/designs/kicad/" })).rejects.toMatchObject({ code: Code.InvalidArgument });
    await expect(dispatch("ApplyLocation", { url: "https://example.com/elsewhere" })).rejects.toMatchObject({ code: Code.InvalidArgument });
    await expect(dispatch("ApplyLocation", { url: link, location: { sheet: "s1" } })).rejects.toMatchObject({ code: Code.InvalidArgument });
  });

  it("knows a link-only request from every other", () => {
    expect(isLinkOnly("ApplyLocation", { url: link })).toBe(true);
    expect(isLinkOnly("agni.v1.viewer.ViewerPageService/ApplyLocation", { url: link })).toBe(true);
    expect(isLinkOnly("ApplyLocation", { url: link, location: {} })).toBe(false);
    expect(isLinkOnly("ApplyLocation", { location: { sheet: "s1" } })).toBe(false);
    expect(isLinkOnly("ApplyLocation", {})).toBe(false);
    expect(isLinkOnly("ShowSheet", { url: link })).toBe(false);
    expect(isLinkOnly("GetLocation", {})).toBe(false);
  });
});
