// An AREA is a set of parts, pins and wires selected together on one sheet (agni issue 904), by
// Shift-dragging a rectangle over the drawing or Shift-clicking entities in and out of it.
//
// What is inside is read off the rendered document, the same keyed elements a click picks, so forming
// the set asks the server nothing. Nets are never members, because a net may continue off the sheet:
// a net is classified from the area's pins instead, internal when every one of its pins is in the
// area and boundary otherwise (properties.ts).

import type { Selection } from "./selection.js";

// ClientRect is a rectangle in client coordinates, the space getBoundingClientRect answers in, so a
// drag and the drawing's elements compare with no camera maths.
export interface ClientRect {
  left: number;
  top: number;
  right: number;
  bottom: number;
}

// Members are what an area holds. refs are its whole parts, pins its pins whose part is not wholly
// inside ("REF.PIN"), and wires the nets that have a wire drawn wholly inside.
export interface Members {
  refs: string[];
  pins: string[];
  wires: string[];
}

export function emptyMembers(): Members {
  return { refs: [], pins: [], wires: [] };
}

// rectOf is the rectangle a drag from one corner to the other spans, whichever way it went.
export function rectOf(x0: number, y0: number, x1: number, y1: number): ClientRect {
  return { left: Math.min(x0, x1), top: Math.min(y0, y1), right: Math.max(x0, x1), bottom: Math.max(y0, y1) };
}

function inside(b: ClientRect, r: ClientRect): boolean {
  return b.left >= r.left && b.right <= r.right && b.top >= r.top && b.bottom <= r.bottom;
}

// membersIn is what lies wholly inside r on the drawing root. An entity drawn as several elements (a
// part is one per symbol shape, a net one per wire) is inside only when all of its elements are, for
// a part, and when any one wire element is, for a net, since a net's wires are its separate segments.
// A pin of a part that is wholly inside is the part's, not listed apart. boxOf measures an element and
// is the browser's own unless a test passes one, since jsdom has no layout.
export function membersIn(root: ParentNode, r: ClientRect, boxOf: (el: Element) => ClientRect = (el) => el.getBoundingClientRect()): Members {
  const parts = new Map<string, boolean>();
  for (const el of root.querySelectorAll('[data-kind="component"][data-ref]')) {
    const ref = el.getAttribute("data-ref") ?? "";
    parts.set(ref, (parts.get(ref) ?? true) && inside(boxOf(el), r));
  }
  const refs = [...parts].filter(([, all]) => all).map(([ref]) => ref);
  const whole = new Set(refs);
  const pins = new Set<string>();
  for (const el of root.querySelectorAll('[data-kind="pin"][data-ref][data-pin]')) {
    const ref = el.getAttribute("data-ref") ?? "";
    if (whole.has(ref) || !inside(boxOf(el), r)) continue;
    pins.add(`${ref}.${el.getAttribute("data-pin")}`);
  }
  const wires = new Set<string>();
  for (const el of root.querySelectorAll('[data-kind="net"][data-net]')) {
    const net = el.getAttribute("data-net") ?? "";
    if (net && inside(boxOf(el), r)) wires.add(net);
  }
  return { refs: refs.sort(), pins: [...pins].sort(), wires: [...wires].sort() };
}

// toggle adds the entity a Shift-click names to the area or takes it out. A pin of a part already in
// the area toggles the whole part, since the part holds it, and a bus is not an area member.
export function toggle(m: Members, sel: Selection): Members {
  const flip = (list: string[], v: string) => (list.includes(v) ? list.filter((x) => x !== v) : [...list, v].sort());
  switch (sel.kind) {
    case "component":
      return sel.ref ? { ...m, refs: flip(m.refs, sel.ref), pins: m.pins.filter((p) => !p.startsWith(sel.ref + ".")) } : m;
    case "pin": {
      if (!sel.ref || !sel.pin) return m;
      if (m.refs.includes(sel.ref)) return { ...m, refs: flip(m.refs, sel.ref) };
      return { ...m, pins: flip(m.pins, `${sel.ref}.${sel.pin}`) };
    }
    case "net":
      return sel.net ? { ...m, wires: flip(m.wires, sel.net) } : m;
    default:
      return m;
  }
}

// size is how many members an area holds, the count its breadcrumb shows.
export function size(m: Members): number {
  return m.refs.length + m.pins.length + m.wires.length;
}

// areaSelection is the Selection an area is everywhere a selection goes: the panel, the selection
// bar, an Ask row. nets is every net the area touches, through a member part's pins, a member pin or
// a wire, given pinNets (a part's pins and their nets) to reach a part's nets.
export function areaSelection(m: Members, pinNets: (ref: string) => { pin: string; net: string }[]): Selection {
  const nets = new Set(m.wires);
  for (const ref of m.refs) for (const p of pinNets(ref)) if (p.net) nets.add(p.net);
  for (const key of m.pins) {
    const dot = key.lastIndexOf(".");
    const ref = key.slice(0, dot);
    const pin = key.slice(dot + 1);
    const net = pinNets(ref).find((p) => p.pin === pin)?.net;
    if (net) nets.add(net);
  }
  return { kind: "area", refs: m.refs, pins: m.pins, wires: m.wires, nets: [...nets].sort() };
}

// membersOf reads an area Selection back into its members.
export function membersOf(sel: Selection): Members {
  return { refs: sel.refs ?? [], pins: sel.pins ?? [], wires: sel.wires ?? [] };
}
