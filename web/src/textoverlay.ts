import { DEFAULT_FONT_STACK, type Label, type Image } from "./packed.js";
import type { CameraView } from "./camera.js";

const SVGNS = "http://www.w3.org/2000/svg";

// Browsers clamp an SVG <text> font-size to a few thousand px, applied before any ancestor
// transform. The overlay bakes text at world coordinates and shrinks the layer with one tiny CSS
// scale, so a nanometer-scale font-size (1.27e6 user units for 1.27 mm) hits the clamp and then
// shrinks to a sub-pixel glyph, taking the images on the same layer with it. So the overlay divides
// the baked space by a per-sheet factor that puts the largest font-size near SAFE_FONT_UNITS, and
// update() multiplies the CSS scale back by it. The clamp ignores a CSS scale, so geometry is unchanged.
const SAFE_FONT_UNITS = 2000;

// bakeDivisor picks the per-sheet divisor that brings the largest font-size near SAFE_FONT_UNITS.
// It never goes below 1, so a sheet already under the clamp is baked unchanged.
export function bakeDivisor(maxFontUnits: number): number {
  return maxFontUnits > SAFE_FONT_UNITS ? maxFontUnits / SAFE_FONT_UNITS : 1;
}

// imageAttrs computes the SVG <image> attributes for a sheet image in the overlay's Y-flipped
// world space. The image's world min corner is (x, y) and it spans w/h, so its top edge is the
// world max-Y, and baked (y -> -y) it is -(y + h), the SVG top-left an image draws down from.
// Rotation and mirror follow core/render/svg.go's drawImage about the image center, in the baked
// space. k is the shared bake multiplier (1 / bakeScale) that keeps images in the labels' user
// space, and defaults to 1 (no rescale).
export function imageAttrs(
  im: Image,
  k = 1,
): {
  x: number;
  y: number;
  width: number;
  height: number;
  href: string;
  transform?: string;
} {
  const bakedY = -(im.y + im.h);
  const attrs = { x: im.x * k, y: bakedY * k, width: im.w * k, height: im.h * k, href: im.href };
  if (!im.rotationDeg && !im.mirror) return attrs;
  const cx = (im.x + im.w / 2) * k;
  const cy = (bakedY + im.h / 2) * k;
  let t = im.rotationDeg ? `rotate(${im.rotationDeg} ${cx} ${cy})` : "";
  if (im.mirror) t += `${t ? " " : ""}translate(${2 * cx} 0) scale(-1 1)`;
  return { ...attrs, transform: t };
}

// naturalTextWidth estimates a run's rendered width in fontSize units as ~0.6*fontSize per code
// point, mirroring core/render/svg.go's naturalTextWidthPx. setContent uses it to decide whether a
// box-bounded caption needs condensing. DEFAULT_FONT_STACK is proportional, so 0.6 is an average;
// core/render/svg.go's glyphAdvanceEm carries the calibration and both must move together.
export function naturalTextWidth(text: string, fontSize: number): number {
  return 0.6 * fontSize * [...text].length;
}

// lineHeight is the multiplier on font size for stacking multi-line label text. It mirrors
// core/render/svg.go's lineHeight so both backends stack the same way.
export const lineHeight = 1.2;

// splitLabelLines splits a label's (possibly multi-line) text into per-line runs, each stacked one
// line height below the previous in the baked Y-down space. EDIF %10% decodes to a real newline
// (an OrCAD table-of-contents sheet list, say), which an SVG <text> does not break on, so the
// overlay emits one <text> per line as core/render/svg.go's drawText does. A single-line label
// yields one run at bakedY0.
export function splitLabelLines(
  text: string,
  bakedY0: number,
  fontPx: number,
  justify = "",
): { text: string; y: number }[] {
  const lines = text.split("\n");
  const step = fontPx * lineHeight;
  // A justify anchors the whole BLOCK, not its first line, so only a top-anchored block starts at
  // the anchor; bottom grows up from it and centered grows both ways. Mirrors core/render/svg.go's
  // blockTop, whose comment says why this is not cosmetic. The baked space is Y-DOWN (labels are
  // stored at -y), so "up" is a NEGATIVE offset here, same sign as the SVG backend.
  const span = (lines.length - 1) * step;
  let top = bakedY0;
  if (lines.length > 1 && !justify.includes("top")) {
    top = justify.includes("bottom") ? bakedY0 - span : bakedY0 - span / 2;
  }
  return lines.map((line, n) => ({ text: line, y: top + n * step }));
}

// justify -> SVG text-anchor / dominant-baseline, mirroring core/render/svg.go's justifyText so the
// overlay aligns text the same way the SVG oracle does.
export function labelAnchor(justify: string): string {
  if (justify.includes("left")) return "start";
  if (justify.includes("right")) return "end";
  return "middle";
}
export function labelBaseline(justify: string): string {
  if (justify.includes("top")) return "text-before-edge";
  if (justify.includes("bottom")) return "text-after-edge";
  return "central";
}

