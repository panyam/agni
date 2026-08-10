// @vitest-environment jsdom
import { describe, it, expect, vi } from "vitest";
import { reviewPanelIsland } from "./reviewpanel.jsx";
import { type ReviewState, type ReviewRunView, emptyReview, outcomeClass, OUTCOME_PASS } from "./review.js";

function mount(state: Partial<ReviewState>) {
  const handlers = {
    onSelectRun: vi.fn(),
    onSelectChecklist: vi.fn(),
    onCreate: vi.fn(),
    onLocate: vi.fn(),
  };
  const el = document.createElement("div");
  document.body.appendChild(el);
  const panel = reviewPanelIsland(el, null, handlers);
  panel.island.activate();
  panel.view.setState({ ...emptyReview(), ...state });
  return { el, handlers, setState: panel.view.setState };
}

function run(overrides: Partial<ReviewRunView> = {}): ReviewRunView {
  return {
    name: "reviews/r1",
    design: "proj/board.edn",
    designHash: "sha256:abc",
    createdAt: "2026-08-10T20:04:22Z",
    manifest: "Gateway ECU review",
    producerVersion: "v0.1.1",
    areas: [
      {
        name: "Power",
        items: [
          { id: "P1", title: "bulk capacitance", outcome: "pass", note: "", findings: [] },
          {
            id: "P2",
            title: "reverse polarity protection",
            outcome: "fail",
            note: "",
            findings: [
              {
                rule: "reverse-blocking-absent", category: "", profile: "", severity: "error",
                kind: "net", subject: "VIN", pin: "", netId: "n1", busId: "",
                message: "no blocking element", sheets: [], locateReason: 0,
              },
            ],
          },
          { id: "P3", title: "needs a datasheet value", outcome: "needs-data", note: "no seeded spec", findings: [] },
          { id: "P4", title: "reviewed by hand", outcome: "not-automated", note: "the EE signs this off", findings: [] },
          { id: "P5", title: "no CAN on this board", outcome: "not-applicable", note: "", findings: [] },
        ],
      },
    ],
    ...overrides,
  };
}

const withRun = { runs: [run()], selected: "reviews/r1", checklists: [{ ref: "proj/review.yaml", label: "review.yaml" }], checklist: "proj/review.yaml" };

describe("reviewPanel", () => {
  it("renders each area's items with their outcomes", () => {
    const { el } = mount(withRun);
    expect(el.querySelectorAll(".rv-item")).toHaveLength(5);
    expect(el.querySelector(".rv-area-name")?.textContent).toBe("Power");
    expect(el.textContent).toContain("bulk capacitance");
  });

  // The acceptance assertion from the issue: "An item that could not be evaluated must not look like
  // a pass; that distinction is the whole point of the vocabulary." Asserted structurally, on the
  // class the chip carries, rather than on wording that could change.
  it("gives no unevaluated item the pass style", () => {
    const { el } = mount(withRun);
    const passClass = outcomeClass(OUTCOME_PASS);
    const chips = Array.from(el.querySelectorAll(".rv-outcome"));
    expect(chips).toHaveLength(5);
    const passing = chips.filter((c) => c.classList.contains(passClass));
    expect(passing).toHaveLength(1);
    expect(passing[0].getAttribute("title")).toBe("pass");
    // Every other verdict, including the two that produced no findings, is styled as something else.
    for (const chip of chips) {
      if (chip === passing[0]) continue;
      expect(chip.classList.contains(passClass), `${chip.getAttribute("title")} looks like a pass`).toBe(false);
    }
  });

  it("shows the coverage axis first in the summary, not just pass/fail", () => {
    const { el } = mount(withRun);
    const summary = el.querySelector(".rv-summary")?.textContent ?? "";
    // Four of the five items have a mechanism; only the not-automated one does not.
    expect(summary).toContain("4 of 5 covered");
    expect(summary.indexOf("covered")).toBeLessThan(summary.indexOf("pass"));
  });

  it("carries an item's note, which is where a not-automated item says who answers it", () => {
    const { el } = mount(withRun);
    expect(el.textContent).toContain("the EE signs this off");
  });

  it("collapses findings until asked, then locates the one clicked", () => {
    const { el, handlers } = mount(withRun);
    expect(el.querySelectorAll(".rv-finding")).toHaveLength(0);
    const toggle = el.querySelector(".rv-findings-toggle") as HTMLButtonElement;
    expect(toggle.textContent).toContain("1 finding");
    toggle.click();
    const finding = el.querySelector(".rv-finding") as HTMLButtonElement;
    expect(finding.textContent).toContain("VIN");
    finding.click();
    expect(handlers.onLocate).toHaveBeenCalledWith("net", "VIN");
  });

  it("renders an unknown outcome under its own name rather than dropping or passing it", () => {
    const r = run();
    r.areas[0].items = [{ id: "X", title: "future verdict", outcome: "covered-externally", note: "", findings: [] }];
    const { el } = mount({ ...withRun, runs: [r] });
    const chip = el.querySelector(".rv-outcome");
    expect(chip?.textContent).toBe("covered-externally");
    expect(chip?.classList.contains(outcomeClass(OUTCOME_PASS))).toBe(false);
  });
});

