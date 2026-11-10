// Board layer visibility (agni 1351). The engine lists a board's layers as BoardLayerRow, each
// carrying the selector and the packed groups that hide it, so this module knows nothing about how
// a board is drawn. It turns a set of hidden row names into what each renderer applies, and keeps
// that set per design.

import type { BoardLayerRow } from "./gen/agni/v1/geom/geom_packed_pb.js";

// LayerPreset names the one-click choices above the layer rows.
export type LayerPreset = "all" | "none" | "front" | "back";

// presetHidden is the set of rows a preset hides. The front side keeps the front layers and the
// rows that belong to both sides (edge, through copper, zones), and hides the back and the inner
// layers. The back side is its mirror.
export function presetHidden(rows: BoardLayerRow[], preset: LayerPreset): Set<string> {
  const hide = (r: BoardLayerRow): boolean => {
    switch (preset) {
      case "all":
        return false;
      case "none":
        return true;
      case "front":
        return r.side === "back" || r.side === "inner";
      case "back":
        return r.side === "front" || r.side === "inner";
    }
  };
  return new Set(rows.filter(hide).map((r) => r.name));
}

// defaultHidden is the set of rows a board opens with hidden, before a reader has chosen any. It
// hides the fab rows. Their text is each part's value and reference, often long, and on a dense
// board it covers the copper and runs past the edges. On Jetson's board most of the 5,732 texts are
// fab text (agni issue 1486). A reader turns them on in the layer list or with the "all" preset.
export function defaultHidden(rows: BoardLayerRow[]): Set<string> {
  return new Set(rows.filter((r) => r.kind === "fab").map((r) => r.name));
}

// soloHidden shows one row alone.
export function soloHidden(rows: BoardLayerRow[], name: string): Set<string> {
  return new Set(rows.filter((r) => r.name !== name).map((r) => r.name));
}

// toggled flips one row.
export function toggled(hidden: ReadonlySet<string>, name: string): Set<string> {
  const next = new Set(hidden);
  if (!next.delete(name)) next.add(name);
  return next;
}

// LayerHiding is what the render port applies. css hides the SVG board's elements with
// display:none, which also takes them out of elementFromPoint, so a hidden layer picks nothing.
// groups are the packed groups the WebGL canvas skips.
export interface LayerHiding {
  css: string;
  groups: number[];
}

// layerHiding turns a hidden set into each renderer's form. scope is the selector of the element
// the board SVG is drawn in, so the rules reach no other drawing on the page.
export function layerHiding(rows: BoardLayerRow[], hidden: ReadonlySet<string>, scope: string): LayerHiding {
  const css: string[] = [];
  const groups = new Set<number>();
  for (const r of rows) {
    if (!hidden.has(r.name)) continue;
    if (r.svgSelector) css.push(`${scope} :is(${r.svgSelector}) { display: none; }`);
    for (const g of r.groups) groups.add(g);
  }
  return { css: css.join("\n"), groups: [...groups].sort((a, b) => a - b) };
}

// layersKey is the browser-storage key of a design's hidden layers, one per design as a board
// view's turn is (agni issue 1327).
export function layersKey(mount: string, path: string): string {
  return `agni-board-layers:${mount}/${path}`;
}

// parseHidden reads a stored hidden set back. Anything but a JSON array of strings is null, so a
// value from an older page or a hand edit shows every layer rather than failing.
export function parseHidden(raw: string | null): string[] | null {
  if (raw == null) return null;
  try {
    const v: unknown = JSON.parse(raw);
    if (Array.isArray(v) && v.every((x) => typeof x === "string")) return v;
  } catch {
    // fall through
  }
  return null;
}

// LayerStore keeps a design's hidden layers between visits. load returns null when the reader has
// never chosen for that design, and an empty list when they chose to show every layer.
export interface LayerStore {
  load(mount: string, path: string): string[] | null;
  save(mount: string, path: string, hidden: string[]): void;
}

// storageLayers is a LayerStore over browser storage. Storage can be absent, full or blocked, and
// then the board opens with every layer shown and a choice is kept for the page alone.
export function storageLayers(storage: () => Storage): LayerStore {
  return {
    load: (mount, path) => {
      try {
        return parseHidden(storage().getItem(layersKey(mount, path)));
      } catch {
        return null;
      }
    },
    save: (mount, path, hidden) => {
      try {
        // An empty list is stored too, since a board with no choice opens with its fab rows hidden.
        storage().setItem(layersKey(mount, path), JSON.stringify(hidden));
      } catch {
        // the choice still holds on this page
      }
    },
  };
}
