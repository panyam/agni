// What the reader clicked, and what to ask about it.
//
// A Selection is deliberately the same shape as a finding's subject (kind + ref/pin/net), because
// that is what lets three entry points converge: clicking the drawing, clicking a search result, and
// clicking a finding all produce one value, so the highlighter, the query generator and (later) the
// entity panel each take one input type rather than three.
//
// Resolution is view-local by design (CONSTRAINTS C11: the view turns a cursor position into a
// semantic intent). This module holds the parts that are NOT view-local — reading a keyed element,
// ranking overlapping candidates, and writing the query — so the SVG and WebGL views can differ in
// how they find candidates while agreeing on what a candidate means.

export type SelectionKind = "pin" | "component" | "bus" | "net";

// Selection names one entity. ref is set for a component or pin, net/netId for a net, busId for a
// bus; pin is the designator within ref. The optional fields mirror the finding subject exactly.
export interface Selection {
  kind: SelectionKind;
  ref?: string;
  pin?: string;
  net?: string;
  netId?: string;
  busId?: string;
}

// PRIORITY ranks overlapping candidates, most specific first. A click inside a symbol that lands on
// a pin means the pin: the pin is the thing with the least area, so the reader who hit it meant it.
// Topmost-wins would give the symbol body every time, since it is drawn over its own pins.
const PRIORITY: SelectionKind[] = ["pin", "component", "bus", "net"];

// selectionFromElement reads one keyed element, or null for an unkeyed one (the page rect, a label,
// a highlight overlay). The renderer writes these attributes; see core/render/svg.go's entityKeys.
export function selectionFromElement(el: Element | null): Selection | null {
  const kind = el?.getAttribute("data-kind");
  if (!el || !kind) return null;
  switch (kind) {
    case "pin": {
      const ref = el.getAttribute("data-ref") ?? "";
      const pin = el.getAttribute("data-pin") ?? "";
      return ref && pin ? { kind: "pin", ref, pin } : null;
    }
    case "component": {
      const ref = el.getAttribute("data-ref") ?? "";
      return ref ? { kind: "component", ref } : null;
    }
    case "bus": {
      const busId = el.getAttribute("data-bus") ?? "";
      return busId ? { kind: "bus", busId } : null;
    }
    case "wire": {
      const net = el.getAttribute("data-net") ?? "";
      const netId = el.getAttribute("data-net-id") ?? "";
      return net || netId ? { kind: "net", net, netId } : null;
    }
    default:
      return null;
  }
}

// bestOf picks the most specific selection among overlapping candidates, in probe order so a tie
// between two of one kind goes to the nearer probe.
export function bestOf(candidates: (Selection | null)[]): Selection | null {
  for (const kind of PRIORITY) {
    const hit = candidates.find((c) => c?.kind === kind);
    if (hit) return hit;
  }
  return null;
}

// PROBES are the offsets sampled around a click, in CSS pixels: the exact point first, then a ring.
//
// A schematic wire is a 1px stroke, and hit-testing one exactly is a game of skill. Sampling a small
// ring gives it a tolerance without widening anything in the document: the alternative, an invisible
// fat stroke under every wire, would make every consumer of the SVG (a report, a saved file, a diff
// artifact) carry the viewer's interaction model. The ring is deliberately small — 5px at cursor
// scale — so that clicking BETWEEN two close wires still misses rather than guessing.
const PROBE_R = 5;
const PROBES: [number, number][] = [
  [0, 0],
  [PROBE_R, 0],
  [-PROBE_R, 0],
  [0, PROBE_R],
  [0, -PROBE_R],
  [PROBE_R, PROBE_R],
  [-PROBE_R, PROBE_R],
  [PROBE_R, -PROBE_R],
  [-PROBE_R, -PROBE_R],
];

// pickAt resolves a viewport point to an entity by asking the document what is under it, at the
// point and around it. Client coordinates, because that is what elementFromPoint takes and what a
// pointer event carries; no camera maths is needed here, which is the whole advantage of letting
// the browser hit-test its own document.
export function pickAt(doc: Document, clientX: number, clientY: number): Selection | null {
  const seen: (Selection | null)[] = [];
  for (const [dx, dy] of PROBES) {
    seen.push(selectionFromElement(doc.elementFromPoint(clientX + dx, clientY + dy)));
  }
  return bestOf(seen);
}

// queryFor writes the datalog a selection asks. This is the answer to "what is known about this
// thing", expressed in the language the query panel already speaks, which means a click TEACHES the
// language: the query it ran is sitting in the box, editable.
//
// Each is deliberately a starting point rather than an exhaustive dump. A reader edits it, and the
// edit is the moment they learn the join.
export function queryFor(sel: Selection): string {
  switch (sel.kind) {
    case "pin":
      // A pin's net first, then what that net is: the "is this wired correctly" question starts by
      // naming what the pin is attached to.
      return `pin.net("${sel.ref}", "${sel.pin}", ?net), pin.role("${sel.ref}", "${sel.pin}", ?role), net.pin_count(?net, ?fanout) => ?net, ?role, ?fanout`;
    case "component":
      return `component-on-net("${sel.ref}", ?net), net.pin_count(?net, ?fanout) => ?net, ?fanout`;
    case "net": {
      const net = sel.net ?? "";
      return `component-on-net(?ref, "${net}"), pin.net(?ref, ?pin, "${net}") => ?ref, ?pin`;
    }
    case "bus":
      return `bus("${sel.busId}", ?member) => ?member`;
  }
}

// labelFor is the one-line human name for a selection, for a status line or a panel heading.
export function labelFor(sel: Selection): string {
  switch (sel.kind) {
    case "pin":
      return `${sel.ref}.${sel.pin}`;
    case "component":
      return sel.ref ?? "";
    case "net":
      return sel.net ?? sel.netId ?? "";
    case "bus":
      return sel.busId ?? "";
  }
}
