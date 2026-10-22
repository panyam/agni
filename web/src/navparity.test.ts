import { describe, expect, it, vi } from "vitest";
import { Outcome } from "./gen/agni/v1/checks/checks_pb.js";
import { SheetFormat } from "./gen/agni/v1/webapi/design_pb.js";
import type { HighlightSpec } from "./highlights.js";
import { pageApi } from "./pageapi.js";
import { emptyLocation, type ViewerLocation } from "./router.js";
import { fakeSnapshots } from "./testsnaps.js";
import { stubQueryView } from "./testviews.js";
import { ViewerPresenter, type RenderView } from "./viewer.js";

// Navigation parity (agni issue 1151). A verdict or finding can be reached three ways: a link (a
// cold restore from the URL), a click in the Checks panel, and a ViewerPageAPI call. All three must
// land on the same sheet with the subject highlighted, or a link someone shares opens on a sheet the
// reader was never shown.
//
// The synthetic demo-board has two sheets. SDA is drawn on both, so reaching it from s-root moves
// nowhere. PG and SCL are drawn on s-power alone, so reaching either from s-root has to move. PG
// fails demo-pull-up and has a finding; SCL passes and has only a verdict, which is the case a
// verdict's own sheets (Verdict.sheets) exist for.

const MOUNT = "demo";
const PATH = "boards/demo-board.kicad_sch";
const RULE = "demo-pull-up";
const PG = `${RULE}:(net:PG)`;
const SCL = `${RULE}:(net:SCL)`;
const SDA = `${RULE}:(net:SDA)`;

const verdicts = [
  { id: PG, rule: RULE, outcome: Outcome.FAIL, subjects: [{ kind: "net", ref: "PG" }], witness: { statement: "PG reaches no rail" }, sheets: ["s-power"] },
  { id: SCL, rule: RULE, outcome: Outcome.PASS, subjects: [{ kind: "net", ref: "SCL" }], witness: { statement: "SCL reaches +3V3 through R2" }, sheets: ["s-power"] },
  { id: SDA, rule: RULE, outcome: Outcome.PASS, subjects: [{ kind: "net", ref: "SDA" }], witness: { statement: "SDA reaches +3V3 through R1" }, sheets: ["s-root", "s-power"] },
];
const findings = [{ rule: RULE, severity: "error", subject: { kind: "net", ref: "PG", pin: "" }, message: "PG has no pull-up", sheets: ["s-power"], verdictIds: [PG] }];

function harness() {
  const getDesign = vi.fn(async () => ({
    name: "demo-board",
    layout: "faithful",
    sourceFormat: "kicad",
    componentCount: 4,
    netCount: 3,
    sheets: [
      { id: "s-root", name: "Root" },
      { id: "s-power", name: "Power" },
    ],
    nativeAvailable: false,
    availableLayouts: ["faithful"],
  }));
  const getSheet = vi.fn(async (req: { sheet?: string; format?: SheetFormat }) => ({ content: { case: "svg", value: `<svg data-sheet="${req.sheet}"/>` } }));
  // A scoped request (one rule about some subjects, as a link asks) answers those subjects alone; a
  // whole run answers everything.
  const checkDesign = vi.fn(async (req: { rules?: string[]; subjects?: { kind: string; ref: string }[] }) => {
    const refs = (req.subjects ?? []).map((s) => s.ref);
    const scoped = refs.length > 0;
    return {
      findings: scoped ? findings.filter((f) => refs.includes(f.subject.ref)) : findings,
      verdicts: scoped ? verdicts.filter((v) => refs.includes(v.subjects[0].ref)) : verdicts,
    };
  });
  const listRules = vi.fn(async () => ({
    rules: [{ name: RULE, severity: "error", summary: "pull-up", reads: [], tags: {}, available: true, unavailableReason: "" }],
  }));
  const getExpectations = vi.fn(async () => ({ expectations: [], hasSidecar: false }));
  const client = { getDesign, getSheet, getLayoutReport: vi.fn(), highlightSheet: vi.fn(async () => ({ content: { case: "svg", value: "" } })) } as any;
  const checks = { checkDesign, listRules, getExpectations } as any;
  const canvas = { showSheet: vi.fn(), setHighlights: vi.fn() } as any;
  const render: RenderView = { showWebgl: vi.fn(), showSvg: vi.fn(), setSvgOverlay: vi.fn(), setBusy: vi.fn(), getView: vi.fn(), setView: vi.fn(), setBoardLayers: vi.fn() };
  const onFindings = vi.fn();
  const onLocation = vi.fn();
  const presenter = new ViewerPresenter(
    client,
    checks,
    canvas,
    render,
    {
      sheetNavs: [{ setState: vi.fn() }],
      summary: vi.fn(),
      controls: { setState: vi.fn() },
      findings: { setState: onFindings, setFindingLocateNote: () => {} },
      expectationCaption: vi.fn(),
      undrawnNote: vi.fn(),
      staleLinkNote: vi.fn(),
      hierarchyNote: vi.fn(),
      rules: { setState: vi.fn() },
      report: vi.fn(),
      location: onLocation,
      overview: { setState: vi.fn() },
      query: stubQueryView(),
    },
    undefined,
    undefined,
    undefined,
    undefined,
    fakeSnapshots(),
  );
  return { presenter, api: pageApi(presenter), getSheet, checkDesign, canvas, onFindings, onLocation };
}

