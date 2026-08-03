import { describe, it, expect } from "vitest";
import { Camera } from "./camera.js";

const bounds = { minX: 0, minY: 0, maxX: 1000, maxY: 800 };

// getView/setView must round-trip the pan/zoom so a remembered view restores exactly, and the
// restored camera must produce the same view matrix as the one it was captured from.
describe("Camera view snapshot", () => {
  it("round-trips a panned/zoomed view onto a fresh camera", () => {
    const a = new Camera(bounds);
    a.fit(640, 480);
    a.panPixels(37, -12);
    a.zoomAt(100, 200, 1.5);

    const v = a.getView();
    const b = new Camera(bounds);
    b.setView(v);
    b.setViewport(640, 480); // viewport is not part of the view; set it to match for the matrix

    expect(b.getView()).toEqual(v);
    expect(Array.from(b.matrix())).toEqual(Array.from(a.matrix()));
  });
});

// geom is Y-up (EDIF; see transform.go / svg.go), and WebGL NDC is also Y-up (clip y=+1 is
// the top of the canvas). So a higher world-Y must map toward the top of the screen, matching
// the SVG oracle. Guards against re-introducing the inverted flip that rendered WebGL
// upside-down relative to SVG.
describe("Camera Y orientation (Y-up, matches SVG)", () => {
  const cam = new Camera(bounds);
  cam.setViewport(640, 480);
  cam.setView({ centerX: 500, centerY: 400, worldPerPixel: 2 });
  // clip = M * [wx, wy, 1] with M column-major: clip.y = m[1]*wx + m[4]*wy + m[7].
  const m = cam.matrix();
  const clipY = (wx: number, wy: number): number => m[1] * wx + m[4] * wy + m[7];

  it("maps a higher world-Y toward the top (clip +Y)", () => {
    expect(clipY(500, 600)).toBeGreaterThan(0); // above center -> top
    expect(clipY(500, 200)).toBeLessThan(0); // below center -> bottom
  });

  it("pans Y-up: dragging down keeps the grabbed point under the cursor", () => {
    const c = new Camera(bounds);
    c.setViewport(640, 480);
    c.setView({ centerX: 0, centerY: 0, worldPerPixel: 1 });
    c.panPixels(0, 10); // 10px down
    expect(c.getView().centerY).toBe(10);
  });
});
