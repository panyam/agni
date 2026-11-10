import { describe, it, expect } from "vitest";
import { labelAnchor, labelBaseline, overlayTransform, turnedOverlayCss, TextOverlay, imageAttrs, bakeDivisor, naturalTextWidth, splitLabelLines, lineHeight, appendRuns } from "./textoverlay.js";
import { Camera } from "./camera.js";
import { orientTurn, type Rotation } from "./orient.js";
import type { Image } from "./packed.js";

// The overlay's alignment mapping must mirror core/render/svg.go's justifyText, so the overlay puts
// text where the SVG oracle draws it.
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
// it mirrors core/render/svg.go's drawText so the WebGL overlay and the SVG oracle stack the same way.
describe("splitLabelLines", () => {
  it("leaves a single-line label at its origin", () => {
    expect(splitLabelLines("hello", 10, 5)).toEqual([{ text: "hello", y: 10 }]);
  });
  it("stacks each line one line-height below the previous (Y-down baked space)", () => {
    const step = 5 * lineHeight;
    expect(splitLabelLines("a\nb\nc", 10, 5, "left top")).toEqual([
      { text: "a", y: 10 },
      { text: "b", y: 10 + step },
      { text: "c", y: 10 + 2 * step },
    ]);
  });
  // A justify anchors the whole BLOCK. A bottom-anchored note grows UP from its anchor. The tool
  // that writes these notes places the NEXT note relative to that same bottom, so a multi-line note
  // that grew down would cover whatever sits below it.
  it("grows a bottom-anchored block upward, leaving its LAST line on the anchor", () => {
    const step = 5 * lineHeight;
    expect(splitLabelLines("a\nb\nc", 10, 5, "left bottom")).toEqual([
      { text: "a", y: 10 - 2 * step },
      { text: "b", y: 10 - step },
      { text: "c", y: 10 },
    ]);
  });
  it("centers a block with no vertical justify", () => {
    const step = 5 * lineHeight;
    expect(splitLabelLines("a\nb\nc", 10, 5, "left")).toEqual([
      { text: "a", y: 10 - step },
      { text: "b", y: 10 },
      { text: "c", y: 10 + step },
    ]);
  });
});

// naturalTextWidth decides whether a box-bounded caption is condensed to fit (textLength). It
// must mirror core/render/svg.go's naturalTextWidthPx so both backends condense the same captions.
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

// imageAttrs bakes the world min corner + span into upright SVG <image> attributes. The top
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
    // Every geometry value is halved, so the image is still aligned with the rescaled labels.
    expect(a).toMatchObject({ x: 10, y: -40, width: 20, height: 25 });
    expect(a.transform).toBe("rotate(90 20 -27.5)"); // center halved too
  });
});

// bakeDivisor guards the font-size clamp regression. Nanometer-scale fonts (~1.27e6 units) must be
// divided down under the browser's SVG font-size cap, and already-small fonts must be left
// untouched.
describe("bakeDivisor", () => {
  it("shrinks a font-size that would exceed the browser clamp", () => {
    // 1.27 mm in nanometer units, divided down to the SAFE_FONT_UNITS target (2000).
    expect(bakeDivisor(1_270_000)).toBeCloseTo(635);
  });
  it("leaves already-safe font sizes at a divisor of 1", () => {
    expect(bakeDivisor(1000)).toBe(1);
    expect(bakeDivisor(0)).toBe(1); // a sheet with no labels
  });
});

// appendRuns draws a label's markup runs (geom.PackedLabel.runs) as core/render/svg.go's
// markupSpans does, so the overlay and the SVG backend draw ~{RESET} and V_{OUT} alike (agni 1344).
describe("appendRuns", () => {
  it("writes a plain run as text and a marked run as a styled tspan", () => {
    const t = document.createElementNS("http://www.w3.org/2000/svg", "text");
    appendRuns(t, [
      { text: "SWD.", kind: 0 },
      { text: "RST", kind: 1 },
      { text: "n", kind: 2 },
      { text: "2", kind: 3 },
    ]);
    expect(t.innerHTML).toBe(
      'SWD.<tspan text-decoration="overline">RST</tspan>' +
        '<tspan baseline-shift="sub" font-size="70%">n</tspan>' +
        '<tspan baseline-shift="super" font-size="70%">2</tspan>',
    );
    expect(t.textContent).toBe("SWD.RSTn2");
  });
});

