import { describe, it, expect, beforeEach } from "vitest";
import { expectationCaptionStrip } from "./expectcaption.js";

describe("expectationCaptionStrip", () => {
  let el: HTMLElement;
  beforeEach(() => {
    el = document.createElement("div");
  });

  it("renders a passing verdict and shows the strip", () => {
    expectationCaptionStrip(el)({ pass: true, expected: 3, matched: 3, unexpected: 0, missing: [], silent: false });
    expect(el.textContent).toBe("expectations ✓ 3/3 expected");
    expect(el.classList.contains("on")).toBe(true);
    expect(el.classList.contains("pass")).toBe(true);
    expect(el.classList.contains("fail")).toBe(false);
  });

  it("renders a failing verdict with missing + unexpected counts", () => {
    expectationCaptionStrip(el)({ pass: false, expected: 2, matched: 1, unexpected: 1, missing: ["r"], silent: false });
    expect(el.textContent).toBe("expectations ✗ 1/2 expected, 1 missing, 1 unexpected");
    expect(el.classList.contains("fail")).toBe(true);
  });

  it("renders the fires:{} silent assertion", () => {
    const set = expectationCaptionStrip(el);
    set({ pass: true, expected: 0, matched: 0, unexpected: 0, missing: [], silent: true });
    expect(el.textContent).toBe("expectations ✓ silent");
    set({ pass: false, expected: 0, matched: 0, unexpected: 2, missing: [], silent: true });
    expect(el.textContent).toBe("expectations ✗ silent — 2 unexpected");
  });

  it("hides the strip when the caption is null (no sidecar)", () => {
    const set = expectationCaptionStrip(el);
    set({ pass: true, expected: 1, matched: 1, unexpected: 0, missing: [], silent: false });
    set(null);
    expect(el.classList.contains("on")).toBe(false);
    expect(el.textContent).toBe("");
  });
});
