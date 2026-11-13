// walk.ts is the framework-neutral state for a walk outwards from one part (agni issue 1341). The
// reader picks a walk kind on a part in Properties, the presenter calls WalkDesign and pushes a
// WalkState, the Trace tab lists it hop by hop, and the canvas lights each hop in its own shade.
import { TraceOutcome, type Walk, type WalkGoalAnswer } from "./gen/agni/v1/webapi/design_pb.js";
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

// WalkGoalRequest is a goal question asked from the walk (agni issue 1340): "pin", "pull-up" or
// "rail", from a reached net (`at`, "" for the start's own net), and for a pin goal the pin as ref.pin.
export interface WalkGoalRequest {
  kind: string;
  at: string;
  to: string;
}

// WalkRequest is what the reader asked: the start (a part, or a pin as ref.pin), the kind, how many
// hops, the parts to cross whatever the kind says, and a goal question when one was asked. The URL
// keeps it whole.
export interface WalkRequest {
  start: string;
  kind: string;
  hops: number;
  through: string[];
  goal?: WalkGoalRequest;
}

// WalkRouteItem is one route a goal found, its nets in order and the crossing into each net after
// the first. sheetIds holds each net's sheets, in the same order.
export interface WalkRouteItem {
  nets: string[];
  sheetIds: string[][];
  crossings: { refDes: string; enterPin: string; exitPin: string; paired: boolean }[];
  endsAt: string;
  note: string;
}

// WalkGoalItem is a goal's answer. The outcome keeps trace's three apart: a route, no route within
// the radius, and a goal that names nothing.
export interface WalkGoalItem {
  kind: string;
  at: string;
  to: string;
  outcome: "routed" | "no-route" | "unresolved";
  reason: string;
  radius: number;
  routes: WalkRouteItem[];
}

// WalkState is the walk section's whole state. `ran` separates "not asked" from an answer, and
// `reason` holds a start that named nothing the design has, which is an answer rather than an error.
export interface WalkState {
  request: WalkRequest;
  hops: WalkHop[];
  boundary: WalkBoundaryItem[];
  elided: { hop: number; nets: number }[];
  goal: WalkGoalItem | null;
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
  return { request: noRequest, hops: [], boundary: [], elided: [], goal: null, reason: "", error: "", loading: false, ran: false };
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
    goal: w.goal ? goalFromResponse(w.goal) : null,
    reason: w.reason,
    error: "",
    loading: false,
    ran: true,
  };
}

function goalFromResponse(g: WalkGoalAnswer): WalkGoalItem {
  const outcome = g.outcome === TraceOutcome.ROUTED ? "routed" : g.outcome === TraceOutcome.NO_ROUTE ? "no-route" : "unresolved";
  const to = g.to?.endpoint;
  return {
    kind: g.kind,
    at: g.at,
    to: to?.refDes ? `${to.refDes}.${to.pin}${g.to?.pinName ? ` (${g.to.pinName})` : ""}` : "",
    outcome,
    reason: g.reason,
    radius: g.radius,
    routes: g.routes.map((r) => ({
      nets: r.nets.map((n) => n.name),
      sheetIds: r.nets.map((n) => n.sheetIds),
      crossings: r.crossings.map((c) => ({ refDes: c.refDes, enterPin: c.enterPin, exitPin: c.exitPin, paired: c.paired })),
      endsAt: r.endsAt,
      note: r.note,
    })),
  };
}

// routeLine spells a route as the engine's RenderRoute does, "SDA -> [R1] -> +3V3".
export function routeLine(r: WalkRouteItem): string {
  const parts = [r.nets[0] ?? ""];
  r.crossings.forEach((c, i) => parts.push(`[${c.refDes}]`, r.nets[i + 1] ?? ""));
  return parts.join(" -> ");
}

// HOP_COLORS shade each hop apart, the start's own nets first. A hop past the list takes the last.
export const HOP_COLORS = ["#2563eb", "#f59e0b", "#16a34a", "#9333ea"];
export const WALK_START_COLOR = "#0f766e";
export const WALK_CROSSED_COLOR = "#e11d48";
export const WALK_ENDED_COLOR = "#94a3b8";
export const WALK_BOUNDARY_COLOR = "#64748b";
export const WALK_GOAL_COLOR = "#c026d3";

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

