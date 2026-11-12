import { describe, it, expect, vi, type Mock } from "vitest";
import { create } from "@bufbuild/protobuf";
import { artifactUri } from "@agni/web-shared/uri.js";
import { type ViewerLocation } from "./router.js";
import { ViewerPresenter, type RenderView } from "./viewer.js";
import { SheetFormat, TraceSchema, TraceOutcome } from "./gen/agni/v1/webapi/design_pb.js";
import { stubQueryView } from "./testviews.js";
import type { TraceState } from "./trace.js";
import { fakeSnapshots, snapName } from "./testsnaps.js";

// harness builds a presenter with only what the trace path needs. Every other collaborator is a stub
// that does just enough for openFile to complete. With wireTrace off the panel is unwired, and that
// case must be a no-op rather than a throw, since the presenter's view ports are optional by design.
function harness(opts: { wireTrace?: boolean; trace?: ReturnType<typeof create<typeof TraceSchema>>; fail?: Error; onLocation?: Mock<(loc: ViewerLocation) => void>; sheets?: string[] } = {}) {
  const wireTrace = opts.wireTrace !== false;
  const answer =
    opts.trace ??
    create(TraceSchema, {
      from: { endpoint: { refDes: "U1", pin: "3" }, pinName: "SDA", net: "SDA" },
      to: { endpoint: { refDes: "J1", pin: "1" }, pinName: "VBUS", net: "VCC" },
      outcome: TraceOutcome.ROUTED,
      radius: 6,
      crossings: [{ refDes: "R1", class: "resistor", enterPin: "2", exitPin: "1", fromNet: "SDA", toNet: "VCC" }],
      nets: [
        { name: "SDA", stubs: [], stubsElided: 0, busLike: false, sheetIds: opts.sheets ?? [] },
        { name: "VCC", stubs: [], stubsElided: 0, busLike: true, sheetIds: [] },
      ],
    });
  const traceDesign = vi.fn(async (_req: { snapshot: string; from: unknown; to: unknown; hops?: number }) => {
    if (opts.fail) throw opts.fail;
    return { trace: answer };
  });
  const client = {
    getDesign: vi.fn(async () => ({
      name: "D", layout: "faithful", sourceFormat: "", componentCount: 0, netCount: 0,
      sheets: [{ id: "s1", name: "S1" }, { id: "s2", name: "S2" }], nativeAvailable: false, availableLayouts: ["faithful"],
    })),
    getSheet: vi.fn(async (req: { format?: SheetFormat }) =>
      req.format === SheetFormat.SVG
        ? { content: { case: "svg", value: "<svg/>" } }
        : { content: { case: "packed", value: { sheetId: "s1" } } },
    ),
    highlightSheet: vi.fn(async () => ({ content: { case: "svg", value: "<svg/>" } })),
    getLayoutReport: vi.fn(async () => ({ report: { components: [] } })),
    traceDesign,
    reachablePins: vi.fn(async (req: { from: { refDes: string; pin: string } }) =>
      req.from.refDes === "U99"
        ? { reason: "no component U99 in this design", radius: 6, pins: [], rails: [] }
        : {
            reason: "",
            radius: 2,
            pins: [
              { end: { endpoint: { refDes: "TP1", pin: "1" }, pinName: "", net: "SDA" }, depth: 0 },
              { end: { endpoint: { refDes: "J1", pin: "1" }, pinName: "VBUS", net: "VCC" }, depth: 1 },
            ],
            rails: [{ net: "GND", depth: 2, pins: 412 }],
          },
    ),
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
  } as any;
  const checks = {
    listRules: vi.fn(async () => ({ rules: [] })),
    checkDesign: vi.fn(async () => ({ findings: [] })),
    getExpectations: vi.fn(async () => ({ expectations: [] })),
    getInterfaceCoverage: vi.fn(async () => ({ interfaces: [] })),
    getPartParams: vi.fn(async () => ({ parts: [] })),
    getNamingConvention: vi.fn(async () => ({ convention: { name: "", rules: [], lexicon: undefined } })),
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
  } as any;
  const canvas = {
    setSheet: vi.fn(), setHighlights: vi.fn(), clear: vi.fn(), resize: vi.fn(),
    onPick: vi.fn(), setBoardLayers: vi.fn(),
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
  } as any;
  const render: RenderView = {
    showSvg: vi.fn(), setSvgOverlay: vi.fn(), setBusy: vi.fn(),
    getView: vi.fn((mode) => `view-${mode}`), setView: vi.fn(), setBoardLayers: vi.fn(),
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
  } as any;
  const onTrace = vi.fn();
  const underlay = vi.fn();
  const presenter = new ViewerPresenter(client, checks, canvas, render, {
    sheetNavs: [],
    summary: vi.fn(),
    controls: { setState: vi.fn() },
    findings: { setState: vi.fn(), setFindingLocateNote: vi.fn() },
    expectationCaption: vi.fn(),
    undrawnNote: vi.fn(),
    staleLinkNote: vi.fn(),
    hierarchyNote: vi.fn(),
    drawingNote: vi.fn(),
    rules: { setState: vi.fn() },
    report: vi.fn(),
    query: stubQueryView(),
    trace: wireTrace ? { setState: onTrace } : undefined,
    underlay,
    location: opts.onLocation,
  }, undefined, undefined, undefined, undefined, fakeSnapshots());
  return { presenter, onTrace, traceDesign, canvas, getSheet: client.getSheet, reachablePins: client.reachablePins, underlay };
}

