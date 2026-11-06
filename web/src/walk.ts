// walk.ts is the framework-neutral state for a walk outwards from one part (agni issue 1341). The
// reader picks a walk kind on a part in Properties, the presenter calls WalkDesign and pushes a
// WalkState, the Trace tab lists it hop by hop, and the canvas lights each hop in its own shade.
import type { Walk } from "./gen/agni/v1/webapi/design_pb.js";
import { HighlightShape, type HighlightSpec } from "./highlights.js";

// WalkNetItem is one net the walk reached, with the part it was entered through.
export interface WalkNetItem {
  name: string;
  hop: number;
  // through is the part that crossed into the net, with its pin on each side. On a hop-1 net it is
  // the start itself, and fromNet is "".
  through: string;
  enterPin: string;
  exitPin: string;
  fromNet: string;
  paired: boolean;
  // alsoThrough names the parts that crossed into the net after the first one did.
  alsoThrough: string[];
  // ended says why the walk went no further from this net ("ground", "rail", ...), "" otherwise.
  ended: string;
  parts: { refDes: string; pin: string; cls: string }[];
  partsElided: number;
  sheetIds: string[];
}

// WalkBoundaryItem is a part the walk did not cross, and why. Crossing it walks again with the part
// named in `through`.
export interface WalkBoundaryItem {
  refDes: string;
  cls: string;
  hop: number;
  net: string;
  reason: string;
}

// WalkHop is one hop of the walk, the nets it reached.
export interface WalkHop {
  hop: number;
  nets: WalkNetItem[];
}

// WalkRequest is what the reader asked: the start (a part, or a pin as ref.pin), the kind, how many
// hops, and the parts to cross whatever the kind says. The URL keeps it whole.
export interface WalkRequest {
  start: string;
  kind: string;
  hops: number;
  through: string[];
}

// WalkState is the walk section's whole state. `ran` separates "not asked" from an answer, and
// `reason` holds a start that named nothing the design has, which is an answer rather than an error.
export interface WalkState {
  request: WalkRequest;
  hops: WalkHop[];
  boundary: WalkBoundaryItem[];
  elided: { hop: number; nets: number }[];
  reason: string;
  error: string;
  loading: boolean;
  ran: boolean;
}

// WalkKindItem is one walk kind Properties offers for a part.
export interface WalkKindItem {
  name: string;
  title: string;
  summary: string;
}

// WalkView is the command-down surface the presenter pushes each walk to, and the kinds Properties
// offers.
export interface WalkView {
  setWalk: (s: WalkState) => void;
}

// DEFAULT_WALK_HOPS is the radius a fresh walk asks at. Hop 1 is the start's own nets, so the reader
// sees them first and widens from there.
export const DEFAULT_WALK_HOPS = 1;

const noRequest: WalkRequest = { start: "", kind: "", hops: 0, through: [] };

export function emptyWalk(): WalkState {
  return { request: noRequest, hops: [], boundary: [], elided: [], reason: "", error: "", loading: false, ran: false };
}

export function loadingWalk(request: WalkRequest): WalkState {
  return { ...emptyWalk(), request, loading: true, ran: true };
}

export function errorWalk(request: WalkRequest, message: string): WalkState {
  return { ...emptyWalk(), request, error: message, ran: true };
}

// walkFromResponse maps the wire Walk into the section's state, grouping the nets by hop in the
// order the walk reached them.
export function walkFromResponse(request: WalkRequest, w: Walk): WalkState {
  const byHop = new Map<number, WalkNetItem[]>();
  for (const n of w.nets) {
    const e = n.entered;
    const item: WalkNetItem = {
      name: n.net?.name ?? "",
      hop: n.hop,
      through: e?.refDes ?? "",
      enterPin: e?.enterPin ?? "",
      exitPin: e?.exitPin ?? "",
      fromNet: e?.fromNet ?? "",
      paired: e?.paired ?? false,
      alsoThrough: n.alsoEntered.map((a) => a.refDes),
      ended: n.ended,
      parts: (n.net?.stubs ?? []).map((s) => ({ refDes: s.refDes, pin: s.pin, cls: s.class })),
      partsElided: n.net?.stubsElided ?? 0,
      sheetIds: n.net?.sheetIds ?? [],
    };
    byHop.set(n.hop, [...(byHop.get(n.hop) ?? []), item]);
  }
  return {
    request,
    hops: [...byHop.entries()].sort((a, b) => a[0] - b[0]).map(([hop, nets]) => ({ hop, nets })),
    boundary: w.boundary.map((b) => ({ refDes: b.refDes, cls: b.class, hop: b.hop, net: b.net, reason: b.reason })),
    elided: w.elided.map((e) => ({ hop: e.hop, nets: e.nets })),
    reason: w.reason,
    error: "",
    loading: false,
    ran: true,
  };
}

