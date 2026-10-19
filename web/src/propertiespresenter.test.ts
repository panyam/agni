import { describe, it, expect, vi } from "vitest";
import { create } from "@bufbuild/protobuf";
import { GetDesignResponseSchema, type GetDesignResponse } from "./gen/agni/v1/webapi/design_pb.js";
import { INVENTORY_PATHS, SERVED_QUERIES, type PropertiesState } from "./properties.js";
import { PropertiesPresenter } from "./propertiespresenter.js";
import type { VerdictItem } from "./findings.js";
import type { SubjectRef } from "./router.js";

function design(name: string, nets: string[]): GetDesignResponse {
  return create(GetDesignResponseSchema, { name, design: { nets: nets.map((n) => ({ name: n })) } });
}

function harness(getDesign = vi.fn(async (_uri: string, _readMask: string[]) => design("gateway", ["VIN"]))) {
  const states: PropertiesState[] = [];
  const p = new PropertiesPresenter({ setState: (s) => states.push(s) }, { getDesign, drawn: () => ({ refs: ["U1"], nets: [] }) });
  const last = () => states[states.length - 1];
  return { p, getDesign, last };
}

const sheets = [
  { id: "s1", name: "Power" },
  { id: "s2", name: "IO" },
];

describe("PropertiesPresenter", () => {
  it("reads the design's inventory once per open, under the inventory mask", async () => {
    const { p, getDesign, last } = harness();
    p.setSheets({ mount: "m", path: "d", sheets, activeId: "s1" });
    p.setSheets({ mount: "m", path: "d", sheets, activeId: "s2" });
    await vi.waitFor(() => expect(last().crumbs[0].label).toBe("gateway"));
    expect(getDesign).toHaveBeenCalledTimes(1);
    expect(getDesign.mock.calls[0]).toEqual(["mount://m/d", ["name", ...INVENTORY_PATHS]]);
  });

  // agni issue 1045: a net's test points and a pin's name are the engine's answers, asked once per open.
  it("asks the engine for pin names and test points once per open, and shows its answers", async () => {
    const states: PropertiesState[] = [];
    const getDesign = vi.fn(async () =>
      create(GetDesignResponseSchema, { name: "g", design: { nets: [{ name: "3V3", connections: [{ componentRef: "TP1", pinRef: "1" }, { componentRef: "U1", pinRef: "10" }] }] } }),
    );
    const ask = vi.fn(async (_uri: string, q: string) => (q === SERVED_QUERIES.testPoints ? [["TP1"]] : [["U1", "10", "OUT"]]));
    const p = new PropertiesPresenter({ setState: (s) => states.push(s) }, { getDesign, drawn: () => null, ask });
    p.setSheets({ mount: "m", path: "d", sheets, activeId: "s1" });
    p.setSheets({ mount: "m", path: "d", sheets, activeId: "s2" });
    p.selectEntity({ kind: "net", net: "3V3" });
    const last = () => states[states.length - 1];
    await vi.waitFor(() => expect(last().groups.find((g) => g.name === "test points")?.refs.map((r) => r.label)).toEqual(["TP1"]));
    expect(last().groups.find((g) => g.name === "pins")?.refs.find((r) => r.label === "U1.10")?.detail).toBe("OUT");
    expect(ask.mock.calls.map((c) => c[1]).sort()).toEqual([SERVED_QUERIES.pinNames, SERVED_QUERIES.testPoints].sort());
  });

  it("selects the sheet on screen for a click on empty paper, and follows it to the next sheet", () => {
    const { p, last } = harness();
    p.setSheets({ mount: "m", path: "d", sheets, activeId: "s1" });
    p.selectSheet();
    expect(last().crumbs.map((c) => c.label)).toEqual(["d", "Power"]);
    expect(last().groups.find((g) => g.name === "components")?.refs.map((r) => r.label)).toEqual(["U1"]);
    p.setSheets({ mount: "m", path: "d", sheets, activeId: "s2" });
    expect(last().crumbs.map((c) => c.label)).toEqual(["d", "IO"]);
  });

  it("keeps an entity selected across a sheet change, since it names a thing rather than a place", () => {
    const { p, last } = harness();
    p.setSheets({ mount: "m", path: "d", sheets, activeId: "s1" });
    p.selectEntity({ kind: "net", net: "VIN" });
    p.setSheets({ mount: "m", path: "d", sheets, activeId: "s2" });
    expect(last().title).toBe("VIN");
  });

  it("returns to the design when another design opens, and drops a slow answer for the old one", async () => {
    let release: (r: GetDesignResponse) => void = () => {};
    const slow = new Promise<GetDesignResponse>((r) => (release = r));
    const getDesign = vi.fn().mockReturnValueOnce(slow).mockResolvedValueOnce(design("second", ["B"]));
    const { p, last } = harness(getDesign);
    p.setSheets({ mount: "m", path: "first", sheets, activeId: "s1" });
    p.selectEntity({ kind: "net", net: "VIN" });
    p.setSheets({ mount: "m", path: "second", sheets, activeId: "s1" });
    expect(last().kind).toBe("design");
    await vi.waitFor(() => expect(last().crumbs[0].label).toBe("second"));
    release(design("first", ["A"]));
    await Promise.resolve();
    expect(last().crumbs[0].label).toBe("second");
    expect(last().rows.find((r) => r.label === "nets")?.value).toBe("1");
  });

  it("asks where a selected part is drawn, and drops an answer for a part no longer selected", async () => {
    let releaseU1: (ids: string[]) => void = () => {};
    const drawnOn = vi
      .fn()
      .mockReturnValueOnce(new Promise<string[]>((r) => (releaseU1 = r)))
      .mockResolvedValueOnce(["s2"]);
    const states: PropertiesState[] = [];
    const p = new PropertiesPresenter({ setState: (s) => states.push(s) }, { getDesign: async () => design("d", []), drawn: () => null, drawnOn });
    const sheetsGroup = () => states[states.length - 1].groups.find((g) => g.name === "sheets");
    p.setSheets({ mount: "m", path: "d", sheets, activeId: "s1" });
    p.selectEntity({ kind: "component", ref: "U1" });
    expect(sheetsGroup()?.empty).toBe("locating…");
    p.selectEntity({ kind: "net", net: "VIN" });
    await vi.waitFor(() => expect(sheetsGroup()?.refs).toEqual([{ label: "IO", sheet: "s2" }]));
    releaseU1(["s1"]);
    await Promise.resolve();
    expect(states[states.length - 1].title).toBe("VIN");
    expect(sheetsGroup()?.refs).toEqual([{ label: "IO", sheet: "s2" }]);
    expect(drawnOn.mock.calls.map((c) => c[1])).toEqual([{ kind: "component", ref: "U1" }, { kind: "net", net: "VIN" }]);
    expect(drawnOn.mock.calls[0][0]).toBe("mount://m/d");
  });

  it("offers the served questions it was handed for the selection's kind", () => {
    const { p, last } = harness();
    p.setSheets({ mount: "m", path: "d", sheets, activeId: "s1" });
    p.setQuestions([{ kind: "net", label: "What pins are on it?", query: "q", teaches: "", binds: ["net"] }]);
    p.selectEntity({ kind: "net", net: "VIN" });
    expect(last().ask?.questions.map((q) => q.label)).toEqual(["What pins are on it?"]);
  });

  it("selects the design on Escape", () => {
    const { p, last } = harness();
    p.setSheets({ mount: "m", path: "d", sheets, activeId: "s1" });
    p.selectEntity({ kind: "component", ref: "U1" });
    p.selectDesign();
    expect(last().kind).toBe("design");
  });
});

