// SvgView shows an SVG document in a host element with CSS-transform pan/zoom, so the SVG
// reference render navigates the same way as the WebGL canvas (drag to pan, wheel to zoom
// toward the cursor). The navigation math comes from panzoom.ts, which every Agni viewport
// shares. It is a plain view adapter with no framework or presenter coupling, and the presenter
// only calls setSvg / show / hide.
import { wheelZoomFactor, zoomAbout, panBy } from "@agni/web-shared/panzoom.js";
import { CLICK_SLOP_PX, pickAt, type Selection } from "./selection.js";
import { rectOf, type ClientRect } from "./area.js";
import { AreaBand } from "./areaband.js";
import { UPRIGHT, orientBox, orientCss, orientPoint, orientedSize, unorientPoint, type Orientation, type Point } from "./orient.js";

// SvgViewState is a snapshot of the SVG pan/zoom transform (CSS translate + scale). It is
// structurally panzoom.PanZoom, so the shared helpers operate on it directly.
export interface SvgViewState {
  tx: number;
  ty: number;
  scale: number;
}

export class SvgView {
  private readonly layer: HTMLDivElement;
  // orient turns the drawing within the layer (agni issue 1327), so pan and zoom stay a translate and
  // a scale over the oriented drawing.
  private readonly orient: HTMLDivElement;
  private orientation: Orientation = UPRIGHT;
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
  // pan). The renderer keys every element, so the document is its own pick index and the browser
  // resolves the hit.
  onPick: ((sel: Selection) => void) | null = null;
  // onPickEmpty fires when a click hits nothing keyed, such as the paper, a label or the frame. That
  // click names the sheet itself rather than an entity on it (agni issue 902).
  onPickEmpty: (() => void) | null = null;
  // onArea fires when the reader Shift-drags a rectangle over the drawing, and passes the rectangle in
  // client coordinates. onToggle fires when they Shift-click an entity, to add it to the area or take
  // it out (agni issue 904). A plain drag still pans and a plain click still picks.
  onArea: ((r: ClientRect) => void) | null = null;
  onToggle: ((sel: Selection) => void) | null = null;
  // band is the rectangle drawn while a Shift-drag is under way, in the host's own space so it does not
  // pan or zoom with the drawing.
  private readonly band: AreaBand;

  constructor(private readonly host: HTMLElement) {
    this.band = new AreaBand(host);
    this.layer = document.createElement("div");
    this.layer.style.transformOrigin = "0 0";
    this.layer.style.position = "absolute";
    this.layer.style.left = "0";
    this.layer.style.top = "0";
    // The base sheet document, with the transparent highlight overlay stacked exactly over it.
    // Both live inside the transformed layer, so pan/zoom moves them as one. The server frames
    // both documents identically (same width/height/viewBox), so a plain top-left stack aligns
    // them. The overlay ignores pointer events, so input goes to the host.
    this.base = document.createElement("div");
    this.overlay = document.createElement("div");
    // Named so the highlight layer is easy to find, for a browser test asserting what the reader
    // sees or for anyone in devtools looking at a wrong highlight.
    this.overlay.className = "highlight-overlay";
    this.overlay.style.position = "absolute";
    this.overlay.style.left = "0";
    this.overlay.style.top = "0";
    this.overlay.style.pointerEvents = "none";
    this.orient = document.createElement("div");
    this.orient.style.transformOrigin = "0 0";
    this.orient.appendChild(this.base);
    this.orient.appendChild(this.overlay);
    this.layer.appendChild(this.orient);
    host.appendChild(this.layer);
    this.bindInput();
    this.observeResize();
  }

