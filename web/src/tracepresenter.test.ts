import { describe, it, expect, vi } from "vitest";
import { create } from "@bufbuild/protobuf";
import { artifactUri } from "./uri.js";
import { ViewerPresenter, type RenderView } from "./viewer.js";
import { SheetFormat, TraceSchema, TraceOutcome } from "./gen/agni/v1/webapi/design_pb.js";
import { stubQueryView } from "./testviews.js";
import type { TraceState } from "./trace.js";

// harness builds a presenter with only what the trace path needs; every other collaborator is a stub
// sufficient for openFile to complete. wireTrace off is the unwired-panel case, which must be a
// no-op rather than a throw, since the presenter's view ports are optional by design.
function harness(opts: { wireTrace?: boolean; trace?: ReturnType<typeof create<typeof TraceSchema>>; fail?: Error; onLocation?: ReturnType<typeof vi.fn> } = {}) {
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
        { name: "SDA", stubs: [], stubsElided: 0, busLike: false },
        { name: "VCC", stubs: [], stubsElided: 0, busLike: true },
      ],
    });
  const traceDesign = vi.fn(async (_req: { uri: string; from: unknown; to: unknown; hops?: number }) => {
    if (opts.fail) throw opts.fail;
    return { trace: answer };
  });
  const client = {
    getDesign: vi.fn(async () => ({
      name: "D", layout: "faithful", sourceFormat: "", componentCount: 0, netCount: 0,
      sheets: [{ id: "s1", name: "S1" }], nativeAvailable: false, availableLayouts: ["faithful"],
    })),
    getSheet: vi.fn(async (req: { format?: SheetFormat }) =>
      req.format === SheetFormat.SVG
        ? { content: { case: "svg", value: "<svg/>" } }
        : { content: { case: "packed", value: { sheetId: "s1" } } },
    ),
    highlightSheet: vi.fn(async () => ({ content: { case: "svg", value: "<svg/>" } })),
    getLayoutReport: vi.fn(async () => ({ report: { components: [] } })),
    traceDesign,
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
  const presenter = new ViewerPresenter(client, checks, canvas, render, {
    sheetNavs: [],
    summary: vi.fn(),
    controls: { setState: vi.fn() },
    findings: { setState: vi.fn(), setFindingLocateNote: vi.fn() },
    expectationCaption: vi.fn(),
    undrawnNote: vi.fn(),
    staleLinkNote: vi.fn(),
    rules: { setState: vi.fn() },
    report: vi.fn(),
    query: stubQueryView(),
    trace: wireTrace ? { setState: onTrace } : undefined,
    location: opts.onLocation,
  });
  return { presenter, onTrace, traceDesign, canvas };
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
      uri: artifactUri("m", "proj/board.edn"),
      from: { refDes: "U1", pin: "3" },
      to: { refDes: "J1", pin: "1" },
      hops: 0,
    });
    const s = lastState(h.onTrace);
    expect(s.outcome).toBe(TraceOutcome.ROUTED);
    expect(s.crossings.map((c) => c.refDes)).toEqual(["R1"]);
  });

  // The route has to reach the canvas, or the panel is a text box that happens to sit beside a
  // drawing. This is the assertion the whole PR exists for.
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

  // A typo in the box is caught here rather than at the server: the panel is showing the box, and a
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
      symbols: false, verdict: "", hash: "", rule: "", trace: "U1.3,J1.1", traceHops: 0,
    });
    expect(h.traceDesign).toHaveBeenCalledWith({
      uri: artifactUri("m", "proj/board.edn"),
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
      symbols: false, verdict: "", hash: "", rule: "", trace: "U1.3,J1.1", traceHops: 2,
    });
    const calls = h.traceDesign.mock.calls;
    expect(calls[calls.length - 1][0].hops).toBe(2);
  });

  it("does not trace when the URL names none", async () => {
    const h = harness();
    await h.presenter.restore({
      mount: "m", path: "proj/board.edn", isDir: false, sheet: "", mode: "", layout: "",
      symbols: false, verdict: "", hash: "", rule: "", trace: "", traceHops: 0,
    });
    expect(h.traceDesign).not.toHaveBeenCalled();
  });
});

// The address bar and the panel must not disagree: a route found by typing pins is addressable
// without the reader doing anything, and a NEGATIVE answer is addressable too, since "these two pins
// do not join" is a thing worth sending someone.
describe("a trace reaching the URL", () => {
  it("reports the question it asked, whatever the outcome", async () => {
    const onLocation = vi.fn();
    const h = harness({ onLocation });
    await h.presenter.openFile("m", "proj/board.edn");
    await h.presenter.runTrace("U1.3", "J1.1");
    const locCalls = onLocation.mock.calls;
    const loc = locCalls[locCalls.length - 1][0];
    expect(loc.trace).toBe("U1.3,J1.1");
    expect(loc.traceHops).toBe(0);
  });
});
