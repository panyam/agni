// Stepping a schematic view through the sheets that draw the selection (agni issue 1589). Say a net
// crosses six sheets. The reader presses next, and the view opens the next sheet that draws the net
// and centres on it. The view reads "3 of 6", so the reader keeps no count. With nothing selected,
// the same keys move through every sheet. This module says where a step goes and what the view reads,
// and the view does the moving.

import { BOARD } from "./sheetfollow.js";

// SheetStep is where a view can step from the sheet it shows. prev and next are the sheets a step
// opens, undefined at either end, since stepping stops there rather than wrapping. at counts the
// shown sheet among the stops from 1, and is 0 when the shown sheet is not one of them. of counts
// the stops.
export interface SheetStep {
  prev?: string;
  next?: string;
  at: number;
  of: number;
}

// sheetStep places current among the stops. The stops are the sheets in drawnOn, in the design's
// order, or every sheet when drawnOn is empty, as it is with nothing selected. The board is never a
// stop, since a schematic view steps through schematics. From a sheet that is not a stop, such as the
// board or a sheet that does not draw the selection, next is the first stop after it in the design's
// order and prev the last one before it. From the board, next is the first stop.
export function sheetStep(order: readonly string[], drawnOn: readonly string[], current: string): SheetStep {
  const drawn = new Set(drawnOn);
  const stops = order.filter((id) => id !== BOARD && (drawn.size === 0 || drawn.has(id)));
  const at = stops.indexOf(current);
  if (at >= 0) return { prev: stops[at - 1], next: stops[at + 1], at: at + 1, of: stops.length };
  const here = current === BOARD ? -1 : order.indexOf(current);
  const rank = (id: string) => order.indexOf(id);
  const before = here < 0 ? [] : stops.filter((id) => rank(id) < here);
  const after = here < 0 ? stops : stops.filter((id) => rank(id) > here);
  return { prev: before[before.length - 1], next: after[0], at: 0, of: stops.length };
}

// stepLabel is what a view reads for its place, "3 of 6", or "6 sheets" on a sheet that is not one
// of the stops.
export function stepLabel(s: SheetStep): string {
  if (s.of === 0) return "";
  if (s.at === 0) return s.of === 1 ? "1 sheet" : `${s.of} sheets`;
  return `${s.at} of ${s.of}`;
}
