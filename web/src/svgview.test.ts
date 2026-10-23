// @vitest-environment jsdom
import { describe, it, expect } from "vitest";
import { SvgView, computeReveal, drawnOnSheet } from "./svgview.js";

describe("computeReveal (WS9-006)", () => {
  it("centers the target box in the host", () => {
    const v = computeReveal({ x: 100, y: 200, w: 80, h: 80 }, 800, 600);
    // Center of the box maps to the center of the host.
    expect((100 + 40) * v.scale + v.tx).toBeCloseTo(400);
    expect((200 + 40) * v.scale + v.ty).toBeCloseTo(300);
  });

  it("zooms so the target fills a fraction of the view, limited by the tighter axis", () => {
    const v = computeReveal({ x: 0, y: 0, w: 100, h: 50 }, 1000, 250);
    // Width would allow 1000/(100*2.5)=4, height allows 250/(50*2.5)=2, so height wins.
    expect(v.scale).toBeCloseTo(2);
  });

  it("floors a degenerate target (dot, zero-height wire) instead of zooming unboundedly", () => {
    const dot = computeReveal({ x: 10, y: 10, w: 0, h: 0 }, 800, 800);
    const wire = computeReveal({ x: 0, y: 5, w: 500, h: 0 }, 800, 800);
    expect(dot.scale).toBeCloseTo(800 / (40 * 2.5));
    // The wire's width still bounds the zoom; only its zero height is floored.
    expect(wire.scale).toBeCloseTo(800 / (500 * 2.5));
    // Still centered on the true (unfloored) box.
    expect(10 * dot.scale + dot.tx).toBeCloseTo(400);
  });
});

describe("setOverlays (WS9-007)", () => {
  it("stacks several overlay documents pinned to the origin, and setOverlay stays single-doc", () => {
    const host = document.createElement("div");
    document.body.appendChild(host);
    const view = new SvgView(host);
    view.setOverlays(['<svg data-k="b"></svg>', '<svg data-k="ghost"></svg>']);
    const svgs = [...host.querySelectorAll("svg")];
    expect(svgs.map((s) => s.getAttribute("data-k"))).toEqual(["b", "ghost"]);
    for (const s of svgs) {
      expect((s as SVGElement).style.position).toBe("absolute");
      expect((s as SVGElement).style.left).toBe("0px");
    }
    view.setOverlay('<svg data-k="one"></svg>');
    expect([...host.querySelectorAll("svg")].map((s) => s.getAttribute("data-k"))).toEqual(["one"]);
    view.setOverlay("");
    expect(host.querySelectorAll("svg").length).toBe(0);
  });
});

// A pane that changes size after the drawing was framed used to leave the drawing framed for the
// old one. The WebGL canvas has always observed its element, and this view never did. A boot layout
// that sizes the columns after the first render made that visible on every load, but dragging a
// splitter or opening a panel did the same thing.
describe("refit on resize", () => {
  function harness(): { host: HTMLElement; view: SvgView; resize: (w: number, h: number) => void } {
    let fire = (): void => {};
    class RO {
      constructor(private readonly cb: () => void) {
        fire = this.cb;
      }
      observe(): void {}
      disconnect(): void {}
    }
    (globalThis as unknown as { ResizeObserver: unknown }).ResizeObserver = RO;

    const host = document.createElement("div");
    document.body.appendChild(host);
    const size = (w: number, h: number): void => {
      Object.defineProperty(host, "clientWidth", { configurable: true, value: w });
      Object.defineProperty(host, "clientHeight", { configurable: true, value: h });
    };
    size(400, 400);
    const view = new SvgView(host);
    return { host, view, resize: (w, h) => (size(w, h), fire()) };
  }

  const scaleOf = (host: HTMLElement): number =>
    Number(/scale\(([\d.]+)\)/.exec((host.firstElementChild as HTMLElement).style.transform)?.[1] ?? NaN);

  it("refits a freshly framed drawing when the pane grows", () => {
    const { host, view, resize } = harness();
    view.setSvg('<svg width="200" height="200"></svg>');
    expect(scaleOf(host)).toBeCloseTo(2); // 400/200

    resize(800, 800);
    expect(scaleOf(host)).toBeCloseTo(4);
  });

  it("leaves a drawing the reader has navigated where they put it", () => {
    const { host, view, resize } = harness();
    view.setSvg('<svg width="200" height="200"></svg>');

    // A drag is the user taking the camera; after it, a resize must not re-frame their view.
    host.dispatchEvent(new MouseEvent("mousedown", { clientX: 0, clientY: 0, bubbles: true }));
    window.dispatchEvent(new MouseEvent("mousemove", { clientX: 40, clientY: 0, bubbles: true }));
    window.dispatchEvent(new MouseEvent("mouseup", { bubbles: true }));
    const afterDrag = scaleOf(host);

    resize(800, 800);
    expect(scaleOf(host)).toBeCloseTo(afterDrag);
  });

  it("ignores a parked panel measuring 0x0 rather than zeroing the scale", () => {
    const { host, view, resize } = harness();
    view.setSvg('<svg width="200" height="200"></svg>');
    resize(0, 0);
    expect(scaleOf(host)).toBeCloseTo(2);
  });

  // A new document arrives unframed, so the reader's camera on the previous one must not survive it.
  it("resumes refitting after a new document is set", () => {
    const { host, view, resize } = harness();
    view.setSvg('<svg width="200" height="200"></svg>');
    host.dispatchEvent(new MouseEvent("mousedown", { clientX: 0, clientY: 0, bubbles: true }));
    window.dispatchEvent(new MouseEvent("mousemove", { clientX: 40, clientY: 0, bubbles: true }));
    window.dispatchEvent(new MouseEvent("mouseup", { bubbles: true }));

    view.setSvg('<svg width="200" height="200"></svg>');
    resize(800, 800);
    expect(scaleOf(host)).toBeCloseTo(4);
  });
});

