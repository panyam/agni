import { describe, it, expect, vi } from "vitest";
import { artifactUri } from "./uri.js";
import { ConnectError, Code } from "@connectrpc/connect";
import { ViewerPresenter, type RenderView } from "./viewer.js";
import { SheetFormat } from "./gen/agni/v1/webapi/design_pb.js";
import type { ReviewState } from "./review.js";
import type { ConventionState } from "./conventions.js";
import { stubQueryView } from "./testviews.js";

// A run document as the server returns it: a Review resource wrapping a CheckResults.
function doc(name: string, createdAt: string, outcomes: string[]) {
  return {
    name,
    results: {
      meta: { createdAt, producerVersion: "v0.1.1" },
      design: { source: "proj/board.edn", contentHash: "sha256:abc" },
      manifest: "Gateway ECU review",
      areas: [
        {
          name: "Power",
          items: outcomes.map((outcome, i) => ({
            id: `P${i + 1}`,
            title: `item ${i + 1}`,
            outcome,
            note: "",
            findings: [],
          })),
        },
      ],
    },
  };
}

// harness builds a presenter with only what the review path needs; every other collaborator is a
// stub sufficient for openFile to complete.
function harness(opts: { wireReview?: boolean; wireClients?: boolean; wireConventionBar?: boolean } = {}) {
  const wireReview = opts.wireReview !== false;
  const wireClients = opts.wireClients !== false;
  const wireConventionBar = opts.wireConventionBar !== false;
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
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
  } as any;
  const getNamingConvention = vi.fn(async (_req: { mount: string; ref: string }) => ({
    convention: { name: "house", rules: [], lexicon: undefined },
  }));
  const checks = {
    // One available rule, so opening a design default-selects it and a check run actually calls
    // checkDesign. With an empty catalog nothing is selected and the run is a no-op, which would make
    // the overlay assertions below vacuous.
    listRules: vi.fn(async () => ({
      rules: [{ name: "bulk-cap", severity: "warning", summary: "", reads: [], tags: {}, available: true, unavailableReason: "" }],
    })),
    checkDesign: vi.fn(async () => ({ findings: [] })),
    getExpectations: vi.fn(async () => ({ expectations: [] })),
    getInterfaceCoverage: vi.fn(async () => ({ interfaces: [] })),
    getPartParams: vi.fn(async () => ({ parts: [] })),
    getNamingConvention,
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
  } as any;
  const listReviews = vi.fn(async (_req: { filter?: string }) => ({
    reviews: [doc("reviews/r2", "2026-08-10T20:00:00Z", ["pass", "fail"]), doc("reviews/r1", "2026-08-01T09:00:00Z", ["pass", "pass"])],
  }));
  const getReviewManifest = vi.fn(async (_req: { mount: string; ref: string }) => ({
    manifest: { name: "Gateway ECU review", areas: [] },
  }));
  const createReview = vi.fn(async (_req: { parent: string; designUri: string; manifest: unknown }) =>
    doc("reviews/r3", "2026-08-11T08:00:00Z", ["pass", "pass", "fail"]),
  );
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  const reviews = { listReviews, getReviewManifest, createReview } as any;
  const listDir = vi.fn(async (_req: { uri: string }) => ({
    entries: [
      { name: "board.edn", uri: "mount://m/proj/board.edn", isDir: false, format: "edif" },
      { name: "review.yaml", uri: "mount://m/proj/review.yaml", isDir: false, format: "" },
      { name: "profiles", uri: "mount://m/proj/profiles", isDir: true, format: "" },
    ],
  }));
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  const workspace = { listDir } as any;

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

  const runQuery = vi.fn(async (_req: { uri: string; query: string; overlay?: unknown }) => ({
    columns: ["n"], columnKinds: [], rows: [],
  }));
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  const query = { runQuery } as any;
  const onReview = vi.fn();
  const onConvention = vi.fn();
  const onSummary = vi.fn();
  const presenter = new ViewerPresenter(
    client, checks, canvas, render,
    {
      sheetNavs: [],
      summary: onSummary,
      controls: { setState: vi.fn() },
      findings: { setState: vi.fn(), setFindingLocateNote: vi.fn() },
      expectationCaption: vi.fn(),
      rules: { setState: vi.fn() },
      report: vi.fn(),
      query: stubQueryView(),
      review: wireReview ? { setState: onReview } : undefined,
      conventionBar: wireConventionBar ? { setState: onConvention } : undefined,
    },
    wireClients ? query : undefined,
    wireClients ? reviews : undefined,
    wireClients ? workspace : undefined,
  );
  return { presenter, onReview, onConvention, listReviews, getReviewManifest, createReview, listDir, getNamingConvention, checkDesign: checks.checkDesign, listRules: checks.listRules, runQuery };
}

