import { describe, it, expect } from "vitest";
import { orientPoint, type Orientation, type Rotation } from "./orient.js";
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

// geom is Y-up (EDIF; see internal/geomath/transform.go and core/render/svg.go), and WebGL NDC is
// also Y-up (clip y=+1 is the top of the canvas). So a higher world-Y must map toward the top of
// the screen, matching the SVG oracle. Guards against re-introducing the inverted flip that
// rendered WebGL upside-down relative to SVG.
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

// pixelToWorld inverts matrix(), so a click picks the world point drawn under it (agni issue 1332).
// The matrix is Float32, so the round trip holds to a thousandth of a pixel.
describe("Camera pixelToWorld", () => {
  it("inverts the view matrix after a pan and a zoom", () => {
    const cam = new Camera(bounds);
    cam.fit(640, 480);
    cam.panPixels(37, -12);
    cam.zoomAt(100, 200, 1.5);
    const m = cam.matrix();
    for (const [px, py] of [[0, 0], [320, 240], [639, 17], [100, 200]]) {
      const w = cam.pixelToWorld(px, py);
      const clipX = m[0] * w.x + m[3] * w.y + m[6];
      const clipY = m[1] * w.x + m[4] * w.y + m[7];
      expect((clipX + 1) * 320).toBeCloseTo(px, 3);
      expect((1 - clipY) * 240).toBeCloseTo(py, 3);
    }
  });
});

// A turned WebGL view turns as an SVG view does (agni issue 1368). orientPoint is the SVG view's map,
// in the drawing's own frame with y down, and the camera's has to agree with it.
describe("Camera orientation", () => {
  const ALL: Orientation[] = [];
  for (const rot of [0, 90, 180, 270] as Rotation[]) for (const flipH of [false, true]) for (const flipV of [false, true]) ALL.push({ rot, flipH, flipV });
  const W = 800;
  const H = 600;
  const camera = (o: Orientation): Camera => {
    const c = new Camera({ minX: 0, minY: 0, maxX: 1000, maxY: 500 });
    c.setViewport(W, H);
    c.setView({ centerX: 400, centerY: 200, worldPerPixel: 2 });
    c.setOrientation(o);
    return c;
  };
  // pixelOf runs a world point through matrix() into canvas pixels.
  const pixelOf = (c: Camera, x: number, y: number): { x: number; y: number } => {
    const m = c.matrix();
    const cx = m[0] * x + m[3] * y + m[6];
    const cy = m[1] * x + m[4] * y + m[7];
    return { x: ((cx + 1) / 2) * W, y: ((1 - cy) / 2) * H };
  };

  it("maps a world point to a pixel and back in every orientation", () => {
    for (const o of ALL) {
      const c = camera(o);
      const p = pixelOf(c, 530, 120);
      const w = c.pixelToWorld(p.x, p.y);
      expect([o, Math.round(w.x), Math.round(w.y)]).toEqual([o, 530, 120]);
    }
  });

  it("moves a point about the centre as orientPoint moves it in a y-down frame", () => {
    for (const o of ALL) {
      const c = camera(o);
      // 50 world units right of the centre and 30 above it is (25, -15) pixels, y down, upright.
      const p = pixelOf(c, 450, 230);
      const want = orientPoint(o, 0, 0, { x: 25, y: -15 });
      expect([o, Math.round(p.x - W / 2), Math.round(p.y - H / 2)]).toEqual([o, Math.round(want.x), Math.round(want.y)]);
    }
  });

  it("puts a point right of the centre below it after a quarter turn clockwise", () => {
    const p = pixelOf(camera({ rot: 90, flipH: false, flipV: false }), 500, 200);
    expect(Math.round(p.x)).toBe(W / 2);
    expect(Math.round(p.y)).toBe(H / 2 + 50);
  });

  it("keeps the grabbed point under the cursor when a turned view pans", () => {
    for (const o of ALL) {
      const c = camera(o);
      const before = c.pixelToWorld(100, 80);
      c.panPixels(37, -12);
      const after = c.pixelToWorld(137, 68);
      expect([o, Math.round(after.x), Math.round(after.y)]).toEqual([o, Math.round(before.x), Math.round(before.y)]);
    }
  });

  it("culls against a view rect that holds every corner of a turned view", () => {
    for (const o of ALL) {
      const c = camera(o);
      const r = c.viewRect();
      for (const [px, py] of [[0, 0], [W, 0], [0, H], [W, H]]) {
        const w = c.pixelToWorld(px, py);
        expect(w.x >= r.minX - 1e-6 && w.x <= r.maxX + 1e-6 && w.y >= r.minY - 1e-6 && w.y <= r.maxY + 1e-6).toBe(true);
      }
    }
  });
});
