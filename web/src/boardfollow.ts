// How a board view turns to a selection and shows its layers (agni issue 1587). Say a reader clicks a
// capacitor on the schematic, and the board places it on the bottom. A board view seen from the top
// centres on it and still shows the top copper, so the part is centred and hidden. This module says
// which side to show and which layer rows to turn on, from what the engine says about the selection.
// It is pure, so the board view applies its answer and the tests read it directly.

import type { BoardLayerRow } from "./gen/agni/v1/geom/geom_packed_pb.js";

// BoardFacts is where the board places one selected subject, from LocateSubjects. part is set for a
// part or a pin, which sit on a side. side is "front", "back", or "" when the board states neither.
// layers name the copper layers that carry the subject, as the board's layer rows name them.
export interface BoardFacts {
  part: boolean;
  side: string;
  layers: readonly string[];
}

// BoardFollow is what a board view does about a selection. side is the side to show, or undefined to
// keep the one shown. show names the layer rows to turn on. The view hides nothing it shows already.
export interface BoardFollow {
  side?: "front" | "back";
  show: Set<string>;
}

// followBoard turns to the side every selected part and pin shares. A selection on both sides, a part
// whose side the board does not state, or a selection with no part keeps the view's side. It shows
// each copper row a subject uses and the silkscreen of each side a part sits on, so a selection of
// several subjects shows the union of theirs.
export function followBoard(facts: readonly BoardFacts[], rows: readonly BoardLayerRow[]): BoardFollow {
  const parts = facts.filter((f) => f.part);
  const sides = new Set(parts.map((f) => f.side));
  let side: BoardFollow["side"];
  if (sides.size === 1) {
    const only = [...sides][0];
    if (only === "front" || only === "back") side = only;
  }
  const copper = new Set(facts.flatMap((f) => f.layers));
  const silkSides = new Set(parts.map((f) => f.side).filter((s) => s !== ""));
  const show = new Set<string>();
  for (const r of rows) {
    if (r.kind === "copper" && copper.has(r.name)) show.add(r.name);
    if (r.kind === "silk" && silkSides.has(r.side)) show.add(r.name);
  }
  return { side, show };
}
