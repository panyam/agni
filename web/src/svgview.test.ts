// @vitest-environment jsdom
import { afterEach, beforeEach, describe, it, expect } from "vitest";
import { SvgView, computeReveal, drawnOnSheet, unitAround } from "./svgview.js";

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

// A test point drawn at the end of a long leg: its pin runs from the tip at x=0 to the body at x=200,
// and the body is a 20-unit circle there. A second unit of the same part is drawn far away.
const LEG = { x: 0, y: 95, w: 200, h: 10 };
const BODY = { x: 200, y: 90, w: 20, h: 20 };
const FAR_UNIT = { x: 2000, y: 2000, w: 50, h: 50 };

describe("unitAround (agni issue 1419)", () => {
  it("grows from a pin's leg into the body the leg ends on", () => {
    expect(unitAround(LEG, [BODY])).toEqual({ x: 0, y: 90, w: 220, h: 20 });
  });

  it("leaves out a unit of the same part that nothing touches", () => {
    expect(unitAround(LEG, [FAR_UNIT, BODY])).toEqual({ x: 0, y: 90, w: 220, h: 20 });
  });

  it("follows a chain of touching boxes, so a label beside the body comes along", () => {
    const label = { x: 221, y: 90, w: 30, h: 8 };
    expect(unitAround(LEG, [label, BODY])).toEqual({ x: 0, y: 90, w: 251, h: 20 });
  });

  it("keeps the seed when the part draws nothing near it", () => {
    expect(unitAround(LEG, [])).toEqual(LEG);
  });
});

// jsdom lays nothing out, so getBBox reads a data-box attribute that each test element carries.
describe("revealOverlay for a pin (agni issue 1419)", () => {
  const proto = (window as unknown as { SVGElement: { prototype: Record<string, unknown> } }).SVGElement.prototype;
  const had = Object.prototype.hasOwnProperty.call(proto, "getBBox");
  const before = proto.getBBox;
  beforeEach(() => {
    proto.getBBox = function (this: Element) {
      const b = this.getAttribute("data-box");
      if (!b) throw new Error("not rendered");
      const [x, y, width, height] = b.split(",").map(Number);
      return { x, y, width, height };
    };
  });
  afterEach(() => {
    if (had) proto.getBBox = before;
    else delete proto.getBBox;
  });

  const box = (b: { x: number; y: number; w: number; h: number }) => `${b.x},${b.y},${b.w},${b.h}`;
  function harness(): SvgView {
    const host = document.createElement("div");
    Object.defineProperty(host, "clientWidth", { value: 800 });
    Object.defineProperty(host, "clientHeight", { value: 600 });
    document.body.appendChild(host);
    const view = new SvgView(host);
    view.setSvg(
      `<svg width="3000" height="3000">` +
        `<line data-kind="pin" data-ref="TP3" data-pin="1" data-box="${box(LEG)}"/>` +
        `<circle data-kind="component" data-ref="TP3" data-box="${box(BODY)}"/>` +
        `<rect data-kind="component" data-ref="TP3" data-box="${box(FAR_UNIT)}"/>` +
        `<circle data-kind="component" data-ref="TP4" data-box="200,110,20,20"/>` +
        `</svg>`,
    );
    // The overlay lights the pin as a dot at its tip.
    view.setOverlay(`<svg width="3000" height="3000"><circle data-box="-2,98,4,4"/></svg>`);
    return view;
  }
  const centre = (v: { tx: number; ty: number; scale: number }) => ({ x: (400 - v.tx) / v.scale, y: (300 - v.ty) / v.scale });

  it("frames the pin's leg and the body it ends on, centred on the unit", () => {
    const v = harness().revealOverlay({ ref: "TP3", pin: "1" })!;
    // The unit spans x 0..220 and y 90..110 (the dot pokes out to x=-2), so its middle is x=109.
    expect(centre(v).x).toBeCloseTo(109);
    expect(centre(v).y).toBeCloseTo(100);
    // 222 units wide in an 800 px host at a 2.5 window.
    expect(v.scale).toBeCloseTo(800 / (222 * 2.5));
  });

  it("frames the dot alone when the caller names no pin", () => {
    const v = harness().revealOverlay()!;
    expect(centre(v).x).toBeCloseTo(0);
    expect(centre(v).y).toBeCloseTo(100);
  });

  it("frames the dot alone for a pin the drawing does not key", () => {
    const v = harness().revealOverlay({ ref: "U9", pin: "1" })!;
    expect(centre(v).x).toBeCloseTo(0);
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

// A click picks what is under it, and a click on nothing keyed names the sheet (agni issue 902). A pan
// is neither. The click-versus-pan slop already enforces that rule for picks.
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
