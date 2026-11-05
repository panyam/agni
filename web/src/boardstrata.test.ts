// @vitest-environment jsdom
import { describe, it, expect } from "vitest";
import { orderStrata } from "./boardstrata.js";

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
