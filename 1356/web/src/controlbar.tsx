import { For } from "solid-js";
import { SolidIsland, signalView } from "@panyam/tsappkit-solid";
import type { EventBus } from "@panyam/tsappkit";
import type { RenderMode } from "./viewer.js";
import type { ControlsState, ControlsView } from "./controls.js";

const MODES: { mode: RenderMode; label: string }[] = [
  { mode: "webgl", label: "WebGL" },
  { mode: "svg", label: "SVG" },
  { mode: "native", label: "Native" },
];

// Controls renders the render-mode buttons and the other toggles from ControlsState. It offers no
// layout or symbol-source choice. A design draws as its schematic does, or on agni's default layout
// with a note saying why (GetDesignResponse.drawing_note). `?layout=` and `?symbols=` still choose
// one from the address, for experiments. Everything derives from the signal, so the presenter drives
// it by pushing state, and every click or selection emits an intent up.
function Controls(props: {
  state: () => ControlsState;
  onMode: (m: RenderMode) => void;
  onClearHighlights: () => void;
  staticHost: boolean;
}) {
  const nativeDisabled = (m: RenderMode) => m === "native" && !props.state().nativeAvailable;
  return (
    <div class="controls">
      <div class="render-mode">
        {/* WebGL is offered only to a page already in it (`?mode=webgl`), since it lags on a large
            design (agni issue 1132). */}
        <For each={MODES.filter((m) => m.mode !== "webgl" || props.state().mode === "webgl")}>
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
    onClearHighlights: () => void;
  },
  opts: { staticHost?: boolean } = {},
): { island: SolidIsland; view: ControlsView } {
  const [state, setState] = signalView<ControlsState>({
    mode: "svg",
    nativeAvailable: false,
    hasHighlights: false,
  });
  const island = new SolidIsland(
    "controls",
    el,
    () => (
      <Controls
        state={state}
        onMode={handlers.onMode}
        onClearHighlights={handlers.onClearHighlights}
        staticHost={opts.staticHost ?? false}
      />
    ),
    eventBus,
  );
  return { island, view: { setState } };
}