function lastState(onReview: ReturnType<typeof vi.fn>): ReviewState {
  const calls = onReview.mock.calls;
  return calls[calls.length - 1][0] as ReviewState;
}

function lastConvention(onConvention: ReturnType<typeof vi.fn>): ConventionState {
  const calls = onConvention.mock.calls;
  return calls[calls.length - 1][0] as ConventionState;
}

describe("review presenter — loading", () => {
  it("lists the stored runs for the open design and selects the newest", async () => {
    const h = harness();
    await h.presenter.openFile("m", "proj/board.edn");
    const s = lastState(h.onReview);
    expect(s.runs.map((r) => r.name)).toEqual(["reviews/r2", "reviews/r1"]);
    expect(s.selected).toBe("reviews/r2");
    expect(s.storeConfigured).toBe(true);
  });

  // The filter is what makes the panel show THIS board's history rather than every board's. Getting
  // it wrong would show another design's verdicts under this one's name.
  it("filters the listing to the open design", async () => {
    const h = harness();
    await h.presenter.openFile("m", "proj/board.edn");
    expect(h.listReviews).toHaveBeenCalledWith({ filter: `design="${artifactUri("m", "proj/board.edn")}"` });
  });

  it("offers the checklists sitting beside the design, and nothing else in the directory", async () => {
    const h = harness();
    await h.presenter.openFile("m", "proj/board.edn");
    const s = lastState(h.onReview);
    expect(s.checklists.map((c) => c.label)).toEqual(["review.yaml"]);
    expect(s.checklist).toBe("proj/review.yaml");
    expect(h.listDir).toHaveBeenCalledWith({ uri: artifactUri("m", "proj") });
  });

  // A server with no --review-store is a DEPLOYMENT state, not a failure of this request, so it must
  // not surface as an error banner. The panel shows a different empty state for it.
  it("records a server that keeps no reviews as unconfigured, not as an error", async () => {
    const h = harness();
    h.listReviews.mockRejectedValueOnce(new ConnectError("no review store configured", Code.FailedPrecondition));
    await h.presenter.openFile("m", "proj/board.edn");
    const s = lastState(h.onReview);
    expect(s.storeConfigured).toBe(false);
    expect(s.error).toBe("");
    expect(s.runs).toEqual([]);
  });

  it("surfaces any other listing failure as an error", async () => {
    const h = harness();
    h.listReviews.mockRejectedValueOnce(new ConnectError("boom", Code.Internal));
    await h.presenter.openFile("m", "proj/board.edn");
    const s = lastState(h.onReview);
    expect(s.storeConfigured).toBe(true);
    expect(s.error).toContain("boom");
  });

  // Switching designs must not leave the previous board's verdicts on screen. They would look like
  // this design's, which is the worst possible way to be wrong in a review panel.
  it("clears the previous design's runs on switch", async () => {
    const h = harness();
    await h.presenter.openFile("m", "proj/board.edn");
    expect(lastState(h.onReview).runs).toHaveLength(2);
    h.listReviews.mockResolvedValueOnce({ reviews: [] });
    await h.presenter.openFile("m", "other/board2.edn");
    const s = lastState(h.onReview);
    expect(s.runs).toEqual([]);
    expect(s.selected).toBe("");
  });

  it("does nothing at all when the panel is unwired", async () => {
    const h = harness({ wireReview: false });
    await h.presenter.openFile("m", "proj/board.edn");
    expect(h.listReviews).not.toHaveBeenCalled();
    expect(h.onReview).not.toHaveBeenCalled();
  });

  // Regression: the panel was wired but the CLIENT was not, and the load path asserted the client
  // was present. Opening a design then threw a TypeError that surfaced as "Cannot read properties of
  // undefined" in the panel's error banner, which tells a user nothing about what is actually wrong.
  // A host that wires a view without its client should degrade to an inert panel, not a crash.
  it("stays inert when the panel is wired but the client is not", async () => {
    const h = harness({ wireClients: false });
    await h.presenter.openFile("m", "proj/board.edn");
    await h.presenter.createReview();
    const calls = h.onReview.mock.calls;
    if (calls.length > 0) expect((calls[calls.length - 1][0] as ReviewState).error).toBe("");
  });
});

