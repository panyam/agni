import { describe, expect, it, vi } from "vitest";
import { Code, ConnectError } from "@connectrpc/connect";
import { ViewerLocationSchema } from "./gen/agni/v1/viewer/page_pb.js";
import type { ViewerPageAPI } from "./pageapi.js";
import { locationFromWire, locationToWire, pageDispatcher } from "./pageservice.js";
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
