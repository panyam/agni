// @vitest-environment jsdom
import { describe, it, expect, vi } from "vitest";
import { controlBarIsland } from "./controlbar.jsx";
import type { ControlsState } from "./controls.js";

const base: ControlsState = {
  mode: "svg",
  nativeAvailable: false,
  hasHighlights: false,
};

function mount(over: Partial<ControlsState> = {}) {
  const onClearHighlights = vi.fn();
  const onMode = vi.fn();
  const el = document.createElement("div");
  document.body.appendChild(el);
  const bar = controlBarIsland(el, null, { onMode, onClearHighlights });
  bar.island.activate();
  bar.view.setState({ ...base, ...over });
  return {
    el,
    onMode,
    onClearHighlights,
    clear: () => el.querySelector(".clear-highlights") as HTMLButtonElement | null,
    webgl: () => el.querySelector(".render-webgl") as HTMLInputElement | null,
    native: () => el.querySelector(".render-native") as HTMLInputElement | null,
  };
}

// The control that had nowhere to live (agni issue 348). setHighlights([]) was reachable from one
// place in the client, on opening a different design, so a reader could not turn the field off.
describe("the clear-highlight control", () => {
  it("is always present, so the reader learns it once", () => {
    // Rendered rather than shown-on-demand, since a control that appears on a state change has to
    // be discovered twice, and this exists because the gesture was undiscoverable.
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

// The renderer is one WebGL checkbox, off for SVG (agni issue 1469). It replaced a WebGL, SVG and
// Native switch, whose Native button was greyed out on almost every server.
describe("the WebGL checkbox", () => {
  it("is off in SVG and on in WebGL", () => {
    expect(mount({ mode: "svg" }).webgl()!.checked).toBe(false);
    expect(mount({ mode: "webgl" }).webgl()!.checked).toBe(true);
  });

  it("asks for WebGL when checked and for SVG when cleared", () => {
    const svg = mount({ mode: "svg" });
    svg.webgl()!.click();
    expect(svg.onMode).toHaveBeenLastCalledWith("webgl");
    const gl = mount({ mode: "webgl" });
    gl.webgl()!.click();
    expect(gl.onMode).toHaveBeenLastCalledWith("svg");
  });

  it("is the only renderer control where Native is not offered", () => {
    const m = mount({ nativeAvailable: false });
    expect(m.native()).toBeNull();
    expect(m.el.querySelectorAll(".render-mode input").length).toBe(1);
  });
});

describe("the Native checkbox", () => {
  it("shows where the server can draw the design natively, and asks for it when checked", () => {
    const m = mount({ nativeAvailable: true, mode: "svg" });
    expect(m.native()!.checked).toBe(false);
    m.native()!.click();
    expect(m.onMode).toHaveBeenLastCalledWith("native");
    expect(mount({ nativeAvailable: true, mode: "native" }).native()!.checked).toBe(true);
  });
});