// applyCss runs a point through a CSS transform list of translate, matrix and scale, the last one
// first, as the browser applies it.
function applyCss(css: string, x: number, y: number): { x: number; y: number } {
  const ops = [...css.matchAll(/(translate|matrix|scale)\(([^)]*)\)/g)].map((m) => ({ op: m[1], n: m[2].split(",").map((v) => parseFloat(v)) }));
  let p = { x, y };
  for (const { op, n } of ops.reverse()) {
    if (op === "translate") p = { x: p.x + n[0], y: p.y + n[1] };
    else if (op === "scale") p = { x: p.x * n[0], y: p.y * (n[1] ?? n[0]) };
    else p = { x: n[0] * p.x + n[2] * p.y + n[4], y: n[1] * p.x + n[3] * p.y + n[5] };
  }
  return p;
}

// A turned WebGL view turns its text with its geometry (agni issue 1368), so a label lands on the
// pixel where the camera draws its anchor.
describe("turnedOverlayCss", () => {
  const view = { centerX: 400, centerY: 200, worldPerPixel: 2 };
  const W = 800;
  const H = 600;
  it("is overlayTransform's translate and scale for an upright view", () => {
    const { scale, tx, ty } = overlayTransform(view, W, H, 1);
    expect(turnedOverlayCss(view, W, H, 1, 4, { a: 1, b: 0, c: 0, d: 1 })).toBe(`translate(${tx}px, ${ty}px) scale(${scale * 4})`);
  });

  it("puts a baked label where the turned camera draws its anchor", () => {
    for (const rot of [0, 90, 180, 270] as Rotation[]) {
      for (const flipH of [false, true]) {
        const o = { rot, flipH, flipV: false };
        const camera = new Camera({ minX: 0, minY: 0, maxX: 1000, maxY: 500 });
        camera.setViewport(W, H);
        camera.setView(view);
        camera.setOrientation(o);
        const m = camera.matrix();
        const [wx, wy] = [530, 120];
        const want = { x: ((m[0] * wx + m[3] * wy + m[6] + 1) / 2) * W, y: ((1 - (m[1] * wx + m[4] * wy + m[7])) / 2) * H };
        const bake = 4;
        const got = applyCss(turnedOverlayCss(view, W, H, 1, bake, orientTurn(o)).replace(/px/g, ""), wx / bake, -wy / bake);
        expect([o, Math.round(got.x), Math.round(got.y)]).toEqual([o, Math.round(want.x), Math.round(want.y)]);
      }
    }
  });
});

describe("mirrored labels", () => {
  it("flips a mirrored label about its anchor after turning it, as BoardSVG draws back-side text", () => {
    const host = document.createElement("div");
    const overlay = new TextOverlay(host);
    const label = { x: 10, y: 20, text: "", height: 5, rotationDeg: 0, justify: "", color: "#000", maxWidth: 0, runs: [] };
    overlay.setContent([], [{ ...label, text: "TOP" }, { ...label, text: "BOTTOM", rotationDeg: 90, mirror: true }], "");
    const transform = (text: string) => [...host.querySelectorAll("text")].find((t) => t.textContent === text)?.getAttribute("transform") ?? null;
    expect(transform("TOP")).toBeNull();
    expect(transform("BOTTOM")).toMatch(/^rotate\(90 [-\d.]+ [-\d.]+\) translate\([-\d.]+ 0\) scale\(-1 1\)$/);
  });
});

describe("board run width", () => {
  it("draws a label at exactly its width, and leaves a label without one to its font", () => {
    const host = document.createElement("div");
    const overlay = new TextOverlay(host);
    const label = { x: 10, y: 20, text: "", height: 5, rotationDeg: 0, justify: "left", color: "#000", maxWidth: 0, runs: [] };
    overlay.setContent([], [{ ...label, text: "SIZED", width: 40 }, { ...label, text: "FREE" }], "");
    const text = (t: string) => [...host.querySelectorAll("text")].find((el) => el.textContent === t);
    const sized = text("SIZED");
    const k = Number(sized?.getAttribute("font-size")) / 5;
    expect(Number(sized?.getAttribute("textLength"))).toBeCloseTo(40 * k);
    expect(sized?.getAttribute("lengthAdjust")).toBe("spacingAndGlyphs");
    expect(text("FREE")?.getAttribute("textLength")).toBeNull();
  });
});
