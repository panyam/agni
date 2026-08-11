import { describe, it, expect, vi } from "vitest";
import { artifactUri } from "./uri.js";
import { ViewerPresenter, type RenderView } from "./viewer.js";
import { HighlightShape } from "./highlights.js";
import { SheetFormat, SymbolSource } from "./gen/agni/v1/webapi/design_pb.js";
import { LocateReason } from "./gen/agni/v1/checks/checks_pb.js";

// Fakes for the presenter's collaborators. The presenter only calls typed methods, so plain
// stubs suffice — no DOM, no transport. getSheet returns the oneof-shaped response the real
// client would, keyed on the requested format.
function harness() {
  // getDesign echoes the requested layout (empty -> faithful) so the presenter's adopted
  // layout reflects the request, as the real server's effective-layout does.
  const getDesign = vi.fn(async (req: { layout?: string }) => ({
    name: "D",
    layout: req.layout || "faithful",
    sourceFormat: "",
    componentCount: 0,
    netCount: 0,
    sheets: [{ id: "s1", name: "S1" }],
    nativeAvailable: true,
    availableLayouts: ["faithful", "grid", "layered"],
  }));
  const getSheet = vi.fn(async (req: { mount?: string; path?: string; sheet?: string; format?: SheetFormat; layout?: string; symbols?: SymbolSource }) =>
    req.format === SheetFormat.SVG || req.format === SheetFormat.NATIVE
      ? { content: { case: "svg", value: "<svg data-mode='svg'/>" } }
      : { content: { case: "packed", value: { sheetId: "s1" } } },
  );
  const checkDesign = vi.fn(async (_req: { uri: string; rules?: string[] }) => ({ findings: [] as { rule: string; severity: string; subject: { kind: string; ref: string; pin: string }; message: string; sheets?: string[] }[] }));
  // listRules returns a small two-rule catalog by default (one connectivity, one naming), both
  // available, so opening a file default-selects both. Tests that need a specific catalog override.
  const listRules = vi.fn(async (_req: { uri: string }) => ({
    rules: [
      { name: "single-pin-net", severity: "info", summary: "stub net", reads: ["net.pin_count"], tags: { category: "connectivity" }, available: true, unavailableReason: "" },
      { name: "diff-pair-naming", severity: "warning", summary: "diff pair", reads: ["net.names"], tags: { category: "naming" }, available: true, unavailableReason: "" },
    ] as { name: string; severity: string; summary: string; reads: string[]; tags: Record<string, string>; available: boolean; unavailableReason: string }[],
  }));
  const getLayoutReport = vi.fn(async (req: { symbols?: SymbolSource }) => ({
    report: { components: [{ refDes: "R1", deviceClass: "resistor", kind: "glyph" }] },
  }));
  // highlightSheet returns a canned SVG overlay; the presenter only fetches it in SVG mode.
  const highlightSheet = vi.fn(async (_req: object) => ({ content: { case: "svg", value: "<svg data-overlay/>" } }));
  // getExpectations returns no sidecar by default, so opening a file leaves the panel empty; tests
  // that exercise the expectations panel override it.
  const getExpectations = vi.fn(async (_req: { uri: string }) => ({
    expectations: [] as { rule: string; subjects: string[]; pending: boolean }[],
    hasSidecar: false,
  }));
  // Two mock clients mirror the WS9-026 service split: rendering RPCs on the design client,
  // rules/findings/expectations on the checks client.
  const client = { getDesign, getSheet, getLayoutReport, highlightSheet } as any;
  const checks = { checkDesign, listRules, getExpectations } as any;
  const canvas = { showSheet: vi.fn(), setHighlights: vi.fn() } as any;
  const query = { setState: vi.fn(), setRelations: vi.fn(), setExamples: vi.fn(), setLocateNote: vi.fn() };
  // Two nav surfaces (tabs + tree), as in the app, to prove the presenter fans state to both.
  const navA = { setState: vi.fn() };
  const navB = { setState: vi.fn() };
  // getView returns a distinct value per mode so save/restore can be asserted by identity.
  const render: RenderView = {
    showWebgl: vi.fn(),
    showSvg: vi.fn(),
    setSvgOverlay: vi.fn(),
    setBusy: vi.fn(),
    getView: vi.fn((mode) => `view-${mode}`),
    setView: vi.fn(),
    setBoardLayers: vi.fn(),
  };
  const onControls = vi.fn();
  const onSummary = vi.fn();
  const onFindings = vi.fn();
  const onExpectCaption = vi.fn();
  const onRules = vi.fn();
  const onReport = vi.fn();
  const onLocation = vi.fn();
  const onOverview = vi.fn();
  const presenter = new ViewerPresenter(client, checks, canvas, render, {
    sheetNavs: [navA, navB],
    summary: onSummary,
    controls: { setState: onControls },
    findings: { setState: onFindings, setFindingLocateNote: () => {} },
    expectationCaption: onExpectCaption,
    rules: { setState: onRules },
    report: onReport,
    location: onLocation,
    overview: { setState: onOverview },
    query,
  });
  return { presenter, getDesign, getSheet, checkDesign, listRules, getLayoutReport, highlightSheet, getExpectations, canvas, query, navA, navB, render, onControls, onSummary, onFindings, onExpectCaption, onRules, onReport, onLocation, onOverview };
}

