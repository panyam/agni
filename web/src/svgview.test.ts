// @vitest-environment jsdom
import { describe, it, expect } from "vitest";
import { SvgView, computeReveal } from "./svgview.js";

describe("computeReveal (WS9-006)", () => {
  it("centers the target box in the host", () => {
    const v = computeReveal({ x: 100, y: 200, w: 80, h: 80 }, 800, 600);
    // Center of the box maps to the center of the host.
    expect((100 + 40) * v.scale + v.tx).toBeCloseTo(400);
    expect((200 + 40) * v.scale + v.ty).toBeCloseTo(300);
  });

  it("zooms so the target fills a fraction of the view, limited by the tighter axis", () => {
    const v = computeReveal({ x: 0, y: 0, w: 100, h: 50 }, 1000, 250);
    // Width would allow 1000/(100*2.5)=4, height allows 250/(50*2.5)=2 — height wins.
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