function lastState(onTrace: ReturnType<typeof vi.fn>): TraceState {
  const calls = onTrace.mock.calls;
  return calls[calls.length - 1][0] as TraceState;
}

describe("trace presenter", () => {
  it("asks the service about the open design and pushes the answer", async () => {
    const h = harness();
    await h.presenter.openFile("m", "proj/board.edn");
    await h.presenter.runTrace("U1.3", "J1.1");
    expect(h.traceDesign).toHaveBeenCalledWith({
      snapshot: snapName(artifactUri("m", "proj/board.edn")),
      from: { refDes: "U1", pin: "3" },
      to: { refDes: "J1", pin: "1" },
      hops: 0,
    });
    const s = lastState(h.onTrace);
    expect(s.outcome).toBe(TraceOutcome.ROUTED);
    expect(s.crossings.map((c) => c.refDes)).toEqual(["R1"]);
  });

  // The route has to reach the canvas, or the panel is a text box that happens to sit beside a
  // drawing. The whole PR exists for this assertion.
  it("lights the route on the canvas", async () => {
    const h = harness();
    await h.presenter.openFile("m", "proj/board.edn");
    h.canvas.setHighlights.mockClear();
    await h.presenter.runTrace("U1.3", "J1.1");
    expect(h.canvas.setHighlights).toHaveBeenCalled();
    const specs = h.canvas.setHighlights.mock.calls.at(-1)![0] as { nets?: string[]; components?: string[] }[];
    const nets = specs.flatMap((s) => s.nets ?? []);
    const comps = specs.flatMap((s) => s.components ?? []);
    expect(nets).toEqual(expect.arrayContaining(["SDA", "VCC"]));
    expect(comps).toContain("R1");
  });

  // A typo in the box is caught here rather than at the server. The panel is showing the box, and a
  // round trip to be told the same thing is only slower.
  it("rejects a malformed pin without calling the service", async () => {
    const h = harness();
    await h.presenter.openFile("m", "proj/board.edn");
    await h.presenter.runTrace("U1", "J1.1");
    expect(h.traceDesign).not.toHaveBeenCalled();
    expect(lastState(h.onTrace).error).toContain("<ref-des>.<pin>");
  });

  // A failed REQUEST and an unresolved ANSWER are different things, and the panel renders them
  // differently, so the presenter must not fold one into the other.
  it("reports a transport failure as an error, not as an outcome", async () => {
    const h = harness({ fail: new Error("connection refused") });
    await h.presenter.openFile("m", "proj/board.edn");
    await h.presenter.runTrace("U1.3", "J1.1");
    const s = lastState(h.onTrace);
    expect(s.error).toContain("connection refused");
    expect(s.outcome).toBe(TraceOutcome.UNSPECIFIED);
  });

  it("does nothing when the host left the panel out", async () => {
    const h = harness({ wireTrace: false });
    await h.presenter.openFile("m", "proj/board.edn");
    await expect(h.presenter.runTrace("U1.3", "J1.1")).resolves.toBeUndefined();
    expect(h.traceDesign).not.toHaveBeenCalled();
  });
});

