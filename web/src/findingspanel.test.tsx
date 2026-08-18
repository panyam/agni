// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach } from "vitest";
import { findingsPanelIsland } from "./findingspanel.jsx";
import type { FindingItem, FindingsState } from "./findings.js";

function f(over: Partial<FindingItem>): FindingItem {
  return { rule: "r", category: "c", profile: "", severity: "info", kind: "net", subject: "N", pin: "", netId: "", busId: "", message: "m", sheets: [], locateReason: 0, ...over };
}

function state(over: Partial<FindingsState>): FindingsState {
  return { findings: [], selected: "", ruleCount: 1, pending: 0, running: false, skipped: [], ruleSummaries: {}, ...over };
}

function mountPanel(over: Partial<FindingsState>) {
  const onSelect = vi.fn();
  const onRun = vi.fn();
  const el = document.createElement("div");
  document.body.appendChild(el);
  const panel = findingsPanelIsland(el, null, { onSelect, onRun });
  panel.island.activate();
  panel.view.setState(state(over));
  return { el, panel, onSelect, onRun };
}

beforeEach(() => document.body.replaceChildren());

describe("checks panel sheet badges (WS9-024)", () => {
  const spanning = f({ subject: "SDA", sheets: [{ id: "s1", name: "Root" }, { id: "s2", name: "Power" }] });

  it("shows one named badge per sheet the subject appears on; none when the item has no sheets", () => {
    const { el } = mountPanel({ findings: [spanning, f({ subject: "STUB" })] });
    const badges = [...el.querySelectorAll(".sheet-badge")].map((b) => b.textContent);
    expect(badges).toEqual(["Root", "Power"]);
  });

  it("clicking a badge emits onSelect with the subject and that sheet id, not a plain row click", () => {
    const { el, onSelect } = mountPanel({ findings: [spanning] });
    const badge = [...el.querySelectorAll<HTMLElement>(".sheet-badge")].find((b) => b.textContent === "Power");
    badge!.click();
    expect(onSelect).toHaveBeenCalledOnce();
    expect(onSelect).toHaveBeenCalledWith("SDA", "s2", "");
  });

  // A net on 21 sheets used to render 21 chips, which made the row five lines tall here and made
  // the query table bleed across its columns. The strip now shows the first few and counts the rest.
  it("shows the first three sheets and a count of the rest, expanding on demand", () => {
    const many = Array.from({ length: 21 }, (_, i) => ({ id: `s${i}`, name: `S${i}` }));
    const { el, onSelect } = mountPanel({ findings: [f({ subject: "DGND", sheets: many })] });
    const labels = () => [...el.querySelectorAll(".sheet-badge")].map((b) => b.textContent);
    expect(labels()).toEqual(["S0", "S1", "S2", "+18"]);

    el.querySelector<HTMLElement>(".sheet-badge-more")!.click();
    expect(labels()).toHaveLength(22); // all 21, plus the collapse chip
    expect(labels()[21]).toBe("\u2212");

    // A revealed badge still navigates to its own sheet.
    [...el.querySelectorAll<HTMLElement>(".sheet-badge")].find((b) => b.textContent === "S20")!.click();
    expect(onSelect).toHaveBeenCalledWith("DGND", "s20", "");
  });

  it("a subject click emits onSelect with the subject only (the presenter picks the sheet)", () => {
    const { el, onSelect } = mountPanel({ findings: [spanning] });
    el.querySelector<HTMLButtonElement>("button.check-locate")!.click();
    expect(onSelect).toHaveBeenCalledOnce();
    expect(onSelect.mock.calls[0][0]).toBe("SDA");
    expect(onSelect.mock.calls[0][1]).toBeUndefined();
  });
});

describe("checks panel Run button (on-demand checks)", () => {
  it("badges the pending rule count and emits onRun when pressed", () => {
    const { el, onRun } = mountPanel({ findings: [], pending: 3 });
    const run = el.querySelector<HTMLButtonElement>("button.checks-run")!;
    expect(run.textContent).toContain("Run checks (3)");
    run.click();
    expect(onRun).toHaveBeenCalledOnce();
  });

  it("disables the button and reads Running while a run is in flight", () => {
    const { el } = mountPanel({ findings: [], running: true });
    const run = el.querySelector<HTMLButtonElement>("button.checks-run")!;
    expect(run.disabled).toBe(true);
    expect(run.textContent).toContain("Running");
  });
});

