import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { delayedBusy, RENDER_BUSY_DELAY_MS } from "./busy.js";

describe("delayedBusy", () => {
  beforeEach(() => vi.useFakeTimers());
  afterEach(() => vi.useRealTimers());

  it("does not show the overlay for an instantaneous render (clears before the delay)", () => {
    const el = document.createElement("div");
    const setBusy = delayedBusy(el);
    setBusy(true);
    vi.advanceTimersByTime(RENDER_BUSY_DELAY_MS - 1);
    setBusy(false); // render finished before the delay elapsed
    vi.advanceTimersByTime(RENDER_BUSY_DELAY_MS);
    expect(el.classList.contains("on")).toBe(false);
  });

  it("shows the overlay when the render outlasts the delay, then hides on done", () => {
    const el = document.createElement("div");
    const setBusy = delayedBusy(el);
    setBusy(true);
    expect(el.classList.contains("on")).toBe(false); // not yet
    vi.advanceTimersByTime(RENDER_BUSY_DELAY_MS);
    expect(el.classList.contains("on")).toBe(true);
    setBusy(false);
    expect(el.classList.contains("on")).toBe(false);
  });

  it("repeated busy(true) does not restack the timer", () => {
    const el = document.createElement("div");
    const setBusy = delayedBusy(el);
    setBusy(true);
    vi.advanceTimersByTime(RENDER_BUSY_DELAY_MS - 20);
    setBusy(true); // a nested render; must not reset the countdown
    vi.advanceTimersByTime(20);
    expect(el.classList.contains("on")).toBe(true);
  });

  it("updates the phase label immediately, even before the overlay shows", () => {
    const el = document.createElement("div");
    const label = document.createElement("span");
    label.className = "render-busy-label";
    el.appendChild(label);
    const setBusy = delayedBusy(el);
    setBusy(true, "loading design…");
    expect(label.textContent).toBe("loading design…"); // set before the show-delay elapses
    expect(el.classList.contains("on")).toBe(false);
    setBusy(true, "running checks…"); // a later phase updates the text without restacking
    vi.advanceTimersByTime(RENDER_BUSY_DELAY_MS);
    expect(el.classList.contains("on")).toBe(true);
    expect(label.textContent).toBe("running checks…");
  });

  it("is a no-op for a null element", () => {
    const setBusy = delayedBusy(null);
    expect(() => {
      setBusy(true);
      vi.advanceTimersByTime(RENDER_BUSY_DELAY_MS);
      setBusy(false);
    }).not.toThrow();
  });
});
