import { describe, it, expect } from "vitest";
import { DrawGate, type DrawState } from "./drawgate.js";

const content = {};
const identity = [1, 0, 0, 0, 1, 0, 0, 0, 1];
const base: DrawState = { matrix: identity, width: 800, height: 600, dpr: 2, content, version: 0 };

function drawn(first: DrawState) {
  const gate = new DrawGate();
  expect(gate.shouldDraw(first)).toBe(true);
  return gate;
}

describe("a view's draw gate", () => {
  it("draws the first frame and skips the frames after it while nothing changes", () => {
    const gate = drawn(base);
    expect(gate.shouldDraw({ ...base, matrix: [...identity] })).toBe(false);
    expect(gate.shouldDraw({ ...base })).toBe(false);
  });

  it("draws when the camera moves, whoever moved it", () => {
    const gate = drawn(base);
    const panned = [1, 0, 0, 0, 1, 0, 0.25, 0, 1];
    expect(gate.shouldDraw({ ...base, matrix: panned })).toBe(true);
    expect(gate.shouldDraw({ ...base, matrix: panned })).toBe(false);
  });

  it("draws when the view is resized or moves to a screen of another pixel ratio", () => {
    expect(drawn(base).shouldDraw({ ...base, width: 801 })).toBe(true);
    expect(drawn(base).shouldDraw({ ...base, height: 599 })).toBe(true);
    expect(drawn(base).shouldDraw({ ...base, dpr: 1 })).toBe(true);
  });

  it("draws when the drawing changes, a highlight or a hidden layer, or a new sheet arrives", () => {
    expect(drawn(base).shouldDraw({ ...base, version: 1 })).toBe(true);
    expect(drawn(base).shouldDraw({ ...base, content: {} })).toBe(true);
  });

  it("draws nothing while the view has no area, and draws once it is shown again", () => {
    const gate = drawn(base);
    expect(gate.shouldDraw({ ...base, width: 0, height: 0, version: 1 })).toBe(false);
    expect(gate.shouldDraw({ ...base, version: 1 })).toBe(true);
  });

  it("draws after it is invalidated, though nothing it compares changed", () => {
    const gate = drawn(base);
    gate.invalidate();
    expect(gate.shouldDraw(base)).toBe(true);
    expect(gate.shouldDraw(base)).toBe(false);
  });
});
