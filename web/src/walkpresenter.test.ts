import { describe, it, expect, vi, type Mock } from "vitest";
import { create } from "@bufbuild/protobuf";
import { artifactUri } from "@agni/web-shared/uri.js";
import { type ViewerLocation } from "./router.js";
import { ViewerPresenter, type RenderView } from "./viewer.js";
import { SheetFormat, TraceOutcome, WalkGoalAnswerSchema, WalkSchema } from "./gen/agni/v1/webapi/design_pb.js";
import { stubQueryView } from "./testviews.js";
import type { WalkState } from "./walk.js";
import { HighlightShape } from "./highlights.js";
import { fakeSnapshots, snapName } from "./testsnaps.js";

// walkAnswer is R1's walk: its two nets at hop 1, a capacitor crossed into GND at hop 2, and U3 left
// at the boundary.
function walkAnswer() {
  return create(WalkSchema, {
    from: { endpoint: { refDes: "R1", pin: "" } },
    kind: "small-parts",
    radius: 2,
    nets: [
      { hop: 1, net: { name: "SDA", sheetIds: ["s2"] }, entered: { refDes: "R1", exitPin: "1", toNet: "SDA" } },
      { hop: 1, net: { name: "+3V3", sheetIds: ["s2"] }, entered: { refDes: "R1", exitPin: "2", toNet: "+3V3" }, ended: "rail" },
      { hop: 2, net: { name: "GND", sheetIds: ["s2"] }, entered: { refDes: "C1", enterPin: "1", exitPin: "2", fromNet: "SDA", toNet: "GND" }, ended: "ground", alsoEntered: [{ refDes: "C2" }] },
    ],
    boundary: [{ refDes: "U3", class: "ic", hop: 1, net: "SDA", reason: "8 connected pins" }],
  });
}