// agni issue 897: selecting a net, part or pin asks the selected rules about it alone.
describe("a selection's own checks", () => {
  function checking(check: (s: SubjectRef) => Promise<{ verdicts: VerdictItem[]; notAsked: { name: string; reason: string }[] } | null>, key = () => "k1") {
    const states: PropertiesState[] = [];
    const p = new PropertiesPresenter({ setState: (s) => states.push(s) }, { getDesign: async () => design("g", ["VIN"]), drawn: () => null, check, checkKey: key });
    p.setSheets({ mount: "m", path: "d", sheets, activeId: "s1" });
    return { p, last: () => states[states.length - 1] };
  }
  const answer = (rule: string) => ({ verdicts: [{ id: `${rule}:(net:VIN)`, rule, outcome: "fail", subjects: [], statement: "s", terms: [], context: [], reason: "" }], notAsked: [] });
  const checksRow = (s: PropertiesState) => s.rows.find((r) => r.label === "checks")?.value;

  it("asks about the selection on selecting it, saying so while it waits", async () => {
    let resolve!: (v: ReturnType<typeof answer>) => void;
    const check = vi.fn((_s: SubjectRef) => new Promise<ReturnType<typeof answer>>((r) => (resolve = r)));
    const { p, last } = checking(check);
    p.selectEntity({ kind: "pin", ref: "U1", pin: "3" });
    expect(check).toHaveBeenCalledWith({ kind: "pin", ref: "U1", pin: "3" });
    expect(checksRow(last())).toBe("checking…");
    resolve(answer("r"));
    await vi.waitFor(() => expect(checksRow(last())).toBe("1 fail"));
  });

  it("drops an answer for a selection the reader has left", async () => {
    const pending: ((v: ReturnType<typeof answer>) => void)[] = [];
    const check = vi.fn((_s: SubjectRef) => new Promise<ReturnType<typeof answer>>((r) => pending.push(r)));
    const { p, last } = checking(check);
    p.selectEntity({ kind: "net", net: "VIN" });
    p.selectEntity({ kind: "component", ref: "U1" });
    pending[0](answer("about-vin"));
    await new Promise((r) => setTimeout(r, 0));
    expect(checksRow(last())).toBe("checking…");
    pending[1](answer("about-u1"));
    await vi.waitFor(() => expect(last().groups.find((g) => g.name === "checks")?.refs.map((r) => r.label)).toEqual(["about-u1"]));
  });

  it("asks again when the rules it was asked under change, and not otherwise", async () => {
    let key = "k1";
    const check = vi.fn(async (_s: SubjectRef) => answer("r"));
    const { p } = checking(check, () => key);
    p.selectEntity({ kind: "net", net: "VIN" });
    await vi.waitFor(() => expect(check).toHaveBeenCalledTimes(1));
    const ran = { findings: [], verdicts: [], focusedVerdict: "", selected: "", ruleCount: 1, pending: 0, running: false, catalogLoading: false, skipped: [], ruleSummaries: {}, saveNote: "" };
    p.setFindings(ran);
    p.select({ level: "entity", sel: { kind: "net", net: "VIN" } });
    expect(check).toHaveBeenCalledTimes(1);
    key = "k2";
    p.setFindings(ran);
    await vi.waitFor(() => expect(check).toHaveBeenCalledTimes(2));
  });

  it("asks nothing about a bus, which no scope covers, and lists the run's findings instead", () => {
    const check = vi.fn(async (_s: SubjectRef) => answer("r"));
    const { p, last } = checking(check);
    p.selectEntity({ kind: "bus", busId: "D[7:0]" });
    expect(check).not.toHaveBeenCalled();
    expect(last().groups.find((g) => g.name === "findings")).toBeDefined();
  });

  it("says no rule is selected, and says why it could not ask", async () => {
    const { p, last } = checking(async () => null);
    p.selectEntity({ kind: "net", net: "VIN" });
    await vi.waitFor(() => expect(checksRow(last())).toBe("no rules selected"));
    const failing = checking(async () => {
      throw new Error("no netlist");
    });
    failing.p.selectEntity({ kind: "net", net: "VIN" });
    await vi.waitFor(() => expect(checksRow(failing.last())).toBe("could not check: no netlist"));
  });
});

