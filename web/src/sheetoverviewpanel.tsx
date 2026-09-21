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
                  {/*
                    Two counts, because an inconclusive result is not a defect (agni issue 350). The
                    zero only goes green when there is nothing unresolved either: a sheet whose three
                    findings are all undecided is not clean, it is unexamined, and the green badge
                    was the tile saying otherwise.
                  */}
                  <span class="sheet-tile-counts">
                    <span class={`sheet-tile-count${t.count > 0 ? " firing" : t.unresolved > 0 ? " open" : " clean"}`}>{t.count}</span>
                    <Show when={t.unresolved > 0}>
                      <span class="sheet-tile-count unresolved" title={`${t.unresolved} inconclusive: the rule could not decide`}>
                        {t.unresolved}?
                      </span>
                    </Show>
                  </span>
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
