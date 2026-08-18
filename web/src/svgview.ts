// SvgView shows an SVG document in a host element with CSS-transform pan/zoom, so the SVG
// reference render navigates the same way as the WebGL canvas (drag to pan, wheel to zoom
// toward the cursor). The navigation math itself comes from panzoom.ts, which every Agni viewport
// shares. It is a plain view adapter — no framework, no presenter coupling; the presenter only
// calls setSvg / show / hide.
import { wheelZoomFactor, zoomAbout, panBy } from "./panzoom.js";
import { pickAt, type Selection } from "./selection.js";

// CLICK_SLOP_PX is how far the cursor may travel between press and release and still count as a
// click rather than a pan.
const CLICK_SLOP_PX = 3;

// SvgViewState is a snapshot of the SVG pan/zoom transform (CSS translate + scale). It is
// structurally panzoom.PanZoom, which is what lets the shared helpers operate on it directly.
export interface SvgViewState {
  tx: number;
  ty: number;
  scale: number;
}

export class SvgView {
  private readonly layer: HTMLDivElement;
  private readonly base: HTMLDivElement;
  private readonly overlay: HTMLDivElement;
  private tx = 0;
  private ty = 0;
  private scale = 1;
  // touched records that the reader has moved the camera, so the resize observer stops refitting.
  private touched = false;

  // onViewChange fires after a USER pan/zoom (drag or wheel), never from setView/fit, so a
  // host can mirror this view onto another SvgView (the WS9-005 synced diff canvases)
  // without the mirroring feeding back.
  onViewChange: ((v: SvgViewState) => void) | null = null;

  // onPick fires when the reader CLICKS an entity in the drawing (a press and release that did not
  // pan). The document is its own pick index — the renderer keys every element — so resolution is a
  // question for the browser, not a join against a second representation of the same picture.
  onPick: ((sel: Selection) => void) | null = null;

  constructor(private readonly host: HTMLElement) {
    this.layer = document.createElement("div");
    this.layer.style.transformOrigin = "0 0";
    this.layer.style.position = "absolute";
    this.layer.style.left = "0";
    this.layer.style.top = "0";
    // The base sheet document and, stacked exactly over it, the transparent highlight
    // overlay. Both live inside the transformed layer so pan/zoom moves them as one; the
    // server frames both documents identically (same width/height/viewBox), so a plain
    // top-left stack aligns them. The overlay ignores pointer events — input stays on the host.
    this.base = document.createElement("div");
    this.overlay = document.createElement("div");
    // Named so the highlight layer is addressable from outside: a browser test asserting what the
    // reader actually sees needs to find it, and so does anyone opening devtools on a highlight that
    // looks wrong. The base document is reachable through its own content; this one was not.
    this.overlay.className = "highlight-overlay";
    this.overlay.style.position = "absolute";
    this.overlay.style.left = "0";
    this.overlay.style.top = "0";
    this.overlay.style.pointerEvents = "none";
    this.layer.appendChild(this.base);
    this.layer.appendChild(this.overlay);
    host.appendChild(this.layer);
    this.bindInput();
    this.observeResize();
  }

  // setSvg injects a standalone SVG document and fits it to the host. The highlight overlay
  // is cleared: it described the previous document, and the caller re-fetches one framed for
  // the new sheet if highlights are active.
  setSvg(markup: string): void {
    this.base.innerHTML = markup;
    this.overlay.innerHTML = "";
    // A new document arrives unframed, so the reader's camera on the PREVIOUS one does not carry
    // over and the resize refit is live again.
    this.touched = false;
    this.fit();
  }

  // setOverlay stacks a transparent SVG document (a HighlightSheet overlay, framed like the
  // base document) above the sheet; "" clears it.
  setOverlay(markup: string): void {
    this.setOverlays(markup ? [markup] : []);
  }

  // setOverlays stacks several overlay documents on top of each other (the WS9-007 union
  // canvas composes b's highlight overlay with a's removed-ghost overlay). Each document is
  // pinned to the layer origin — inline SVGs would otherwise flow side by side — and all are
  // framed like the base document, so they superimpose exactly.
  setOverlays(markups: string[]): void {
    this.overlay.innerHTML = markups.join("");
    for (const el of this.overlay.querySelectorAll(":scope > svg")) {
      const s = (el as SVGElement).style;
      s.position = "absolute";
      s.left = "0";
      s.top = "0";
    }
  }