type H = ReturnType<typeof harness>;

// last is a mock's last call (or any list's last element); mock calls are loosely typed here, as the
// collaborators are.
function last(xs: readonly any[]): any {
  return xs[xs.length - 1];
}

// Landing is where a path left the page: the sheet on screen, the last sheet fetched, the nets the
// canvas lights, and the address the host was last handed.
interface Landing {
  sheet: string;
  fetched: string;
  nets: string[];
  url: ViewerLocation;
}

function landing(h: H): Landing {
  const sheets = h.getSheet.mock.calls;
  const specs: HighlightSpec[] = last(h.canvas.setHighlights.mock.calls)?.[0] ?? [];
  return {
    sheet: h.presenter.location().sheet,
    fetched: last(sheets)?.[0].sheet ?? "",
    nets: [...new Set(specs.flatMap((s) => s.nets ?? []))].sort(),
    url: last(h.onLocation.mock.calls)?.[0],
  };
}

const linkTo = (verdict: string, ref: string): ViewerLocation => ({ ...emptyLocation(), mount: MOUNT, path: PATH, sheet: "s-root", verdict, rule: RULE, subjects: [`net:${ref}`] });

async function opened(): Promise<H> {
  const h = harness();
  await h.presenter.openFile(MOUNT, PATH);
  return h;
}

async function ran(): Promise<H> {
  const h = await opened();
  await h.presenter.runChecks();
  return h;
}

// expectSameLanding compares every path's landing with the first, naming the path that diverged.
function expectSameLanding(paths: Record<string, Landing>, want: { sheet: string; net: string }) {
  for (const [name, l] of Object.entries(paths)) {
    expect(l.sheet, `${name}: sheet on screen`).toBe(want.sheet);
    expect(l.fetched, `${name}: last sheet fetched`).toBe(want.sheet);
    expect(l.nets, `${name}: highlighted nets`).toContain(want.net);
    expect(l.url.sheet, `${name}: sheet in the address`).toBe(want.sheet);
  }
}