function harness(opts: { onLocation?: Mock<(loc: ViewerLocation) => void> } = {}) {
  const walkDesign = vi.fn(async (_req: unknown) => ({ walk: walkAnswer() }));
  const client = {
    getDesign: vi.fn(async () => ({
      name: "D", layout: "faithful", sourceFormat: "", componentCount: 0, netCount: 0,
      sheets: [{ id: "s1", name: "S1" }, { id: "s2", name: "S2" }, { id: "s3", name: "S3" }], nativeAvailable: false, availableLayouts: ["faithful"],
    })),
    getSheet: vi.fn(async (req: { format?: SheetFormat }) =>
      req.format === SheetFormat.SVG ? { content: { case: "svg", value: "<svg/>" } } : { content: { case: "packed", value: { sheetId: "s1" } } },
    ),
    highlightSheet: vi.fn(async () => ({ content: { case: "svg", value: "<svg/>" } })),
    getLayoutReport: vi.fn(async () => ({ report: { components: [] } })),
    walkDesign,
    locateSubjects: vi.fn(async (_req: unknown) => ({ subjects: [{ sheetIds: ["s2"] }] })),
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
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  const canvas = { setSheet: vi.fn(), setHighlights: vi.fn(), clear: vi.fn(), resize: vi.fn(), onPick: vi.fn(), setBoardLayers: vi.fn() } as any;
  const render: RenderView = {
    showSvg: vi.fn(), setSvgOverlay: vi.fn(), setBusy: vi.fn(),
    getView: vi.fn((mode) => `view-${mode}`), setView: vi.fn(), setBoardLayers: vi.fn(),
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
  } as any;
  const order: string[] = [];
  const onWalk = vi.fn();
  const walkLit = vi.fn();
  const reveal = vi.fn(() => order.push("reveal"));
  canvas.setHighlights.mockImplementation((specs: { components?: string[] }[]) => order.push(specs.length === 1 ? "start" : "walk"));
  const presenter = new ViewerPresenter(client, checks, canvas, render, {
    sheetNavs: [], summary: vi.fn(), controls: { setState: vi.fn() },
    findings: { setState: vi.fn(), setFindingLocateNote: vi.fn() },
    expectationCaption: vi.fn(), undrawnNote: vi.fn(), staleLinkNote: vi.fn(), hierarchyNote: vi.fn(), drawingNote: vi.fn(),
    rules: { setState: vi.fn() }, report: vi.fn(), query: stubQueryView(),
    walk: { setWalk: onWalk },
    underlay: walkLit,
    reveal,
    showPanel: vi.fn(),
    location: opts.onLocation,
  }, undefined, undefined, undefined, undefined, fakeSnapshots());
  return { presenter, walkDesign, canvas, onWalk, reveal, order, walkLit, getSheet: client.getSheet, locateSubjects: client.locateSubjects };
}

const lastWalk = (onWalk: Mock): WalkState => onWalk.mock.calls[onWalk.mock.calls.length - 1][0] as WalkState;

describe("walk presenter", () => {
  it("asks WalkDesign about the open design and pushes the walk by hop", async () => {
    const h = harness();
    await h.presenter.openFile("m", "proj/board.edn");
    await h.presenter.runWalk({ start: "R1", kind: "small-parts", hops: 2, through: ["U9"] });
    expect(h.walkDesign).toHaveBeenCalledWith({
      snapshot: snapName(artifactUri("m", "proj/board.edn")),
      from: { refDes: "R1", pin: "" }, kind: "small-parts", hops: 2, through: ["U9"],
    });
    const s = lastWalk(h.onWalk);
    expect(s.hops.map((x) => [x.hop, x.nets.map((n) => n.name)])).toEqual([[1, ["SDA", "+3V3"]], [2, ["GND"]]]);
    expect(s.boundary.map((b) => b.refDes)).toEqual(["U3"]);
  });

  // The ticket asks the view to zoom to the start, so the start is lit alone and revealed before the
  // walk is drawn around it.
  it("lights the start, zooms to it, then lights the walk", async () => {
    const h = harness();
    await h.presenter.openFile("m", "proj/board.edn");
    h.order.length = 0;
    await h.presenter.runWalk({ start: "R1", kind: "small-parts", hops: 2, through: [] });
    expect(h.order).toEqual(["start", "reveal", "walk"]);
    const calls = h.canvas.setHighlights.mock.calls;
    const start = calls[calls.length - 2][0];
    expect(start).toEqual([expect.objectContaining({ pins: [{ refDes: "R1", pin: "1" }, { refDes: "R1", pin: "2" }] })]);
  });

  it("opens the sheet the walk starts on", async () => {
    const h = harness();
    await h.presenter.openFile("m", "proj/board.edn");
    h.getSheet.mockClear();
    await h.presenter.runWalk({ start: "R1", kind: "small-parts", hops: 2, through: [] });
    expect(h.getSheet.mock.calls.some((c: unknown[]) => (c[0] as { sheet: string }).sheet === "s2")).toBe(true);
  });

  it("keeps the walk in the address bar and walks a link's walk on arrival", async () => {
    const onLocation = vi.fn();
    const h = harness({ onLocation });
    await h.presenter.openFile("m", "proj/board.edn");
    await h.presenter.runWalk({ start: "R1", kind: "small-parts", hops: 2, through: ["U3"] });
    const loc = onLocation.mock.calls[onLocation.mock.calls.length - 1][0] as ViewerLocation;
    expect(loc.walk).toEqual({ start: "R1", kind: "small-parts", hops: 2, through: ["U3"] });

    const cold = harness();
    await cold.presenter.restore({ ...loc, walk: { start: "R1", kind: "series", hops: 3, through: [] } });
    expect(cold.walkDesign).toHaveBeenCalledWith(expect.objectContaining({ from: { refDes: "R1", pin: "" }, kind: "series", hops: 3 }));
  });

  it("locating a row keeps the walk lit", async () => {
    const h = harness();
    await h.presenter.openFile("m", "proj/board.edn");
    await h.presenter.runWalk({ start: "R1", kind: "small-parts", hops: 2, through: [] });
    await h.presenter.locateInWalk("component", "C1", "s2");
    const hl = h.canvas.setHighlights.mock.calls;
    const specs = hl[hl.length - 1][0] as { nets?: string[]; components?: string[]; shape?: HighlightShape }[];
    expect(specs.flatMap((s) => s.nets ?? [])).toContain("SDA");
    expect(specs.flatMap((s) => s.components ?? [])).toEqual(expect.arrayContaining(["C1"]));
    expect(specs.flatMap((s) => s.components ?? [])).not.toContain("R1");
  });

  // The Trace tab lists a part under a net, and the net's first sheet need not draw it, as J16 sits
  // under Jetson's SDA net, which SoM_IO draws first (agni issue 1580).
  it("opens a sheet that draws a located part, not the sheet of the net it sits under", async () => {
    const h = harness();
    await h.presenter.openFile("m", "proj/board.edn");
    await h.presenter.runWalk({ start: "R1", kind: "small-parts", hops: 2, through: [] });
    h.locateSubjects.mockResolvedValueOnce({ subjects: [{ sheetIds: ["s3"] }] });
    h.getSheet.mockClear();
    await h.presenter.locateInWalk("component", "J16", "s2");
    expect(h.locateSubjects).toHaveBeenLastCalledWith(expect.objectContaining({ subjects: [{ kind: "component", ref: "J16", pin: "" }] }));
    const opened = h.getSheet.mock.calls.map((c: unknown[]) => (c[0] as { sheet: string }).sheet);
    expect(opened).toContain("s3");
    expect(opened).not.toContain("s2");
  });

  it("keeps the shown sheet when it draws the located part", async () => {
    const h = harness();
    await h.presenter.openFile("m", "proj/board.edn");
    await h.presenter.runWalk({ start: "R1", kind: "small-parts", hops: 2, through: [] });
    h.locateSubjects.mockResolvedValueOnce({ subjects: [{ sheetIds: ["s1", "s2"] }] });
    h.getSheet.mockClear();
    await h.presenter.locateInWalk("component", "C1", "s1");
    expect(h.getSheet).not.toHaveBeenCalled();
  });

  // The walk's shades reach the sheet views beside the canvas too (agni issue 1480).
  it("hands the walk's shades to the views beside the canvas, and takes them back when it clears", async () => {
    const h = harness();
    await h.presenter.openFile("m", "proj/board.edn");
    await h.presenter.runWalk({ start: "R1", kind: "small-parts", hops: 2, through: [] });
    const lit = h.walkLit.mock.calls[h.walkLit.mock.calls.length - 1][0] as { nets?: string[] }[];
    expect(lit.flatMap((s) => s.nets ?? [])).toContain("SDA");
    // GND is where the walk stopped, which no view lights (agni issue 1562).
    expect(lit.flatMap((s) => s.nets ?? [])).not.toContain("GND");
    await h.presenter.locateInWalk("component", "C1", "s2");
    expect(h.walkLit).toHaveBeenCalledTimes(1);
    await h.presenter.clearWalk();
    expect(h.walkLit).toHaveBeenLastCalledWith([]);
  });

  it("sends a goal question on the walk's own request", async () => {
    const onLocation = vi.fn();
    const h = harness({ onLocation });
    await h.presenter.openFile("m", "proj/board.edn");
    const goal = { kind: "pin", at: "SDA", to: "J4.2" };
    await h.presenter.runWalk({ start: "R1", kind: "small-parts", hops: 2, through: [], goal });
    expect(h.walkDesign).toHaveBeenLastCalledWith(expect.objectContaining({ goal: { kind: "pin", at: "SDA", to: { refDes: "J4", pin: "2" } } }));
    const loc = onLocation.mock.calls[onLocation.mock.calls.length - 1][0] as ViewerLocation;
    expect(loc.walk?.goal).toEqual(goal);
  });

  // agni issue 1423: a routed goal opens the sheet its route ends on and zooms to the start and the
  // parts the route crosses, before the walk is lit around them.
  it("moves the view to a routed goal's route", async () => {
    const h = harness();
    const routed = walkAnswer();
    routed.goal = create(WalkGoalAnswerSchema, {
      kind: "pull-up",
      at: "SDA",
      outcome: TraceOutcome.ROUTED,
      routes: [{ nets: [{ name: "SDA", sheetIds: ["s2"] }, { name: "+3V3", sheetIds: ["s3"] }], crossings: [{ refDes: "R9", fromNet: "SDA", toNet: "+3V3" }], endsAt: "+3V3" }],
    });
    h.walkDesign.mockResolvedValue({ walk: routed });
    await h.presenter.openFile("m", "proj/board.edn");
    h.getSheet.mockClear();
    h.order.length = 0;
    await h.presenter.runWalk({ start: "R1", kind: "small-parts", hops: 2, through: [], goal: { kind: "pull-up", at: "SDA", to: "" } });
    expect(h.getSheet).toHaveBeenCalledWith(expect.objectContaining({ sheet: "s3" }));
    expect(h.order).toEqual(["walk", "reveal", "walk"]);
    const calls = h.canvas.setHighlights.mock.calls;
    const zoomed = calls[calls.length - 2][0] as { components?: string[] }[];
    expect(zoomed.flatMap((x) => x.components ?? [])).toEqual(["R9"]);
  });

  it("states a start that names nothing and leaves the canvas alone", async () => {
    const h = harness();
    h.walkDesign.mockResolvedValueOnce({ walk: create(WalkSchema, { reason: "no component R77 in this design" }) });
    await h.presenter.openFile("m", "proj/board.edn");
    h.canvas.setHighlights.mockClear();
    await h.presenter.runWalk({ start: "R77", kind: "series", hops: 1, through: [] });
    expect(lastWalk(h.onWalk).reason).toBe("no component R77 in this design");
    expect(h.canvas.setHighlights).not.toHaveBeenCalled();
  });
});
