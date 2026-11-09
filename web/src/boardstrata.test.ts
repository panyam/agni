// @vitest-environment jsdom
import { describe, it, expect } from "vitest";
import { orderStrata, strataDrawOrder } from "./boardstrata.js";

function board(): SVGSVGElement {
  const NS = "http://www.w3.org/2000/svg";
  const svg = document.createElementNS(NS, "svg");
  for (const cls of ["page", "zones-back", "zones-front", "picks-back", "picks-front", "copper-back", "copper-inner In1", "copper-inner In2", "copper-front", "through", "labels"]) {
    const g = svg.appendChild(document.createElementNS(NS, "g"));
    g.setAttribute("class", cls);
  }
  return svg;
}
const order = (svg: Element) => [...svg.children].map((c) => c.getAttribute("class"));

describe("the order a board draws its strata in", () => {
  it("draws from the front of the board, back copper nearest, in a bottom view", () => {
    const svg = board();
    orderStrata(svg, true);
    expect(order(svg)).toEqual(["page", "zones-front", "zones-back", "picks-front", "picks-back", "copper-front", "copper-inner In2", "copper-inner In1", "copper-back", "through", "labels"]);
  });

  it("comes back to the top view's order exactly", () => {
    const svg = board();
    const top = order(svg);
    orderStrata(svg, true);
    orderStrata(svg, false);
    expect(order(svg)).toEqual(top);
  });

  it("leaves a document alone when it is already drawn that way", () => {
    const svg = board();
    orderStrata(svg, true);
    const once = order(svg);
    orderStrata(svg, true);
    expect(order(svg)).toEqual(once);
  });
});

// A WebGL board view from below draws in the order an SVG one does (agni issue 1368). The groups are
// PackBoard's, in its pack order, and the rows are GetSheet's board layer rows, copper from the front.
describe("the order a WebGL board draws its primitives in", () => {
  const ZONE_F = 14, ZONE_B = 15, POUR_B = 16, IN1 = 13, SILK = 20;
  const rows = [
    { kind: "copper", groups: [6, ZONE_F, 255] },
    { kind: "copper", groups: [IN1] },
    { kind: "copper", groups: [7, ZONE_B, POUR_B, 254] },
    { kind: "edge", groups: [5] },
    { kind: "through", groups: [9, 10] },
    { kind: "pours", groups: [POUR_B] },
    { kind: "zones", groups: [ZONE_F, ZONE_B] },
    { kind: "silk", groups: [SILK] },
  ];
  const packed = [ZONE_B, ZONE_F, 254, 255, POUR_B, 7, IN1, 6, 9, 10, 5, SILK];

  it("draws in pack order from above", () => {
    expect(strataDrawOrder(packed, rows, false)).toBeNull();
  });

  it("draws each family from the front, back nearest, and keeps the rest in place from below", () => {
    const order = [...strataDrawOrder(packed, rows, true)!].map((i) => packed[i]);
    expect(order).toEqual([ZONE_F, ZONE_B, 255, 254, 6, IN1, POUR_B, 7, 9, 10, 5, SILK]);
  });
});
