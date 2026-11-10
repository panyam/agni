import { describe, it, expect } from "vitest";
import { BURST_IDLE_MS, DrawProfile, stat, type DrawEnv, type ProfileClock } from "./drawprofile.js";

// manualClock runs animation frames only when the test steps it, each frame 16 ms after the last.
function manualClock(): ProfileClock & { step(n: number): void; t: number } {
  const queue: ((t: number) => void)[] = [];
  const c = {
    t: 0,
    now: () => c.t,
    frame: (cb: (t: number) => void) => void queue.push(cb),
    step(n: number) {
      for (let i = 0; i < n; i++) {
        c.t += 16;
        const due = queue.splice(0);
        for (const cb of due) cb(c.t);
      }
    },
  };
  return c;
}

const env: DrawEnv = { gpu: "Test GPU", gpuVendor: "Test", dpr: 2, cores: 8, memoryGb: 16, cssWidth: 800, cssHeight: 600 };

function profile() {
  const lines: string[] = [];
  const clock = manualClock();
  const p = new DrawProfile("canvas", () => env, (l) => lines.push(l), clock);
  const reports = () => lines.map((l) => JSON.parse(l.slice(l.indexOf(": ") + 2)));
  return { p, clock, lines, reports };
}

describe("DrawProfile", () => {
  it("logs a sheet's scene when it is drawn", () => {
    const { p, lines, reports } = profile();
    p.sheet("webgl", { primitives: 120, vertices: 900, labels: 40, buildMs: 12.345 });
    expect(lines[0]).toMatch(/^agni draw canvas: \{/);
    expect(reports()[0]).toMatchObject({ renderer: "webgl", scene: { primitives: 120, vertices: 900, labels: 40, images: 0, elements: 0, buildMs: 12.35 }, bursts: 0 });
  });

  it("times every frame of a burst and logs once the input stops", () => {
    const { p, clock, lines, reports } = profile();
    p.sheet("webgl", {});
    p.input();
    for (let i = 0; i < 10; i++) {
      clock.step(1);
      p.rendered(3, 1, 500);
      p.input();
    }
    expect(lines).toHaveLength(1);
    clock.step(Math.ceil(BURST_IDLE_MS / 16) + 2);
    expect(lines).toHaveLength(2);
    const r = reports()[1];
    expect(r.bursts).toBe(1);
    // Ten frames followed an input. The idle frames after the last one drew nothing and are left out.
    expect(r.frame.n).toBe(10);
    expect(r.frame.p95).toBe(16);
    expect(r.draw).toEqual({ n: 10, p50: 3, p95: 3, max: 3 });
    expect(r.drawCalls.max).toBe(500);
    // The loop stopped with the burst, so stepping on logs nothing more.
    clock.step(50);
    expect(lines).toHaveLength(2);
  });

  it("ignores a frame's cost outside a burst, as a highlight's redraw is not a pan", () => {
    const { p, reports } = profile();
    p.sheet("webgl", {});
    p.rendered(40, 1, 9);
    expect(p.report().draw.n).toBe(0);
    expect(reports()).toHaveLength(1);
  });

  it("starts a sheet's record afresh", () => {
    const { p, clock } = profile();
    p.sheet("webgl", {});
    p.input();
    clock.step(Math.ceil(BURST_IDLE_MS / 16) + 2);
    expect(p.report().bursts).toBe(1);
    p.sheet("svg", { elements: 3000 });
    expect(p.report()).toMatchObject({ renderer: "svg", bursts: 0, frame: { n: 0 }, scene: { elements: 3000 } });
  });

  // A report is pasted into a public ticket from a private board, so it may carry no text but the
  // renderer's and the machine's names.
  it("holds no text but the renderer's and the GPU's names", () => {
    const { p } = profile();
    p.sheet("webgl", { primitives: 1 });
    const texts: string[] = [];
    const walk = (v: unknown, path: string) => {
      if (typeof v === "string") texts.push(path);
      else if (v && typeof v === "object") for (const [k, x] of Object.entries(v)) walk(x, path ? `${path}.${k}` : k);
    };
    walk(p.report(), "");
    expect(texts.sort()).toEqual(["env.gpu", "env.gpuVendor", "renderer"]);
  });
});

describe("stat", () => {
  it("takes the nearest rank and summarizes nothing as zeros", () => {
    expect(stat([])).toEqual({ n: 0, p50: 0, p95: 0, max: 0 });
    expect(stat([5, 1, 3, 2, 4])).toEqual({ n: 5, p50: 3, p95: 5, max: 5 });
  });
});