describe("a trace arriving in the URL", () => {
  it("asks the question on load, with no check run first", async () => {
    const h = harness();
    await h.presenter.restore({
      mount: "m", path: "proj/board.edn", isDir: false, sheet: "", mode: "", layout: "",
      symbols: false, flatten: false, verdict: "", hash: "", rule: "", subjects: [], trace: "U1.3,J1.1", traceHops: 0,
      query: "",
      title: "",
      binds: [],
      set: "",
      q: "",
      against: "", renameApprox: false,
    });
    expect(h.traceDesign).toHaveBeenCalledWith({
      snapshot: snapName(artifactUri("m", "proj/board.edn")),
      from: { refDes: "U1", pin: "3" },
      to: { refDes: "J1", pin: "1" },
      hops: 0,
    });
  });

  // A link that dropped the radius would re-ask a NARROWER question wider, so a run reporting "no
  // route within 2 crossings" could show a route to whoever followed the link.
  it("carries the radius the link pinned", async () => {
    const h = harness();
    await h.presenter.restore({
      mount: "m", path: "proj/board.edn", isDir: false, sheet: "", mode: "", layout: "",
      symbols: false, flatten: false, verdict: "", hash: "", rule: "", subjects: [], trace: "U1.3,J1.1", traceHops: 2,
      query: "",
      title: "",
      binds: [],
      set: "",
      q: "",
      against: "", renameApprox: false,
    });
    const calls = h.traceDesign.mock.calls;
    expect(calls[calls.length - 1][0].hops).toBe(2);
  });

  it("does not trace when the URL names none", async () => {
    const h = harness();
    await h.presenter.restore({
      mount: "m", path: "proj/board.edn", isDir: false, sheet: "", mode: "", layout: "",
      symbols: false, flatten: false, verdict: "", hash: "", rule: "", subjects: [], trace: "", traceHops: 0,
      query: "",
      title: "",
      binds: [],
      set: "",
      q: "",
      against: "", renameApprox: false,
    });
    expect(h.traceDesign).not.toHaveBeenCalled();
  });
});

// The address bar and the panel must not disagree. A route found by typing pins is addressable
// without the reader doing anything, and a NEGATIVE answer is addressable too, since "these two pins
// do not join" is a thing worth sending someone.
describe("a trace reaching the URL", () => {
  it("reports the question it asked, whatever the outcome", async () => {
    const onLocation = vi.fn<(loc: ViewerLocation) => void>();
    const h = harness({ onLocation });
    await h.presenter.openFile("m", "proj/board.edn");
    await h.presenter.runTrace("U1.3", "J1.1");
    const locCalls = onLocation.mock.calls;
    const loc = locCalls[locCalls.length - 1][0];
    expect(loc.trace).toBe("U1.3,J1.1");
    expect(loc.traceHops).toBe(0);
  });
});

