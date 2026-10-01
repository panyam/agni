// trace.ts is the framework-neutral state for the trace panel (agni issue 600). The reader names two
// pins, the presenter calls TraceDesign and pushes a TraceState, the panel renders it and the canvas
// highlights the route. The shape lives here rather than in the .tsx, as in coverage.ts and
// findings.ts, so the presenter and its tests build state without touching Solid.
import { type Trace, TraceOutcome } from "./gen/agni/v1/webapi/design_pb.js";
import type { HighlightSubject } from "./findings.js";

export { TraceOutcome };

// TraceCrossItem is one series part the route passes through, with its own pin on each side.
export interface TraceCrossItem {
  refDes: string;
  cls: string;
  enterPin: string;
  exitPin: string;
}

// TraceStubItem is a part sitting on a route net that the route does not pass through.
export interface TraceStubItem {
  refDes: string;
  pin: string;
  cls: string;
}

// TraceNetItem is one net on the route. busLike marks a rail or plane, which a route may END on and
// never passes through, so the panel can say why the walk stopped there rather than leaving a reader
// to assume it gave up.
export interface TraceNetItem {
  name: string;
  stubs: TraceStubItem[];
  stubsElided: number;
  busLike: boolean;
  // The sheets this net is drawn on, empty when it is drawn on none. Per net rather than one per
  // trace, because a route crossing three sheets is when a reader wants to choose which to open (see
  // service.AnnotateTraceSheets).
  sheetIds: string[];
}

export interface TraceEndItem {
  refDes: string;
  pin: string;
  pinName: string;
  net: string;
  // Where this endpoint is DRAWN, resolved from its placement and falling back to its net. Filled on
  // a no-route too, since the two nets that fail to join are still drawn somewhere.
  sheetIds: string[];
}

// TraceState is the panel's whole state.
//
// `ran` separates "not asked yet" (a blank panel) from an answer. Two of the three outcomes are
// NEGATIVE, so a panel rendering "no route" before anyone asked would state something false about
// the design.
//
// `error` is for a request that failed (an unloadable design, a transport error). An UNRESOLVED
// outcome is different, a successful answer saying the question named something the design does not
// have.
export interface TraceState {
  from: TraceEndItem;
  to: TraceEndItem;
  outcome: TraceOutcome;
  reason: string;
  radius: number;
  crossings: TraceCrossItem[];
  nets: TraceNetItem[];
  error: string;
  loading: boolean;
  ran: boolean;
}

const emptyEnd: TraceEndItem = { refDes: "", pin: "", pinName: "", net: "", sheetIds: [] };

export function emptyTrace(): TraceState {
  return {
    from: emptyEnd,
    to: emptyEnd,
    outcome: TraceOutcome.UNSPECIFIED,
    reason: "",
    radius: 0,
    crossings: [],
    nets: [],
    error: "",
    loading: false,
    ran: false,
  };
}

export function loadingTrace(): TraceState {
  return { ...emptyTrace(), loading: true, ran: true };
}

export function errorTrace(message: string): TraceState {
  return { ...emptyTrace(), error: message, ran: true };
}

// traceFromResponse maps the wire Trace into the panel's view state.
export function traceFromResponse(t: Trace): TraceState {
  const end = (e: { endpoint?: { refDes: string; pin: string }; pinName: string; net: string; sheetIds?: string[] } | undefined): TraceEndItem => ({
    refDes: e?.endpoint?.refDes ?? "",
    pin: e?.endpoint?.pin ?? "",
    pinName: e?.pinName ?? "",
    net: e?.net ?? "",
    sheetIds: e?.sheetIds ?? [],
  });
  return {
    from: end(t.from),
    to: end(t.to),
    outcome: t.outcome,
    reason: t.reason,
    radius: t.radius,
    crossings: t.crossings.map((c) => ({
      refDes: c.refDes,
      cls: c.class,
      enterPin: c.enterPin,
      exitPin: c.exitPin,
    })),
    nets: t.nets.map((n) => ({
      name: n.name,
      stubs: n.stubs.map((s) => ({ refDes: s.refDes, pin: s.pin, cls: s.class })),
      stubsElided: n.stubsElided,
      busLike: n.busLike,
      sheetIds: n.sheetIds ?? [],
    })),
    error: "",
    loading: false,
    ran: true,
  };
}

// traceSubjects is what the canvas should light up for a trace, and it draws the answer whatever the
// answer was.
//
// A route lights its nets and the parts it crossed. A no-route lights the two nets that fail to
// join. An unresolved endpoint lights whichever end DID resolve, since the panel beside it already
// says the other end named nothing.
//
// It is the twin of traceSpecs in cmd/agni. The CLI draws an SVG and the viewer paints a canvas, so
// neither can call the other, and the CLAIM about which entities a trace is about is stated and
// tested in both places.
export function traceSubjects(s: TraceState): HighlightSubject[] {
  if (!s.ran || s.error) return [];
  const subjects: HighlightSubject[] = [];
  if (s.outcome === TraceOutcome.ROUTED) {
    for (const n of s.nets) subjects.push({ kind: "net", subject: n.name, pin: "" });
    for (const c of s.crossings) subjects.push({ kind: "component", subject: c.refDes, pin: "" });
  } else {
    for (const e of [s.from, s.to]) {
      if (e.net) subjects.push({ kind: "net", subject: e.net, pin: "" });
    }
  }
  for (const e of [s.from, s.to]) {
    if (e.refDes && e.pin) subjects.push({ kind: "pin", subject: e.refDes, pin: e.pin });
  }
  return subjects;
}

// TraceView is the command-down surface the presenter pushes each answer to.
export interface TraceView {
  setState: (s: TraceState) => void;
}

// parseEndpoint reads "U1.3" into its two halves, splitting at the FIRST dot because a ref-des does
// not contain one and a pin designator occasionally does. null when the text is not a pin at all.
//
// Same reading the CLI takes, duplicated because the two live in different languages. Keep the
// SPLIT RULE (first dot) in step with it.
export function parseEndpoint(text: string): { refDes: string; pin: string } | null {
  const t = text.trim();
  const i = t.indexOf(".");
  if (i <= 0 || i === t.length - 1) return null;
  return { refDes: t.slice(0, i), pin: t.slice(i + 1) };
}

// splitTraceParam reads a `?trace=U1.3,J1.1` URL parameter into its two pins.
//
// The FIRST comma separates them, so a pin designator containing a comma cannot be named this way.
// A comma in a designator is unusual, and the alternative is an escaping scheme in a parameter meant
// to be readable in an address bar.
export function splitTraceParam(param: string): [string, string] {
  const i = param.indexOf(",");
  if (i < 0) return [param.trim(), ""];
  return [param.slice(0, i).trim(), param.slice(i + 1).trim()];
}

// traceSheet is the sheet a trace answer should open on, "" when it is drawn nowhere.
//
// The FROM endpoint's first, because the reader named that pin and a route reads in that direction;
// then the first net of the route that is drawn. It picks ONE because the canvas shows one sheet at
// a time. Every other sheet of every net stays reachable as a badge, as it is for a finding or a
// query cell on several sheets.
export function traceSheet(t: TraceState): string {
  if (t.from.sheetIds.length > 0) return t.from.sheetIds[0];
  for (const n of t.nets) {
    if (n.sheetIds.length > 0) return n.sheetIds[0];
  }
  return "";
}