// openAndCheck opens a file and then runs the on-demand checks — the two-step the app does when the
// user opens a design and presses Run. Checks no longer fire on open (WS9), so a test asserting on
// findings/highlights/expectations must run them explicitly.
async function openAndCheck(h: ReturnType<typeof harness>, mount: string, path: string, wantSheet = "") {
  await h.presenter.openFile(mount, path, wantSheet);
  await h.presenter.runChecks();
}

// lastReport returns the argument of the most recent onReport push.
function lastReport(h: ReturnType<typeof harness>) {
  const calls = h.onReport.mock.calls;
  return calls[calls.length - 1]?.[0];
}

// lastControls returns the ControlsState from the most recent onControls push.
function lastControls(h: ReturnType<typeof harness>) {
  const calls = h.onControls.mock.calls;
  return calls[calls.length - 1][0];
}

// lastLocation returns the ViewerLocation from the most recent onLocation push.
function lastLocation(h: ReturnType<typeof harness>) {
  const calls = h.onLocation.mock.calls;
  return calls[calls.length - 1][0];
}

// lastFindings returns the FindingsState from the most recent onFindings push.
function lastFindings(h: ReturnType<typeof harness>) {
  const calls = h.onFindings.mock.calls;
  return calls[calls.length - 1][0];
}

// lastRules returns the RulesState from the most recent onRules push.
function lastRules(h: ReturnType<typeof harness>) {
  const calls = h.onRules.mock.calls;
  return calls[calls.length - 1][0];
}

