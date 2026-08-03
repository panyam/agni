import { For, Show } from "solid-js";
import { SolidIsland, signalView } from "@panyam/tsappkit-solid";
import type { EventBus } from "@panyam/tsappkit";
import { emptyDiffState, type DiffMode, type DiffState } from "./diffpresenter.js";

// DiffPanel is the diff view's chrome bar: the two file labels, the sheet-pair selector,
// the change-class legend (swatch + label + design-wide count), and the close button. The
// two canvases below it are plain DOM the composition root owns (the SvgViews are not
// islands, same as the single-file viewer); this island renders only from the pushed
// DiffState and emits pair/close intents up (C3).
function DiffPanel(props: {
  state: () => DiffState;
  onPair: (i: number) => void;
  onMode: (m: DiffMode) => void;
  onClose: () => void;
}) {
  const pairLabel = (p: { name: string; aId: string; bId: string }) =>
    p.aId && p.bId ? p.name : p.aId ? `${p.name} (A only)` : `${p.name} (B only)`;
  return (
    <Show when={props.state().active} fallback={<div class="diff-empty">No comparison open. Use Compare in the top bar.</div>}>
      <div class="diff-head">
        <div class="diff-files">
          <span class="diff-file" title={props.state().aLabel}>
            <b>A</b> {props.state().aLabel}
          </span>
          <span class="diff-file" title={props.state().bLabel}>
            <b>B</b> {props.state().bLabel}
          </span>
        </div>
        <Show when={props.state().pairs.length > 1}>
          <select
            class="diff-pair-select"
            title="sheet pair"
            value={String(props.state().activePair)}
            onChange={(e) => props.onPair(Number(e.currentTarget.value))}
          >
            <For each={props.state().pairs}>{(p, i) => <option value={String(i())}>{pairLabel(p)}</option>}</For>
          </select>
        </Show>
        <div class="diff-modes">
          <button
            type="button"
            class={`mode-btn${props.state().mode === "side" ? " active" : ""}`}
            onClick={() => props.onMode("side")}
          >
            Side by side
          </button>
          <button
            type="button"
            class={`mode-btn${props.state().mode === "overlay" ? " active" : ""}`}
            disabled={!props.state().overlayOk}
            title={props.state().overlayOk ? "superimpose both revisions on one canvas" : props.state().overlayReason}
            onClick={() => props.onMode("overlay")}
          >
            Overlay
          </button>
        </div>
        <Show when={props.state().error}>
          <span class="diff-error">{props.state().error}</span>
        </Show>
        <div class="diff-legend">
          <Show when={!props.state().error && props.state().legend.length === 0}>
            <span class="diff-chip diff-nochange">no changes</span>
          </Show>
          <For each={props.state().legend}>
            {(e) => (
              <span class="diff-chip" title={e.cls}>
                <span class="diff-swatch" style={{ background: e.color }} />
                {e.count} {e.label}
              </span>
            )}
          </For>
        </div>
        <button type="button" class="diff-close" title="close the comparison" onClick={() => props.onClose()}>
          ✕
        </button>
      </div>
    </Show>
  );
}

// DiffPanelView is the command-down surface the presenter's onState feeds.
export interface DiffPanelView {
  setState(s: DiffState): void;
}

// diffPanelIsland mounts the chrome bar and returns its command-down view; onPair/onClose
// are the intents up, same island shape as controlBarIsland.
export function diffPanelIsland(
  el: HTMLElement,
  eventBus: EventBus | null,
  handlers: { onPair: (i: number) => void; onMode: (m: DiffMode) => void; onClose: () => void },
): { island: SolidIsland; view: DiffPanelView } {
  const [state, setState] = signalView<DiffState>(emptyDiffState());
  const island = new SolidIsland(
    "diff-bar",
    el,
    () => <DiffPanel state={state} onPair={handlers.onPair} onMode={handlers.onMode} onClose={handlers.onClose} />,
    eventBus,
  );
  return { island, view: { setState } };
}
