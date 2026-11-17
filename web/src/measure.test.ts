import { describe, expect, it } from "vitest";
import { Camera } from "./camera.js";
import {
  Measurement,
  formatLength,
  formatSpan,
  mmOfSvg,
  mmOfWorld,
  spanOf,
  svgBoardFrame,
  svgOfMm,
  worldOfMm,
} from "./measure.js";

describe("the measuring tool's model", () => {
  it("takes a packed world point to millimetres and back", () => {
    // A KiCad board is in nanometres. The sheet's origin is its min corner, (20 mm, -80 mm).
    const at = mmOfWorld({ x: 1_000_000, y: 2_500_000 }, 20_000_000, -80_000_000, 1e6);
    expect(at).toEqual({ x: 21, y: -77.5 });
    expect(worldOfMm(at, 20_000_000, -80_000_000, 1e6)).toEqual({ x: 1_000_000, y: 2_500_000 });
  });

  it("reads a board in another unit by its units_per_mm, not by a fixed scale", () => {
    // An IPC-2581 board in micrometres has 1000 units to the millimetre.
    expect(mmOfWorld({ x: 2540, y: 0 }, 0, 0, 1000)).toEqual({ x: 2.54, y: 0 });
  });

  it("reads BoardSVG's frame off its root and maps both ways", () => {
    const root = document.createElementNS("http://www.w3.org/2000/svg", "svg");
    root.setAttribute("data-mm-per-px", "0.0625");
    root.setAttribute("data-mm-x0", "17.5");
    root.setAttribute("data-mm-y0", "-17.5");
    const f = svgBoardFrame(root);
    expect(f).not.toBeNull();
    // The fixture's outline corner draws at (40, 40) and sits at (20, -20) mm.
    expect(mmOfSvg(40, 40, f!)).toEqual({ x: 20, y: -20 });
    expect(svgOfMm({ x: 120, y: -80 }, f!)).toEqual({ x: 1640, y: 1000 });
  });

  it("finds no frame on a drawing that states none", () => {
    const root = document.createElementNS("http://www.w3.org/2000/svg", "svg");
    expect(svgBoardFrame(root)).toBeNull();
    root.setAttribute("data-mm-per-px", "0");
    root.setAttribute("data-mm-x0", "0");
    root.setAttribute("data-mm-y0", "0");
    expect(svgBoardFrame(root)).toBeNull();
  });

  it("sets two points, then starts again on the third click", () => {
    const m = new Measurement();
    const p = (x: number, y: number) => ({ at: { x, y }, pad: null });
    m.click(p(0, 0));
    expect(m.span()).toBeNull();
    m.click(p(3, 4));
    expect(m.span()).toEqual({ dx: 3, dy: 4, d: 5 });
    m.click(p(10, 10));
    expect(m.points).toHaveLength(1);
    expect(m.span()).toBeNull();
    m.clear();
    expect(m.points).toHaveLength(0);
  });

  it("writes lengths in mm and mil", () => {
    expect(formatLength(2.54, "mm")).toBe("2.540 mm");
    expect(formatLength(2.54, "mil")).toBe("100.0 mil");
    expect(formatSpan(spanOf({ x: 0, y: 0 }, { x: 0, y: -2.54 }), "mm")).toBe("2.540 mm  (Δx 0.000, Δy -2.540)");
  });
});

describe("Camera.worldToPixel", () => {
  it("inverts pixelToWorld in every orientation", () => {
    const c = new Camera({ minX: 0, minY: 0, maxX: 2000, maxY: 1000 });
    c.setViewport(800, 600);
    c.setView({ centerX: 1000, centerY: -500, worldPerPixel: 7 });
    for (const o of [
      { rot: 0, flipH: false, flipV: false },
      { rot: 90, flipH: false, flipV: false },
      { rot: 180, flipH: true, flipV: false },
      { rot: 270, flipH: false, flipV: true },
    ] as const) {
      c.setOrientation(o);
      for (const [px, py] of [
        [0, 0],
        [123, 456],
        [799, 1],
      ]) {
        const w = c.pixelToWorld(px, py);
        const back = c.worldToPixel(w.x, w.y);
        expect(back.x).toBeCloseTo(px, 6);
        expect(back.y).toBeCloseTo(py, 6);
      }
    }
  });
});
