// @vitest-environment jsdom
import { describe, it, expect, vi } from "vitest";
import { controlBarIsland } from "./controlbar.jsx";
import type { ControlsState } from "./controls.js";

const base: ControlsState = {
  mode: "svg",
  nativeAvailable: false,
  layouts: ["faithful"],
  layout: "faithful",
  providedSymbols: false,
  faithfulSymbols: false,
  board: false,
  boardLayers: "all",
  hasHighlights: false,
};

function mount(over: Partial<ControlsState> = {}) {
  const onClearHighlights = vi.fn();
  const el = document.createElement("div");
  document.body.appendChild(el);
  const bar = controlBarIsland(el, null, {
    onMode: vi.fn(),
    onLayout: vi.fn(),
    onSymbols: vi.fn(),
    onBoardLayers: vi.fn(),
    onClearHighlights,
  });
  bar.island.activate();
  bar.view.setState({ ...base, ...over });
  return { el, onClearHighlights, clear: () => el.querySelector(".clear-highlights") as HTMLButtonElement | null };
}

// The control that had nowhere to live (agni issue 348). setHighlights([]) was reachable from one
// place in the client, on opening a different design, so a reader could not turn the field off.
describe("the clear-highlight control", () => {
  it("is always present, so the reader learns it once", () => {
    // Rendered rather than shown-on-demand: a control that appears on a state change has to be
    // discovered twice, and this exists precisely because the gesture was undiscoverable.
    expect(mount({ hasHighlights: false }).clear()).not.toBeNull();
    expect(mount({ hasHighlights: true }).clear()).not.toBeNull();
  });

  it("disables when nothing is lit and enables when something is", () => {
    expect(mount({ hasHighlights: false }).clear()!.disabled).toBe(true);
    expect(mount({ hasHighlights: true }).clear()!.disabled).toBe(false);
  });

  it("emits the clear intent when clicked", () => {
    const m = mount({ hasHighlights: true });
    m.clear()!.click();
    expect(m.onClearHighlights).toHaveBeenCalledTimes(1);
  });

  it("names the keyboard route, so the button teaches the shortcut", () => {
    // The two are one affordance. A reader who finds the button should not have to discover Escape
    // separately, and a reader who tries Escape first should find the button confirms it.
    expect(mount({ hasHighlights: true }).clear()!.title).toContain("Escape");
  });
});
