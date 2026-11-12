// Test-only. A Clock for glide.ts that runs frames only when a test steps it (agni issue 1546).
import type { Clock } from "./glide.js";

// manualClock runs frames only when the test steps it, at the time it has been stepped to.
export function manualClock(): Clock & { at: number; step: (ms: number) => void } {
  let next = 1;
  const pending = new Map<number, () => void>();
  const c = {
    at: 0,
    now: () => c.at,
    frame: (cb: () => void) => {
      const id = next++;
      pending.set(id, cb);
      return id;
    },
    cancel: (id: number) => void pending.delete(id),
    step: (ms: number) => {
      c.at += ms;
      const due = [...pending.values()];
      pending.clear();
      for (const cb of due) cb();
    },
  };
  return c;
}

