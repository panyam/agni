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
  // board is whether the shown sheet is the physical board (WS7-034), which offers the
  // layer-visibility selector; boardLayers is its value ("all" | "front" | "back").
  board: boolean;
  boardLayers: string;
  // hasHighlights is whether anything is currently lit, so the clear control can disable itself
  // rather than offering an action with no effect (agni issue 348).
  hasHighlights: boolean;
}

// ControlsView is what the presenter pushes ControlsState to, implemented by the controls island
// (C3). ViewSink in viewer.ts describes the pattern.
export interface ControlsView {
  setState(s: ControlsState): void;
}