describe("reviewPanel empty states", () => {
  // The two empty states are deliberately different. One says the deployment cannot keep runs at
  // all; the other says nobody has reviewed this board. A user shown the wrong one goes looking for
  // a button that was never going to appear.
  it("names the flag when the server keeps no reviews, and offers no run control", () => {
    const { el } = mount({ storeConfigured: false });
    expect(el.textContent).toContain("--review-store");
    expect(el.querySelector(".rv-run")).toBeNull();
    expect(el.querySelector(".rv-checklist")).toBeNull();
  });

  it("invites a first run when the design simply has none", () => {
    const { el } = mount({ checklists: [{ ref: "proj/review.yaml", label: "review.yaml" }], checklist: "proj/review.yaml" });
    expect(el.textContent).toContain("No review runs for this design yet");
    expect(el.textContent).not.toContain("--review-store");
    expect((el.querySelector(".rv-run") as HTMLButtonElement).disabled).toBe(false);
  });

  it("disables the run control when no checklist sits beside the design", () => {
    const { el } = mount({});
    expect((el.querySelector(".rv-run") as HTMLButtonElement).disabled).toBe(true);
    expect(el.querySelector(".rv-checklist")?.textContent).toContain("no checklist");
  });

  it("disables the run control while a run is in flight", () => {
    const { el } = mount({ ...withRun, running: true });
    const button = el.querySelector(".rv-run") as HTMLButtonElement;
    expect(button.disabled).toBe(true);
    expect(button.textContent).toContain("Running");
  });

  it("shows a server error inline instead of results", () => {
    const { el } = mount({ ...withRun, runs: [], selected: "", error: "review manifest item \"P1\": declares more than one binding" });
    expect(el.querySelector(".rv-error")?.textContent).toContain("more than one binding");
    expect(el.textContent).not.toContain("No review runs for this design yet");
  });
});

describe("reviewPanel history", () => {
  it("lists past runs newest first and emits the one chosen", () => {
    const older = run({ name: "reviews/r0", createdAt: "2026-08-01T09:00:00Z" });
    const { el, handlers } = mount({ ...withRun, runs: [run(), older] });
    const history = el.querySelector(".rv-history") as HTMLSelectElement;
    expect(history.options).toHaveLength(2);
    expect(history.options[0].value).toBe("reviews/r1");
    expect(history.options[1].value).toBe("reviews/r0");
    history.value = "reviews/r0";
    history.dispatchEvent(new Event("change"));
    expect(handlers.onSelectRun).toHaveBeenCalledWith("reviews/r0");
  });

  it("hides the history control when there is only nothing to pick between", () => {
    const { el } = mount({ checklists: [{ ref: "a.yaml", label: "a.yaml" }], checklist: "a.yaml" });
    expect(el.querySelector(".rv-history")).toBeNull();
  });

  it("labels a run with when it ran and what it found", () => {
    const { el } = mount(withRun);
    const label = (el.querySelector(".rv-history") as HTMLSelectElement | null)?.options[0]?.text ?? "";
    expect(label).toContain("2026-08-10");
    expect(label).toContain("1 fail");
    expect(label).toContain("4/5 covered");
  });

  it("emits the chosen checklist and the create intent", () => {
    const { el, handlers } = mount(withRun);
    const picker = el.querySelector(".rv-checklist") as HTMLSelectElement;
    picker.dispatchEvent(new Event("change"));
    expect(handlers.onSelectChecklist).toHaveBeenCalled();
    (el.querySelector(".rv-run") as HTMLButtonElement).click();
    expect(handlers.onCreate).toHaveBeenCalled();
  });
});
