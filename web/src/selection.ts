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

// fillEntityQuery substitutes a selection into a preset the SERVER wrote (query.EntityQueries).
//
// The templates deliberately do not live here. Every one names relations defined in Go, so a client
// that held its own copy would be the one caller nothing checks: rename a relation and the server's
// example tests go red while every click in the viewer starts producing a query that errors. Beside
// the relations, a preset gets a parse check and an evaluate-against-a-real-design check.
//
// So this file keeps the part that IS the client's: turning what was clicked into values.
//
// Quotes are stripped rather than escaped because the query grammar has no escape sequence — a
// string literal is '"' { char } '"' — so a designator carrying a quote cannot be represented at
// all, and splicing one in would end the literal early and produce a query that means something
// else. Stripping yields a query that finds nothing, which is the honest failure.
export function fillEntityQuery(template: string, sel: Selection): string {
  const lit = (v: string | undefined): string => (v ?? "").replace(/"/g, "");
  return template
    .replace(/\{ref\}/g, lit(sel.ref))
    .replace(/\{pin\}/g, lit(sel.pin))
    .replace(/\{net\}/g, lit(sel.net))
    .replace(/\{bus\}/g, lit(sel.busId));
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
// A scalar cell, or an entity kind with no selection shape yet, yields null: the cell still locates,
// it just names nothing to ask about.
//
// `ref` is the other half of a pin's identity, from the row's cell_refs. A pin cell holds "5", which
// names nothing until you know it means U7's pin 5, and that is the whole reason a pin column used
// to type as a scalar. Ignored for every other kind, where the cell names the entity by itself.
//
// A bus is here because a search can now return one (agni issue 338): entity() enumerates buses,
// and a bus with no drawn wire is exactly the sort of thing a reviewer goes looking for by name.
// Its subject IS its label, the same key a drawn bus element carries in data-bus, so the two entry
// points converge on one value the way component and net already do.
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
      // Both halves or nothing. A pin with no component is not a less precise pin, it is not a pin.
      return ref ? { kind: "pin", ref, pin: subject } : null;
    default:
      return null;
  }
}

// sameSelection reports whether two picks name the same thing, which is how a surface marks the one
// it is currently showing. It is an identity test rather than a deep equality: the canvas pushes a
// net with both its name and its id, while a result cell can only carry the name, and those two are
// the same net. Comparing ids when BOTH carry one keeps the precise case precise (two nets can share
// a display name across sheets) without making the imprecise case a mismatch.
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
// takes the next hop. The copy lives on the client for the same reason the relation-group labels and
// the locate-reason messages do: the SERVER owns the query (it names relations defined there), the
// client owns how it is worded on screen.
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
