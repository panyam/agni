import { For, Show } from "solid-js";
import { SolidIsland, signalView } from "@panyam/tsappkit-solid";
import type { EventBus } from "@panyam/tsappkit";
import { type CoverageState, type CoverageView, type SignalCoverageItem, emptyCoverage, presentCount } from "./coverage.js";

// STATE_LABEL is the short label a signal chip shows for each coverage state (WS9-041); the chip's
// `cov-<state>` class colors it (present green, the three problem states amber/red).
const STATE_LABEL: Record<string, string> = {
  present: "✓",
  missing: "missing",
  dangling: "dangling",
  pullup_missing: "no pull-up",
};

// CoveragePanel shows one block per DETECTED interface profile: a header with the profile name and a
// present/total summary, then a chip per required signal (name + state). A signal with a matched net
// is clickable — it emits onLocate(net) so the presenter highlights that net on the canvas, the same
// locate path a finding or a query cell uses. An undetected design shows an empty-state message.
function CoveragePanel(props: { state: () => CoverageState; onLocate: (net: string) => void }) {
  const locate = (s: SignalCoverageItem) => {
    if (s.net) props.onLocate(s.net);
  };
  return (
    <div class="coverage">
      <Show
        when={props.state().interfaces.length > 0}
        fallback={<div class="coverage-empty">No interface profiles detected in this design.</div>}
      >
        <For each={props.state().interfaces}>
          {(iface) => (
            <div class="coverage-iface">
              <div class="coverage-head">
                <span class="coverage-profile">{iface.profile}</span>
                <span class="coverage-count">
                  {presentCount(iface)}/{iface.signals.length}
                </span>
              </div>
              <ul class="coverage-signals">
                <For each={iface.signals}>
                  {(s) => (
                    <li>
                      <button
                        type="button"
                        class={`coverage-signal cov-${s.state}${s.net ? "" : " no-net"}`}
                        title={s.net ? `${s.name} = ${s.net} (${s.state})` : `${s.name} (${s.state})`}
                        disabled={!s.net}
                        onClick={() => locate(s)}
                      >
                        <span class="coverage-signal-name">{s.name}</span>
                        <span class="coverage-signal-state">{STATE_LABEL[s.state] ?? s.state}</span>
                      </button>
                    </li>
                  )}
                </For>
              </ul>
            </div>
          )}
        </For>
      </Show>
    </div>
  );
}

// coveragePanelIsland mounts the panel and returns its command-down view (setState). onLocate is the
// intent up: the user clicked a signal, so the presenter locates its net. Same island shape as
// findingsPanelIsland / queryPanelIsland.
export function coveragePanelIsland(
  el: HTMLElement,
  eventBus: EventBus | null,
  handlers: { onLocate: (net: string) => void },
): { island: SolidIsland; view: CoverageView } {
  const [state, setState] = signalView<CoverageState>(emptyCoverage());
  const island = new SolidIsland("coverage", el, () => <CoveragePanel state={state} onLocate={handlers.onLocate} />, eventBus);
  return { island, view: { setState } };
}