describe("review presenter — creating", () => {
  it("resolves the checklist server-side, then creates, then shows the new run", async () => {
    const h = harness();
    await h.presenter.openFile("m", "proj/board.edn");
    await h.presenter.createReview();
    expect(h.getReviewManifest).toHaveBeenCalledWith({ uri: artifactUri("m", "proj/review.yaml") });
    expect(h.createReview).toHaveBeenCalledWith({
      // No project client in this harness, so the run stores unparented — which is the right answer
      // for a design that resolves to none.
      parent: "",
      designUri: artifactUri("m", "proj/board.edn"),
      manifest: { name: "Gateway ECU review", areas: [] },
    });
    const s = lastState(h.onReview);
    expect(s.selected).toBe("reviews/r3");
    expect(s.runs[0].name).toBe("reviews/r3");
    expect(s.runs).toHaveLength(3);
    expect(s.running).toBe(false);
  });

  // The manifest travels as a VALUE (C22): the browser holds a ref and no filesystem, so the read is
  // a named rpc rather than something the run does behind its back.
  it("sends the manifest value rather than a path", async () => {
    const h = harness();
    await h.presenter.openFile("m", "proj/board.edn");
    await h.presenter.createReview();
    const sent = h.createReview.mock.calls[0][0];
    expect(sent.manifest).toBeDefined();
    expect(Object.keys(sent)).not.toContain("manifestPath");
    expect(sent.designUri).toBe(artifactUri("m", "proj/board.edn"));
  });

  it("reports a bad checklist inline and keeps the existing history", async () => {
    const h = harness();
    await h.presenter.openFile("m", "proj/board.edn");
    h.getReviewManifest.mockRejectedValueOnce(
      new ConnectError('review manifest item "P1": declares more than one binding', Code.InvalidArgument),
    );
    await h.presenter.createReview();
    const s = lastState(h.onReview);
    expect(s.error).toContain("more than one binding");
    expect(s.runs).toHaveLength(2);
    expect(s.running).toBe(false);
  });

  it("does not create without a design open or a checklist chosen", async () => {
    const h = harness();
    await h.presenter.createReview();
    expect(h.createReview).not.toHaveBeenCalled();
    h.listDir.mockResolvedValueOnce({ entries: [] });
    await h.presenter.openFile("m", "proj/board.edn");
    await h.presenter.createReview();
    expect(h.createReview).not.toHaveBeenCalled();
  });
});

describe("review presenter — selection", () => {
  it("shows a chosen run without refetching it", async () => {
    const h = harness();
    await h.presenter.openFile("m", "proj/board.edn");
    h.listReviews.mockClear();
    h.presenter.showReview("reviews/r1");
    expect(lastState(h.onReview).selected).toBe("reviews/r1");
    expect(h.listReviews).not.toHaveBeenCalled();
  });

  it("records the chosen checklist for the next run", async () => {
    const h = harness();
    await h.presenter.openFile("m", "proj/board.edn");
    h.presenter.setChecklist("proj/other.yaml");
    expect(lastState(h.onReview).checklist).toBe("proj/other.yaml");
  });
});

// ---- Naming convention (WS9-128) --------------------------------------------------------------

function convHarness() {
  const h = harness();
  return h;
}