describe("ViewerPresenter", () => {
  it("opens a file in the default SVG mode: requests SVG and shows the svg", async () => {
    const h = harness();
    await h.presenter.openFile("m", "board.eds");
    expect(h.getDesign).toHaveBeenCalledOnce();
    expect(h.getSheet).toHaveBeenCalledOnce();
    expect(h.getSheet.mock.calls[0][0].format).toBe(SheetFormat.SVG);
    expect(h.render.showSvg).toHaveBeenCalledWith("<svg data-mode='svg'/>");
    // Sheet state fans out to BOTH nav surfaces (tabs + tree), in sync, with the active sheet.
    for (const nav of [h.navA, h.navB]) {
      const last = nav.setState.mock.calls[nav.setState.mock.calls.length - 1][0];
      expect(last).toMatchObject({ mount: "m", path: "board.eds", activeId: "s1" });
      expect(last.sheets).toHaveLength(1);
    }
  });

  it("switching to WebGL re-renders via GetSheet(format=PACKED) and reveals the canvas", async () => {
    const h = harness();
    await h.presenter.openFile("m", "board.eds");
    await h.presenter.setMode("webgl");
    const c = h.getSheet.mock.calls;
    expect(c[c.length - 1][0].format).toBe(SheetFormat.PACKED);
    expect(h.canvas.showSheet).toHaveBeenCalled();
    expect(h.render.showWebgl).toHaveBeenCalled();
  });

  it("switching to Native mode requests format=NATIVE", async () => {
    const h = harness();
    await h.presenter.openFile("m", "board.eds");
    await h.presenter.setMode("native");
    const c = h.getSheet.mock.calls;
    expect(c[c.length - 1][0].format).toBe(SheetFormat.NATIVE);
  });

  it("setMode to the current mode is a no-op", async () => {
    const h = harness();
    await h.presenter.openFile("m", "board.eds");
    h.getSheet.mockClear();
    await h.presenter.setMode("svg");
    expect(h.getSheet).not.toHaveBeenCalled();
  });

  it("defaults to FAITHFUL symbols when the design provides its own", async () => {
    const h = harness();
    await h.presenter.openFile("m", "board.eds");
    expect(h.getSheet.mock.calls[0][0].symbols).toBe(SymbolSource.FAITHFUL);
    expect(lastControls(h).providedSymbols).toBe(true); // availableLayouts has "faithful"
    expect(lastControls(h).faithfulSymbols).toBe(true);
  });

  it("keeps GLYPH symbols for a netlist-only design (no faithful layout)", async () => {
    const h = harness();
    h.getDesign.mockResolvedValueOnce({
      name: "N", layout: "grid", sourceFormat: "", componentCount: 0, netCount: 0,
      sheets: [{ id: "s1", name: "S1" }], nativeAvailable: false, availableLayouts: ["grid", "layered"],
    });
    await h.presenter.openFile("m", "netlist.edn");
    expect(h.getSheet.mock.calls[0][0].symbols).toBe(SymbolSource.GLYPH);
    expect(lastControls(h).providedSymbols).toBe(false);
    expect(lastControls(h).faithfulSymbols).toBe(false);
  });

  it("toggling the symbol source off re-requests the sheet with SymbolSource.GLYPH", async () => {
    const h = harness();
    await h.presenter.openFile("m", "board.eds"); // defaults to faithful
    await h.presenter.setLayout("grid");
    h.getSheet.mockClear();
    await h.presenter.setSymbols(false);
    const c = h.getSheet.mock.calls;
    expect(c[c.length - 1][0].symbols).toBe(SymbolSource.GLYPH);
    expect(lastControls(h).faithfulSymbols).toBe(false);
  });

  it("setSymbols to the current value is a no-op", async () => {
    const h = harness();
    await h.presenter.openFile("m", "board.eds"); // default faithful
    h.getSheet.mockClear();
    await h.presenter.setSymbols(true); // already faithful
    expect(h.getSheet).not.toHaveBeenCalled();
  });

  it("fetches the conversion report for an auto-layout and clears it for the faithful layout", async () => {
    const h = harness();
    await h.presenter.openFile("m", "board.eds"); // opens in faithful -> no report
    expect(lastReport(h)).toBeNull();
    expect(h.getLayoutReport).not.toHaveBeenCalled();

    await h.presenter.setLayout("grid"); // auto-layout -> report fetched
    expect(h.getLayoutReport).toHaveBeenCalled();
    expect(lastReport(h)?.components?.[0]?.refDes).toBe("R1");
  });

  it("re-fetches the report with the symbol source when the toggle flips", async () => {
    const h = harness();
    await h.presenter.openFile("m", "board.eds"); // defaults to faithful
    await h.presenter.setLayout("grid");
    h.getLayoutReport.mockClear();
    await h.presenter.setSymbols(false);
    const c = h.getLayoutReport.mock.calls;
    expect(c[c.length - 1][0].symbols).toBe(SymbolSource.GLYPH);
  });

  it("falls back to WebGL when opening a file whose format has no native render", async () => {
    const h = harness();
    await h.presenter.openFile("m", "a.kicad_sch");
    await h.presenter.setMode("native");
    h.getSheet.mockClear();
    h.getDesign.mockResolvedValueOnce({
      name: "B",
      layout: "grid",
      sourceFormat: "",
      componentCount: 0,
      netCount: 0,
      sheets: [{ id: "s1", name: "S1" }],
      nativeAvailable: false,
    } as any);
    await h.presenter.openFile("m", "b.edn");
    const c = h.getSheet.mock.calls;
    expect(c[c.length - 1][0].format).toBe(SheetFormat.SVG); // fell back to SVG, not NATIVE
    expect(lastControls(h).nativeAvailable).toBe(false);
  });

  it("remembers each renderer's view and restores it on return", async () => {
    const h = harness();
    await h.presenter.openFile("m", "board.eds"); // shown: svg|m|board.eds|s1
    await h.presenter.setMode("webgl"); // saves the svg view; webgl has no memory yet -> no restore
    (h.render.setView as any).mockClear();
    await h.presenter.setMode("svg"); // saves the webgl view; svg has a remembered view -> restore it
    expect(h.render.setView).toHaveBeenCalledWith("svg", "view-svg");
  });

  it("does not restore a view on first visit (lets the fresh fit stand)", async () => {
    const h = harness();
    await h.presenter.openFile("m", "board.eds");
    expect(h.render.setView).not.toHaveBeenCalled();
  });

  it("falls back to SVG when the native render fails", async () => {
    const h = harness();
    // NATIVE rejects (external tool failure); SVG succeeds.
    h.getSheet.mockImplementation(async (req: { format?: SheetFormat }) => {
      if (req.format === SheetFormat.NATIVE) throw new Error("kicad-cli failed");
      if (req.format === SheetFormat.SVG) return { content: { case: "svg", value: "<svg fallback/>" } };
      return { content: { case: "packed", value: { sheetId: "s1" } } };
    });
    await h.presenter.openFile("m", "board.kicad_sch");
    await h.presenter.setMode("native");
    expect(h.render.showSvg).toHaveBeenCalledWith("<svg fallback/>");
    const formats = h.getSheet.mock.calls.map((c: [{ format?: SheetFormat }]) => c[0].format);
    expect(formats).toContain(SheetFormat.NATIVE); // it tried native first
    expect(h.onSummary.mock.calls.some((c: string[]) => String(c[0]).includes("native unavailable"))).toBe(true);
  });

  it("toggles the loading indicator and reports native availability", async () => {
    const h = harness();
    await h.presenter.openFile("m", "board.eds");
    // Busy went on then off, and ended off (balanced nesting).
    const calls = (h.render.setBusy as any).mock.calls.map((c: [boolean]) => c[0]);
    expect(calls[0]).toBe(true);
    expect(calls[calls.length - 1]).toBe(false);
    expect(lastControls(h).nativeAvailable).toBe(true);
  });

  it("switching designs drops the sticky layout and adopts the new design's default", async () => {
    const h = harness();
    await h.presenter.openFile("m", "a.eds"); // faithful
    await h.presenter.setLayout("grid");
    h.getDesign.mockClear();
    await h.presenter.openFile("m", "b.eds");
    // The new design is requested with no layout, so the server's effective default
    // (faithful, per the fake's empty->faithful echo) wins over the previous grid choice.
    expect(h.getDesign.mock.calls[0][0].layout).toBe("");
    expect(lastControls(h).layout).toBe("faithful");
  });

  it("re-opening the same design keeps the chosen layout", async () => {
    const h = harness();
    await h.presenter.openFile("m", "a.eds");
    await h.presenter.setLayout("grid");
    h.getDesign.mockClear();
    await h.presenter.openFile("m", "a.eds");
    expect(h.getDesign.mock.calls[0][0].layout).toBe("grid");
  });

  it("changing layout re-opens the design and threads the layout through GetDesign and GetSheet", async () => {
    const h = harness();
    await h.presenter.openFile("m", "board.edn"); // opens at faithful
    h.getDesign.mockClear();
    h.getSheet.mockClear();
    await h.presenter.setLayout("grid");
    expect(h.getDesign).toHaveBeenCalledOnce();
    expect(h.getDesign.mock.calls[0][0].layout).toBe("grid");
    const gs = h.getSheet.mock.calls;
    expect(gs[gs.length - 1][0].layout).toBe("grid"); // sheet render uses the new layout
    expect(lastControls(h).layout).toBe("grid");
  });

  it("setLayout to the current layout is a no-op", async () => {
    const h = harness();
    await h.presenter.openFile("m", "board.edn"); // faithful
    h.getDesign.mockClear();
    await h.presenter.setLayout("faithful");
    expect(h.getDesign).not.toHaveBeenCalled();
  });

  it("reports the URL-addressable location on open and on sheet/mode/layout change", async () => {
    const h = harness();
    await h.presenter.openFile("m", "board.eds");
    expect(lastLocation(h)).toMatchObject({ mount: "m", path: "board.eds", sheet: "s1", mode: "svg", layout: "faithful" });
    await h.presenter.setMode("webgl");
    expect(lastLocation(h).mode).toBe("webgl");
    await h.presenter.setLayout("grid");
    expect(lastLocation(h).layout).toBe("grid");
  });

  it("restore opens the named file at the requested sheet and adopts the URL's mode/layout/symbols", async () => {
    const h = harness();
    // A two-sheet design so restoring a non-first sheet is observable.
    h.getDesign.mockResolvedValueOnce({
      name: "D",
      layout: "grid",
      sourceFormat: "",
      componentCount: 0,
      netCount: 0,
      sheets: [{ id: "s1", name: "S1" }, { id: "s2", name: "S2" }],
      nativeAvailable: true,
      availableLayouts: ["faithful", "grid"],
    } as any);
    await h.presenter.restore({ mount: "m", path: "board.eds", isDir: false, sheet: "s2", mode: "webgl", layout: "grid", symbols: true });
    // The design loaded at the URL's layout, and the render used the URL's mode + symbol source.
    expect(h.getDesign.mock.calls[0][0].layout).toBe("grid");
    const gs = h.getSheet.mock.calls[h.getSheet.mock.calls.length - 1][0];
    expect(gs).toMatchObject({ sheet: "s2", format: SheetFormat.PACKED, layout: "grid", symbols: SymbolSource.FAITHFUL });
    // Both nav surfaces mark the restored (non-first) sheet active.
    for (const nav of [h.navA, h.navB]) {
      const last = nav.setState.mock.calls[nav.setState.mock.calls.length - 1][0];
      expect(last.activeId).toBe("s2");
    }
  });

  it("restore falls back to the first sheet when the URL names a sheet the design lacks", async () => {
    const h = harness(); // default design has only s1
    await h.presenter.restore({ mount: "m", path: "board.eds", isDir: false, sheet: "ghost", mode: "", layout: "", symbols: false });
    expect(h.getSheet.mock.calls[h.getSheet.mock.calls.length - 1][0].sheet).toBe("s1");
  });

  it("entering Native snaps the layout to faithful", async () => {
    const h = harness();
    await h.presenter.openFile("m", "board.edn");
    await h.presenter.setLayout("grid"); // now on a non-faithful layout
    await h.presenter.setMode("native");
    const last = lastControls(h);
    expect(last.mode).toBe("native");
    expect(last.layout).toBe("faithful"); // native forced the layout back to faithful
  });

  it("running checks runs the default selection and pushes the findings", async () => {
    const h = harness();
    h.checkDesign.mockResolvedValueOnce({
      findings: [{ rule: "single-pin-net", severity: "info", subject: { kind: "net", ref: "STUB", pin: "" }, message: "stub" }],
    });
    await openAndCheck(h, "m", "board.edn");
    // The active ruleset is the default (both catalog rules), run in one call by the Run button.
    expect(h.checkDesign).toHaveBeenCalledWith({ uri: artifactUri("m", "board.edn"), rules: ["single-pin-net", "diff-pair-naming"] });
    const last = lastFindings(h);
    expect(last.findings.map((f: { subject: string }) => f.subject)).toEqual(["STUB"]);
    expect(last.selected).toBe("");
    expect(last.ruleCount).toBe(2);
  });

  it("maps finding sheets to named badges and navigates to the subject's sheet before highlighting (WS9-024)", async () => {
    const h = harness();
    h.getDesign.mockResolvedValue({
      name: "D",
      layout: "faithful",
      sourceFormat: "",
      componentCount: 0,
      netCount: 0,
      sheets: [
        { id: "s1", name: "Root" },
        { id: "s2", name: "Power" },
      ],
      nativeAvailable: false,
      availableLayouts: ["faithful"],
    });
    h.checkDesign.mockResolvedValue({
      findings: [{ rule: "single-pin-net", severity: "info", subject: { kind: "net", ref: "STUB", pin: "" }, message: "stub", sheets: ["s2"] }],
    });
    await openAndCheck(h, "m", "board.kicad_sch"); // opens on s1
    // The wire's sheet ids are denormalized to {id, name} badges for display.
    expect(lastFindings(h).findings[0].sheets).toEqual([{ id: "s2", name: "Power" }]);

    // Focusing the finding renders its sheet (s2) BEFORE applying the focused highlight.
    await h.presenter.selectFinding("STUB");
    const navIdx = h.getSheet.mock.calls.findIndex((c) => c[0].sheet === "s2");
    expect(navIdx).toBeGreaterThanOrEqual(0);
    const navOrder = h.getSheet.mock.invocationCallOrder[navIdx];
    const hlOrders = h.canvas.setHighlights.mock.invocationCallOrder;
    expect(navOrder).toBeLessThan(hlOrders[hlOrders.length - 1]);
    // A net focus is a translucent PATH highlighter along its wire (WS9-040), and the focused
    // net drops out of the base layer so the opaque underlay does not show through it. STUB is
    // the only finding, so the base is empty and the stack is the highlighter alone.
    expect(h.canvas.setHighlights).toHaveBeenLastCalledWith([{ nets: ["STUB"], shape: HighlightShape.PATH }]);
    // The nav surfaces now show s2 active.
    const navCalls = h.navA.setState.mock.calls;
    expect(navCalls[navCalls.length - 1][0].activeId).toBe("s2");
  });

  it("an explicit badge click navigates to that sheet even when the subject is already focused", async () => {
    const h = harness();
    h.getDesign.mockResolvedValue({
      name: "D",
      layout: "faithful",
      sourceFormat: "",
      componentCount: 0,
      netCount: 0,
      sheets: [
        { id: "s1", name: "Root" },
        { id: "s2", name: "Power" },
      ],
      nativeAvailable: false,
      availableLayouts: ["faithful"],
    });
    h.checkDesign.mockResolvedValue({
      findings: [{ rule: "i2c-pull-up", severity: "error", subject: { kind: "net", ref: "SDA", pin: "" }, message: "m", sheets: ["s1", "s2"] }],
    });
    await openAndCheck(h, "m", "board.kicad_sch");
    await h.presenter.selectFinding("SDA"); // s1 is current and SDA is on it: no navigation
    expect(h.getSheet.mock.calls.some((c) => c[0].sheet === "s2")).toBe(false);
    await h.presenter.selectFinding("SDA", "s2"); // badge click: navigate, keep the focus
    expect(h.getSheet.mock.calls.some((c) => c[0].sheet === "s2")).toBe(true);
    expect(lastFindings(h).selected).toBe("SDA");
  });

  it("locateEntity navigates to a query cell's sheet then highlights the entity (WS9-038)", async () => {
    const h = harness();
    h.getDesign.mockResolvedValue({
      name: "D",
      layout: "faithful",
      sourceFormat: "",
      componentCount: 0,
      netCount: 0,
      sheets: [
        { id: "s1", name: "Root" },
        { id: "s2", name: "Power" },
      ],
      nativeAvailable: false,
      availableLayouts: ["faithful"],
    });
    await h.presenter.openFile("m", "board.kicad_sch"); // opens on s1
    h.canvas.setHighlights.mockClear();
    await h.presenter.locateEntity("component", "R1", "s2");
    // Navigates to s2 (the badge's sheet) and the last highlight frames R1 as a bounding rect.
    expect(h.getSheet.mock.calls.some((c) => c[0].sheet === "s2")).toBe(true);
    expect(h.canvas.setHighlights).toHaveBeenLastCalledWith([{ components: ["R1"], shape: HighlightShape.BOUNDING_RECT }]);
  });

  it("locateEntity highlights an entity on the current sheet with no navigation when no sheet is given", async () => {
    const h = harness();
    h.getDesign.mockResolvedValue({
      name: "D",
      layout: "faithful",
      sourceFormat: "",
      componentCount: 0,
      netCount: 0,
      sheets: [
        { id: "s1", name: "Root" },
        { id: "s2", name: "Power" },
      ],
      nativeAvailable: false,
      availableLayouts: ["faithful"],
    });
    await h.presenter.openFile("m", "board.kicad_sch"); // s1 current
    h.getSheet.mockClear();
    await h.presenter.locateEntity("net", "SDA");
    expect(h.getSheet).not.toHaveBeenCalled(); // no sheet argument -> stay put
    // A located net is a translucent PATH highlighter along its wire (WS9-040).
    expect(h.canvas.setHighlights).toHaveBeenLastCalledWith([{ nets: ["SDA"], shape: HighlightShape.PATH }]);
  });

  it("locateEntity explains an undrawn entity on a faithful layout, clears it for a drawn one (WS9-039)", async () => {
    const h = harness();
    await h.presenter.openFile("m", "board.kicad_sch"); // layout: faithful
    // A server-set reason (an undrawn rail) surfaces as a note on the faithful layout.
    await h.presenter.locateEntity("net", "GND", undefined, LocateReason.POWER_RAIL_NO_WIRE);
    expect(h.query.setLocateNote).toHaveBeenLastCalledWith(expect.stringContaining("power rail"));
    // A drawn entity (UNSPECIFIED) clears any prior note.
    await h.presenter.locateEntity("component", "R1", undefined, LocateReason.UNSPECIFIED);
    expect(h.query.setLocateNote).toHaveBeenLastCalledWith("");
  });

  it("locateEntity suppresses the locate note on an auto-layout, where every entity is drawn", async () => {
    const h = harness();
    h.getDesign.mockResolvedValue({
      name: "D",
      layout: "grid",
      sourceFormat: "",
      componentCount: 0,
      netCount: 0,
      sheets: [{ id: "s1", name: "S1" }],
      nativeAvailable: false,
      availableLayouts: ["grid"],
    });
    await h.presenter.openFile("m", "board.edn"); // auto-layout: everything resolves
    await h.presenter.locateEntity("net", "GND", undefined, LocateReason.POWER_RAIL_NO_WIRE);
    expect(h.query.setLocateNote).toHaveBeenLastCalledWith(""); // no note on grid
  });

  it("suppresses sheet badges for a single-sheet design", async () => {
    const h = harness();
    h.checkDesign.mockResolvedValue({
      findings: [{ rule: "single-pin-net", severity: "info", subject: { kind: "net", ref: "STUB", pin: "" }, message: "stub", sheets: ["s1"] }],
    });
    await openAndCheck(h, "m", "board.edn"); // default harness design has one sheet
    expect(lastFindings(h).findings[0].sheets).toEqual([]);
  });

  it("opening a file fetches the rule catalog and default-selects the available rules", async () => {
    const h = harness();
    await h.presenter.openFile("m", "board.edn");
    expect(h.listRules).toHaveBeenCalledWith({ uri: artifactUri("m", "board.edn") });
    const rs = lastRules(h);
    expect(rs.rules.map((r: { name: string }) => r.name)).toEqual(["single-pin-net", "diff-pair-naming"]);
    expect(rs.selected).toEqual(["single-pin-net", "diff-pair-naming"]);
  });

  it("reconciles a design's expectation sidecar into a status-colored overlay + caption (WS9-045)", async () => {
    const h = harness();
    h.getExpectations.mockResolvedValueOnce({
      expectations: [
        { rule: "single-pin-net", subjects: ["STUB"], pending: false },
        { rule: "net-naming-convention", subjects: ["BAD"], pending: true },
      ],
      hasSidecar: true,
    });
    h.checkDesign.mockResolvedValue({
      findings: [{ rule: "single-pin-net", severity: "info", subject: { kind: "net", ref: "STUB", pin: "" }, message: "" }],
    });
    await openAndCheck(h, "m", "board.edn");

    expect(h.getExpectations).toHaveBeenCalledWith({ uri: artifactUri("m", "board.edn") });
    // Caption (the non-anchored verdict): single-pin-net matched exactly; the pending row is excluded.
    const capCalls = h.onExpectCaption.mock.calls;
    expect(capCalls[capCalls.length - 1][0]).toMatchObject({ pass: true, expected: 1, matched: 1, unexpected: 0, silent: false });
    // Overlay (the anchored assertion): matched subject STUB highlighted green.
    type Spec = { color?: string; nets?: string[] };
    const specs = h.canvas.setHighlights.mock.calls.flatMap((c: Spec[][]) => c[0]) as Spec[];
    expect(specs.find((s) => s?.color === "#22c55e")?.nets).toEqual(["STUB"]);
  });

  it("a design with no sidecar shows no expectation caption (WS9-045)", async () => {
    const h = harness(); // default getExpectations: hasSidecar=false
    await h.presenter.openFile("m", "board.edn");
    const capCalls = h.onExpectCaption.mock.calls;
    expect(capCalls[capCalls.length - 1][0]).toBeNull();
  });

  it("an unavailable rule is not in the default selection", async () => {
    const h = harness();
    h.listRules.mockResolvedValueOnce({
      rules: [
        { name: "single-pin-net", severity: "info", summary: "s", reads: [], tags: { category: "connectivity" }, available: true, unavailableReason: "" },
        { name: "cap-voltage", severity: "error", summary: "s", reads: ["param(mpn, v)"], tags: { category: "datasheet" }, available: false, unavailableReason: "needs datasheet layer" },
      ],
    });
    await h.presenter.openFile("m", "board.edn");
    expect(lastRules(h).selected).toEqual(["single-pin-net"]);
  });

  it("denormalizes the rule's profile tag onto its findings, for the by-interface group-by (WS9-041)", async () => {
    const h = harness();
    h.listRules.mockResolvedValueOnce({
      rules: [
        { name: "spi_nor-signal-missing", severity: "warning", summary: "s", reads: [], tags: { category: "connectivity", profile: "SPI_NOR" }, available: true, unavailableReason: "" },
      ],
    });
    h.checkDesign.mockResolvedValueOnce({
      findings: [{ rule: "spi_nor-signal-missing", severity: "warning", subject: { kind: "net", ref: "SPI_CS", pin: "" }, message: "missing IO2" }],
    });
    await openAndCheck(h, "m", "board.edn");
    const f = lastFindings(h).findings.find((x: { rule: string }) => x.rule === "spi_nor-signal-missing");
    expect(f?.profile).toBe("SPI_NOR");
  });

  it("caches findings per rule: a run does all, then unticking hides with no refetch", async () => {
    const h = harness();
    h.checkDesign.mockResolvedValueOnce({
      findings: [
        { rule: "single-pin-net", severity: "info", subject: { kind: "net", ref: "STUB", pin: "" }, message: "s" },
        { rule: "diff-pair-naming", severity: "warning", subject: { kind: "net", ref: "TXP", pin: "" }, message: "d" },
      ],
    });
    await openAndCheck(h, "m", "board.edn");
    expect(lastFindings(h).findings.map((f: { subject: string }) => f.subject).sort()).toEqual(["STUB", "TXP"]);
    h.checkDesign.mockClear();

    // Both rules already ran, so unticking one refetches nothing; its finding is hidden.
    await h.presenter.setRuleSelection(["single-pin-net"]);
    expect(h.checkDesign).not.toHaveBeenCalled();
    expect(lastFindings(h).findings.map((f: { subject: string }) => f.subject)).toEqual(["STUB"]);
    expect(lastRules(h).selected).toEqual(["single-pin-net"]);
    // The deselected rule drops out of the fired counts (badges track the current selection).
    expect(lastRules(h).fired["diff-pair-naming"]).toBeUndefined();
    expect(lastRules(h).fired["single-pin-net"]).toBe(1);

    // Re-ticking shows the cached finding again, still no refetch.
    await h.presenter.setRuleSelection(["single-pin-net", "diff-pair-naming"]);
    expect(h.checkDesign).not.toHaveBeenCalled();
    expect(lastFindings(h).findings).toHaveLength(2);
  });

  it("empty selection shows no rules selected and runs nothing", async () => {
    const h = harness();
    await h.presenter.openFile("m", "board.edn");
    h.checkDesign.mockClear();
    await h.presenter.setRuleSelection([]);
    expect(h.checkDesign).not.toHaveBeenCalled();
    const fs = lastFindings(h);
    expect(fs.findings).toHaveLength(0);
    expect(fs.ruleCount).toBe(0);
  });

  it("pushes per-rule fired counts to the rules panel (0 for a rule that ran clean)", async () => {
    const h = harness();
    h.checkDesign.mockResolvedValueOnce({
      findings: [
        { rule: "single-pin-net", severity: "info", subject: { kind: "net", ref: "STUB", pin: "" }, message: "s" },
        { rule: "single-pin-net", severity: "info", subject: { kind: "net", ref: "STUB2", pin: "" }, message: "s" },
      ],
    });
    await openAndCheck(h, "m", "board.edn");
    expect(lastRules(h).fired["single-pin-net"]).toBe(2);
    expect(lastRules(h).fired["diff-pair-naming"]).toBe(0);
  });

  it("highlights every current finding's subject at once after a run", async () => {
    const h = harness();
    h.checkDesign.mockResolvedValueOnce({
      findings: [
        { rule: "single-pin-net", severity: "info", subject: { kind: "net", ref: "STUB", pin: "" }, message: "s" },
        { rule: "diff-pair-naming", severity: "warning", subject: { kind: "net", ref: "TXP", pin: "" }, message: "d" },
      ],
    });
    await openAndCheck(h, "m", "board.edn");
    // The same spec drives both renderers (canvas.setHighlights is the WebGL half, always called).
    const spec = h.canvas.setHighlights.mock.calls.at(-1)?.[0];
    expect(new Set(spec[0].nets)).toEqual(new Set(["STUB", "TXP"]));
  });

  it("a CheckDesign failure clears the findings rather than erroring", async () => {
    const h = harness();
    h.checkDesign.mockRejectedValueOnce(new Error("no netlist"));
    await openAndCheck(h, "m", "board.eds"); // geometry-only, no netlist
    expect(h.onSummary.mock.calls.every((c) => !String(c[0]).startsWith("error:"))).toBe(true);
    expect(lastFindings(h).findings).toHaveLength(0);
  });

  it("focusing a finding in SVG mode highlights just it (exact by kind) and toggles back to all", async () => {
    const h = harness();
    h.checkDesign.mockResolvedValueOnce({
      findings: [
        { rule: "single-pin-net", severity: "info", subject: { kind: "net", ref: "STUB", pin: "" }, message: "s" },
        { rule: "diff-pair-naming", severity: "warning", subject: { kind: "net", ref: "TXP", pin: "" }, message: "d" },
      ],
    });
    await openAndCheck(h, "m", "board.edn"); // opens in SVG mode; highlight is all subjects
    h.canvas.setHighlights.mockClear();

    await h.presenter.selectFinding("STUB");
    expect(lastControls(h).mode).toBe("svg"); // no mode hop: the overlay draws in place
    // Exact by kind: STUB is a net, so the focus spec lists it only under nets (not components).
    // Focus stacks (WS9-017): the OTHER finding (TXP) keeps its outline, the focused net STUB
    // drops out of the base and paints as a translucent PATH highlighter on top (WS9-040), so
    // its marker is not muddied by an opaque underlay.
    const stack = [{ nets: ["TXP"] }, { nets: ["STUB"], shape: HighlightShape.PATH }];
    expect(h.canvas.setHighlights).toHaveBeenLastCalledWith(stack);
    expect(h.highlightSheet).toHaveBeenCalledWith(
      expect.objectContaining({ format: SheetFormat.SVG, specs: stack }),
    );
    expect(h.render.setSvgOverlay).toHaveBeenLastCalledWith("<svg data-overlay/>");
    expect(lastFindings(h).selected).toBe("STUB");

    await h.presenter.selectFinding("STUB"); // toggle off -> back to the whole-selection highlight
    const spec = h.canvas.setHighlights.mock.calls.at(-1)?.[0];
    expect(new Set(spec[0].nets)).toEqual(new Set(["STUB", "TXP"]));
    expect(lastFindings(h).selected).toBe("");
  });

  it("focusing a component finding lights it up as a component (not a net)", async () => {
    const h = harness();
    h.checkDesign.mockResolvedValueOnce({
      findings: [{ rule: "single-pin-net", severity: "warning", subject: { kind: "component", ref: "C1", pin: "" }, message: "c" }],
    });
    await openAndCheck(h, "m", "board.edn");
    h.canvas.setHighlights.mockClear();
    await h.presenter.selectFinding("C1");
    expect(h.canvas.setHighlights).toHaveBeenLastCalledWith([{ components: ["C1"] }, { components: ["C1"], shape: HighlightShape.BOUNDING_RECT }]);
  });

  it("focusing a finding in Native mode hops to WebGL (no overlay for the golden document)", async () => {
    const h = harness();
    h.checkDesign.mockResolvedValueOnce({
      findings: [{ rule: "single-pin-net", severity: "warning", subject: { kind: "component", ref: "R1", pin: "" }, message: "r" }],
    });
    await openAndCheck(h, "m", "board.edn");
    await h.presenter.setMode("native");
    h.highlightSheet.mockClear();
    await h.presenter.selectFinding("R1");
    expect(lastControls(h).mode).toBe("webgl");
    expect(h.canvas.setHighlights).toHaveBeenLastCalledWith([{ components: ["R1"] }, { components: ["R1"], shape: HighlightShape.BOUNDING_RECT }]);
    expect(h.highlightSheet).not.toHaveBeenCalled(); // WebGL resolves locally, no overlay fetch
  });

  it("setHighlights re-fetches the SVG overlay when the sheet re-renders", async () => {
    const h = harness();
    await h.presenter.openFile("m", "board.eds"); // SVG mode
    await h.presenter.setHighlights([{ nets: ["NET1"], color: "#00ff00", alpha: 0.5 }]);
    expect(h.render.setSvgOverlay).toHaveBeenLastCalledWith("<svg data-overlay/>");
    const fetches = h.highlightSheet.mock.calls.length;
    await h.presenter.showSheet("s1"); // re-render: the overlay must be re-framed with the sheet
    expect(h.highlightSheet.mock.calls.length).toBe(fetches + 1);
    expect(h.render.setSvgOverlay).toHaveBeenLastCalledWith("<svg data-overlay/>");
  });
});

