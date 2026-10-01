// What the reader clicked, and what to ask about it.
//
// A Selection has the same shape as a finding's subject (kind + ref/pin/net), so clicking the
// drawing, a search result or a finding all produce one value for the highlighter and the query
// generator to take.
//
// Resolution is view-local (CONSTRAINTS C11, the view turns a cursor position into a semantic
// intent). This module holds the parts that are NOT view-local (reading a keyed element, ranking
// overlapping candidates, writing the query), so the SVG and WebGL views can find candidates their
// own way and still agree on what one means. The cross-renderer contract is on
// docsite/content/architecture/web-picking.md#picking-from-the-drawing.

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
// a pin means the pin, the entity with the least area. Topmost-wins would give the symbol body every
// time, since it is drawn over its own pins.
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
    case "net": {
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

// PROBES are the offsets sampled around a click in CSS pixels, the exact point first and then a
// ring that gives small targets some tolerance. The ring does not rescue a bare wire, since every
// probe faces the same sub-pixel stroke; the served render's invisible wide wire companion does
// (core/render/svg.go, WithPickTargets). The ring stays small, 5px at cursor scale, so a click
// BETWEEN two close wires still misses rather than guessing.
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
// pointer event carries, so no camera maths is needed here.
export function pickAt(doc: Document, clientX: number, clientY: number): Selection | null {
  const seen: (Selection | null)[] = [];
  for (const [dx, dy] of PROBES) {
    seen.push(selectionFromElement(doc.elementFromPoint(clientX + dx, clientY + dy)));
  }
  return bestOf(seen);
}

// entityBindings gives a preset the SERVER wrote (query.EntityQueries) the values of what was
// clicked. The preset names the variables to bind (binds), each also the selection field that fills
// it, and the query text stays as written (agni issue 793).
//
// The templates name relations defined in Go, so they live beside them, where each preset gets a
// parse check and an evaluate-against-a-real-design check. A client copy would be the one caller
// nothing checks, so renaming a relation would turn the server's tests red while every click in the
// viewer produced a query that errors. This file only turns what was clicked into values.
//
// A value is bound as itself, so a designator carrying a quote is asked about exactly. Splicing it
// into a string literal, which has no escape sequence, could only strip it.
export function entityBindings(binds: readonly string[], sel: Selection): Record<string, string> {
  const field: Record<string, string | undefined> = { ref: sel.ref, pin: sel.pin, net: sel.net, bus: sel.busId };
  const out: Record<string, string> = {};
  for (const v of binds) out[v] = field[v] ?? "";
  return out;
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

// selectionFromCell reads a query result cell, the second way a reader names an entity. The panel
// knows a cell's kind because the server typed the column (webapi RunQueryResponse.column_kinds,
// derived from the relation catalog's arg labels), and those kind strings are check.KindComponent /
// check.KindNet, the same vocabulary a picked element carries.
//
// A scalar cell, or an entity kind with no selection shape yet, yields null. The cell still locates
// but names nothing to ask about.
//
// `ref` is the other half of a pin's identity, from the row's cell_refs. A pin cell holds "5", which
// names nothing until you know it means U7's pin 5. Ignored for every other kind, where the cell
// names the entity by itself.
//
// A search can return a bus (agni issue 338), since entity() enumerates buses and a bus with no
// drawn wire is the sort of thing a reviewer looks for by name. Its subject IS its label, the key a
// drawn bus element carries in data-bus, so both entry points converge on one value.
export function selectionFromCell(kind: string, subject: string, ref = ""): Selection | null {
  if (!subject) return null;
  switch (kind) {
    case "component":
      return { kind: "component", ref: subject };
    case "net":
      return { kind: "net", net: subject };
    case "bus":
      return { kind: "bus", busId: subject };
    case "pin":
      // Both halves or nothing, since a pin number with no component names no pin.
      return ref ? { kind: "pin", ref, pin: subject } : null;
    default:
      return null;
  }
}

// sameSelection reports whether two picks name the same thing, which is how a surface marks the one
// it is currently showing. It tests identity rather than deep equality, because the canvas pushes a
// net with both its name and its id while a result cell carries only the name, and those are the
// same net. Ids are compared only when BOTH carry one, since two nets can share a display name
// across sheets.
export function sameSelection(a: Selection | null, b: Selection | null): boolean {
  if (!a || !b || a.kind !== b.kind) return false;
  switch (a.kind) {
    case "pin":
      return a.ref === b.ref && a.pin === b.pin;
    case "component":
      return a.ref === b.ref;
    case "net":
      return a.netId && b.netId ? a.netId === b.netId : a.net === b.net;
    case "bus":
      return a.busId === b.busId;
  }
}

// askLabel is the plain-language question the preset for this selection answers, for the button that
// takes the next hop. The copy lives on the client, like the relation-group labels and the
// locate-reason messages, because the SERVER owns the query and the client owns its wording.
export function askLabel(sel: Selection): string {
  const name = labelFor(sel);
  switch (sel.kind) {
    case "pin":
      return `What is ${name} wired to?`;
    case "component":
      return `What is ${name} connected to?`;
    case "net":
      return `What is on ${name}?`;
    case "bus":
      return `What is in ${name}?`;
  }
}
