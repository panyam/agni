// The order a board view draws its copper in (agni issue 1327). BoardSVG draws from the back of the
// board to the front, so the front copper covers the back and a front part picks over a back one. A
// bottom view looks at the board from underneath, so the back is nearest and draws last. The
// strata are the classed groups BoardSVG writes at the top of its document.

const ZONES = ["zones-back", "zones-inner", "zones-front"];
const PICKS = ["picks-back", "picks-front"];
const COPPER = ["copper-back", "copper-inner", "copper-front"];

function strataOf(el: Element): { family: number; side: number } | null {
  for (const [family, names] of [ZONES, PICKS, COPPER].entries()) {
    for (const [side, name] of names.entries()) if (el.classList.contains(name)) return { family, side };
  }
  return null;
}

// orderStrata redraws a board document's strata from the front of the board when fromBelow is set,
// and from the back otherwise. Each family keeps its place among the document's other groups, so a
// zone still draws under the copper and the labels still draw over it. The inner layers reverse
// too, so from below the one nearest the back draws last among them.
export function orderStrata(svg: Element, fromBelow: boolean): void {
  const drawnFromBelow = svg.getAttribute("data-strata") === "below";
  if (drawnFromBelow === fromBelow) return;
  const strata = [...svg.children].filter((c) => strataOf(c) !== null);
  if (strata.length === 0) return;
  const after = strata[strata.length - 1].nextSibling;
  // Each family sorts by side, nearest last. Layers on one side keep their drawn order reversed,
  // since the direction has changed, so the inner layers turn over with the board.
  const ranked = strata.map((el, i) => {
    const s = strataOf(el)!;
    return { el, family: s.family, side: fromBelow ? -s.side : s.side, i: -i };
  });
  ranked.sort((a, b) => a.family - b.family || a.side - b.side || a.i - b.i);
  for (const { el } of ranked) svg.insertBefore(el, after);
  svg.setAttribute("data-strata", fromBelow ? "below" : "above");
}
