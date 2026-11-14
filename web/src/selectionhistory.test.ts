import { describe, it, expect } from "vitest";
import { SelectionHistory } from "./selectionhistory.js";
import type { Selection } from "./selection.js";

const part = (ref: string): Selection => ({ kind: "component", ref });

describe("SelectionHistory", () => {
  it("steps back and forward through the picks", () => {
    const h = new SelectionHistory();
    for (const r of ["A", "B", "C"]) h.push(part(r));
    expect(h.back()).toEqual(part("B"));
    expect(h.back()).toEqual(part("A"));
    expect(h.forward()).toEqual(part("B"));
    expect(h.forward()).toEqual(part("C"));
  });

  it("can neither step back from the first pick nor forward from the last", () => {
    const h = new SelectionHistory();
    expect([h.canBack(), h.canForward()]).toEqual([false, false]);
    expect(h.back()).toBeUndefined();
    h.push(part("A"));
    expect([h.canBack(), h.canForward()]).toEqual([false, false]);
    h.push(part("B"));
    expect([h.canBack(), h.canForward()]).toEqual([true, false]);
    expect(h.forward()).toBeUndefined();
    h.back();
    expect([h.canBack(), h.canForward()]).toEqual([false, true]);
    expect(h.back()).toBeUndefined();
  });

  it("drops the picks ahead after a new pick made from a step back", () => {
    const h = new SelectionHistory();
    for (const r of ["A", "B", "C"]) h.push(part(r));
    h.back();
    h.back();
    h.push(part("D"));
    expect(h.canForward()).toBe(false);
    expect(h.size).toBe(2);
    expect(h.back()).toEqual(part("A"));
    expect(h.forward()).toEqual(part("D"));
  });

  it("keeps at most its limit and drops the oldest", () => {
    const h = new SelectionHistory(3);
    for (const r of ["A", "B", "C", "D"]) h.push(part(r));
    expect(h.size).toBe(3);
    expect(h.back()).toEqual(part("C"));
    expect(h.back()).toEqual(part("B"));
    expect(h.back()).toBeUndefined();
  });

  it("keeps 100 picks by default", () => {
    const h = new SelectionHistory();
    for (let i = 0; i < 150; i++) h.push(part(`R${i}`));
    expect(h.size).toBe(100);
    let steps = 0;
    while (h.back()) steps++;
    expect(steps).toBe(99);
  });

  it("records the same pick twice in a row once", () => {
    const h = new SelectionHistory();
    h.push(part("A"));
    h.push(part("A"));
    h.push({ kind: "net", net: "GND" });
    h.push({ kind: "net", net: "GND" });
    expect(h.size).toBe(2);
    expect(h.back()).toEqual(part("A"));
  });

  it("records nothing for a step that lands on the selection it holds", () => {
    const h = new SelectionHistory();
    for (const r of ["A", "B"]) h.push(part(r));
    const back = h.back()!;
    h.push(back);
    expect(h.canForward()).toBe(true);
  });

  it("starts empty again after a reset", () => {
    const h = new SelectionHistory();
    for (const r of ["A", "B"]) h.push(part(r));
    h.reset();
    expect([h.size, h.canBack(), h.canForward()]).toEqual([0, false, false]);
  });
});
