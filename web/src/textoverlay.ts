import { DEFAULT_FONT_STACK, type Label, type Image } from "./packed.js";
import type { CameraView } from "./camera.js";

const SVGNS = "http://www.w3.org/2000/svg";

// Browsers clamp an SVG <text> font-size to a few-thousand-px maximum, applied to the specified
// value before any ancestor transform. The overlay bakes text at raw world coordinates and
// shrinks the whole layer with one tiny CSS scale, so a nanometer-scale font-size (e.g. 1.27e6
// user units for 1.27 mm) hits that clamp and, once the CSS scale is applied, collapses to a
// sub-pixel glyph — text (and, sharing the layer, images) vanishes. The overlay pre-divides the
// baked user space by a per-sheet factor so the largest font-size lands near SAFE_FONT_UNITS,
// well under the clamp, and folds the same factor back into the CSS scale in update(). A CSS
// scale is a geometric transform the clamp does not touch, so geometry is unchanged and the
// font-size is honored. Small-unit formats already under the clamp keep a factor of 1.
const SAFE_FONT_UNITS = 2000;

// bakeDivisor picks the per-sheet divisor that brings the largest label's font-size down near
// SAFE_FONT_UNITS so it clears the browser's font-size clamp (see the note above). It never goes
// below 1, so a sheet whose units are already under the clamp is baked unchanged. setContent
// divides the baked user space by it and update() multiplies the CSS scale back by it.
export function bakeDivisor(maxFontUnits: number): number {
  return maxFontUnits > SAFE_FONT_UNITS ? maxFontUnits / SAFE_FONT_UNITS : 1;
}

// imageAttrs computes the SVG <image> attributes for a sheet image in the overlay's Y-flipped
// world space. The image's world min corner is (x, y) and it spans w/h, so its top edge is the
// world max-Y; baked (y -> -y) that top edge is -(y + h), which is the SVG top-left (SVG draws
// an image downward from there, keeping it upright). Rotation/mirror mirror render/svg.go's
// drawImage about the image center, in the baked space (positive rotate, like the text nodes).
// k is the shared bake multiplier (1 / bakeScale) that keeps images in the same user space as the
// labels; it defaults to 1 so callers that do not rescale (and the unit tests) are unaffected.
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

// naturalTextWidth estimates a run's rendered width in the same units as fontSize, mirroring
// render/svg.go's naturalTextWidthPx: n code points at fontSize span ~0.6*fontSize*n. Used to
// decide whether a box-bounded caption needs condensing (see setContent). The overlay draws
// DEFAULT_FONT_STACK, a proportional face, so 0.6 is an average rather than an exact advance;
// render/svg.go's glyphAdvanceEm carries the calibration and both must move together.
export function naturalTextWidth(text: string, fontSize: number): number {
  return 0.6 * fontSize * [...text].length;
}

// lineHeight is the multiplier on font size for stacking multi-line label text; mirrors
// render/svg.go's lineHeight so both backends stack the same way.
export const lineHeight = 1.2;

// splitLabelLines splits a label's (possibly multi-line) text into per-line runs, each stacked one
// line height below the previous in the baked Y-down space. EDIF %10% decodes to a real newline
// (e.g. an OrCAD table-of-contents sheet list), which an SVG <text> does not break on its own, so
// the overlay emits one <text> per line — matching render/svg.go's drawText. A single-line label
// yields one run at bakedY0, unchanged.
export function splitLabelLines(
  text: string,
  bakedY0: number,
  fontPx: number,
  justify = "",
): { text: string; y: number }[] {
  const lines = text.split("\n");
  const step = fontPx * lineHeight;
  // A justify anchors the whole BLOCK, not its first line, so only a top-anchored block starts at
  // the anchor; bottom grows up from it and centered grows both ways. Mirrors render/svg.go's
  // blockTop — see that comment for why this is not cosmetic. The baked space is Y-DOWN (labels
  // are stored at -y), so "up" is a NEGATIVE offset here, same sign as the SVG backend.
  const span = (lines.length - 1) * step;
  let top = bakedY0;
  if (lines.length > 1 && !justify.includes("top")) {
    top = justify.includes("bottom") ? bakedY0 - span : bakedY0 - span / 2;
  }
  return lines.map((line, n) => ({ text: line, y: top + n * step }));
}

// justify -> SVG text-anchor / dominant-baseline, mirroring render/svg.go's justifyText so the
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

// overlayTransform maps world -> CSS pixels for the whole label layer: a uniform scale plus a
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
  // Per-sheet world-units-per-baked-unit divisor, chosen in setContent from the label sizes so
  // font-size clears the browser clamp (see SAFE_FONT_UNITS). update() multiplies the CSS scale
  // by it, mapping the shrunk baked space back to screen 1:1.
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

  // setContent rebuilds the overlay for a new sheet: raster images first (so text draws over
  // them, and they sit under the labels the way the SVG backend layers them), then the text
  // nodes in the given font family (chosen server-side to match the SVG backend). Positions are
  // baked in Y-flipped world space; the per-frame camera transform (update) does pan/zoom
  // without re-touching these.
  setContent(images: Image[], labels: Label[], fontFamily: string): void {
    this.svg.style.fontFamily = fontFamily || DEFAULT_FONT_STACK;
    this.svg.replaceChildren();
    // Pick the per-sheet bake divisor so the biggest label lands near SAFE_FONT_UNITS (see the note
    // on SAFE_FONT_UNITS). It is >= 1, so formats whose units already clear the clamp are untouched.
    // Images share the layer, so they are baked with the same multiplier to stay in one user space.
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
      // Multi-line labels stack one <text> per line (EDIF %10% decodes to a newline); a single-line
      // label yields one run, unchanged. See splitLabelLines / render/svg.go drawText.
      for (const line of splitLabelLines(l.text, -l.y * k, fontPx, l.justify)) {
        const t = document.createElementNS(SVGNS, "text");
        t.setAttribute("x", String(x));
        t.setAttribute("y", String(line.y));
        t.setAttribute("font-size", String(fontPx));
        t.setAttribute("fill", l.color);
        t.setAttribute("text-anchor", labelAnchor(l.justify));
        t.setAttribute("dominant-baseline", labelBaseline(l.justify));
        // A box-bounded caption (maxWidth > 0) is condensed to fit its symbol box width rather than
        // spilling past it, matching render/svg.go's drawText. textLength forces the run to that
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

  // update maps world -> CSS pixels with one GPU-composited transform, from the same camera the
  // WebGL renderer uses. worldPerPixel is world units per DEVICE pixel and the overlay is in CSS
  // pixels, so divide by dpr. Baked coordinates (x, -y) plus a positive scale keep glyphs upright.
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
