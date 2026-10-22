import type { Inventory } from "./properties.js";
import { compareNatural } from "./query.js";

// TracePick is one entry a trace endpoint's picker offers. value is what choosing it puts in the field:
// a bare ref-des for a component, which then lists that component's pins, or "U7.3" for a pin, which
// is an endpoint. detail is the grey text beside it.
export interface TracePick {
  value: string;
  kind: "component" | "pin";
  detail: string;
}

// TracePicks is the open design's parts and pins as the pickers read them, built once per design from
// the inventory Properties already holds, so picking asks the engine nothing. examples are two real
// pins for the fields' placeholders, empty strings when the design has none.
export interface TracePicks {
  components: { ref: string; detail: string }[];
  pins: Map<string, { pin: string; name: string; net: string }[]>;
  examples: [string, string];
}

// PICK_LIMIT caps the entries a picker lists, since a large board has tens of thousands of pins and a
// reader narrows by typing rather than by scrolling.
export const PICK_LIMIT = 50;

export function emptyPicks(): TracePicks {
  return { components: [], pins: new Map(), examples: ["", ""] };
}

// tracePicks indexes the inventory for the pickers. Only components with a connected pin are listed,
// since a trace starts and ends at a pin on a net.
export function tracePicks(inv: Inventory | null): TracePicks {
  if (!inv) return emptyPicks();
  const refs = [...inv.pinsOf.keys()].sort(compareNatural);
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

// pickCandidates is what a picker lists for the text typed so far, best match first and at most
// PICK_LIMIT. Text naming a component and a dot ("U7." or "u7.sd") lists that component's pins,
// matched on designator and function name. Other text matches components by ref-des and pins by
// "ref.pin" or by function name, so "SDA" finds the pins a datasheet calls SDA. Empty text lists the
// components, which is where a reader who does not know a name starts.
export function pickCandidates(picks: TracePicks, typed: string): TracePick[] {
  const t = typed.trim();
  const pinPick = (ref: string, p: { pin: string; name: string; net: string }): TracePick => ({
    value: `${ref}.${p.pin}`,
    kind: "pin",
    detail: [p.name, p.net].filter(Boolean).join(" · "),
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