describe("naming convention", () => {
  // The whole point of the feature: what the user picks has to reach the requests that RUN rules.
  // Before this, OverlayConfig existed on all three and no client ever populated it.
  it("carries the resolved convention on a review create", async () => {
    const h = convHarness();
    await h.presenter.openFile("m", "proj/board.edn");
    await h.presenter.setConvention("proj/house.yaml");
    await h.presenter.createReview();
    const sent = h.createReview.mock.calls[0][0] as Record<string, unknown>;
    expect(sent.overlay).toBeDefined();
  });

  it("sends no overlay while the server's convention applies", async () => {
    const h = convHarness();
    await h.presenter.openFile("m", "proj/board.edn");
    await h.presenter.createReview();
    const sent = h.createReview.mock.calls[0][0] as Record<string, unknown>;
    expect(sent.overlay).toBeUndefined();
  });

  it("resolves through the server rather than parsing yaml in the browser", async () => {
    const h = convHarness();
    await h.presenter.openFile("m", "proj/board.edn");
    await h.presenter.setConvention("proj/house.yaml");
    expect(h.getNamingConvention).toHaveBeenCalledWith({ uri: artifactUri("m", "proj/house.yaml") });
  });

  it("reports which vocabulary is in effect, and goes back to the server's", async () => {
    const h = convHarness();
    await h.presenter.openFile("m", "proj/board.edn");
    await h.presenter.setConvention("proj/house.yaml");
    expect(lastConvention(h.onConvention).active).toBe("proj/house.yaml");
    expect(lastConvention(h.onConvention).name).toBe("house");
    await h.presenter.setConvention("");
    expect(lastConvention(h.onConvention).active).toBe("");
    expect(lastConvention(h.onConvention).name).toBe("");
  });

  // A failed resolve must leave the PREVIOUS vocabulary in effect and say so. Half-applying it would
  // put the indicator and the findings into different worlds, which is the exact confusion the
  // indicator exists to prevent.
  it("keeps the previous vocabulary when a resolve fails", async () => {
    const h = convHarness();
    await h.presenter.openFile("m", "proj/board.edn");
    h.getNamingConvention.mockRejectedValueOnce(new ConnectError("pattern does not compile", Code.InvalidArgument));
    await h.presenter.setConvention("proj/broken.yaml");
    const s = lastConvention(h.onConvention);
    expect(s.active).toBe("");
    expect(s.error).toContain("does not compile");
  });

  // Cached findings were computed under a different vocabulary, and a convention changes which rules
  // exist AND what the engine believes a rail is. Keeping them would show two vocabularies at once.
  it("drops cached findings so the two vocabularies never mix", async () => {
    const h = convHarness();
    await h.presenter.openFile("m", "proj/board.edn");
    await h.presenter.runChecks();
    const before = h.checkDesign.mock.calls.length;
    await h.presenter.setConvention("proj/house.yaml");
    await h.presenter.runChecks();
    expect(h.checkDesign.mock.calls.length).toBeGreaterThan(before);
    const last = h.checkDesign.mock.calls.at(-1)![0] as Record<string, unknown>;
    expect(last.overlay).toBeDefined();
  });

  // Regression, found by driving the real app: switching the vocabulary changes which rules EXIST,
  // because a request convention replaces the server's. Keeping the old catalog left a selection
  // naming rules that no longer existed, so none of them ran and none of the new ones did either —
  // no naming findings at all, which reads as a design with no naming problems.
  it("re-reads the rule catalog under the new vocabulary, not just the findings", async () => {
    const h = convHarness();
    await h.presenter.openFile("m", "proj/board.edn");
    const before = h.listRules.mock.calls.length;
    await h.presenter.setConvention("proj/house.yaml");
    expect(h.listRules.mock.calls.length).toBeGreaterThan(before);
    const last = h.listRules.mock.calls.at(-1)![0] as Record<string, unknown>;
    expect(last.overlay, "the catalog must be listed under the SAME overlay a run would use").toBeDefined();
  });

  it("re-reads the catalog when going back to the server's vocabulary too", async () => {
    const h = convHarness();
    await h.presenter.openFile("m", "proj/board.edn");
    await h.presenter.setConvention("proj/house.yaml");
    const before = h.listRules.mock.calls.length;
    await h.presenter.setConvention("");
    expect(h.listRules.mock.calls.length).toBeGreaterThan(before);
    expect((h.listRules.mock.calls.at(-1)![0] as Record<string, unknown>).overlay).toBeUndefined();
  });

  // The Query panel must answer under the SAME vocabulary the bar names. Before this, the bar could
  // say "acme" while query answered under the server's default — the exact over-claim the bar exists
  // to prevent, and the one surface whose entire purpose is asking what the engine believes.
  it("carries the vocabulary on an ad-hoc query", async () => {
    const h = convHarness();
    await h.presenter.openFile("m", "proj/board.edn");
    await h.presenter.setConvention("proj/house.yaml");
    await h.presenter.runQuery("rail(?n) => ?n");
    const calls = h.runQuery.mock.calls;
    const sent = calls[calls.length - 1][0] as Record<string, unknown>;
    expect(sent.overlay, "the query panel would otherwise answer under a different vocabulary than the bar names").toBeDefined();
  });

  it("sends no overlay on a query while the server's vocabulary applies", async () => {
    const h = convHarness();
    await h.presenter.openFile("m", "proj/board.edn");
    await h.presenter.runQuery("rail(?n) => ?n");
    const qcalls = h.runQuery.mock.calls;
    expect((qcalls[qcalls.length - 1][0] as Record<string, unknown>).overlay).toBeUndefined();
  });

  it("offers the yaml files beside the design as choices", async () => {
    const h = convHarness();
    await h.presenter.openFile("m", "proj/board.edn");
    expect(lastConvention(h.onConvention).choices.map((c) => c.label)).toEqual(["review.yaml"]);
  });

  it("does nothing when the bar is unwired", async () => {
    const h = harness({ wireConventionBar: false });
    await h.presenter.openFile("m", "proj/board.edn");
    await h.presenter.setConvention("proj/house.yaml");
    expect(h.getNamingConvention).not.toHaveBeenCalled();
  });
});