// overlayTransform maps world -> CSS pixels for the whole label layer as a uniform scale plus a
// translation, applied as one GPU-composited CSS transform. worldPerPixel is world units per
// DEVICE pixel and the overlay is in CSS pixels, so scale divides by dpr. With labels baked at
// (x, -y), screen = (tx + x*scale, ty - y*scale), i.e. Y-up with upright glyphs.
export function overlayTransform(
  view: CameraView,
  cssW: number,
  cssH: number,
  dpr: number,
): { scale: number; tx: number; ty: number } {
  const scale = 1 / (view.worldPerPixel * dpr);
  return { scale, tx: cssW / 2 - view.centerX * scale, ty: cssH / 2 + view.centerY * scale };
}

// TextOverlay draws schematic text over the WebGL canvas as an SVG layer, since a GPU line
// pipeline renders no glyphs. Text nodes are positioned once per sheet in a Y-flipped world
// space (y -> -y) so glyphs stay upright; pan/zoom is a single CSS transform on the <svg>
// layer (translate + uniform scale), which the browser composites on the GPU in lockstep with
// the canvas, so per-frame cost is one style write and no per-node relayout. pointer-events
// are off, so the layer never intercepts canvas input; hide() removes it in non-WebGL modes.
export class TextOverlay {
  private svg: SVGSVGElement;
  // Per-sheet divisor from bakeDivisor (see SAFE_FONT_UNITS). update() multiplies the CSS scale by it.
  private bakeScale = 1;

  constructor(host: HTMLElement) {
    const svg = document.createElementNS(SVGNS, "svg");
    svg.setAttribute("width", "1");
    svg.setAttribute("height", "1");
    Object.assign(svg.style, {
      position: "absolute",
      left: "0",
      top: "0",
      overflow: "visible", // text sits at world coords, well outside the 1x1 box
      transformOrigin: "0 0",
      pointerEvents: "none",
    });
    host.appendChild(svg);
    this.svg = svg;
  }

  // setContent rebuilds the overlay for a new sheet. Raster images go first so text draws over
  // them, as the SVG backend layers them, then the text nodes in the given font family (chosen
  // server-side to match the SVG backend). Positions are baked in Y-flipped world space, and
  // update() does pan/zoom without touching them.
  setContent(images: Image[], labels: Label[], fontFamily: string): void {
    this.svg.style.fontFamily = fontFamily || DEFAULT_FONT_STACK;
    this.svg.replaceChildren();
    // Images share the layer, so they bake with the same multiplier as the labels.
    const maxFont = labels.reduce((mx, l) => Math.max(mx, l.height), 0);
    this.bakeScale = bakeDivisor(maxFont);
    const k = 1 / this.bakeScale;
    for (const im of images) {
      const a = imageAttrs(im, k);
      const el = document.createElementNS(SVGNS, "image");
      el.setAttribute("x", String(a.x));
      el.setAttribute("y", String(a.y));
      el.setAttribute("width", String(a.width));
      el.setAttribute("height", String(a.height));
      el.setAttribute("href", a.href);
      if (a.transform) el.setAttribute("transform", a.transform);
      this.svg.appendChild(el);
    }
    for (const l of labels) {
      const x = l.x * k;
      const fontPx = l.height * k;
      // One <text> per line; see splitLabelLines.
      for (const line of splitLabelLines(l.text, -l.y * k, fontPx, l.justify)) {
        const t = document.createElementNS(SVGNS, "text");
        t.setAttribute("x", String(x));
        t.setAttribute("y", String(line.y));
        t.setAttribute("font-size", String(fontPx));
        t.setAttribute("fill", l.color);
        t.setAttribute("text-anchor", labelAnchor(l.justify));
        t.setAttribute("dominant-baseline", labelBaseline(l.justify));
        // A box-bounded caption (maxWidth > 0) is condensed to fit its symbol box width rather than
        // spilling past it, matching core/render/svg.go's drawText. textLength forces the run to that
        // width (baked by k, like x/height) and lengthAdjust squeezes glyphs horizontally, keeping
        // the font height. Only when it would otherwise overflow, so a short caption is not stretched.
        if (l.maxWidth > 0) {
          const maxW = l.maxWidth * k;
          if (naturalTextWidth(line.text, fontPx) > maxW) {
            t.setAttribute("textLength", String(maxW));
            t.setAttribute("lengthAdjust", "spacingAndGlyphs");
          }
        }
        // Rotation is per-label and static (world CCW degrees; the baked Y-down space makes SVG's
        // clockwise-positive rotate() match, same as the SVG backend).
        if (l.rotationDeg) t.setAttribute("transform", `rotate(${l.rotationDeg} ${x} ${line.y})`);
        t.textContent = line.text;
        this.svg.appendChild(t);
      }
    }
  }

  // update applies overlayTransform from the same camera the WebGL renderer uses.
  update(view: CameraView, cssW: number, cssH: number, dpr: number): void {
    const { scale, tx, ty } = overlayTransform(view, cssW, cssH, dpr);
    // Multiply by bakeScale to undo the per-sheet shrink baked into the coordinates (see setContent).
    this.svg.style.transform = `translate(${tx}px, ${ty}px) scale(${scale * this.bakeScale})`;
  }

  show(): void {
    this.svg.style.display = "";
  }
  hide(): void {
    this.svg.style.display = "none";
  }
}
