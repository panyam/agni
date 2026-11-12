import { describe, it, expect, vi } from "vitest";
import { artifactUri, uriPath } from "@agni/web-shared/uri.js";
import { DiffPresenter, type DiffRenderView, type DiffSideView, type DiffState } from "./diffpresenter.js";
import { DIFF_COLORS } from "./diff.js";
import { fakeSnapshots, snapName } from "./testsnaps.js";
import { create } from "@bufbuild/protobuf";
import { OverlayConfigSchema, type OverlayConfig } from "./gen/agni/v1/webapi/config_pb.js";
import { PackedSheetSchema, type PackedSheet } from "./gen/agni/v1/geom/geom_packed_pb.js";
import { SheetFormat } from "./gen/agni/v1/webapi/design_pb.js";

// packedSheet is a packed sheet naming the side and sheet it came from, 1000 by 800 world units.
function packedSheet(id: string): PackedSheet {
  const v = new Int32Array([0, 0, 1000, 0, 1000, 800, 0, 800]);
  return create(PackedSheetSchema, { sheetId: id, vertices: new Uint8Array(v.buffer) });
}

// snapPath is the design path a fake snapshot name carries.
function snapPath(snapshot: string): string {
  return uriPath(snapshot.replace(/^snapshots\//, ""));
}

// Fakes mirror the viewer.test.ts harness, with plain stubs behind the typed clients, a
// recording view per side, and the onState pushes captured for assertion.
function harness(over: { diffDesigns?: ReturnType<typeof vi.fn>; overlay?: OverlayConfig; renderer?: "svg" | "webgl" } = {}) {
  const diffDesigns =
    over.diffDesigns ??
    vi.fn(async (_req: object) => ({
      report: {
        componentsAdded: ["R4"],
        componentsRemoved: ["R2"],
        componentsChanged: [{ refDes: "R3", field: "Value", old: "10k", new: "22k" }],
        nets: [{ kind: "renamed", name: "SIG_CLK", oldName: "SIG", added: [], removed: [] }],
      },
      componentStatus: { R2: "removed", R3: "changed", R4: "added" },
      netStatus: { SIG: "renamed", SIG_CLK: "renamed" },
      componentSheetsA: { R2: { ids: ["s1"] }, R3: { ids: ["s1"] } },
      componentSheetsB: { R3: { ids: ["s1"] }, R4: { ids: ["s1"] } },
      netSheetsA: { SIG: { ids: ["s1"] } },
      netSheetsB: { SIG_CLK: { ids: ["s1"] } },
    }));
  // Each side's design carries its path so per-side requests are distinguishable; the layout
  // echoes a per-path value to prove renderSide uses each side's own effective layout.
  const getDesign = vi.fn(async (req: { snapshot: string }) => ({
    layout: `layout-${snapPath(req.snapshot)}`,
    sheets: [
      { id: "s1", name: "Top" },
      { id: "s2", name: snapPath(req.snapshot) === "a.edn" ? "OldOnly" : "NewOnly" },
    ],
  }));
  const getSheet = vi.fn(async (req: { snapshot: string; sheet: string; format?: SheetFormat }) =>
    req.format === SheetFormat.PACKED
      ? { content: { case: "packed" as const, value: packedSheet(`${snapPath(req.snapshot)}/${req.sheet}`) } }
      : { content: { case: "svg" as const, value: `<svg data-doc="${snapPath(req.snapshot)}/${req.sheet}"/>` } },
  );
  const highlightSheet = vi.fn(async (req: { snapshot: string }) => ({
    content: { case: "svg" as const, value: `<svg data-overlay="${snapPath(req.snapshot)}"/>` },
  }));

  const side = (): DiffSideView & {
    svg: ReturnType<typeof vi.fn>;
    overlay: ReturnType<typeof vi.fn>;
    overlays: ReturnType<typeof vi.fn>;
    ph: ReturnType<typeof vi.fn>;
    reveal: ReturnType<typeof vi.fn>;
    packed: ReturnType<typeof vi.fn>;
    lit: ReturnType<typeof vi.fn>;
    ghost: ReturnType<typeof vi.fn>;
  } => {
    const svg = vi.fn();
    const overlay = vi.fn();
    const overlays = vi.fn();
    const ph = vi.fn();
    const reveal = vi.fn();
    const packed = vi.fn();
    const lit = vi.fn();
    const ghost = vi.fn();
    return {
      showSvg: svg, setOverlay: overlay, setOverlays: overlays, showPlaceholder: ph, reveal,
      showPacked: packed, setHighlights: lit, setGhost: ghost,
      svg, overlay, overlays, ph, packed, lit, ghost,
    };
  };
  const a = side();
  const b = side();
  const setBusy = vi.fn();
  const setOverlayMode = vi.fn();
  const view: DiffRenderView = { a, b, setBusy, setOverlayMode };
  const states: DiffState[] = [];
  const presenter = new DiffPresenter(
    { diffDesigns } as never,
    { getDesign, getSheet, highlightSheet } as never,
    view,
    (s) => states.push(s),
    fakeSnapshots(),
    () => over.overlay,
    () => over.renderer ?? "svg",
  );
  const A = { mount: "m", path: "a.edn" };
  const B = { mount: "m", path: "b.edn" };
  return { presenter, diffDesigns, getDesign, getSheet, highlightSheet, a, b, setBusy, setOverlayMode, states, A, B };
}

function last(states: DiffState[]): DiffState {
  return states[states.length - 1];
}

describe("DiffPresenter.open", () => {
  it("diffs once, pairs sheets by name, renders both sides with per-side overlays", async () => {
    const h = harness();
    await h.presenter.open(h.A, h.B);

    expect(h.diffDesigns).toHaveBeenCalledWith({ aSnapshot: snapName(artifactUri("m", "a.edn")), bSnapshot: snapName(artifactUri("m", "b.edn")) });
    // Both sides drew their own sheet document in their own effective layout.
    expect(h.a.svg).toHaveBeenCalledWith('<svg data-doc="a.edn/s1"/>');
    expect(h.b.svg).toHaveBeenCalledWith('<svg data-doc="b.edn/s1"/>');
    const sheetReqs = h.getSheet.mock.calls.map((c) => c[0] as unknown as { snapshot: string; layout: string });
    expect(sheetReqs.find((r) => snapPath(r.snapshot) === "a.edn")?.layout).toBe("layout-a.edn");
    expect(sheetReqs.find((r) => snapPath(r.snapshot) === "b.edn")?.layout).toBe("layout-b.edn");
    // Overlay specs are side-filtered, removed only on A, added only on B, renamed on both.
    const hlReqs = h.highlightSheet.mock.calls.map((c) => c[0] as { snapshot: string; specs: { color?: string; components?: string[]; nets?: string[] }[] });
    const aSpecs = hlReqs.find((r) => snapPath(r.snapshot) === "a.edn")!.specs;
    const bSpecs = hlReqs.find((r) => snapPath(r.snapshot) === "b.edn")!.specs;
    expect(aSpecs.flatMap((s) => s.components ?? [])).toEqual(expect.arrayContaining(["R2", "R3"]));
    expect(aSpecs.flatMap((s) => s.components ?? [])).not.toContain("R4");
    expect(bSpecs.flatMap((s) => s.components ?? [])).toEqual(expect.arrayContaining(["R3", "R4"]));
    expect(bSpecs.flatMap((s) => s.nets ?? [])).toEqual(["SIG", "SIG_CLK"]);
    expect(h.a.overlay).toHaveBeenCalledWith('<svg data-overlay="a.edn"/>');

    const s = last(h.states);
    expect(s.active).toBe(true);
    expect(s.error).toBe("");
    expect(s.aLabel).toBe("m:a.edn");
    expect(s.bLabel).toBe("m:b.edn");
    // "Top" matches; the odd sheets are one-sided pairs.
    expect(s.pairs).toEqual([
      { name: "Top", aId: "s1", bId: "s1" },
      { name: "OldOnly", aId: "s2", bId: "" },
      { name: "NewOnly", aId: "", bId: "s2" },
    ]);
    expect(s.activePair).toBe(0); // the first pair with both sides
    expect(s.legend.map((e) => e.cls)).toEqual(["added", "removed", "changed", "renamed"]);
    expect(s.legend.find((e) => e.cls === "renamed")?.color).toBe(DIFF_COLORS.renamed);
    // Busy wrapped the whole open.
    expect(h.setBusy).toHaveBeenCalledWith(true);
    expect(h.setBusy).toHaveBeenLastCalledWith(false);
  });

  it("surfaces a diff failure as an error state with placeholders, no sheet renders", async () => {
    const h = harness({ diffDesigns: vi.fn(async () => Promise.reject(new Error("no netlist"))) });
    await h.presenter.open(h.A, h.B);
    const s = last(h.states);
    expect(s.active).toBe(true);
    expect(s.error).toContain("no netlist");
    expect(s.pairs).toEqual([]);
    expect(s.legend).toEqual([]);
    expect(h.getSheet).not.toHaveBeenCalled();
    expect(h.a.ph).toHaveBeenCalledWith("comparison failed");
    expect(h.b.ph).toHaveBeenCalledWith("comparison failed");
    expect(h.setBusy).toHaveBeenLastCalledWith(false);
  });

  it("skips the overlay round-trip when nothing changed", async () => {
    const h = harness({
      diffDesigns: vi.fn(async () => ({
        report: { componentsAdded: [], componentsRemoved: [], componentsChanged: [], nets: [] },
        componentStatus: {},
        netStatus: {},
      })),
    });
    await h.presenter.open(h.A, h.B);
    expect(h.highlightSheet).not.toHaveBeenCalled();
    expect(h.a.overlay).toHaveBeenCalledWith("");
    expect(last(h.states).legend).toEqual([]);
  });
});

describe("DiffPresenter under an applied convention", () => {
  it("asks the diff under the page's convention and draws each sheet under none", async () => {
    const overlay = create(OverlayConfigSchema, { config: { conventions: { name: "draft" } } });
    const h = harness({ overlay });
    await h.presenter.open(h.A, h.B);
    expect(h.diffDesigns).toHaveBeenCalledWith({
      aSnapshot: snapName(artifactUri("m", "a.edn"), overlay),
      bSnapshot: snapName(artifactUri("m", "b.edn"), overlay),
    });
    expect(h.getSheet.mock.calls.length).toBeGreaterThan(0);
    expect(h.getSheet.mock.calls.some(([r]) => r.snapshot.includes("@draft"))).toBe(false);
  });
});

describe("DiffPresenter.selectPair", () => {
  it("renders the chosen pair, with a placeholder on the absent side", async () => {
    const h = harness();
    await h.presenter.open(h.A, h.B);
    await h.presenter.selectPair(1); // OldOnly, A has s2 and B has nothing
    expect(h.a.svg).toHaveBeenCalledWith('<svg data-doc="a.edn/s2"/>');
    expect(h.b.ph).toHaveBeenCalledWith('no sheet "OldOnly" in B');
    expect(last(h.states).activePair).toBe(1);
  });

  it("ignores out-of-range and no-op selections", async () => {
    const h = harness();
    await h.presenter.open(h.A, h.B);
    const renders = h.getSheet.mock.calls.length;
    await h.presenter.selectPair(7);
    await h.presenter.selectPair(-1);
    await h.presenter.selectPair(0); // already active
    expect(h.getSheet.mock.calls.length).toBe(renders);
  });
});

describe("DiffPresenter.selectItem (WS9-006)", () => {
  it("emphasizes just the item per side and reveals its location", async () => {
    const h = harness();
    await h.presenter.open(h.A, h.B);
    // open pushed the changed-item list
    expect(last(h.states).items.map((i) => `${i.cls}:${i.key}`)).toEqual([
      "added:R4",
      "removed:R2",
      "changed:R3",
      "renamed:SIG_CLK",
    ]);
    h.highlightSheet.mockClear();
    h.b.overlay.mockClear();

    await h.presenter.selectItem("component:R2");
    expect(last(h.states).selected).toBe("component:R2");
    // A removed component emphasizes only on the old side; the new side's overlay clears
    // without a round-trip.
    const reqs = h.highlightSheet.mock.calls.map((c) => c[0] as unknown as { snapshot: string; specs: { components?: string[] }[] });
    expect(reqs.map((r) => snapPath(r.snapshot))).toEqual(["a.edn"]);
    expect(reqs[0].specs.flatMap((s) => s.components ?? [])).toEqual(["R2"]);
    expect(h.b.overlay).toHaveBeenCalledWith("");
    // The old side reveals (removed lives only there), with no sheet re-render for a same-pair focus.
    expect(h.a.reveal).toHaveBeenCalledTimes(1);
    expect(h.b.reveal).not.toHaveBeenCalled();
  });

  it("toggles off back to the all-changes overlays", async () => {
    const h = harness();
    await h.presenter.open(h.A, h.B);
    await h.presenter.selectItem("component:R2");
    h.highlightSheet.mockClear();
    await h.presenter.selectItem("component:R2");
    expect(last(h.states).selected).toBe("");
    const reqs = h.highlightSheet.mock.calls.map((c) => c[0] as unknown as { snapshot: string; specs: { components?: string[] }[] });
    // Both sides fetch full overlays again (multi-entity specs).
    expect(reqs.map((r) => snapPath(r.snapshot)).sort()).toEqual(["a.edn", "b.edn"]);
    expect(reqs.find((r) => snapPath(r.snapshot) === "a.edn")!.specs.flatMap((s) => s.components ?? [])).toEqual(
      expect.arrayContaining(["R2", "R3"]),
    );
    expect(h.a.reveal).toHaveBeenCalledTimes(1); // only the select revealed, not the deselect
  });

  it("switches to the sheet pair that shows the item when the current one does not", async () => {
    const h = harness({
      diffDesigns: vi.fn(async () => ({
        report: { componentsAdded: ["R9"], componentsRemoved: [], componentsChanged: [], nets: [] },
        componentStatus: { R9: "added" },
        netStatus: {},
        componentSheetsA: {},
        componentSheetsB: { R9: { ids: ["s2"] } },
        netSheetsA: {},
        netSheetsB: {},
      })),
    });
    h.getDesign.mockImplementation(async (req: { snapshot: string }) => ({
      layout: `layout-${snapPath(req.snapshot)}`,
      sheets: [
        { id: "s1", name: "Top" },
        { id: "s2", name: "Power" },
      ],
    }));
    await h.presenter.open(h.A, h.B);
    expect(last(h.states).activePair).toBe(0);
    h.getSheet.mockClear();

    await h.presenter.selectItem("component:R9");
    expect(last(h.states).activePair).toBe(1); // jumped to the Power pair
    const sheets = h.getSheet.mock.calls.map((c) => (c[0] as { sheet: string }).sheet);
    expect(sheets).toEqual(["s2", "s2"]); // both sides re-rendered on the new pair
    expect(h.b.reveal).toHaveBeenCalledTimes(1); // added lives on the new side
  });

  it("switches to the sub-sheet pair for a renamed net that lives only there (WS9-027)", async () => {
    // The server fix uses each side's design as NetSource, and that populates netSheets for a
    // wireless sub-sheet net. The panel then navigates to it the same way a component does.
    // A's side joins by the OLD name, B's by the NEW, and both resolve to the sub pair (s2).
    const h = harness({
      diffDesigns: vi.fn(async () => ({
        report: { componentsAdded: [], componentsRemoved: [], componentsChanged: [], nets: [{ kind: "renamed", name: "/sub/OUT", oldName: "/sub/SIG", added: [], removed: [] }] },
        componentStatus: {},
        netStatus: { "/sub/SIG": "renamed", "/sub/OUT": "renamed" },
        componentSheetsA: {},
        componentSheetsB: {},
        netSheetsA: { "/sub/SIG": { ids: ["s2"] } },
        netSheetsB: { "/sub/OUT": { ids: ["s2"] } },
      })),
    });
    h.getDesign.mockImplementation(async (req: { snapshot: string }) => ({
      layout: `layout-${snapPath(req.snapshot)}`,
      sheets: [
        { id: "s1", name: "Top" },
        { id: "s2", name: "sub" },
      ],
    }));
    await h.presenter.open(h.A, h.B);
    expect(last(h.states).activePair).toBe(0);
    h.getSheet.mockClear();

    await h.presenter.selectItem("net:/sub/OUT");
    expect(last(h.states).activePair).toBe(1); // jumped to the sub pair
    const sheets = h.getSheet.mock.calls.map((c) => (c[0] as { sheet: string }).sheet);
    expect(sheets).toEqual(["s2", "s2"]); // both sides re-rendered on the sub pair
    expect(h.b.reveal).toHaveBeenCalledTimes(1); // a rename reveals on the new side
  });

  // agni issue 1560. A net that crosses sheets shows on the top sheet through its sheet pins, so the
  // top pair "showed" it and a click left the drawings there at fit, with nothing to look at.
  function crossingNet() {
    const h = harness({
      diffDesigns: vi.fn(async () => ({
        report: { componentsAdded: [], componentsRemoved: [], componentsChanged: [], nets: [{ kind: "new", name: "/DCDC/FAN_PWM", oldName: "", added: [], removed: [] }] },
        componentStatus: {},
        netStatus: { "/DCDC/FAN_PWM": "new" },
        componentSheetsA: {},
        componentSheetsB: {},
        netSheetsA: {},
        netSheetsB: { "/DCDC/FAN_PWM": { ids: ["s1", "s2", "s3"] } },
      })),
    });
    h.getDesign.mockImplementation(async (req: { snapshot: string }) => ({
      layout: `layout-${snapPath(req.snapshot)}`,
      sheets: [
        { id: "s1", name: "Top" },
        { id: "s2", name: "DCDC" },
        { id: "s3", name: "Peripherals" },
      ],
    }));
    return h;
  }

  it("takes a net that crosses sheets from the top pair to the first sheet that draws it", async () => {
    const h = crossingNet();
    await h.presenter.open(h.A, h.B);
    expect(last(h.states).activePair).toBe(0);
    await h.presenter.selectItem("net:/DCDC/FAN_PWM");
    expect(last(h.states).activePair).toBe(1);
    expect(h.b.reveal).toHaveBeenCalledTimes(1);
  });

  it("leaves a reader on a sub-sheet that draws the net where they are", async () => {
    const h = crossingNet();
    await h.presenter.open(h.A, h.B);
    await h.presenter.selectPair(2);
    await h.presenter.selectItem("net:/DCDC/FAN_PWM");
    expect(last(h.states).activePair).toBe(2);
  });

  it("stays on the current pair for an item with no sheet entries (no geometry / KiCad nets)", async () => {
    const h = harness({
      diffDesigns: vi.fn(async () => ({
        report: { componentsAdded: [], componentsRemoved: [], componentsChanged: [], nets: [{ kind: "new", name: "X", oldName: "", added: [], removed: [] }] },
        componentStatus: {},
        netStatus: { X: "new" },
        componentSheetsA: {},
        componentSheetsB: {},
        netSheetsA: {},
        netSheetsB: {},
      })),
    });
    await h.presenter.open(h.A, h.B);
    h.getSheet.mockClear();
    await h.presenter.selectItem("net:X");
    expect(last(h.states).activePair).toBe(0);
    expect(h.getSheet).not.toHaveBeenCalled(); // overlays refresh, sheets do not re-render
    expect(h.b.reveal).toHaveBeenCalledTimes(1); // still attempts; an empty overlay makes it a view-side no-op
  });
});

describe("DiffPresenter overlay mode (WS9-007)", () => {
  // framedHarness serves sheet documents with real frames (same size both sides), so the
  // alignment check passes on frame evidence (the default diffDesigns fake carries no
  // shared placements).
  function framedHarness(over: Parameters<typeof harness>[0] = {}) {
    const h = harness(over);
    h.getSheet.mockImplementation(async (req: { snapshot: string; sheet: string }) => ({
      content: { case: "svg" as const, value: `<svg width="1000.0" height="800.0" data-doc="${snapPath(req.snapshot)}/${req.sheet}"/>` },
    }));
    return h;
  }
  type HlReq = { snapshot: string; specs: { components?: string[]; nets?: string[] }[] };

  it("offers overlay for aligned pairs and composes the union from retained docs", async () => {
    const h = framedHarness();
    await h.presenter.open(h.A, h.B);
    expect(last(h.states).overlayOk).toBe(true);
    expect(last(h.states).mode).toBe("side");
    const sheetFetches = h.getSheet.mock.calls.length;
    h.highlightSheet.mockClear();

    await h.presenter.setMode("overlay");
    expect(h.setOverlayMode).toHaveBeenCalledWith(true);
    expect(last(h.states).mode).toBe("overlay");
    // The union base is B's RETAINED document on the a-canvas, with no sheet re-fetch.
    expect(h.a.svg).toHaveBeenLastCalledWith(expect.stringContaining('data-doc="b.edn/s1"'));
    expect(h.getSheet.mock.calls.length).toBe(sheetFetches);
    // Two layers, b's own change classes on b's sheet and removed-only ghosts on a's sheet.
    const reqs = h.highlightSheet.mock.calls.map((c) => c[0] as unknown as HlReq);
    const ghost = reqs.find((r) => snapPath(r.snapshot) === "a.edn")!;
    expect(ghost.specs.flatMap((s) => s.components ?? [])).toEqual(["R2"]);
    expect(ghost.specs.flatMap((s) => s.nets ?? [])).toEqual([]);
    const bLayer = reqs.find((r) => snapPath(r.snapshot) === "b.edn")!;
    expect(bLayer.specs.flatMap((s) => s.components ?? [])).toEqual(expect.arrayContaining(["R3", "R4"]));
    expect(h.a.overlays).toHaveBeenCalledTimes(1);
  });

  it("returns to side-by-side from retained docs, refetching only overlays", async () => {
    const h = framedHarness();
    await h.presenter.open(h.A, h.B);
    await h.presenter.setMode("overlay");
    const sheetFetches = h.getSheet.mock.calls.length;
    await h.presenter.setMode("side");
    expect(h.setOverlayMode).toHaveBeenLastCalledWith(false);
    expect(h.getSheet.mock.calls.length).toBe(sheetFetches);
    expect(h.a.svg).toHaveBeenLastCalledWith(expect.stringContaining('data-doc="a.edn/s1"'));
    expect(h.b.svg).toHaveBeenLastCalledWith(expect.stringContaining('data-doc="b.edn/s1"'));
  });

  it("refuses overlay when frames are unknown and ignores the mode switch", async () => {
    const h = harness(); // the default fake's markup carries no width/height
    await h.presenter.open(h.A, h.B);
    expect(last(h.states).overlayOk).toBe(false);
    expect(last(h.states).overlayReason).toContain("frames");
    await h.presenter.setMode("overlay");
    expect(last(h.states).mode).toBe("side");
    expect(h.setOverlayMode).not.toHaveBeenCalledWith(true);
  });

  it("refuses overlay when a shared component moved between revisions", async () => {
    const h = framedHarness();
    h.diffDesigns.mockImplementation(async () => ({
      report: { componentsAdded: [], componentsRemoved: [], componentsChanged: [], nets: [] },
      componentStatus: {},
      netStatus: {},
      componentSheetsA: {},
      componentSheetsB: {},
      netSheetsA: {},
      netSheetsB: {},
      sharedPlacementsA: { R1: { sheet: "s1", x: 0, y: 0 }, C1: { sheet: "s1", x: 900, y: 700 } },
      sharedPlacementsB: { R1: { sheet: "s1", x: 0, y: 0 }, C1: { sheet: "s1", x: 500, y: 700 } },
    }));
    await h.presenter.open(h.A, h.B);
    expect(last(h.states).overlayOk).toBe(false);
    expect(last(h.states).overlayReason).toContain("C1");
  });

  it("narrows the union layers to a focused item and reveals on the union canvas", async () => {
    const h = framedHarness();
    await h.presenter.open(h.A, h.B);
    await h.presenter.setMode("overlay");
    h.highlightSheet.mockClear();
    await h.presenter.selectItem("component:R2"); // removed, so ghost layer only
    const reqs = h.highlightSheet.mock.calls.map((c) => snapPath((c[0] as unknown as HlReq).snapshot));
    expect(reqs).toEqual(["a.edn"]);
    expect(h.a.reveal).toHaveBeenCalledTimes(1);
    expect(h.b.reveal).not.toHaveBeenCalled();
  });

  it("falls back to side-by-side when switching to a pair that cannot align", async () => {
    const h = framedHarness();
    await h.presenter.open(h.A, h.B);
    await h.presenter.setMode("overlay");
    await h.presenter.selectPair(1); // one-sided pair, never aligned
    expect(last(h.states).mode).toBe("side");
    expect(last(h.states).overlayOk).toBe(false);
    expect(h.setOverlayMode).toHaveBeenLastCalledWith(false);
  });
});

describe("DiffPresenter.close", () => {
  it("resets to the inactive state", async () => {
    const h = harness();
    await h.presenter.open(h.A, h.B);
    h.presenter.close();
    const s = last(h.states);
    expect(s.active).toBe(false);
    expect(s.pairs).toEqual([]);
    expect(s.aLabel).toBe(":");
  });
});

describe("DiffPresenter render errors", () => {
  it("turns a failed sheet render into that side's placeholder while the other side draws", async () => {
    const h = harness();
    h.getSheet.mockImplementation(async (req: { snapshot: string; sheet: string }) => {
      if (snapPath(req.snapshot) === "a.edn") throw new Error("boom");
      return { content: { case: "svg" as const, value: `<svg data-doc="${snapPath(req.snapshot)}/${req.sheet}"/>` } };
    });
    await h.presenter.open(h.A, h.B);
    expect(h.a.ph).toHaveBeenCalledWith(expect.stringContaining("boom"));
    expect(h.b.svg).toHaveBeenCalledWith('<svg data-doc="b.edn/s1"/>');
    // The failed side fetches no overlay; the healthy side still does.
    const hlPaths = h.highlightSheet.mock.calls.map((c) => snapPath((c[0] as { snapshot: string }).snapshot));
    expect(hlPaths).toEqual(["b.edn"]);
  });

  it("leaves the sheet unhighlighted when only the overlay fails", async () => {
    const h = harness();
    h.highlightSheet.mockImplementation(async () => Promise.reject(new Error("nope")));
    await h.presenter.open(h.A, h.B);
    expect(h.a.svg).toHaveBeenCalled();
    expect(h.a.overlay).toHaveBeenCalledWith("");
    expect(h.b.overlay).toHaveBeenCalledWith("");
  });
});

// A comparison opened while the canvas draws in WebGL draws in WebGL (agni issue 1335). Each side
// draws its packed sheet and lights its changes over the keys it holds, so no highlight is fetched,
// and the union draws a's removed parts as a ghost over b.
describe("DiffPresenter in WebGL", () => {
  const sheetOf = (m: ReturnType<typeof vi.fn>) => (m.mock.lastCall?.[0] as PackedSheet | undefined)?.sheetId;
  type Specs = { components?: string[]; nets?: string[] }[];

  it("draws each side's packed sheet and lights its changes without a highlight fetch", async () => {
    const h = harness({ renderer: "webgl" });
    await h.presenter.open(h.A, h.B);
    expect(h.getSheet.mock.calls.every((c) => (c[0] as { format: SheetFormat }).format === SheetFormat.PACKED)).toBe(true);
    expect(sheetOf(h.a.packed)).toBe("a.edn/s1");
    expect(sheetOf(h.b.packed)).toBe("b.edn/s1");
    expect(h.a.svg).not.toHaveBeenCalled();
    expect(h.highlightSheet).not.toHaveBeenCalled();
    const aSpecs = h.a.lit.mock.lastCall![0] as Specs;
    const bSpecs = h.b.lit.mock.lastCall![0] as Specs;
    expect(aSpecs.flatMap((s) => s.components ?? [])).toEqual(expect.arrayContaining(["R2", "R3"]));
    expect(bSpecs.flatMap((s) => s.components ?? [])).toEqual(expect.arrayContaining(["R3", "R4"]));
    expect(last(h.states).overlayOk).toBe(true);
  });

  it("draws the union as b's sheet, b's changes, and a's removed parts as a ghost", async () => {
    const h = harness({ renderer: "webgl" });
    await h.presenter.open(h.A, h.B);
    await h.presenter.setMode("overlay");
    expect(sheetOf(h.a.packed)).toBe("b.edn/s1");
    expect((h.a.lit.mock.lastCall![0] as Specs).flatMap((s) => s.components ?? [])).toEqual(expect.arrayContaining(["R3", "R4"]));
    const [ghostSheet, ghostSpecs] = h.a.ghost.mock.lastCall! as [PackedSheet, Specs];
    expect(ghostSheet.sheetId).toBe("a.edn/s1");
    expect(ghostSpecs.flatMap((s) => s.components ?? [])).toEqual(["R2"]);
    expect(h.highlightSheet).not.toHaveBeenCalled();

    await h.presenter.setMode("side");
    expect(h.a.ghost).toHaveBeenLastCalledWith(null, []);
    expect(sheetOf(h.a.packed)).toBe("a.edn/s1");
  });
});
