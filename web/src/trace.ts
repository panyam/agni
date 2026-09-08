// trace.ts is the framework-neutral state for the trace panel (agni issue 600): the reader names two
// pins, the presenter calls TraceDesign and pushes a TraceState, the panel renders it and the canvas
// highlights the route. Keeping the shape here rather than in the .tsx mirrors coverage.ts /
// findings.ts and lets the presenter and its tests build state without touching Solid.
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
}

export interface TraceEndItem {
  refDes: string;
  pin: string;
  pinName: string;
  net: string;
}

// TraceState is the panel's whole state.
//
// `ran` separates "not asked yet" (a blank panel) from an answer, which matters more here than in
// most panels: two of the three outcomes are NEGATIVE, and a panel that rendered "no route" before
// anyone asked anything would be stating something false about the design.
//
// `error` is for a request that failed (an unloadable design, a transport error) and is a different
// thing from an UNRESOLVED outcome, which is a successful answer saying the question named something
// the design does not have.
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

const emptyEnd: TraceEndItem = { refDes: "", pin: "", pinName: "", net: "" };

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
  const end = (e: { endpoint?: { refDes: string; pin: string }; pinName: string; net: string } | undefined): TraceEndItem => ({
    refDes: e?.endpoint?.refDes ?? "",
    pin: e?.endpoint?.pin ?? "",
    pinName: e?.pinName ?? "",
    net: e?.net ?? "",
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
// join, which is the picture a reader goes looking for the moment they read the words, and an
// unresolved endpoint lights whichever end DID resolve, because half an answer located is more use
// than none and the panel beside it already says the other end named nothing.
//
// It is the twin of traceSpecs in cmd/agni, deliberately: the CLI draws an SVG and the viewer paints
// a canvas, so neither can call the other, and what they share is the CLAIM about which entities a
// trace is about. That claim is stated in both places and tested in both.
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

// TraceView is the command-down surface: the presenter pushes each answer.
export interface TraceView {
  setState: (s: TraceState) => void;
}

// parseEndpoint reads "U1.3" into its two halves, splitting at the FIRST dot because a ref-des does
// not contain one and a pin designator occasionally does. null when the text is not a pin at all.
//
// Same reading the CLI takes, and stated here rather than shared because the two live in different
// languages. What must not diverge is the SPLIT RULE, which is why both say which dot they split on.
export function parseEndpoint(text: string): { refDes: string; pin: string } | null {
  const t = text.trim();
  const i = t.indexOf(".");
  if (i <= 0 || i === t.length - 1) return null;
  return { refDes: t.slice(0, i), pin: t.slice(i + 1) };
}
