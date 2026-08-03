import { describe, it, expect } from "vitest";
import { labelAnchor, labelBaseline, overlayTransform, imageAttrs, bakeDivisor, naturalTextWidth, splitLabelLines, lineHeight } from "./textoverlay.js";
import type { Image } from "./packed.js";

// The overlay's alignment mapping must mirror render/svg.go's justifyText so text lands the
// same way the SVG oracle draws it.
describe("label alignment", () => {
  it("maps horizontal justify to text-anchor", () => {
    expect(labelAnchor("left center")).toBe("start");
    expect(labelAnchor("right center")).toBe("end");
    expect(labelAnchor("center center")).toBe("middle");
    expect(labelAnchor("")).toBe("middle");
  });
  it("maps vertical justify to dominant-baseline", () => {
    expect(labelBaseline("left top")).toBe("text-before-edge");
    expect(labelBaseline("left bottom")).toBe("text-after-edge");
    expect(labelBaseline("center center")).toBe("central");
  });
});

// splitLabelLines stacks a multi-line label into one run per line (EDIF %10% decodes to a newline);
// it mirrors render/svg.go's drawText so the WebGL overlay and the SVG oracle stack the same way.
describe("splitLabelLines", () => {
  it("leaves a single-line label at its origin", () => {
    expect(splitLabelLines("hello", 10, 5)).toEqual([{ text: "hello", y: 10 }]);
  });
  it("stacks each line one line-height below the previous (Y-down baked space)", () => {
    const step = 5 * lineHeight;
    expect(splitLabelLines("a\nb\nc", 10, 5)).toEqual([
      { text: "a", y: 10 },
      { text: "b", y: 10 + step },
      { text: "c", y: 10 + 2 * step },
    ]);
  });
});

// naturalTextWidth decides whether a box-bounded caption is condensed to fit (textLength). It
// must mirror render/svg.go's naturalTextWidthPx so both backends condense the same captions.
describe("naturalTextWidth", () => {
  it("estimates ~0.6 em per code point", () => {
    expect(naturalTextWidth("Net Splitter", 10)).toBeCloseTo(0.6 * 10 * 12);
    expect(naturalTextWidth("", 10)).toBe(0);
  });
  it("triggers condensing only when wider than the box", () => {
    const box = 24; // world/baked width of the symbol box
    expect(naturalTextWidth("Net Splitter", 7)).toBeGreaterThan(box); // long caption -> condense
    expect(naturalTextWidth("Ok", 7)).toBeLessThan(box); // short caption -> left alone
  });
});

// overlayTransform maps world -> CSS pixels; combined with a baked (x, -y) coordinate it must
// place higher world-Y toward the top (smaller py), matching the WebGL geometry.
describe("overlayTransform", () => {
  const view = { centerX: 0, centerY: 0, worldPerPixel: 1 };

  it("centers the camera center in the viewport", () => {
    const { scale, tx, ty } = overlayTransform(view, 800, 600, 1);
    expect(scale).toBe(1);
    expect(tx).toBe(400);
    expect(ty).toBe(300);
  });

  it("places higher world-Y toward the top (Y-up)", () => {
    const { scale, tx, ty } = overlayTransform(view, 800, 600, 1);
    // Baked screen position of a world point: (tx + x*scale, ty + (-y)*scale).
    const py = (y: number): number => ty - y * scale;
    expect(tx).toBe(400);
    expect(py(100)).toBeLessThan(py(0)); // above center -> higher on screen (smaller py)
    expect(py(-100)).toBeGreaterThan(py(0)); // below center -> lower
  });

  it("divides world-per-device-pixel by dpr for the CSS-pixel scale", () => {
    const { scale } = overlayTransform({ centerX: 0, centerY: 0, worldPerPixel: 2 }, 800, 600, 2);
    expect(scale).toBe(1 / 4); // 1 / (worldPerPixel * dpr)
  });
});

// imageAttrs bakes the world min corner + span into upright SVG <image> attributes: the top
// edge (world max-Y = y+h) becomes the SVG top-left y (-(y+h)), so the image is not upside down.
describe("imageAttrs", () => {
  const base: Image = { x: 20, y: 30, w: 40, h: 50, href: "data:image/png;base64,AA", rotationDeg: 0, mirror: false };

  it("places the world-top edge as the SVG top-left, upright", () => {
    const a = imageAttrs(base);
    expect(a).toMatchObject({ x: 20, y: -80, width: 40, height: 50, href: base.href }); // y = -(30+50)
    expect(a.transform).toBeUndefined();
  });

  it("adds a rotate transform about the baked center when rotated", () => {
    const a = imageAttrs({ ...base, rotationDeg: 90 });
    // center: cx = 20+20 = 40, cy = -80 + 25 = -55.
    expect(a.transform).toBe("rotate(90 40 -55)");
  });

  it("mirrors across the vertical center axis", () => {
    const a = imageAttrs({ ...base, mirror: true });
    expect(a.transform).toBe("translate(80 0) scale(-1 1)"); // 2*cx = 80
  });

  it("applies the shared bake multiplier to every coordinate", () => {
    const a = imageAttrs({ ...base, rotationDeg: 90 }, 0.5);
    // Every geometry value halved so the image stays aligned with the rescaled labels.
    expect(a).toMatchObject({ x: 10, y: -40, width: 20, height: 25 });
    expect(a.transform).toBe("rotate(90 20 -27.5)"); // center halved too
  });
});

// bakeDivisor guards the font-size clamp regression: nanometer-scale fonts (~1.27e6 units) must
// be divided down under the browser's SVG font-size cap, while already-small fonts stay untouched.
describe("bakeDivisor", () => {
  it("shrinks a font-size that would exceed the browser clamp", () => {
    // 1.27 mm in nanometer units; divided to land at the SAFE_FONT_UNITS target (2000).
    expect(bakeDivisor(1_270_000)).toBeCloseTo(635);
  });
  it("leaves already-safe font sizes at a divisor of 1", () => {
    expect(bakeDivisor(1000)).toBe(1);
    expect(bakeDivisor(0)).toBe(1); // a sheet with no labels
  });
});
