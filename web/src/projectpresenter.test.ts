import { describe, it, expect, vi } from "vitest";
import { ViewerPresenter } from "./viewer.js";
import { artifactUri } from "./uri.js";
import { NO_PROJECT_LABEL, PLAIN_LABEL, projectLabel, type ProjectState } from "./project.js";

// A viewer harness wired with a ProjectService whose resolution the test controls. Everything else
// is stubbed to the minimum the presenter touches on open.
function harness(resolve: unknown) {
  const onProject = vi.fn();
  const resolveDesign = vi.fn(async () => resolve);
  const listRules = vi.fn(async () => ({ rules: [{ name: "bulk-cap", severity: "warning", summary: "", available: true }] }));
  const checkDesign = vi.fn(async () => ({ findings: [] }));
  const getDesign = vi.fn(async () => ({ layout: "grid", sheets: [{ id: "s1", name: "Top" }], availableLayouts: ["grid"] }));
  const getSheet = vi.fn(async () => ({ content: { case: "svg" as const, value: "<svg/>" } }));
  const listDir = vi.fn(async () => ({ entries: [] }));

  const presenter = new ViewerPresenter(
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    { getDesign, getSheet } as any,
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    { listRules, checkDesign, getExpectations: async () => ({ expectations: [] }) } as any,
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    { showSheet: vi.fn(), setHighlights: vi.fn() } as any,
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    {
      showWebgl: vi.fn(), showSvg: vi.fn(), setSvgOverlay: vi.fn(), setBusy: vi.fn(),
      getView: vi.fn(), setView: vi.fn(), setBoardLayers: vi.fn(),
    } as any,
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    {
      sheetNavs: [],
      summary: vi.fn(),
      controls: { setState: vi.fn() },
      findings: { setState: vi.fn(), setFindingLocateNote: vi.fn() },
      expectationCaption: vi.fn(),
      rules: { setState: vi.fn() },
      report: vi.fn(),
      projectBar: { setState: onProject },
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
    } as any,
    undefined,
    undefined,
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    { listDir } as any,
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    { resolveDesign } as any,
  );
  return { presenter, onProject, resolveDesign, listRules, checkDesign };
}

// last is the most recent state the bar was pushed, which is what a user is actually looking at.
function last(onProject: ReturnType<typeof vi.fn>): ProjectState {
  const calls = onProject.mock.calls;
  if (calls.length === 0) throw new Error("the project bar was never pushed");
  return calls[calls.length - 1][0] as ProjectState;
}

const inProject = {
  design: { name: "projects/gateway/designs/gateway", entryUri: "mount://m/d/gateway.edn" },
  project: { name: "projects/gateway", title: "Gateway ECU", checklistUri: "mount://m/review.yaml", profileUris: [] },
};

describe("project resolution is visible", () => {
  it("names the project whose config produced the answers", async () => {
    const h = harness(inProject);
    await h.presenter.openFile("m", "d/gateway.edn");
    expect(projectLabel(last(h.onProject))).toBe("Gateway ECU");
  });

  // The ordinary case on a mounted folder, and it has to look like an answer rather than a blank.
  it("states no-project for a design that belongs to none", async () => {
    const h = harness({});
    await h.presenter.openFile("m", "loose/board.edn");
    expect(projectLabel(last(h.onProject))).toBe(NO_PROJECT_LABEL);
  });

  // The served viewer shows the file it was asked for; a silent swap has no browser equivalent. So
  // the resolution is surfaced and acting on it stays the user's move.
  it("surfaces that the open file is a companion rather than the entry", async () => {
    const h = harness(inProject);
    await h.presenter.openFile("m", "d/gateway.kicad_pcb");
    expect(last(h.onProject).namedIsEntry).toBe(false);
  });

  it("treats the entry itself as the entry", async () => {
    const h = harness(inProject);
    await h.presenter.openFile("m", "d/gateway.edn");
    expect(last(h.onProject).namedIsEntry).toBe(true);
  });

  // A failure must not read as "this design has no project": one is not knowing, the other is knowing
  // there is none, and they lead a reviewer to opposite conclusions about the findings on screen.
  it("reports a resolution failure rather than showing no-project", async () => {
    const h = harness({});
    h.resolveDesign.mockImplementation(async () => {
      throw new Error("mount unreachable");
    });
    await h.presenter.openFile("m", "d/gateway.edn");
    expect(last(h.onProject).error).toContain("mount unreachable");
  });
});

describe("the built-in catalog toggle", () => {
  it("re-runs both the rule list and the findings", async () => {
    const h = harness(inProject);
    await h.presenter.openFile("m", "d/gateway.edn");
    const rulesBefore = h.listRules.mock.calls.length;
    const checksBefore = h.checkDesign.mock.calls.length;

    await h.presenter.setPlainCatalog(true);

    // Findings from one catalog beside the rule set of another is exactly the drift the toggle exists
    // to make visible, so both are recomposed.
    expect(h.listRules.mock.calls.length).toBeGreaterThan(rulesBefore);
    expect(h.checkDesign.mock.calls.length).toBeGreaterThan(checksBefore);
  });

  it("sends ignore_project on the rule-running requests", async () => {
    const h = harness(inProject);
    await h.presenter.openFile("m", "d/gateway.edn");
    await h.presenter.setPlainCatalog(true);
    const calls = h.checkDesign.mock.calls as unknown as Array<[{ overlay?: { ignoreProject?: boolean }; uri: string }]>;
    const req = calls[calls.length - 1][0];
    expect(req.overlay?.ignoreProject).toBe(true);
    expect(req.uri).toBe(artifactUri("m", "d/gateway.edn"));
  });

  it("says the built-in catalog is in effect, distinctly from having no project", async () => {
    const h = harness(inProject);
    await h.presenter.openFile("m", "d/gateway.edn");
    await h.presenter.setPlainCatalog(true);
    expect(projectLabel(last(h.onProject))).toBe(PLAIN_LABEL);
  });

  it("is a no-op when already in the requested state", async () => {
    const h = harness(inProject);
    await h.presenter.openFile("m", "d/gateway.edn");
    const before = h.checkDesign.mock.calls.length;
    await h.presenter.setPlainCatalog(false);
    expect(h.checkDesign.mock.calls.length).toBe(before);
  });
});