// A route is drawn on a sheet, and before agni 657 the viewer never went to it. runTrace painted the
// highlight onto whichever sheet happened to be showing, which on a cold link is the design's first.
// On an 82-sheet export that is a table of contents with no wires on it at all.
describe("a trace goes to a sheet the route is on", () => {
  it("navigates when the route is drawn elsewhere", async () => {
    const h = harness({ sheets: ["s2"] });
    await h.presenter.openFile("m", "proj/board.edn");
    h.getSheet.mockClear();
    await h.presenter.runTrace("U1.3", "J1.1");
    const asked = h.getSheet.mock.calls.map((c: unknown[]) => (c[0] as { sheet?: string }).sheet);
    expect(asked).toContain("s2");
  });

  // The control for the case above. With the route drawn nowhere there is no sheet to go to, and
  // jumping somewhere arbitrary would be worse than staying. Without it the test above passes on a
  // presenter that navigates unconditionally.
  it("stays put when nothing on the route is drawn", async () => {
    const h = harness({ sheets: [] });
    await h.presenter.openFile("m", "proj/board.edn");
    h.getSheet.mockClear();
    await h.presenter.runTrace("U1.3", "J1.1");
    const asked = h.getSheet.mock.calls.map((c: unknown[]) => (c[0] as { sheet?: string }).sheet);
    expect(asked).not.toContain("s2");
  });
});

// The Trace panel narrows its end pin to what a trace from the start pin reaches (agni issue 1541).
describe("reach presenter", () => {
  it("asks the engine what a start pin reaches on the open design, within the hops asked", async () => {
    const h = harness();
    await h.presenter.openFile("m", "proj/board.edn");
    const r = await h.presenter.reachFrom("U1.3", 2);
    expect(h.reachablePins).toHaveBeenCalledWith({ snapshot: snapName(artifactUri("m", "proj/board.edn")), from: { refDes: "U1", pin: "3" }, hops: 2 });
    expect(r).toEqual({
      from: "U1.3",
      hops: 2,
      reason: "",
      pins: [
        { ref: "TP1", pin: "1", name: "", net: "SDA", depth: 0 },
        { ref: "J1", pin: "1", name: "VBUS", net: "VCC", depth: 1 },
      ],
      rails: [{ net: "GND", depth: 2, pins: 412 }],
    });
  });

  it("passes on a reason, and asks nothing for text that names no pin or before a design is open", async () => {
    const h = harness();
    expect(await h.presenter.reachFrom("U1.3", 6)).toBeNull();
    await h.presenter.openFile("m", "proj/board.edn");
    expect((await h.presenter.reachFrom("U99.1", 6))?.reason).toBe("no component U99 in this design");
    expect(await h.presenter.reachFrom("U1", 6)).toBeNull();
    expect(h.reachablePins).toHaveBeenCalledTimes(1);
  });
});

// A trace lights its route on the sheet views beside the canvas too, and a following view frames it
// (agni issue 1551). Before, only the canvas lit it.
describe("trace on the sheet views", () => {
  it("hands the views the route the canvas lights, to frame", async () => {
    const h = harness();
    await h.presenter.openFile("m", "proj/board.edn");
    await h.presenter.runTrace("U1.3", "J1.1");
    expect(h.underlay).toHaveBeenCalledTimes(1);
    const [specs, reveal] = h.underlay.mock.calls[0] as [{ nets?: string[]; components?: string[] }[], boolean];
    expect(reveal).toBe(true);
    expect(specs.flatMap((s) => s.nets ?? []).sort()).toEqual(["SDA", "VCC"]);
    expect(specs.flatMap((s) => s.components ?? [])).toEqual(["R1"]);
  });

  it("leaves the views as they were when the next trace fails, as the canvas is left", async () => {
    const h = harness();
    await h.presenter.openFile("m", "proj/board.edn");
    await h.presenter.runTrace("U1.3", "J1.1");
    h.traceDesign.mockRejectedValueOnce(new Error("the server went away"));
    await h.presenter.runTrace("U1.3", "J1.2");
    expect(h.underlay).toHaveBeenCalledTimes(1);
  });
});
