import type { Inventory } from "./properties.js";
import { compareNatural } from "./query.js";

// TracePick is one entry in a trace endpoint's picker. Choosing it puts value in the field. A
// component's value is a bare ref-des, and choosing it then lists that component's pins. A pin's
// value is "U7.3", and it is an endpoint. detail is the grey text beside it.
export interface TracePick {
  value: string;
  kind: "component" | "pin";
  detail: string;
}

// TracePicks holds the open design's parts and pins for the pickers. Properties already holds an
// inventory, and TracePicks is built from it once per design, so picking asks the engine nothing.
// examples are two real pins for the fields' placeholders, empty strings when the design has none.
export interface TracePicks {
  components: { ref: string; detail: string }[];
  pins: Map<string, PickPin[]>;
  examples: [string, string];
}

// PickPin is one pin a picker can offer. depth is set on a reached pin, the series crossings from
// the start pin's net (agni issue 1541).
export interface PickPin {
  pin: string;
  name: string;
  net: string;
  depth?: number;
}

// Reach is what a trace from one start pin reaches within hops crossings, as
// DesignService.ReachablePins answers it. reason is set when the start pin named nothing.
export interface Reach {
  from: string;
  hops: number;
  reason: string;
  pins: { ref: string; pin: string; name: string; net: string; depth: number }[];
  rails: { net: string; depth: number; pins: number }[];
}

// crossingsLabel says how far a reached pin is, "on the start net" for none.
export function crossingsLabel(depth: number): string {
  return depth === 0 ? "on the start net" : `${depth} crossing${depth === 1 ? "" : "s"}`;
}

// reachPicks is the "to" picker's list once the start pin's reach is known: only the parts with a
// reached pin, each with its nearest pin's distance, and only their reached pins. The engine walked
// as a trace does, so every entry traces (check.ReachablePins).
export function reachPicks(r: Reach): TracePicks {
  const pins = new Map<string, PickPin[]>();
  for (const p of r.pins) {
    const list = pins.get(p.ref) ?? [];
    list.push({ pin: p.pin, name: p.name, net: p.net, depth: p.depth });
    pins.set(p.ref, list);
  }
  const refs = [...pins.keys()].sort(compareNatural);
  for (const ref of refs) pins.get(ref)!.sort((a, b) => compareNatural(a.pin, b.pin));
  const components = refs.map((ref) => {
    const list = pins.get(ref)!;
    const nearest = Math.min(...list.map((p) => p.depth ?? 0));
    return { ref, detail: `${list.length} pin${list.length === 1 ? "" : "s"} · ${crossingsLabel(nearest)}` };
  });
  // The example is the nearest pin past the start net, so a placeholder reads as a route.
  const byNearness = [...r.pins].sort((a, b) => a.depth - b.depth || compareNatural(a.ref, b.ref) || compareNatural(a.pin, b.pin));
  const far = byNearness.find((p) => p.depth > 0) ?? byNearness[0];
  return { components, pins, examples: [r.from, far ? `${far.ref}.${far.pin}` : ""] };
}

// TRACE_HOPS is the radius a trace searches when the reader picks none, check.DefaultTraceHops.
export const TRACE_HOPS = 6;

// isPin says whether text names a pin of the design as "U7.3", ignoring the ref-des's case.
export function isPin(picks: TracePicks, text: string): boolean {
  const dot = text.lastIndexOf(".");
  if (dot <= 0) return false;
  const ref = [...picks.pins.keys()].find((r) => r.toLowerCase() === text.slice(0, dot).toLowerCase());
  const pin = text.slice(dot + 1);
  return !!ref && pin !== "" && picks.pins.get(ref)!.some((p) => p.pin === pin);
}

// PICK_LIMIT caps how many entries a picker lists, since a large board has tens of thousands of
// pins and a reader narrows by typing rather than by scrolling.
export const PICK_LIMIT = 50;

export function emptyPicks(): TracePicks {
  return { components: [], pins: new Map(), examples: ["", ""] };
}

