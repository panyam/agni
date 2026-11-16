// Which steps of a journey `make try` plays (agni issue 1600). UNTIL names the step to stop before,
// by its number or by words from its caption, so a reviewer can take that step themselves. The
// numbers count only the steps a design plays, since a design skips the steps it has nothing for.

import type { Design, Journey } from "./journeys.js";

// Played is one step a design plays: its number from 1, its caption on that design, and its index
// in the journey's step list.
export interface Played {
  n: number;
  caption: string;
  index: number;
}

export function playedSteps(j: Journey, d: Design): Played[] {
  const out: Played[] = [];
  j.steps.forEach((s, index) => {
    if (s.skip?.(d)) return;
    out.push({ n: out.length + 1, caption: typeof s.caption === "string" ? s.caption : s.caption(d), index });
  });
  return out;
}

// stepsBefore is how many played steps come before the one until names, or all of them for an
// empty until. A number counts from 1. Words match a caption without regard to case, and must match
// exactly one step. Anything else throws, and the message lists the steps to choose from.
export function stepsBefore(j: Journey, d: Design, until: string): number {
  const steps = playedSteps(j, d);
  const want = until.trim();
  if (want === "") return steps.length;
  const list = (of: Played[]) => of.map((p) => `  ${p.n}. ${p.caption}`).join("\n");
  if (/^\d+$/.test(want)) {
    const n = Number(want);
    if (n >= 1 && n <= steps.length) return n - 1;
    throw new Error(`UNTIL=${want} is past the ${steps.length} steps of ${j.name}. Its steps are:\n${list(steps)}`);
  }
  const lower = want.toLowerCase();
  const hits = steps.filter((p) => p.caption.toLowerCase().includes(lower));
  if (hits.length === 1) return hits[0].n - 1;
  if (hits.length === 0) throw new Error(`UNTIL=${want} names no step of ${j.name}. Its steps are:\n${list(steps)}`);
  throw new Error(`UNTIL=${want} matches ${hits.length} steps of ${j.name}. Name one by its number:\n${list(hits)}`);
}
