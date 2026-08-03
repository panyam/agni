import type { RenderMode } from "./viewer.js";

// ControlsState is the full state of the viewer's control bar: which renderer is active, whether
// Native is offered for the current file, and the layout axis (available options + the effective
// one). The presenter owns this state and pushes it down; the controls island renders from it.
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
}

// ControlsView is the command-down surface the presenter pushes ControlsState to. The controls
// island implements it (like SheetsView for the sheet navigators), so the bar's buttons and
// layout selector always reflect presenter state without the presenter touching the DOM (C3).
export interface ControlsView {
  setState(s: ControlsState): void;
}
