// @vitest-environment jsdom
import { describe, it, expect, vi } from "vitest";
import { stepBarIsland, type StepBarState } from "./stepbar.jsx";

const none: StepBarState = { canBack: false, canForward: false, canPrev: false, canNext: false, place: "" };

function mount(state: Partial<StepBarState>) {
  const h = { onBack: vi.fn(), onForward: vi.fn(), onPrev: vi.fn(), onNext: vi.fn() };
  const el = document.createElement("div");
  document.body.appendChild(el);
  const bar = stepBarIsland(el, h);
  bar.island.activate();
  bar.view.setState({ ...none, ...state });
  const at = (action: string) => el.querySelector(`[data-action="${action}"]`) as HTMLButtonElement;
  return { el, h, at, bar };
}

describe("the canvas's step controls", () => {
  it("draws every control and disables each that goes nowhere", () => {
    const { at } = mount({});
    for (const a of ["selection-back", "selection-forward", "prev-sheet", "next-sheet"]) expect(at(a).disabled).toBe(true);
  });

  it("enables a control that goes somewhere and calls its handler", () => {
    const { at, h } = mount({ canBack: true, canForward: true, canPrev: true, canNext: true });
    at("selection-back").click();
    at("selection-forward").click();
    at("prev-sheet").click();
    at("next-sheet").click();
    expect([h.onBack, h.onForward, h.onPrev, h.onNext].map((f) => f.mock.calls.length)).toEqual([1, 1, 1, 1]);
  });

  it("reads the canvas's place among the sheets, and follows a new state", () => {
    const { el, bar, at } = mount({ place: "3 of 6", canPrev: true, canNext: true });
    expect(el.querySelector("[data-sheet-place]")?.textContent).toBe("3 of 6");
    bar.view.setState({ ...none, place: "6 of 6", canPrev: true });
    expect(el.querySelector("[data-sheet-place]")?.textContent).toBe("6 of 6");
    expect(at("next-sheet").disabled).toBe(true);
  });

  it("names each control's key in its title", () => {
    const { at } = mount({});
    expect(at("selection-back").title).toContain("([)");
    expect(at("selection-forward").title).toContain("(])");
    expect(at("prev-sheet").title).toContain("(,)");
    expect(at("next-sheet").title).toContain("(.)");
  });
});
