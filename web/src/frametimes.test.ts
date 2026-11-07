import { describe, expect, it } from "vitest";
import { fitFrame, mapPoint, summarize, type Point } from "../browser/frametimes.js";

describe("summarize", () => {
  it("takes nearest-rank percentiles over the frames", () => {
    const deltas = Array.from({ length: 20 }, (_, i) => i + 1); // 1..20 ms, slowest first
    deltas.reverse();
    expect(summarize(deltas)).toEqual({ frames: 20, p50: 10, p95: 19, max: 20 });
  });

  it("reads one slow frame as the max and not the median", () => {
    expect(summarize([16, 16, 17, 16, 250])).toEqual({ frames: 5, p50: 16, p95: 250, max: 250 });
  });

  it("summarizes no frames as zeros", () => {
    expect(summarize([])).toEqual({ frames: 0, p50: 0, p95: 0, max: 0 });
  });
});

describe("fitFrame", () => {
  const truth = { kx: 0.97, cx: 21, ky: 0.97, cy: 11 };
  const grid: Point[] = [];
  for (let x = 100; x <= 1200; x += 110) for (let y = 80; y <= 900; y += 90) grid.push([x, y]);

  it("recovers a scale and offset from anchor pairs", () => {
    const f = fitFrame(grid.map((p) => [p, mapPoint(truth, p)]))!;
    expect(f.kx).toBeCloseTo(0.97, 6);
    expect(f.cy).toBeCloseTo(11, 4);
    expect(f.residual).toBeLessThan(1e-6);
  });

  it("ignores a minority of labels that one renderer justifies differently", () => {
    const pairs = grid.map((p, i): [Point, Point] => {
      const [x, y] = mapPoint(truth, p);
      return [p, i % 5 === 0 ? [x + 40, y - 15] : [x, y]];
    });
    const f = fitFrame(pairs)!;
    expect(f.kx).toBeCloseTo(0.97, 6);
    expect(f.cx).toBeCloseTo(21, 4);
    expect(f.residual).toBeLessThan(1e-6);
  });

  it("needs three pairs", () => {
    expect(fitFrame([[[0, 0], [1, 1]], [[5, 5], [6, 6]]])).toBeNull();
  });
});