describe("navigation parity: link, click and page call", () => {
  it("a failing verdict with a finding lands on its sheet every way", async () => {
    const link = harness();
    await link.presenter.restore(linkTo(PG, "PG"));

    const click = await ran();
    await click.presenter.selectFinding("PG");

    const verdictClick = await ran();
    await verdictClick.presenter.locateVerdict(PG);

    const call = await ran();
    expect(await call.api.locateVerdict(PG)).toBe(true);

    const select = await ran();
    expect(await select.api.selectFinding("PG")).toBe(true);

    const apply = await opened();
    expect((await apply.api.applyLocation({ verdict: PG, rule: RULE, subjects: ["net:PG"] })).verdictResolved).toBe(true);

    expectSameLanding(
      {
        "link (restore)": landing(link),
        "Checks row click (selectFinding)": landing(click),
        "verdict click (locateVerdict)": landing(verdictClick),
        "page call locateVerdict": landing(call),
        "page call selectFinding": landing(select),
        "page call applyLocation": landing(apply),
      },
      { sheet: "s-power", net: "PG" },
    );
    // The verdict paths draw one proof, so they agree on the whole highlight and the address.
    for (const [name, h] of Object.entries({ "verdict click": verdictClick, "page call": call, "applyLocation": apply })) {
      expect(last(h.canvas.setHighlights.mock.calls)?.[0], `${name} vs link: highlight`).toEqual(last(link.canvas.setHighlights.mock.calls)?.[0]);
      expect(landing(h).url.verdict, `${name}: verdict in the address`).toBe(PG);
    }
    expect(landing(link).url.verdict).toBe(PG);
    expect(last(click.onFindings.mock.calls)?.[0].selected, "Checks row click: selected finding").toBe("PG");
    expect(last(select.onFindings.mock.calls)?.[0].selected, "page call selectFinding: selected finding").toBe("PG");
  });

  it("a passing verdict, which has no finding, lands on its sheet every way", async () => {
    const link = harness();
    await link.presenter.restore(linkTo(SCL, "SCL"));

    const verdictClick = await ran();
    await verdictClick.presenter.locateVerdict(SCL);

    const call = await opened();
    expect(await call.api.locateVerdict(SCL, { rule: RULE, subjects: ["net:SCL"] })).toBe(true);

    expectSameLanding(
      { "link (restore)": landing(link), "verdict click (locateVerdict)": landing(verdictClick), "page call locateVerdict": landing(call) },
      { sheet: "s-power", net: "SCL" },
    );
  });

  it("a link asks one rule about the link's subjects before it draws", async () => {
    const link = harness();
    await link.presenter.restore(linkTo(SCL, "SCL"));
    expect(link.checkDesign).toHaveBeenCalledTimes(1);
    expect(link.checkDesign.mock.calls[0][0]).toMatchObject({ rules: [RULE], subjects: [{ kind: "net", ref: "SCL" }] });

    const apply = await opened();
    await apply.api.applyLocation({ verdict: SCL, rule: RULE, subjects: ["net:SCL"] });
    expect(apply.checkDesign.mock.calls[0][0]).toMatchObject({ rules: [RULE], subjects: [{ kind: "net", ref: "SCL" }] });
  });

  it("a subject on the sheet already shown moves nowhere, every way", async () => {
    const link = harness();
    await link.presenter.restore(linkTo(SDA, "SDA"));
    const verdictClick = await ran();
    await verdictClick.presenter.locateVerdict(SDA);
    const call = await ran();
    await call.api.locateVerdict(SDA);
    expectSameLanding(
      { "link (restore)": landing(link), "verdict click (locateVerdict)": landing(verdictClick), "page call locateVerdict": landing(call) },
      { sheet: "s-root", net: "SDA" },
    );
    expect(call.getSheet.mock.calls.map((c) => c[0].sheet)).toEqual(["s-root"]);
  });

  it("an explicit sheet wins, as a badge click does, through the page call too", async () => {
    const click = await ran();
    await click.presenter.selectFinding("PG", "s-root");
    const call = await ran();
    await call.api.selectFinding("PG", { sheet: "s-root" });
    for (const [name, h] of Object.entries({ "badge click": click, "page call": call })) {
      expect(landing(h).sheet, name).toBe("s-root");
      expect(last(h.onFindings.mock.calls)?.[0].selected, name).toBe("PG");
    }
  });
});

describe("ViewerPageAPI leaves the page alone on what it cannot act on", () => {
  it("an unknown verdict resolves false and draws nothing", async () => {
    const h = await ran();
    const before = h.canvas.setHighlights.mock.calls.length;
    expect(await h.api.locateVerdict(`${RULE}:(net:NOPE)`)).toBe(false);
    expect(h.canvas.setHighlights.mock.calls.length).toBe(before);
    expect(h.presenter.location().sheet).toBe("s-root");
  });

  it("an unknown finding resolves false, and selecting the focused one twice keeps it", async () => {
    const h = await ran();
    expect(await h.api.selectFinding("NOPE")).toBe(false);
    expect(await h.api.selectFinding("PG")).toBe(true);
    expect(await h.api.selectFinding("PG")).toBe(true);
    expect(last(h.onFindings.mock.calls)?.[0].selected).toBe("PG");
  });

  it("a sheet the design lacks is refused", async () => {
    const h = await opened();
    await expect(h.api.showSheet("s-nope")).rejects.toThrow(/no sheet "s-nope"/);
    await expect(h.api.applyLocation({ sheet: "s-nope" })).rejects.toThrow(/no sheet "s-nope"/);
    expect(h.presenter.location().sheet).toBe("s-root");
  });

  it("applyLocation with nothing open and no design named is refused", async () => {
    await expect(harness().api.applyLocation({ sheet: "s-root" })).rejects.toThrow(/no design is open/);
  });

  it("applying the location the page is at fetches nothing", async () => {
    const h = await opened();
    const fetched = h.getSheet.mock.calls.length;
    await h.api.applyLocation(h.api.location());
    expect(h.getSheet.mock.calls.length).toBe(fetched);
  });
});