// A click picks what it lands on, and a click on nothing keyed names the sheet (agni issue 902). A pan
// is neither, which is the rule the click-versus-pan slop already enforces for picks.
describe("a click on the drawing", () => {
  function clickAt(host: HTMLElement, hit: Element | null, moveBy = 0): void {
    const real = document.elementFromPoint;
    document.elementFromPoint = (() => hit) as typeof document.elementFromPoint;
    try {
      host.dispatchEvent(new MouseEvent("mousedown", { clientX: 10, clientY: 10, bubbles: true }));
      if (moveBy) window.dispatchEvent(new MouseEvent("mousemove", { clientX: 10 + moveBy, clientY: 10, bubbles: true }));
      window.dispatchEvent(new MouseEvent("mouseup", { clientX: 10 + moveBy, clientY: 10, bubbles: true }));
    } finally {
      document.elementFromPoint = real;
    }
  }

  function setup() {
    const host = document.createElement("div");
    document.body.appendChild(host);
    const view = new SvgView(host);
    const picks: unknown[] = [];
    let empties = 0;
    view.onPick = (sel) => picks.push(sel);
    view.onPickEmpty = () => empties++;
    return { host, picks, empties: () => empties };
  }

  it("on a keyed element picks it and does not name the sheet", () => {
    const { host, picks, empties } = setup();
    const el = document.createElementNS("http://www.w3.org/2000/svg", "rect");
    el.setAttribute("data-kind", "component");
    el.setAttribute("data-ref", "U1");
    clickAt(host, el);
    expect(picks).toEqual([{ kind: "component", ref: "U1" }]);
    expect(empties()).toBe(0);
  });

  it("on empty paper names the sheet", () => {
    const { host, picks, empties } = setup();
    clickAt(host, null);
    expect(picks).toEqual([]);
    expect(empties()).toBe(1);
  });

  it("that pans names nothing", () => {
    const { host, picks, empties } = setup();
    clickAt(host, null, 40);
    expect(picks).toEqual([]);
    expect(empties()).toBe(0);
  });
});

// agni issue 904: Shift turns a drag into an area and a click into a toggle.
describe("a Shift gesture on the drawing", () => {
  function setup() {
    const host = document.createElement("div");
    document.body.appendChild(host);
    const view = new SvgView(host);
    const areas: unknown[] = [];
    const toggles: unknown[] = [];
    const picks: unknown[] = [];
    view.onArea = (r) => areas.push(r);
    view.onToggle = (sel) => toggles.push(sel);
    view.onPick = (sel) => picks.push(sel);
    return { host, areas, toggles, picks };
  }
  function shiftGesture(host: HTMLElement, hit: Element | null, moveBy: number): void {
    const real = document.elementFromPoint;
    document.elementFromPoint = (() => hit) as typeof document.elementFromPoint;
    try {
      host.dispatchEvent(new MouseEvent("mousedown", { clientX: 10, clientY: 10, shiftKey: true, bubbles: true }));
      if (moveBy) window.dispatchEvent(new MouseEvent("mousemove", { clientX: 10 + moveBy, clientY: 10 + moveBy, shiftKey: true, bubbles: true }));
      window.dispatchEvent(new MouseEvent("mouseup", { clientX: 10 + moveBy, clientY: 10 + moveBy, shiftKey: true, bubbles: true }));
    } finally {
      document.elementFromPoint = real;
    }
  }

  it("draws an area rather than panning, and leaves no band behind", () => {
    const { host, areas, picks } = setup();
    shiftGesture(host, null, 40);
    expect(areas).toEqual([{ left: 10, top: 10, right: 50, bottom: 50 }]);
    expect(picks).toEqual([]);
    expect(host.querySelector(".area-band")).toBeNull();
  });

  it("toggles the entity under a Shift-click instead of picking it", () => {
    const { host, toggles, picks } = setup();
    const el = document.createElementNS("http://www.w3.org/2000/svg", "rect");
    el.setAttribute("data-kind", "component");
    el.setAttribute("data-ref", "U1");
    shiftGesture(host, el, 0);
    expect(toggles).toEqual([{ kind: "component", ref: "U1" }]);
    expect(picks).toEqual([]);
  });
});

describe("drawnOnSheet", () => {
  it("lists each component and net the drawing keys, once", () => {
    const host = document.createElement("div");
    host.innerHTML =
      '<svg><rect data-kind="component" data-ref="U1"/><rect data-kind="component" data-ref="U1"/>' +
      '<polyline data-kind="net" data-net="VIN"/><circle data-kind="pin" data-ref="U1" data-pin="1"/></svg>';
    expect(drawnOnSheet(host)).toEqual({ refs: ["U1"], nets: ["VIN"] });
  });

  it("is null when the view holds no drawing", () => {
    expect(drawnOnSheet(document.createElement("div"))).toBeNull();
  });
});