describe("checks panel empty states", () => {
  it("distinguishes no-rules, not-yet-run, and ran-clean", () => {
    const none = mountPanel({ findings: [], ruleCount: 0 });
    expect(none.el.querySelector(".findings-empty")!.textContent).toBe("No rules selected.");

    const pending = mountPanel({ findings: [], ruleCount: 5, pending: 5 });
    expect(pending.el.querySelector(".findings-empty")!.textContent).toContain("Press Run checks to evaluate 5 rules");

    const clean = mountPanel({ findings: [], ruleCount: 5, pending: 0 });
    expect(clean.el.querySelector(".findings-empty")!.textContent).toBe("No findings.");
  });
});

describe("checks panel collapses repeated findings", () => {
  // 7 findings identical in every display field (the &1V8_ETH duplicate-net-name case) render as ONE
  // row with a ×7 expander instead of 7 identical rows.
  const dup = Array.from({ length: 7 }, () =>
    f({ rule: "duplicate-net-name", subject: "&1V8_ETH", severity: "warning", message: "this name is stated by 7 electrically distinct nets" }),
  );

  it("shows one row with a ×N count for N identical findings", () => {
    const { el } = mountPanel({ findings: dup });
    const rows = el.querySelectorAll("tr.check-row");
    expect(rows.length).toBe(1);
    expect(el.querySelector(".check-count")!.textContent).toBe("×7");
    expect(el.querySelectorAll("tr.check-inst").length).toBe(0); // collapsed by default
  });

  it("expands to one sub-row per instance when the ×N control is clicked", () => {
    const { el } = mountPanel({ findings: dup });
    el.querySelector<HTMLButtonElement>("button.check-exp-btn")!.click();
    expect(el.querySelectorAll("tr.check-inst").length).toBe(7);
  });

  it("does not collapse findings that differ in subject", () => {
    const { el } = mountPanel({ findings: [f({ subject: "A" }), f({ subject: "B" })] });
    expect(el.querySelectorAll("tr.check-row").length).toBe(2);
    expect(el.querySelector(".check-count")).toBeNull();
  });
});

describe("checks panel locate note (WS7-042c)", () => {
  it("shows the server-authoritative note pushed by the presenter, and clears it", () => {
    const { el, panel } = mountPanel({ ruleCount: 1 });
    expect(el.querySelector(".checks-locate-note")).toBeNull();
    panel.view.setFindingLocateNote("DATA is a bus with no drawn wire.");
    expect(el.querySelector(".checks-locate-note")!.textContent).toContain("no drawn wire");
    panel.view.setFindingLocateNote("");
    expect(el.querySelector(".checks-locate-note")).toBeNull();
  });
})

// A rule that could not run is the difference between "this board is clean" and "nobody asked". The
// panel is the viewer's default-open surface, so an empty findings list is the first thing most
// people see and the last thing they would think to doubt.
it("says which selected rules could not run, and why", () => {
  const { el } = mountPanel({
    findings: [],
    skipped: [{ rule: "track-width", reason: "design carries no board geometry" }],
  });
  const text = el.textContent ?? "";
  expect(text).toContain("could not run");
  expect(text).toContain("track-width");
  expect(text).toContain("design carries no board geometry");
  // And it still says there were no findings: the two statements are both true and neither replaces
  // the other.
  expect(text).toContain("No findings.");
});

// Shown even when findings exist, because it qualifies them: two findings from ten rules, six of
// which never ran, is not the same claim as two findings from ten.
it("reports skipped rules alongside findings", () => {
  const { el } = mountPanel({
    findings: [f({ rule: "i2c-pull-up" })],
    skipped: [{ rule: "track-width", reason: "design carries no board geometry" }],
  });
  expect(el.textContent ?? "").toContain("could not run");
});
