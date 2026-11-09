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

  it("marks the element busy at once, before the overlay shows (agni issue 1336)", () => {
    const el = document.createElement("div");
    const setBusy = delayedBusy(el);
    setBusy(true);
    expect(el.dataset.busy).toBe("on");
    expect(el.classList.contains("on")).toBe(false);
    setBusy(false);
    expect(el.dataset.busy).toBeUndefined();
  });

  it("softens a blocking overlay into a pill that takes no input (agni issue 1454)", () => {
    const el = document.createElement("div");
    const setBusy = delayedBusy(el);
    setBusy(true);
    vi.advanceTimersByTime(RENDER_BUSY_DELAY_MS);
    expect(el.classList.contains("on")).toBe(true);
    setBusy("soft", "loading rules…");
    expect(el.classList.contains("on")).toBe(false);
    expect(el.classList.contains("soft")).toBe(true);
    expect(el.dataset.busy).toBe("on");
    setBusy(false);
    expect(el.classList.contains("soft")).toBe(false);
    expect(el.dataset.busy).toBeUndefined();
  });

  it("never shows the blocking overlay for a soft span, however long it lasts", () => {
    const el = document.createElement("div");
    const setBusy = delayedBusy(el);
    setBusy(true);
    setBusy("soft");
    vi.advanceTimersByTime(RENDER_BUSY_DELAY_MS * 10);
    expect(el.classList.contains("on")).toBe(false);
    expect(el.classList.contains("soft")).toBe(true);
  });

  it("blocks again, after the delay, when a render starts during a soft span", () => {
    const el = document.createElement("div");
    const setBusy = delayedBusy(el);
    setBusy("soft");
    setBusy(true);
    expect(el.classList.contains("soft")).toBe(false);
    vi.advanceTimersByTime(RENDER_BUSY_DELAY_MS);
    expect(el.classList.contains("on")).toBe(true);
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