  // stats counts the drawable elements in the current SVG, for a readout comparable to the WebGL
  // primitive count. SVG has no vertex buffer, so it reports element count rather than vertices.
  stats(): { elements: number } {
    const svg = this.base.querySelector("svg");
    if (!svg) return { elements: 0 };
    return { elements: svg.querySelectorAll("path, line, rect, circle, ellipse, polyline, polygon, text").length };
  }

  show(): void {
    this.host.style.display = "block";
  }

  hide(): void {
    this.host.style.display = "none";
  }

  // revealOverlay pans/zooms to the highlight overlays' drawn content (WS9-006
  // click-to-locate: the focused item's overlay IS its location, so its bbox drives the
  // camera; with stacked overlays the union of their content is the target). Returns the
  // applied view so a host can mirror it onto a synced sibling, or null when there is
  // nothing to reveal (no overlay, or nothing drawn on this sheet).
  revealOverlay(): SvgViewState | null {
    let box: RevealBox | null = null;
    for (const svg of this.overlay.querySelectorAll(":scope > svg")) {
      let b: { x: number; y: number; width: number; height: number };
      try {
        b = (svg as SVGGraphicsElement).getBBox();
      } catch {
        continue; // detached/unrendered SVG (jsdom, display:none) — nothing to measure
      }
      if (b.width <= 0 && b.height <= 0) continue;
      if (!box) {
        box = { x: b.x, y: b.y, w: b.width, h: b.height };
      } else {
        const x = Math.min(box.x, b.x);
        const y = Math.min(box.y, b.y);
        box = {
          x,
          y,
          w: Math.max(box.x + box.w, b.x + b.width) - x,
          h: Math.max(box.y + box.h, b.y + b.height) - y,
        };
      }
    }
    if (!box) return null;
    const hw = this.host.clientWidth || 1;
    const hh = this.host.clientHeight || 1;
    const v = computeReveal(box, hw, hh);
    this.setView(v);
    return v;
  }

  // getView / setView snapshot and restore the pan/zoom transform, so a caller can remember a
  // view and reapply it (without re-fitting).
  getView(): SvgViewState {
    return { tx: this.tx, ty: this.ty, scale: this.scale };
  }

  setView(v: SvgViewState): void {
    this.tx = v.tx;
    this.ty = v.ty;
    this.scale = v.scale;
    this.apply();
  }

  private apply(): void {
    this.layer.style.transform = `translate(${this.tx}px, ${this.ty}px) scale(${this.scale})`;
  }

  // fit scales the SVG to contain within the host and centers it.
  private fit(): void {
    const svg = this.base.querySelector("svg");
    const hw = this.host.clientWidth || 1;
    const hh = this.host.clientHeight || 1;
    const sw = svgLen(svg, "width") || hw;
    const sh = svgLen(svg, "height") || hh;
    this.scale = Math.min(hw / sw, hh / sh) || 1;
    this.tx = (hw - sw * this.scale) / 2;
    this.ty = (hh - sh * this.scale) / 2;
    this.apply();
  }

  // observeResize refits the drawing when its pane changes size, but only while the reader has not
  // taken over the camera. The SVG view had no observer at all (the WebGL canvas has always had
  // one), so a document framed for one pane size stayed framed for it: dragging a splitter, opening
  // a panel, or a boot layout that sizes the columns after the first render all left the drawing
  // hanging off the edge of a pane it no longer fitted.
  //
  // The `touched` guard is what keeps the refit from being its own annoyance. Re-framing a view
  // somebody has zoomed into, because they nudged a splitter, throws away the thing they were
  // looking at. So a fresh document refits with its pane and a navigated one holds still.
  private observeResize(): void {
    if (typeof ResizeObserver === "undefined") return; // jsdom, and any non-browser host
    new ResizeObserver(() => {
      // A parked (closed) dock panel measures 0x0; refitting to that would zero the scale and the
      // drawing would never come back when the panel reopens. Same guard the canvas keeps.
      if (!this.host.clientWidth || !this.host.clientHeight) return;
      if (this.touched) return;
      this.fit();
    }).observe(this.host);
  }