describe("on-demand checks (WS9)", () => {
  it("opening a file does not run checks — they wait for the Run button", async () => {
    const h = harness();
    await h.presenter.openFile("m", "board.edn");
    expect(h.checkDesign).not.toHaveBeenCalled();
    const fs = lastFindings(h);
    expect(fs.findings).toHaveLength(0);
    expect(fs.ruleCount).toBe(2);
    expect(fs.pending).toBe(2); // both selected rules await a run
  });

  it("runChecks evaluates the current selection and clears the running flag", async () => {
    const h = harness();
    h.checkDesign.mockResolvedValueOnce({
      findings: [{ rule: "single-pin-net", severity: "info", subject: { kind: "net", ref: "STUB", pin: "" }, message: "s" }],
    });
    await h.presenter.openFile("m", "board.edn");
    await h.presenter.runChecks();
    expect(h.checkDesign).toHaveBeenCalledWith({ uri: artifactUri("m", "board.edn"), rules: ["single-pin-net", "diff-pair-naming"] });
    const fs = lastFindings(h);
    expect(fs.findings.map((f: { subject: string }) => f.subject)).toEqual(["STUB"]);
    expect(fs.pending).toBe(0);
    expect(fs.running).toBe(false);
  });

  it("toggling a rule fetches nothing; a not-yet-run rule shows pending until the next run", async () => {
    const h = harness();
    await h.presenter.openFile("m", "board.edn");
    await h.presenter.setRuleSelection(["single-pin-net"]); // toggle only — no fetch
    expect(h.checkDesign).not.toHaveBeenCalled();
    await h.presenter.runChecks(); // runs only the selected rule
    expect(h.checkDesign).toHaveBeenCalledWith({ uri: artifactUri("m", "board.edn"), rules: ["single-pin-net"] });
    h.checkDesign.mockClear();
    await h.presenter.setRuleSelection(["single-pin-net", "diff-pair-naming"]); // adds an unrun rule
    expect(h.checkDesign).not.toHaveBeenCalled(); // still no fetch on a toggle
    expect(lastFindings(h).pending).toBe(1); // diff-pair-naming awaits the next run
  });
});

