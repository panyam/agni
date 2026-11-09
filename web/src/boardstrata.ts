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

// StrataRow is the part of a BoardLayerRow that strataDrawOrder reads.
export interface StrataRow {
  kind: string;
  groups: readonly number[];
}

// The packed groups of each footprint's pick area (core/render/packboard.go). They are the last two
// u8s and sit on their side's copper rows.
const PICK_BACK = 254;
const PICK_FRONT = 255;

// strataDrawOrder is the order a WebGL board view draws its primitives in, as orderStrata orders an
// SVG board's strata (agni issue 1368). PackBoard packs from the back of the board to the front, so
// the view from above draws in pack order and the answer is null. The view from below draws the
// zones, the pick areas and the copper each from the front to the back, nearest last, and keeps
// every other primitive in its place. Within one layer the pack order stands, so a pour still draws
// under its tracks. A copper row's place in rows sets its depth, since rows list the copper from the
// front to the back.
export function strataDrawOrder(groups: ArrayLike<number>, rows: readonly StrataRow[], fromBelow: boolean): Int32Array | null {
  if (!fromBelow) return null;
  const zone = new Set<number>();
  for (const r of rows) if (r.kind === "zones") for (const g of r.groups) zone.add(g);
  const depth = new Map<number, number>();
  rows
    .filter((r) => r.kind === "copper")
    .forEach((r, i) => {
      for (const g of r.groups) if (!depth.has(g)) depth.set(g, i);
    });
  const family = (g: number): number => (zone.has(g) ? 0 : g === PICK_BACK || g === PICK_FRONT ? 1 : 2);
  const slots: number[] = [];
  const strata: number[] = [];
  for (let i = 0; i < groups.length; i++) {
    if (!depth.has(groups[i])) continue;
    slots.push(i);
    strata.push(i);
  }
  strata.sort((a, b) => family(groups[a]) - family(groups[b]) || depth.get(groups[a])! - depth.get(groups[b])! || a - b);
  const order = new Int32Array(groups.length);
  for (let i = 0; i < groups.length; i++) order[i] = i;
  slots.forEach((slot, k) => (order[slot] = strata[k]));
  return order;
}