// walkStartMarks marks where a walk begins. A part start lights the pins its first hop leaves by,
// since the path itself starts there, so the part is not boxed again. A pin start keeps its dot. A
// part whose first hop drew no net falls back to the box, so the start still shows.
export function walkStartMarks(s: WalkState): HighlightSpec[] {
  const [ref, pin] = splitStart(s.request.start);
  if (!ref || pin) return walkStartSpecs(s.request);
  const exits = new Set<string>();
  for (const h of s.hops) {
    for (const n of h.nets) {
      if (n.hop === 1 && n.through === ref && n.exitPin) exits.add(n.exitPin);
    }
  }
  if (exits.size === 0) return walkStartSpecs(s.request);
  return [{ pins: [...exits].map((p) => ({ refDes: ref, pin: p })), color: WALK_START_COLOR }];
}

// walkSpecs lights the whole walk: each hop's nets in its shade, a net where the walk stopped in a
// faint grey (a rail or ground runs across the sheet and would bury the circuit), the parts it
// crossed in red, the parts it did not cross outlined, and the start's pins on top.
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
  return [...specs, ...goalSpecs(s), ...walkStartMarks(s)];
}

// goalSpecs lights a goal's routes over the walk, their nets and the parts they cross in one colour,
// and a pin goal's pin.
export function goalSpecs(s: WalkState): HighlightSpec[] {
  const g = s.goal;
  if (!g || g.outcome !== "routed") return [];
  const nets = new Set<string>();
  const parts = new Set<string>();
  for (const r of g.routes) {
    r.nets.forEach((n) => nets.add(n));
    r.crossings.forEach((c) => parts.add(c.refDes));
  }
  const specs: HighlightSpec[] = [{ nets: [...nets], color: WALK_GOAL_COLOR, shape: HighlightShape.PATH }];
  if (parts.size > 0) specs.push({ components: [...parts], color: WALK_GOAL_COLOR, shape: HighlightShape.BOUNDING_RECT });
  const [ref, pin] = splitStart(s.request.goal?.to ?? "");
  if (g.kind === "pin" && ref && pin) specs.push({ pins: [{ refDes: ref, pin }], color: WALK_GOAL_COLOR });
  return specs;
}

// goalRevealSpecs lights what the view zooms to when a goal found a route (agni issue 1423): the
// start, the parts each route crosses and a pin goal's pin. A route's nets are left out, because a
// pull-up's route ends on a rail that runs across the whole sheet. It is empty when no route was
// found, and the view then zooms to the start alone.
export function goalRevealSpecs(s: WalkState): HighlightSpec[] {
  const g = s.goal;
  if (!g || g.outcome !== "routed" || g.routes.length === 0) return [];
  const parts = new Set<string>();
  for (const r of g.routes) r.crossings.forEach((c) => parts.add(c.refDes));
  const specs: HighlightSpec[] = [];
  if (parts.size > 0) specs.push({ components: [...parts], color: WALK_GOAL_COLOR, shape: HighlightShape.BOUNDING_RECT });
  const [ref, pin] = splitStart(s.request.goal?.to ?? "");
  if (g.kind === "pin" && ref && pin) specs.push({ pins: [{ refDes: ref, pin }], color: WALK_GOAL_COLOR });
  return [...specs, ...walkStartMarks(s)];
}

// goalSheet is the sheet a routed goal should open on, given the sheet the walk opens on. That sheet
// is kept when it draws the first route's last net. Otherwise the last net's first sheet is used, or
// the nearest earlier net's when the last is drawn nowhere. It is "" when no goal was routed.
export function goalSheet(s: WalkState, walkOn: string): string {
  const g = s.goal;
  if (!g || g.outcome !== "routed") return "";
  for (const r of g.routes) {
    for (let i = r.sheetIds.length - 1; i >= 0; i--) {
      const ids = r.sheetIds[i] ?? [];
      if (ids.length === 0) continue;
      return ids.includes(walkOn) ? walkOn : ids[0];
    }
  }
  return "";
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
  if (r.goal?.kind) {
    out.walk_goal = r.goal.kind;
    if (r.goal.at) out.walk_at = r.goal.at;
    if (r.goal.to) out.walk_to = r.goal.to;
  }
  return out;
}

export function parseWalkParams(get: (key: string) => string | null): WalkRequest {
  const start = (get("walk") ?? "").trim();
  if (!start) return noRequest;
  const hops = Number(get("walk_hops") ?? "");
  const r: WalkRequest = {
    start,
    kind: (get("walk_kind") ?? "").trim(),
    hops: Number.isInteger(hops) && hops > 0 ? hops : DEFAULT_WALK_HOPS,
    through: (get("walk_through") ?? "").split(",").map((t) => t.trim()).filter((t) => t !== ""),
  };
  const goal = (get("walk_goal") ?? "").trim();
  if (goal) r.goal = { kind: goal, at: (get("walk_at") ?? "").trim(), to: (get("walk_to") ?? "").trim() };
  return r;
}