  private bindInput(): void {
    let dragging = false;
    let lastX = 0;
    let lastY = 0;
    // downX/downY and moved separate a CLICK from a PAN: both begin with a press on the drawing, and
    // a pan that happens to end over a wire must not select it. The threshold exists because a click
    // always carries a little hand movement.
    let downX = 0;
    let downY = 0;
    let moved = false;
    this.host.addEventListener("mousedown", (e) => {
      dragging = true;
      lastX = e.clientX;
      lastY = e.clientY;
      downX = e.clientX;
      downY = e.clientY;
      moved = false;
    });
    window.addEventListener("mouseup", (e) => {
      const wasDragging = dragging;
      dragging = false;
      if (!wasDragging || moved || !this.onPick) return;
      const sel = pickAt(this.host.ownerDocument, e.clientX, e.clientY);
      if (sel) this.onPick(sel);
    });
    window.addEventListener("mousemove", (e) => {
      if (!dragging) return;
      if (Math.abs(e.clientX - downX) > CLICK_SLOP_PX || Math.abs(e.clientY - downY) > CLICK_SLOP_PX) moved = true;
      this.commitUserView(panBy(this.getView(), e.clientX - lastX, e.clientY - lastY));
      lastX = e.clientX;
      lastY = e.clientY;
    });
    this.host.addEventListener(
      "wheel",
      (e) => {
        e.preventDefault();
        const rect = this.host.getBoundingClientRect();
        this.commitUserView(
          zoomAbout(this.getView(), e.clientX - rect.left, e.clientY - rect.top, wheelZoomFactor(e.deltaY)),
        );
      },
      { passive: false },
    );
  }

  // commitUserView applies a view that came from a USER gesture, so it notifies onViewChange —
  // unlike setView, which exists for a host mirroring one canvas onto another and must not feed
  // back.
  private commitUserView(v: SvgViewState): void {
    this.tx = v.tx;
    this.ty = v.ty;
    this.scale = v.scale;
    this.apply();
    this.touched = true;
    this.onViewChange?.(this.getView());
  }
}

// svgLen reads a pixel length from an <svg> width/height attribute, tolerating a missing
// element or a unit suffix.
function svgLen(svg: SVGSVGElement | null, attr: "width" | "height"): number {
  if (!svg) return 0;
  return parseFloat(svg.getAttribute(attr) ?? "") || 0;
}

// RevealBox is a target region in the SVG document's coordinate space (the sheet documents
// are framed with a 1:1 viewBox, so getBBox units are the same px space fit() works in).
export interface RevealBox {
  x: number;
  y: number;
  w: number;
  h: number;
}

// REVEAL_WINDOW is how much context surrounds a revealed target: the view window is this
// many times the target's extent, so a component is seen with its neighborhood, not
// wall-to-wall.
const REVEAL_WINDOW = 2.5;

// REVEAL_MIN_EXTENT floors a degenerate target (a single pin dot, a zero-height horizontal
// wire) to a sensible window instead of an unbounded zoom.
const REVEAL_MIN_EXTENT = 40;

// computeReveal returns the pan/zoom that centers the target box in a host of the given
// size, zoomed so the box fills 1/REVEAL_WINDOW of the view. Pure — the DOM measurement
// (getBBox) happens in revealOverlay; this is the tested half.
export function computeReveal(b: RevealBox, hostW: number, hostH: number): SvgViewState {
  const w = Math.max(b.w, REVEAL_MIN_EXTENT);
  const h = Math.max(b.h, REVEAL_MIN_EXTENT);
  const scale = Math.min(hostW / (w * REVEAL_WINDOW), hostH / (h * REVEAL_WINDOW)) || 1;
  return {
    scale,
    tx: hostW / 2 - (b.x + b.w / 2) * scale,
    ty: hostH / 2 - (b.y + b.h / 2) * scale,
  };
}
