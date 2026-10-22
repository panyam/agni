// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach } from "vitest";
import { findingsPanelIsland } from "./findingspanel.jsx";
import type { FindingItem, FindingsState, VerdictItem } from "./findings.js";

function f(over: Partial<FindingItem>): FindingItem {
  return { rule: "r", category: "c", profile: "", severity: "info", kind: "net", subject: "N", pin: "", netId: "", busId: "", message: "m", inconclusive: false, context: [], sheets: [], locateReason: 0, ...over };
}

function state(over: Partial<FindingsState>): FindingsState {
  return { findings: [], verdicts: [], focusedVerdict: "", selected: "", ruleCount: 1, pending: 0, running: false, catalogLoading: false, skipped: [], ruleSummaries: {}, saveNote: "", ...over };
}

function mountPanel(over: Partial<FindingsState>) {
  const onSelect = vi.fn();
  const onLocateContext = vi.fn();
  const onRun = vi.fn();
  const onSelectVerdict = vi.fn();
  const onSave = vi.fn();
  const el = document.createElement("div");
  document.body.appendChild(el);
  const panel = findingsPanelIsland(el, null, { onSelect, onLocateContext, onRun, onSelectVerdict, onSave });
  panel.island.activate();
  panel.view.setState(state(over));
  return { el, panel, onSelect, onLocateContext, onRun, onSelectVerdict, onSave };
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

  // A design's first sheet can draw before its catalog arrives, and a Run pressed then ran nothing
  // (agni 868). The empty state says the same thing as the button rather than "No rules selected.".
  it("disables the button and reads Loading rules until the catalog arrives", () => {
    const { el } = mountPanel({ findings: [], ruleCount: 0, catalogLoading: true });
    const run = el.querySelector<HTMLButtonElement>("button.checks-run")!;
    expect(run.disabled).toBe(true);
    expect(run.textContent).toBe("Loading rules…");
    expect(el.querySelector(".findings-empty")!.textContent).toBe("Loading rules…");
  });
});

