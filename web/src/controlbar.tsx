import { For, Show } from "solid-js";
import { SolidIsland, signalView } from "@panyam/tsappkit-solid";
import type { EventBus } from "@panyam/tsappkit";
import type { RenderMode } from "./viewer.js";
import type { ControlsState, ControlsView } from "./controls.js";

const MODES: { mode: RenderMode; label: string }[] = [
  { mode: "webgl", label: "WebGL" },
  { mode: "svg", label: "SVG" },
  { mode: "native", label: "Native" },
];

// Controls renders the render-mode buttons, layout selector and the other toggles from
// ControlsState. Everything derives from the signal, so the presenter drives it by pushing state,
// and every click or selection emits an intent up.
function Controls(props: {
  state: () => ControlsState;
  onMode: (m: RenderMode) => void;
  onLayout: (l: string) => void;
  onSymbols: (faithful: boolean) => void;
  onBoardLayers: (side: string) => void;
  onClearHighlights: () => void;
  staticHost: boolean;
}) {
  // The faithful-symbols toggle is only meaningful for an auto-layout of a design that ships
  // symbols; the faithful layout already draws them, and native has its own pages.
  const showSymbols = () =>
    props.state().providedSymbols && props.state().layout !== "faithful" && props.state().mode !== "native";
  const nativeDisabled = (m: RenderMode) => m === "native" && !props.state().nativeAvailable;
  return (
    <div class="controls">
      <div class="render-mode">
        <For each={MODES}>
          {(m) => (
            <button
              type="button"
              class={`mode-btn${props.state().mode === m.mode ? " active" : ""}`}
              disabled={nativeDisabled(m.mode)}
              title={
                nativeDisabled(m.mode)
                  ? props.staticHost
                    ? "native rendering is not available in this demo"
                    : "no native renderer for this format (or not enabled with --enable-native)"
                  : ""
              }
              onClick={() => props.onMode(m.mode)}
            >
              {m.label}
            </button>
          )}
        </For>
      </div>
      <Show when={props.state().layouts.length > 1}>
        <select
          class="layout-select"
          title="geometry layout"
          disabled={props.state().mode === "native"}
          value={props.state().layout}
          onChange={(e) => props.onLayout(e.currentTarget.value)}
        >
          <For each={props.state().layouts}>{(l) => <option value={l}>{l}</option>}</For>
        </select>
      </Show>
      <Show when={showSymbols()}>
        <label class="symbols-toggle" title="draw the design's own symbols instead of synthetic glyphs">
          <input
            type="checkbox"
            checked={props.state().faithfulSymbols}
            onChange={(e) => props.onSymbols(e.currentTarget.checked)}
          />
          Provided symbols
        </label>
      </Show>
      <Show when={props.state().board}>
        <select
          class="layout-select"
          title="board layer visibility"
          value={props.state().boardLayers}
          onChange={(e) => props.onBoardLayers(e.currentTarget.value)}
        >
          <For each={["all", "front", "back"]}>{(s) => <option value={s}>{s} layers</option>}</For>
        </select>
      </Show>
      {/* Always rendered, and disabled when there is nothing to clear, because a control that
          appears on a state change has to be discovered twice (agni issue 348). */}
      <button
        type="button"
        class="mode-btn clear-highlights"
        disabled={!props.state().hasHighlights}
        title="clear the highlight (Escape)"
        onClick={() => props.onClearHighlights()}
      >
        Clear highlight
      </button>
    </div>
  );
}

// controlBarIsland mounts the control bar and returns its command-down view.
export function controlBarIsland(
  el: HTMLElement,
  eventBus: EventBus | null,
  handlers: {
    onMode: (m: RenderMode) => void;
    onLayout: (l: string) => void;
    onSymbols: (faithful: boolean) => void;
    onBoardLayers: (side: string) => void;
    onClearHighlights: () => void;
  },
  opts: { staticHost?: boolean } = {},
): { island: SolidIsland; view: ControlsView } {
  const [state, setState] = signalView<ControlsState>({
    mode: "svg",
    nativeAvailable: false,
    layouts: [],
    layout: "",
    providedSymbols: false,
    faithfulSymbols: false,
    board: false,
    boardLayers: "all",
    hasHighlights: false,
  });
  const island = new SolidIsland(
    "controls",
    el,
    () => (
      <Controls
        state={state}
        onMode={handlers.onMode}
        onLayout={handlers.onLayout}
        onSymbols={handlers.onSymbols}
        onBoardLayers={handlers.onBoardLayers}
        onClearHighlights={handlers.onClearHighlights}
        staticHost={opts.staticHost ?? false}
      />
    ),
    eventBus,
  );
  return { island, view: { setState } };
}