// tracePicks indexes the inventory for the pickers. Only components with a connected pin are
// listed, since a trace starts and ends at a pin on a net. A KiCad power or flag symbol (#PWR01,
// #FLG01) is left out, because it is a connectivity marker rather than a part. The engine reads a
// leading # the same way (check.IsVirtualRef).
export function tracePicks(inv: Inventory | null): TracePicks {
  if (!inv) return emptyPicks();
  const refs = [...inv.pinsOf.keys()].filter((r) => !r.startsWith("#")).sort(compareNatural);
  const components = refs.map((ref) => {
    const n = inv.pinsOf.get(ref)!.length;
    const value = inv.components.get(ref)?.value?.input ?? "";
    return { ref, detail: [value, `${n} pin${n === 1 ? "" : "s"}`].filter(Boolean).join(" · ") };
  });
  const pins = new Map(refs.map((ref) => [ref, inv.pinsOf.get(ref)!]));
  // Two different parts with at least two pins each, so the example reads as a route between parts
  // rather than a pin to itself.
  const multi = refs.filter((r) => pins.get(r)!.length >= 2);
  const ex = (ref: string | undefined) => (ref ? `${ref}.${pins.get(ref)![0].pin}` : "");
  return { components, pins, examples: [ex(multi[0]), ex(multi[1])] };
}

// matchRank is how well typed matches text, lower being better: 0 exact, 1 a prefix, 2 the typed
// characters in order with gaps (so "u13" finds both U13 and U1.3), and -1 no match. Case is ignored.
export function matchRank(typed: string, text: string): number {
  const t = typed.toLowerCase();
  const s = text.toLowerCase();
  if (s === t) return 0;
  if (s.startsWith(t)) return 1;
  let i = 0;
  for (const c of s) if (c === t[i]) i++;
  return i === t.length ? 2 : -1;
}

// pickCandidates lists a picker's entries for the text typed so far, best match first and at most
// PICK_LIMIT. Text naming a component and a dot ("U7." or "u7.sd") lists that component's pins,
// matched on designator and function name. Other text matches components by ref-des and pins by
// "ref.pin" or by function name, so "SDA" finds every pin whose datasheet name is SDA. Empty text
// lists the components. A reader who does not know a name starts there.
export function pickCandidates(picks: TracePicks, typed: string): TracePick[] {
  const t = typed.trim();
  const pinPick = (ref: string, p: PickPin): TracePick => ({
    value: `${ref}.${p.pin}`,
    kind: "pin",
    detail: [p.name, p.net, p.depth === undefined ? "" : crossingsLabel(p.depth)].filter(Boolean).join(" · "),
  });
  if (t === "") return picks.components.slice(0, PICK_LIMIT).map((c) => ({ value: c.ref, kind: "component", detail: c.detail }));

  const dot = t.lastIndexOf(".");
  if (dot > 0) {
    const ref = [...picks.pins.keys()].find((r) => r.toLowerCase() === t.slice(0, dot).toLowerCase());
    if (ref) {
      const rest = t.slice(dot + 1);
      const scored: [number, TracePick][] = [];
      for (const p of picks.pins.get(ref)!) {
        const rank = rest === "" ? 1 : best(matchRank(rest, p.pin), matchRank(rest, p.name));
        if (rank >= 0) scored.push([rank, pinPick(ref, p)]);
      }
      return ranked(scored);
    }
  }

  const scored: [number, TracePick][] = [];
  for (const c of picks.components) {
    const rank = matchRank(t, c.ref);
    if (rank >= 0) scored.push([rank, { value: c.ref, kind: "component", detail: c.detail }]);
  }
  for (const [ref, list] of picks.pins) {
    for (const p of list) {
      // A pin ranks one step behind a component equally matched, so "U1" lists U1 before U1.1.
      const rank = best(matchRank(t, `${ref}.${p.pin}`), p.name ? matchRank(t, p.name) : -1);
      if (rank >= 0) scored.push([rank + 0.5, pinPick(ref, p)]);
    }
  }
  return ranked(scored);
}

function best(a: number, b: number): number {
  if (a < 0) return b;
  if (b < 0) return a;
  return Math.min(a, b);
}

function ranked(scored: [number, TracePick][]): TracePick[] {
  return scored
    .sort((a, b) => a[0] - b[0] || compareNatural(a[1].value, b[1].value))
    .slice(0, PICK_LIMIT)
    .map(([, p]) => p);
}
