import type { RenderMode } from "./viewer.js";

// ControlsState is the full state of the viewer's control bar. nativeAvailable is whether Native is
// offered for the current file, and layout is the effective choice among layouts. The presenter owns
// this state and pushes it down, and the controls island renders from it.
export interface ControlsState {
  mode: RenderMode;
  nativeAvailable: boolean;
  layouts: string[];
  layout: string;
  // providedSymbols is whether the design ships its own symbols (so the faithful-symbols toggle
  // is offered); faithfulSymbols is whether that toggle is on.
  providedSymbols: boolean;
  faithfulSymbols: boolean;
  // hasHighlights is whether anything is currently lit, so the clear control can disable itself
  // rather than offering an action with no effect (agni issue 348).
  hasHighlights: boolean;
}

// ControlsView receives ControlsState from the presenter. The controls island implements it (C3).
// ViewSink in viewer.ts describes the pattern.
export interface ControlsView {
  setState(s: ControlsState): void;
}