// The panel mounts before the page opens a design, which waits on admission and the engine, and a
// Run pressed in that window found no design and did nothing, with the button still enabled.
it("a freshly mounted panel offers no Run until a design's catalog arrives", () => {
  const onRun = vi.fn();
  const el = document.createElement("div");
  document.body.appendChild(el);
  findingsPanelIsland(el, null, { onSelect: vi.fn(), onLocateContext: vi.fn(), onRun, onSelectVerdict: vi.fn() }).island.activate();
  const run = el.querySelector<HTMLButtonElement>("button.checks-run")!;
  expect(run.disabled).toBe(true);
  expect(run.textContent).toBe("Loading rules…");
  run.click();
  expect(onRun).not.toHaveBeenCalled();
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

  // agni issue 1132: the next action leads, and the rules that could not run qualify it below.
  it("puts Press Run checks above the rules that could not run", () => {
    const p = mountPanel({ findings: [], ruleCount: 5, pending: 3, skipped: [{ rule: "track-width", reason: "no board" }] });
    const press = p.el.querySelector(".findings-empty")!;
    const skipped = p.el.querySelector(".checks-skipped")!;
    expect(press.textContent).toContain("Press Run checks to evaluate 3 rules");
    expect(press.compareDocumentPosition(skipped) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
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
  // And it still says there were no findings, because the two statements are both true and neither
  // replaces the other.
  expect(text).toContain("No findings.");
});

// Shown even when findings exist, because it qualifies them. Two findings from ten rules, six of
// which never ran, is not the same claim as two findings from ten.
it("reports skipped rules alongside findings", () => {
  const { el } = mountPanel({
    findings: [f({ rule: "i2c-pull-up" })],
    skipped: [{ rule: "track-width", reason: "design carries no board geometry" }],
  });
  expect(el.textContent ?? "").toContain("could not run");
});

// The entities a finding's message NAMES but is not ABOUT (agni issue 349).
//
// crystal-load-caps is the case this was filed for. The finding reads "crystal terminal net XOUT1 has
// no load capacitor" and its subject is the crystal Y1, so clicking it sent the reader to a part the
// sentence never mentioned. The net was bound in the rule's query and thrown away when the finding was
// built, so no client-side cleverness could recover it, and the panel had nothing to render.
describe("context chips", () => {
  const withContext = (over: Partial<FindingItem> = {}) =>
    f({
      rule: "crystal-load-caps",
      kind: "component",
      subject: "Y1",
      message: "crystal terminal net XOUT1 has no load capacitor",
      context: [{ kind: "net", subject: "XOUT1", pin: "", role: "terminal" }],
      ...over,
    });

  it("renders a chip for each context entity, labelled with its role", () => {
    const m = mountPanel({ findings: [withContext()] });
    const chips = [...m.el.querySelectorAll(".check-context")];
    expect(chips).toHaveLength(1);
    expect(chips[0].textContent).toContain("XOUT1");
    expect(chips[0].textContent).toContain("terminal");
  });

  it("locates the context entity, not the finding's subject", () => {
    // This is the bug. Clicking must reach XOUT1, and it must go through the ENTITY locate path,
    // because a context entity is not a finding, so a lookup by finding subject would find nothing.
    const m = mountPanel({ findings: [withContext()] });
    (m.el.querySelector(".check-context") as HTMLButtonElement).click();
    expect(m.onLocateContext).toHaveBeenCalledWith("net", "XOUT1", "");
    expect(m.onSelect).not.toHaveBeenCalled();
  });

  it("renders chips in the rule author's order, matching the sentence", () => {
    const m = mountPanel({
      findings: [
        withContext({
          message: "U1 supplies net VRAIL at 5V, above U2's absolute maximum",
          context: [
            { kind: "component", subject: "U1", pin: "", role: "source" },
            { kind: "net", subject: "VRAIL", pin: "", role: "rail" },
          ],
        }),
      ],
    });
    const chips = [...m.el.querySelectorAll(".check-context")].map((c) => c.textContent ?? "");
    expect(chips[0]).toContain("U1");
    expect(chips[1]).toContain("VRAIL");
  });

  it("renders nothing for a finding whose message names only its subject", () => {
    // The common case and always will be. An empty context must render no chip rather than an empty
    // one, or every row in the panel grows a stub.
    const m = mountPanel({ findings: [f({ subject: "N1", context: [] })] });
    expect(m.el.querySelectorAll(".check-context")).toHaveLength(0);
  });
});

// Selecting a finding must not rebuild the table (agni issue 367).
//
// <For> keys by object reference and collapseSorted mints fresh objects per call, so a sections()
// that re-ran on every state push tore down and recreated every row. That resets the scroll
// container, which threw a reader clicking something halfway down a long list back to the top.
//
// Asserted on NODE IDENTITY rather than on scrollTop, because jsdom has no layout and every element
// reports a zero-sized box, so scrollTop is always 0 there and the assertion would pass either way.
// Node identity is the cause; the scroll is the symptom.
describe("selecting a finding keeps the rows it already rendered", () => {
  const many = Array.from({ length: 6 }, (_, i) => f({ subject: `N${i}`, rule: "single-pin-net" }));

  it("does not recreate the row elements when only the selection changes", () => {
    const m = mountPanel({ findings: many });
    const before = [...m.el.querySelectorAll(".check-row")];
    expect(before.length).toBe(6);

    m.panel.view.setState(state({ findings: many, selected: "N4" }));

    const after = [...m.el.querySelectorAll(".check-row")];
    expect(after.length).toBe(6);
    for (let i = 0; i < before.length; i++) {
      expect(after[i], `row ${i} was recreated, which resets the panel's scroll`).toBe(before[i]);
    }
  });

  it("still marks the newly selected row", () => {
    // The control. Keeping the DOM must not cost the selection styling, which is what a reader uses
    // to see which finding they are on.
    const m = mountPanel({ findings: many });
    m.panel.view.setState(state({ findings: many, selected: "N4" }));
    const sel = m.el.querySelector(".check-row.selected");
    expect(sel?.textContent).toContain("N4");
  });

  it("does rebuild when the findings themselves change", () => {
    // The other control. A memo that never invalidated would satisfy the first test perfectly and
    // leave the panel showing stale findings after a re-run.
    const m = mountPanel({ findings: many });
    const before = [...m.el.querySelectorAll(".check-row")];
    m.panel.view.setState(state({ findings: [f({ subject: "DIFFERENT" })] }));
    const after = [...m.el.querySelectorAll(".check-row")];
    expect(after.length).toBe(1);
    expect(after[0]).not.toBe(before[0]);
  });
});

function v(over: Partial<VerdictItem> = {}): VerdictItem {
  return {
    id: "i2c-pull-up:(net:SDA)", rule: "i2c-pull-up", outcome: "pass",
    subjects: [{ kind: "net", subject: "SDA", pin: "" }], statement: "SDA reaches rail +3V3 through R1", terms: [], context: [], reason: "", sheets: [],
    ...over,
  };
}

describe("the considered set", () => {
  // The toggle only exists when a rule reported one. Offering an empty "what was checked" view would
  // answer the question with a blank table, which reads as "nothing was checked" about a run where
  // some rules simply do not report coverage yet.
  it("offers no mode toggle when no rule states a considered set", () => {
    const { el } = mountPanel({ findings: [f({})], verdicts: [] });
    expect(el.querySelector(".checks-mode")).toBeNull();
  });

  it("shows a PASS with its proof, which the violations table cannot", () => {
    const { el } = mountPanel({ verdicts: [v()] });
    const sel = el.querySelector(".checks-mode select") as HTMLSelectElement;
    sel.value = "considered";
    sel.dispatchEvent(new Event("change"));

    const row = el.querySelector(".verdict-row");
    expect(row?.textContent).toContain("pass");
    expect(row?.textContent).toContain("SDA");
    expect(row?.textContent).toContain("SDA reaches rail +3V3 through R1");
  });

  // A not-considered verdict has no statement, and a row showing neither would be the silence this
  // table exists to remove, so the reason takes the proof column.
  it("falls back to the reason when the rule could not decide", () => {
    const { el } = mountPanel({
      verdicts: [v({ outcome: "not-considered", statement: "", reason: "pin is not connected to a net" })],
    });
    const sel = el.querySelector(".checks-mode select") as HTMLSelectElement;
    sel.value = "considered";
    sel.dispatchEvent(new Event("change"));
    expect(el.querySelector(".verdict-row")?.textContent).toContain("pin is not connected to a net");
  });

  // Clicking resolves by ID, not by subject. Two rules can hold verdicts about one subject, and a
  // passing verdict has no finding for the subject lookup to land on.
  it("selects a verdict by its id", () => {
    const { el, onSelectVerdict } = mountPanel({ verdicts: [v()] });
    const sel = el.querySelector(".checks-mode select") as HTMLSelectElement;
    sel.value = "considered";
    sel.dispatchEvent(new Event("change"));

    (el.querySelector(".verdict-row") as HTMLElement).click();
    expect(onSelectVerdict).toHaveBeenCalledWith("i2c-pull-up:(net:SDA)");
  });
});

// THE TWO GAPS A REAL BROWSER FOUND AND THE UNIT TESTS DID NOT. Both are about arriving on a link
// rather than clicking a row, which is the CLI-to-viewer hop and the case nothing exercised.
describe("arriving on a verdict link", () => {
  it("opens on the table containing the verdict, not on violations", () => {
    const { el } = mountPanel({ verdicts: [v()], focusedVerdict: "i2c-pull-up:(net:SDA)" });
    // No toggle was touched. A reader following a link has no reason to know there is one.
    expect(el.querySelector(".verdicts-table")).not.toBeNull();
    expect(el.querySelector(".verdict-row.selected")?.textContent).toContain("SDA");
  });

  it("stays on violations when no verdict is focused", () => {
    const { el } = mountPanel({ findings: [f({})], verdicts: [v()] });
    expect(el.querySelector(".verdicts-table")).toBeNull();
  });
});

// In agni issue 350, the same finding read "unresolved" beside a selection in the query panel and
// "error" in this panel three inches away.
describe("an inconclusive row in the checks panel", () => {
  const undecided = f({ rule: "esd-protection", severity: "error", subject: "U3", inconclusive: true, message: "no datasheet for U3" });
  const defect = f({ rule: "rail-short", severity: "error", subject: "VBUS" });

  it("takes a mark of its own instead of a severity dot", () => {
    const { el } = mountPanel({ findings: [undecided] });
    const row = el.querySelector(".check-row")!;
    expect(row.classList.contains("inconclusive")).toBe(true);
    expect(row.querySelector(".sev-dot")).toBeNull();
    expect(row.querySelector(".sev-unresolved")?.textContent).toBe("?");
    expect(row.querySelector(".sev-unresolved")?.getAttribute("title")).toContain("could not decide");
  });

  it("keeps the severity dot on a real defect", () => {
    const { el } = mountPanel({ findings: [defect] });
    expect(el.querySelector(".check-row .sev-dot.sev-error")).not.toBeNull();
    expect(el.querySelector(".sev-unresolved")).toBeNull();
  });

  // The group badge is what a reader counts, and "2" over one failure and one undecided result is
  // the wrong number however the rows are marked.
  it("is counted beside the group's defects, never inside the badge", () => {
    const { el } = mountPanel({ findings: [defect, undecided] });
    const sel = el.querySelector(".checks-groupby select") as HTMLSelectElement;
    sel.value = "rule";
    sel.dispatchEvent(new Event("change"));

    const heads = [...el.querySelectorAll(".check-group-head")];
    const badges = heads.map((h) => [
      h.querySelector(".check-group-name")?.textContent,
      h.querySelector(".finding-group-badge")?.textContent ?? null,
      h.querySelector(".finding-group-unresolved")?.textContent ?? null,
    ]);
    expect(badges).toEqual([
      ["rail-short", "1", null],
      ["esd-protection", null, "1?"],
    ]);
  });

  it("gets its own section on the severity axis rather than joining the errors", () => {
    const { el } = mountPanel({ findings: [defect, undecided] });
    const sel = el.querySelector(".checks-groupby select") as HTMLSelectElement;
    sel.value = "severity";
    sel.dispatchEvent(new Event("change"));
    expect([...el.querySelectorAll(".check-group-name")].map((n) => n.textContent)).toEqual(["error", "unresolved"]);
  });
});

describe("saving a check report (agni issue 127)", () => {
  const save = (el: HTMLElement, format: string) => el.querySelector<HTMLButtonElement>(`.checks-save[data-format="${format}"]`);

  it("offers both forms once every selected rule has run, and saves the one clicked", () => {
    const { el, onSave } = mountPanel({ ruleCount: 3, pending: 0 });
    expect(save(el, "html")?.disabled).toBe(false);
    expect(save(el, "json")?.disabled).toBe(false);
    save(el, "json")?.click();
    save(el, "html")?.click();
    expect(onSave.mock.calls).toEqual([["json"], ["html"]]);
  });

  // agni issue 956: csv is a third form, under the same gate.
  it("offers the csv form beside them and saves it when clicked", () => {
    const { el, onSave } = mountPanel({ ruleCount: 3, pending: 0 });
    expect(save(el, "csv")?.disabled).toBe(false);
    save(el, "csv")?.click();
    expect(onSave.mock.calls).toEqual([["csv"]]);
  });

  it("holds the save back while a rule is unrun, a run is going, or the catalog is loading", () => {
    for (const over of [{ ruleCount: 3, pending: 1 }, { ruleCount: 3, running: true }, { ruleCount: 3, catalogLoading: true }, { ruleCount: 0 }]) {
      const { el } = mountPanel(over);
      expect(save(el, "html")?.disabled, JSON.stringify(over)).toBe(true);
      expect(save(el, "csv")?.disabled, JSON.stringify(over)).toBe(true);
      document.body.replaceChildren();
    }
  });

  it("says why a save failed", () => {
    const { el } = mountPanel({ ruleCount: 1, saveNote: "Could not save the report: boom" });
    expect(el.querySelector(".checks-save-note")?.textContent).toBe("Could not save the report: boom");
  });
});

// agni issue 1117: a failed run says why and names the rules it left pending, and a catalog that did
// not load says so where "No rules selected." would otherwise stand.
describe("a failed run in the checks panel", () => {
  it("says the run failed, why, and which rules were not run, until the next run starts", () => {
    const { el, panel } = mountPanel({ ruleCount: 3, pending: 2, runError: { message: "[resource_exhausted] work budget exhausted", rules: ["single-pin-net", "diff-pair-naming"] } });
    const box = el.querySelector(".checks-error");
    expect(box?.getAttribute("role")).toBe("alert");
    expect(box?.querySelector(".checks-error-head")?.textContent).toContain("The run failed, so 2 selected rules were not run.");
    expect(box?.querySelector(".checks-error-message")?.textContent).toBe("[resource_exhausted] work budget exhausted");
    expect([...el.querySelectorAll(".checks-error-rules li")].map((li) => li.textContent)).toEqual(["single-pin-net", "diff-pair-naming"]);
    // The button still offers the rules again.
    expect(el.querySelector(".checks-run")?.textContent?.trim()).toBe("Run checks (2)");
    panel.view.setState(state({ ruleCount: 3, pending: 2, running: true, runError: { message: "x", rules: ["a"] } }));
    expect(el.querySelector(".checks-error")).toBeNull();
  });

  it("says one rule in the singular", () => {
    const { el } = mountPanel({ ruleCount: 1, pending: 1, runError: { message: "boom", rules: ["single-pin-net"] } });
    expect(el.querySelector(".checks-error-head")?.textContent).toContain("The run failed, so 1 selected rule was not run.");
  });

  it("says nothing of a failure when the run did not fail", () => {
    const { el } = mountPanel({ ruleCount: 3, pending: 3 });
    expect(el.querySelector(".checks-error")).toBeNull();
  });

  it("says the catalog did not load in place of No rules selected", () => {
    const { el } = mountPanel({ ruleCount: 0, catalogError: "snapshot not found" });
    expect(el.querySelector(".checks-error")?.textContent).toBe("The rule catalog did not load, so no rules can run. snapshot not found");
    expect(el.querySelector(".findings-empty")).toBeNull();
  });
});

// agni issues 858 and 1057: a finding offers to report itself as wrong, through the link the store
// built from a FindingFeedback, and a finding without one offers nothing.
describe("reporting a wrong finding", () => {
  it("links each finding to its prefilled issue", () => {
    const { el } = mountPanel({ findings: [f({ subject: "A", reportUrl: "https://github.com/panyam/agni/issues/new?title=x" }), f({ subject: "B" })] });
    const links = [...el.querySelectorAll<HTMLAnchorElement>("a.check-report")];
    expect(links.map((a) => a.getAttribute("href"))).toEqual(["https://github.com/panyam/agni/issues/new?title=x"]);
    expect(links[0].getAttribute("target")).toBe("_blank");
  });
});

// agni issue 918: the Run button stops a run in flight, and a stopped run says so.
describe("cancelling from the checks panel", () => {
  function mountWithCancel(over: Partial<FindingsState>) {
    const onRun = vi.fn();
    const onCancel = vi.fn();
    const el = document.createElement("div");
    document.body.appendChild(el);
    const panel = findingsPanelIsland(el, null, { onSelect: vi.fn(), onLocateContext: vi.fn(), onRun, onCancel, onSelectVerdict: vi.fn() });
    panel.island.activate();
    panel.view.setState(state(over));
    return { el, onRun, onCancel, panel };
  }

  it("turns Run into an enabled Cancel while a run is in flight, which cancels rather than runs", () => {
    const { el, onRun, onCancel } = mountWithCancel({ running: true, pending: 2 });
    const button = el.querySelector<HTMLButtonElement>(".checks-run")!;
    expect(button.textContent?.trim()).toBe("Cancel");
    expect(button.disabled).toBe(false);
    button.click();
    expect(onCancel).toHaveBeenCalledOnce();
    expect(onRun).not.toHaveBeenCalled();
  });

  it("says a run was cancelled and how many rules it left unrun, until the next run starts", () => {
    const { el, panel } = mountWithCancel({ cancelled: true, pending: 3 });
    expect(el.querySelector(".checks-cancelled")?.textContent).toContain("Cancelled. 3 selected rules were not run.");
    expect(el.querySelector(".checks-run")?.textContent?.trim()).toBe("Run checks (3)");
    panel.view.setState(state({ cancelled: true, running: true, pending: 3 }));
    expect(el.querySelector(".checks-cancelled")).toBeNull();
  });

  it("says the run is under way in the panel, how many rules it is running", () => {
    const { el, panel } = mountWithCancel({ running: true, pending: 4 });
    expect(el.querySelector(".checks-running")?.textContent).toContain("Running 4 selected rules…");
    // "Press Run checks" would contradict the run under way.
    expect(el.querySelector(".findings-empty")).toBeNull();
    panel.view.setState(state({ running: false, pending: 0 }));
    expect(el.querySelector(".checks-running")).toBeNull();
  });

  it("keeps the button disabled while running where the host cannot cancel", () => {
    const { el } = mountPanel({ running: true });
    const button = el.querySelector<HTMLButtonElement>(".checks-run")!;
    expect(button.textContent?.trim()).toBe("Running…");
    expect(button.disabled).toBe(true);
  });
});

describe("a rule group links to the rule's reference page", () => {
  const url = "https://panyam.github.io/agni/reference/rules/i2c-pull-up/";
  const groupBy = (el: HTMLElement, axis: string) => {
    const sel = el.querySelector(".checks-groupby select") as HTMLSelectElement;
    sel.value = axis;
    sel.dispatchEvent(new Event("change"));
  };
  const findings = [f({ rule: "i2c-pull-up", category: "connectivity", subject: "SDA" }), f({ rule: "acme/house-rule", category: "connectivity", subject: "SCL" })];

  it("links a documented rule's header to its page, in a new tab, and leaves an undocumented one bare", () => {
    const { el } = mountPanel({ findings, ruleDocs: { "i2c-pull-up": url } });
    groupBy(el, "rule");
    const links = [...el.querySelectorAll<HTMLAnchorElement>(".check-group-doc")];
    expect(links).toHaveLength(1);
    expect(links[0].href).toBe(url);
    expect(links[0].target).toBe("_blank");
    expect(links[0].closest("tr")!.querySelector(".check-group-name")!.textContent).toBe("i2c-pull-up");
    expect(links[0].closest("button")).toBeNull();
    // The header keeps the rule's spelling, since copper-clearance is an identifier and not a title.
    expect(el.querySelector(".check-group-head")!.classList.contains("rule")).toBe(true);
  });

  it("does not fold the group when the link is clicked", () => {
    const { el } = mountPanel({ findings, ruleDocs: { "i2c-pull-up": url } });
    groupBy(el, "rule");
    const link = el.querySelector<HTMLAnchorElement>(".check-group-doc")!;
    link.addEventListener("click", (e) => e.preventDefault());
    link.click();
    expect([...el.querySelectorAll("tr.check-row")].map((r) => r.textContent)).toEqual(expect.arrayContaining([expect.stringContaining("SDA")]));
  });

  it("shows no link on a header that names a category rather than a rule", () => {
    const { el } = mountPanel({ findings, ruleDocs: { connectivity: url, "i2c-pull-up": url } });
    groupBy(el, "category");
    expect(el.querySelectorAll(".check-group-doc")).toHaveLength(0);
  });
});