// HOP_COLORS shade each hop apart, the start's own nets first. A hop past the list takes the last.
export const HOP_COLORS = ["#2563eb", "#f59e0b", "#16a34a", "#9333ea"];
export const WALK_START_COLOR = "#0f766e";
export const WALK_CROSSED_COLOR = "#e11d48";
export const WALK_ENDED_COLOR = "#94a3b8";
export const WALK_BOUNDARY_COLOR = "#64748b";

export function hopColor(hop: number): string {
  return HOP_COLORS[Math.min(Math.max(hop, 1), HOP_COLORS.length) - 1];
}

// walkStartSpecs lights the start alone, so the view can zoom to it before the rest of the walk is
// drawn around it. A pin start lights its pin.
export function walkStartSpecs(request: WalkRequest): HighlightSpec[] {
  const [ref, pin] = splitStart(request.start);
  if (!ref) return [];
  if (pin) return [{ pins: [{ refDes: ref, pin }], color: WALK_START_COLOR }];
  return [{ components: [ref], color: WALK_START_COLOR, shape: HighlightShape.BOUNDING_RECT }];
}

// walkSpecs lights the whole walk: each hop's nets in its shade, a net where the walk stopped in a
// faint grey (a rail or ground runs across the sheet and would bury the circuit), the parts it
// crossed in red, the parts it did not cross outlined, and the start on top.
export function walkSpecs(s: WalkState): HighlightSpec[] {
  if (!s.ran || s.error || s.reason) return [];
  const specs: HighlightSpec[] = [];
  const ended: string[] = [];
  const crossed = new Set<string>();
  const [startRef] = splitStart(s.request.start);
  for (const h of s.hops) {
    const nets: string[] = [];
    for (const n of h.nets) {
      if (n.ended) ended.push(n.name);
      else nets.push(n.name);
      // A hop-1 net was entered by the start's own pin, which crossed nothing.
      if (n.fromNet && n.through !== startRef) crossed.add(n.through);
      for (const r of n.alsoThrough) if (r !== startRef) crossed.add(r);
    }
    if (nets.length > 0) specs.push({ nets, color: hopColor(h.hop), shape: HighlightShape.PATH });
  }
  if (ended.length > 0) specs.unshift({ nets: ended, color: WALK_ENDED_COLOR, alpha: 0.35, shape: HighlightShape.PATH });
  if (crossed.size > 0) specs.push({ components: [...crossed], color: WALK_CROSSED_COLOR, shape: HighlightShape.BOUNDING_RECT });
  const edge = s.boundary.map((b) => b.refDes).filter((r) => !crossed.has(r));
  if (edge.length > 0) specs.push({ components: edge, color: WALK_BOUNDARY_COLOR, alpha: 0.5 });
  return [...specs, ...walkStartSpecs(s.request)];
}

// walkSheet is the sheet a walk should open on: the first hop-1 net's, "" when it is drawn nowhere.
export function walkSheet(s: WalkState): string {
  for (const h of s.hops) {
    for (const n of h.nets) {
      if (!n.ended && n.sheetIds.length > 0) return n.sheetIds[0];
    }
  }
  for (const h of s.hops) {
    for (const n of h.nets) if (n.sheetIds.length > 0) return n.sheetIds[0];
  }
  return "";
}

// splitStart reads a start as its part and pin, "R1" -> ["R1", ""] and "U7.3" -> ["U7", "3"], split
// at the first dot as parseEndpoint splits a trace pin.
export function splitStart(start: string): [string, string] {
  const t = start.trim();
  const i = t.indexOf(".");
  if (i < 0) return [t, ""];
  return [t.slice(0, i), t.slice(i + 1)];
}

// walkParam writes a request as the URL keeps it, and parseWalkParam reads it back. The start is
// `walk=`, and the kind, hops and the parts crossed by name ride beside it.
export function walkParams(r: WalkRequest): Record<string, string> {
  if (!r.start) return {};
  const out: Record<string, string> = { walk: r.start };
  if (r.kind) out.walk_kind = r.kind;
  if (r.hops > 0) out.walk_hops = String(r.hops);
  if (r.through.length > 0) out.walk_through = r.through.join(",");
  return out;
}

export function parseWalkParams(get: (key: string) => string | null): WalkRequest {
  const start = (get("walk") ?? "").trim();
  if (!start) return noRequest;
  const hops = Number(get("walk_hops") ?? "");
  return {
    start,
    kind: (get("walk_kind") ?? "").trim(),
    hops: Number.isInteger(hops) && hops > 0 ? hops : DEFAULT_WALK_HOPS,
    through: (get("walk_through") ?? "").split(",").map((t) => t.trim()).filter((t) => t !== ""),
  };
}
