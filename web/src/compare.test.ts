// @vitest-environment jsdom
import { describe, it, expect, vi } from "vitest";
import { compareButton } from "./compare.js";

function harness() {
  const host = document.createElement("div");
  const onOpen = vi.fn();
  const control = compareButton(host, onOpen);
  const btn = host.querySelector("button") as HTMLButtonElement;
  return { control, btn, onOpen };
}

describe("compareButton", () => {
  it("starts disabled, because a comparison needs an open design as its other side", () => {
    const { btn } = harness();
    expect(btn.disabled).toBe(true);
    expect(btn.title).toContain("open a design first");
  });

  it("does not fire while disabled", () => {
    const { btn, onOpen } = harness();
    btn.click();
    expect(onOpen).not.toHaveBeenCalled();
  });

  it("asks for the picker on click once a design is open", () => {
    const { control, btn, onOpen } = harness();
    control.setEnabled(true);
    btn.click();
    expect(onOpen).toHaveBeenCalledTimes(1);
  });

  it("re-disables when there is no open design", () => {
    const { control, btn, onOpen } = harness();
    control.setEnabled(true);
    control.setEnabled(false);
    expect(btn.disabled).toBe(true);
    btn.click();
    expect(onOpen).not.toHaveBeenCalled();
  });

  // The armed state is gone (WS9-049 phase 3). These pin its ABSENCE, because the failure mode of
  // a half-removed mode is a button that looks inert but still holds state: it used to toggle
  // .active, relabel itself to "pick file B in the tree", and cancel on Escape.
  it("holds no armed state: every click is a fresh request, not a toggle", () => {
    const { control, btn, onOpen } = harness();
    control.setEnabled(true);
    const label = btn.textContent;
    btn.click();
    btn.click();

    expect(onOpen).toHaveBeenCalledTimes(2);
    expect(btn.textContent).toBe(label);
    expect(btn.classList.contains("active")).toBe(false);
  });

  it("ignores Escape, which used to cancel the armed mode", () => {
    const { control, btn, onOpen } = harness();
    control.setEnabled(true);
    document.dispatchEvent(new KeyboardEvent("keydown", { key: "Escape" }));

    expect(btn.disabled).toBe(false);
    expect(btn.classList.contains("active")).toBe(false);
    expect(onOpen).not.toHaveBeenCalled();
  });
});
