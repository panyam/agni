import { Show } from "solid-js";
import { SolidIsland, signalView } from "@panyam/tsappkit-solid";
import type { EventBus } from "@panyam/tsappkit";
import type { RenderMode } from "./viewer.js";
import type { ControlsState, ControlsView } from "./controls.js";

// Controls renders the renderer checkboxes and the other toggles from ControlsState. It offers no
// layout or symbol-source choice. A design draws as its schematic does, or on agni's default layout
// with a note saying why (GetDesignResponse.drawing_note). `?layout=`, `?symbols=` and `?mode=` still
// choose one from the address, for experiments. Everything derives from the signal, so the presenter
// drives it by pushing state, and every click or selection emits an intent up.
//
// The renderer is one WebGL checkbox, off for SVG (agni issue 1469). SVG is the default, and
// DECISIONS.md says until when. Native gets a checkbox of its own only where the server can draw
// this design natively, since most servers never enable it.
function Controls(props: { state: () => ControlsState; onMode: (m: RenderMode) => void; onClearHighlights: () => void }) {
  const mode = () => props.state().mode;
  return (
    <div class="controls">
      <div class="render-mode">
        <label class="render-toggle" title="draw with WebGL, or with SVG when off">
          <input
            type="checkbox"
            class="render-webgl"
            checked={mode() === "webgl"}
            onChange={(e) => props.onMode(e.currentTarget.checked ? "webgl" : "svg")}
          />
          WebGL
        </label>
        <Show when={props.state().nativeAvailable}>
          <label class="render-toggle" title="draw with the format's own tool">
            <input
              type="checkbox"
              class="render-native"
              checked={mode() === "native"}
              onChange={(e) => props.onMode(e.currentTarget.checked ? "native" : "svg")}
            />
            Native
          </label>
        </Show>
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
      />
    ),
    eventBus,
  );
  return { island, view: { setState } };
}