describe("sheet overview push (WS9-025)", () => {
  it("pushes tiles with the active sheet and re-pushes when the rule selection changes", async () => {
    const h = harness();
    await h.presenter.openFile("m", "d.edn");
    const calls = h.onOverview.mock.calls;
    const afterOpen = calls[calls.length - 1][0];
    // Single-sheet design: the one tile carries the total findings count (none by default).
    expect(afterOpen.tiles).toEqual([{ id: "s1", name: "S1", count: 0 }]);
    expect(afterOpen.activeId).toBe("s1");
    expect(afterOpen.ruleCount).toBe(2); // the default catalog selects both rules
    await h.presenter.setRuleSelection([]);
    const afterClear = h.onOverview.mock.calls[h.onOverview.mock.calls.length - 1][0];
    expect(afterClear.ruleCount).toBe(0);
  });
});

describe("board layer visibility (WS7-034)", () => {
  it("offers the selector only on the board sheet and applies the choice as a view command", async () => {
    const h = harness();
    h.getDesign.mockImplementation(async (req: { layout?: string }) => ({
      name: "B",
      layout: req.layout || "grid",
      sourceFormat: "kicad",
      componentCount: 0,
      netCount: 0,
      sheets: [
        { id: "graph", name: "netlist graph" },
        { id: "board", name: "Board" },
      ],
      nativeAvailable: false,
      availableLayouts: ["grid"],
    }));
    await h.presenter.openFile("m", "x.kicad_pcb");
    expect(lastControls(h).board).toBe(false); // the graph sheet opened first

    await h.presenter.showSheet("board");
    expect(lastControls(h).board).toBe(true);
    expect(h.render.setBoardLayers).toHaveBeenCalledWith("all"); // restored on entry

    h.presenter.setBoardLayers("front");
    expect(h.render.setBoardLayers).toHaveBeenLastCalledWith("front");
    expect(lastControls(h).boardLayers).toBe("front");

    // The choice sticks across a sheet round-trip.
    await h.presenter.showSheet("graph");
    expect(lastControls(h).board).toBe(false);
    await h.presenter.showSheet("board");
    expect(h.render.setBoardLayers).toHaveBeenLastCalledWith("front");
  });
});
