import { For, Show } from "solid-js";
import { SolidIsland, signalView } from "@panyam/tsappkit-solid";
import type { EventBus } from "@panyam/tsappkit";
import type { OverviewState, OverviewView } from "./sheetoverview.js";

// SheetOverviewPanel is the birds-eye sheet list (WS9-025): one tile per sheet with its
// violation count — red-badged when nonzero, an explicit "clean" zero otherwise — and the
// shown sheet marked active. Clicking a tile navigates (the WS9-024 showSheet path).
// Name-only tiles for now; minimaps are the ticket's noted later refinement.
function SheetOverviewPanel(props: { state: () => OverviewState; onSelect: (sheetId: string) => void }) {
  return (
    <Show when={props.state().tiles.length > 0} fallback={<div class="findings-empty">No design open.</div>}>
      <ul class="sheet-tiles">
        <For each={props.state().tiles}>
          {(t) => (
            <li>
              <button
                type="button"
                class={`sheet-tile${props.state().activeId === t.id ? " active" : ""}`}
                onClick={() => props.onSelect(t.id)}
              >
                <span class="sheet-tile-name">{t.name}</span>
                <Show
                  when={props.state().ruleCount > 0}
                  fallback={<span class="sheet-tile-count norules" title="no rules selected">—</span>}
                >
                  <span class={`sheet-tile-count${t.count > 0 ? " firing" : " clean"}`}>{t.count}</span>
                </Show>
              </button>
            </li>
          )}
        </For>
      </ul>
    </Show>
  );
}

// sheetOverviewPanelIsland mounts the panel and returns its command-down view; onSelect is
// the intent up (show this sheet), same island shape as controlBarIsland.
export function sheetOverviewPanelIsland(
  el: HTMLElement,
  eventBus: EventBus | null,
  handlers: { onSelect: (sheetId: string) => void },
): { island: SolidIsland; view: OverviewView } {
  const [state, setState] = signalView<OverviewState>({ tiles: [], activeId: "", ruleCount: 0 });
  const island = new SolidIsland(
    "sheet-overview",
    el,
    () => <SheetOverviewPanel state={state} onSelect={handlers.onSelect} />,
    eventBus,
  );
  return { island, view: { setState } };
}