// agni issue 918: a cancelled run ends the worker a selection's check was queued on, so a failed
// answer is asked again when a run ends, and not on every other push.
describe("a selection's checks after a cancelled run", () => {
  it("asks a failed selection again when a run ends, and only then", async () => {
    let fail = true;
    const check = vi.fn(async (_s: SubjectRef) => {
      if (fail) throw new Error("stopped");
      return { verdicts: [], notAsked: [] };
    });
    const states: PropertiesState[] = [];
    const p = new PropertiesPresenter({ setState: (s) => states.push(s) }, { getDesign: async () => design("g", ["VIN"]), drawn: () => null, check, checkKey: () => "k" });
    p.setSheets({ mount: "m", path: "d", sheets, activeId: "s1" });
    p.selectEntity({ kind: "net", net: "VIN" });
    await vi.waitFor(() => expect(states[states.length - 1].rows.find((r) => r.label === "checks")?.value).toBe("could not check: stopped"));
    const base = { findings: [], verdicts: [], focusedVerdict: "", selected: "", ruleCount: 1, pending: 1, catalogLoading: false, skipped: [], ruleSummaries: {}, saveNote: "" };
    p.setFindings({ ...base, running: false });
    expect(check).toHaveBeenCalledTimes(1);
    p.setFindings({ ...base, running: true });
    fail = false;
    p.setFindings({ ...base, running: false, cancelled: true });
    await vi.waitFor(() => expect(check).toHaveBeenCalledTimes(2));
    await vi.waitFor(() => expect(states[states.length - 1].rows.find((r) => r.label === "checks")?.value).toBe("no selected rule considers it"));
  });
});