  // setSvg injects a standalone SVG document and fits it to the host. It clears the highlight
  // overlay, which described the previous document. The caller re-fetches one framed for the new
  // sheet if highlights are active.
  setSvg(markup: string): void {
    this.base.innerHTML = markup;
    this.overlay.innerHTML = "";
    this.applyOrientation();
    // A new document arrives unframed. The reader's camera on the PREVIOUS one does not apply to
    // it, and the resize refit is live again.
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
  // pinned to the layer origin, since inline SVGs would otherwise flow side by side, and all are
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

  // revealOverlay pans/zooms to the union of the highlight overlays' drawn content, since for
  // WS9-006 click-to-locate the focused item's overlay IS its location. Returns the applied view
  // so a host can mirror it onto a synced sibling, or null when there is nothing to reveal (no
  // overlay, or nothing drawn on this sheet).
  //
  // A pin's highlight is a dot at the pin's tip. Passing the pin frames the unit of its part that
  // draws it instead, since a part drawn at the end of a long leg sits outside a window sized to a
  // dot (agni issue 1419).
  revealOverlay(pin?: { ref: string; pin: string }): SvgViewState | null {
    let box: RevealBox | null = null;
    // Each drawn element is measured on its own. An <svg>'s own getBBox leaves out a child with no
    // size, and a pin's highlight is a point, a box with no size (agni issue 1326). computeReveal
    // frames a point with its minimum extent.
    for (const el of this.overlay.querySelectorAll(":scope > svg > *")) {
      const b = measure(el);
      if (b) box = box ? union(box, b) : b;
    }
    if (!box) return null;
    if (pin) box = this.unitOfPin(box, pin.ref, pin.pin);
    const hw = this.host.clientWidth || 1;
    const hh = this.host.clientHeight || 1;
    const { w, h } = this.docSize();
    const ob = orientBox(this.orientation, w, h, { x: box.x, y: box.y, w: box.w, h: box.h });
    const v = computeReveal(ob, hw, hh);
    this.setView(v);
    // A revealed view holds still as a navigated one does. Otherwise the next resize, such as a
    // panel opening beside it, refits the sheet and throws the reveal away (agni issue 1326).
    this.touched = true;
    return v;
  }

  // unitOfPin widens a pin's highlight box to the drawing of the pin and its part's unit, read off
  // the base drawing's entity keys. A pin the drawing does not key leaves the box as it was.
  private unitOfPin(lit: RevealBox, ref: string, pin: string): RevealBox {
    const svg = this.base.querySelector("svg");
    if (!svg) return lit;
    const pins: RevealBox[] = [];
    const parts: RevealBox[] = [];
    for (const el of svg.querySelectorAll('[data-kind="pin"], [data-kind="component"]')) {
      if (el.getAttribute("data-ref") !== ref) continue;
      const b = measure(el);
      if (!b) continue;
      if (el.getAttribute("data-kind") === "component") parts.push(b);
      else if (el.getAttribute("data-pin") === pin) pins.push(b);
    }
    // A pin drawn on more than one unit, such as a shared supply pin, is taken only where it
    // touches the lit dot.
    const own = pins.filter((b) => touches(b, lit));
    return unitAround(own.reduce(union, lit), parts);
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

  // getOrientation and setOrientation turn the drawing. A view the reader has moved keeps the point
  // at its centre where it is, and a fresh one refits.
  getOrientation(): Orientation {
    return this.orientation;
  }

  setOrientation(o: Orientation): void {
    const centre = this.touched ? this.getCentre() : null;
    this.orientation = o;
    this.applyOrientation();
    if (centre) this.setCentre(centre.at, centre.scale);
    else this.fit();
  }

  // getCentre is the drawing point at the middle of the view, in the drawing's own coordinates, and
  // the zoom. setCentre puts a drawing point at the middle at a zoom. Two views locked together
  // exchange these rather than a translate, so they line up whatever their sizes or orientations.
  getCentre(): { at: Point; scale: number } {
    const hw = this.host.clientWidth || 1;
    const hh = this.host.clientHeight || 1;
    const { w, h } = this.docSize();
    const o = { x: (hw / 2 - this.tx) / this.scale, y: (hh / 2 - this.ty) / this.scale };
    return { at: unorientPoint(this.orientation, w, h, o), scale: this.scale };
  }

  setCentre(at: Point, scale: number): void {
    const hw = this.host.clientWidth || 1;
    const hh = this.host.clientHeight || 1;
    const { w, h } = this.docSize();
    const o = orientPoint(this.orientation, w, h, at);
    this.scale = scale;
    this.tx = hw / 2 - o.x * scale;
    this.ty = hh / 2 - o.y * scale;
    this.touched = true;
    this.apply();
  }

  private applyOrientation(): void {
    const { w, h } = this.docSize();
    this.orient.style.transform = orientCss(this.orientation, w, h);
  }

  private docSize(): { w: number; h: number } {
    const svg = this.base.querySelector("svg");
    return { w: svgLen(svg, "width") || 1, h: svgLen(svg, "height") || 1 };
  }

  // fit scales the oriented SVG to contain within the host and centers it.
  private fit(): void {
    const svg = this.base.querySelector("svg");
    const hw = this.host.clientWidth || 1;
    const hh = this.host.clientHeight || 1;
    const turned = orientedSize(this.orientation, svgLen(svg, "width") || hw, svgLen(svg, "height") || hh);
    const sw = turned.w;
    const sh = turned.h;
    this.scale = Math.min(hw / sw, hh / sh) || 1;
    this.tx = (hw - sw * this.scale) / 2;
    this.ty = (hh - sh * this.scale) / 2;
    this.apply();
  }

  // observeResize refits the drawing when its pane changes size (a splitter drag, a panel opening,
  // a boot layout sizing the columns after the first render), but only while `touched` is false.
  // Re-framing a view somebody has zoomed into throws away what they were looking at, so a fresh
  // document refits with its pane and a navigated one holds still (#325).
  private observeResize(): void {
    if (typeof ResizeObserver === "undefined") return; // jsdom, and any non-browser host
    new ResizeObserver(() => {
      // A parked (closed) dock panel measures 0x0. Refitting to that would zero the scale, and the
      // drawing would never come back when the panel reopens. The canvas keeps the same guard.
      if (!this.host.clientWidth || !this.host.clientHeight) return;
      if (this.touched) return;
      this.fit();
    }).observe(this.host);
  }

  private bindInput(): void {
    let dragging = false;
    let lastX = 0;
    let lastY = 0;
    // downX/downY and moved separate a CLICK from a PAN. Both begin with a press on the drawing, and
    // a pan that ends over a wire must not select it. CLICK_SLOP_PX allows for the hand movement
    // every click carries.
    let downX = 0;
    let downY = 0;
    let moved = false;
    // selecting is a press with Shift held, which draws an area rather than panning (agni issue 904).
    let selecting = false;
    this.host.addEventListener("mousedown", (e) => {
      dragging = true;
      selecting = e.shiftKey && (this.onArea !== null || this.onToggle !== null);
      // A Shift-drag would otherwise select the page's text under the cursor.
      if (selecting) e.preventDefault();
      lastX = e.clientX;
      lastY = e.clientY;
      downX = e.clientX;
      downY = e.clientY;
      moved = false;
    });
    window.addEventListener("mouseup", (e) => {
      const wasDragging = dragging;
      dragging = false;
      if (wasDragging && selecting) {
        selecting = false;
        this.band.clear();
        if (moved) this.onArea?.(rectOf(downX, downY, e.clientX, e.clientY));
        else {
          const sel = pickAt(this.host.ownerDocument, e.clientX, e.clientY);
          if (sel) this.onToggle?.(sel);
        }
        return;
      }
      if (!wasDragging || moved || !this.onPick) return;
      const sel = pickAt(this.host.ownerDocument, e.clientX, e.clientY);
      if (sel) this.onPick(sel);
      else this.onPickEmpty?.();
    });
    window.addEventListener("mousemove", (e) => {
      if (!dragging) return;
      if (Math.abs(e.clientX - downX) > CLICK_SLOP_PX || Math.abs(e.clientY - downY) > CLICK_SLOP_PX) moved = true;
      if (selecting) {
        if (moved) this.band.draw(rectOf(downX, downY, e.clientX, e.clientY));
        return;
      }
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

  // commitUserView applies a view from a USER gesture, so it notifies onViewChange. setView does
  // not, because a host mirroring one canvas onto another must not feed back.
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

// REVEAL_WINDOW is the view window's size as a multiple of the revealed target's extent, so a
// component is seen with its neighborhood rather than wall-to-wall.
const REVEAL_WINDOW = 2.5;

// REVEAL_MIN_EXTENT floors a degenerate target (a single pin dot, a zero-height horizontal
// wire) to a sensible window instead of an unbounded zoom.
const REVEAL_MIN_EXTENT = 40;

// computeReveal returns the pan/zoom that centers the target box in a host of the given
// size, zoomed so the box fills 1/REVEAL_WINDOW of the view. It is pure, and the DOM
// measurement (getBBox) happens in revealOverlay, so this is the tested half.
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

// unitAround grows a box through every part box that touches it, until none is left that does.
// Seeded with a pin's drawing, it takes in the unit that draws the pin and leaves out a unit of the
// same part drawn elsewhere on the sheet, which a union of every box keyed to the part would not.
export function unitAround(seed: RevealBox, parts: RevealBox[]): RevealBox {
  let box = seed;
  let left = parts;
  for (;;) {
    const near = left.filter((b) => touches(b, box));
    if (near.length === 0) return box;
    box = near.reduce(union, box);
    left = left.filter((b) => !near.includes(b));
  }
}

// TOUCH_SLACK is how far apart two boxes may be and still touch, in document units, so a pin's leg
// that ends on its body's outline counts as meeting it despite rounding.
const TOUCH_SLACK = 1;

function touches(a: RevealBox, b: RevealBox): boolean {
  return (
    a.x <= b.x + b.w + TOUCH_SLACK && b.x <= a.x + a.w + TOUCH_SLACK && a.y <= b.y + b.h + TOUCH_SLACK && b.y <= a.y + a.h + TOUCH_SLACK
  );
}

function union(a: RevealBox, b: RevealBox): RevealBox {
  const x = Math.min(a.x, b.x);
  const y = Math.min(a.y, b.y);
  return { x, y, w: Math.max(a.x + a.w, b.x + b.w) - x, h: Math.max(a.y + a.h, b.y + b.h) - y };
}

// measure is an element's box in document units, or null for one with nothing to measure.
function measure(el: Element): RevealBox | null {
  try {
    const b = (el as SVGGraphicsElement).getBBox();
    return { x: b.x, y: b.y, w: b.width, h: b.height };
  } catch {
    return null; // detached/unrendered SVG (jsdom, display:none) has nothing to measure
  }
}

// drawnOnSheet lists what the sheet on screen draws, read off the rendered document's entity keys,
// or null when the SVG view holds no drawing (the WebGL and native modes draw elsewhere). A sheet's
// membership is a property of its drawing, so this is the one place it can be read without asking
// the server again.
export function drawnOnSheet(host: HTMLElement): { refs: string[]; nets: string[] } | null {
  const svg = host.querySelector("svg");
  if (!svg) return null;
  const refs = new Set<string>();
  const nets = new Set<string>();
  for (const el of svg.querySelectorAll('[data-kind="component"]')) {
    const r = el.getAttribute("data-ref");
    if (r) refs.add(r);
  }
  for (const el of svg.querySelectorAll('[data-kind="net"]')) {
    const n = el.getAttribute("data-net");
    if (n) nets.add(n);
  }
  return { refs: [...refs], nets: [...nets] };
}
