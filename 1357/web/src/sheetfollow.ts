// How a view moves when something is selected elsewhere (agni issue 1326). The rule is the one #1326
// settled on. A view stays on its sheet when that sheet draws the selection, switches when it does
// not and another sheet does, and names the other sheets that draw it so the reader can open one.
// The canvas and every sheet view decide the same way, so this module holds the rule and nothing
// else.

// Link is how a view follows the rest of the page.
//   - follow: the view opens a sheet that draws the selection if its own does not, then centres on it.
//   - locked: as follow, and its pan and zoom also mirror every other locked view on the SAME sheet.
//     Two sheets have different frames, so a camera means nothing across them.
//   - off: the view lights the selection where it is drawn and never moves.
export type Link = "follow" | "locked" | "off";

export const LINKS: readonly Link[] = ["follow", "locked", "off"];

// SheetChoice is what a view does about a selection drawn on `drawnOn`. open is the sheet to switch
// to, or "" to stay. others are the sheets that also draw it, for the view to offer.
export interface SheetChoice {
  open: string;
  others: string[];
}

// chooseSheet stays when the shown sheet draws the selection, and otherwise opens the first sheet
// that does, in the design's order. Nothing drawn anywhere stays put with nothing to offer.
export function chooseSheet(current: string, drawnOn: readonly string[]): SheetChoice {
  if (drawnOn.length === 0) return { open: "", others: [] };
  if (drawnOn.includes(current)) return { open: "", others: drawnOn.filter((s) => s !== current) };
  return { open: drawnOn[0], others: drawnOn.slice(1) };
}

// LinkedView is the part of a view that camera locking reads.
export interface LinkedView {
  id: string;
  sheet: string;
  link: Link;
}

// lockedPeers lists the views whose camera mirrors `from`'s, the other locked views on its sheet.
export function lockedPeers(views: readonly LinkedView[], from: LinkedView): LinkedView[] {
  if (from.link !== "locked" || !from.sheet) return [];
  return views.filter((v) => v.id !== from.id && v.link === "locked" && v.sheet === from.sheet);
}
