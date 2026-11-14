// The trail of what a reader selected, so they can step back along it and forward again (agni issue
// 1588). Say an engineer follows a fault from a connector pin to a diode, then a regulator, then its
// feedback divider. Back steps to the regulator, then the diode, and forward goes the other way. The
// page replays each step in every linked view. This module keeps the trail and nothing else, so a
// test reads it directly.

import { sameSelection, type Selection } from "./selection.js";

// HISTORY_LIMIT is how many selections the trail keeps. A pick past it drops the oldest.
export const HISTORY_LIMIT = 100;

// SelectionHistory is a bounded list of selections and a place in it. push records a pick and drops
// whatever lay ahead of the place, as a browser's history does after Back. Picking the selection the
// place already holds records nothing. Clearing a selection is no entry, so the page calls push only
// for a selection.
export class SelectionHistory {
  private entries: Selection[] = [];
  private at = -1;

  constructor(private readonly limit = HISTORY_LIMIT) {}

  push(sel: Selection): void {
    if (sameSelection(this.entries[this.at] ?? null, sel)) return;
    this.entries = this.entries.slice(0, this.at + 1);
    this.entries.push(sel);
    if (this.entries.length > this.limit) this.entries.splice(0, this.entries.length - this.limit);
    this.at = this.entries.length - 1;
  }

  // back moves the place one entry back and returns that selection, or undefined at the start.
  back(): Selection | undefined {
    if (!this.canBack()) return undefined;
    return this.entries[--this.at];
  }

  // forward moves the place one entry forward and returns that selection, or undefined at the end.
  forward(): Selection | undefined {
    if (!this.canForward()) return undefined;
    return this.entries[++this.at];
  }

  canBack(): boolean {
    return this.at > 0;
  }

  canForward(): boolean {
    return this.at < this.entries.length - 1;
  }

  // reset empties the trail, for a page that opened another design.
  reset(): void {
    this.entries = [];
    this.at = -1;
  }

  get size(): number {
    return this.entries.length;
  }
}
