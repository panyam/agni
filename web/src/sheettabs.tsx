import { For, Show } from "solid-js";
import { SolidIsland, signalView } from "@panyam/tsappkit-solid";
import type { EventBus } from "@panyam/tsappkit";
import type { SheetsState, SheetsView } from "./sheets.js";

// SheetTab is one strip entry: a sheet the user has actually VISITED in the open design. The
// strip deliberately holds a plain id/name pair rather than the proto SheetRef, so the view
// state does not track the wire type.
export interface SheetTab {
  id: string;
  name: string;
}

// TabStripState is the strip's own view state. It is NOT a mirror of the design's sheet list:
// a design's full hierarchy lives in the Sheets overview panel, and opening a file would put
// dozens of tabs on screen at once if the strip mirrored it. Instead a sheet earns a tab the
// first time it is shown (from the Sheets panel, a finding, a query locate, or a deep link),
// the way an editor accumulates open-file tabs. mount/path identify the design the tabs belong
// to, so switching designs clears them.
export interface TabStripState {
  mount: string;
  path: string;
  tabs: SheetTab[];
  activeId: string;
}

export function emptyTabStrip(): TabStripState {
  return { mount: "", path: "", tabs: [], activeId: "" };
}

// visitSheet folds a presenter SheetsState push into the strip. Tabs accumulate in VISIT order
// and keep their position on a revisit: reordering the strip on every navigation (a true MRU)
// moves tabs out from under the cursor, which is why editors do not do it either. A push for a
// different design resets the strip, and a push with no active sheet adds nothing — a design
// whose sheet is still loading shows an empty strip rather than a placeholder tab.
export function visitSheet(prev: TabStripState, s: SheetsState): TabStripState {
  const sameFile = prev.mount === s.mount && prev.path === s.path;
  const tabs = sameFile ? prev.tabs : [];
  if (!s.activeId) return { mount: s.mount, path: s.path, tabs, activeId: "" };
  if (tabs.some((t) => t.id === s.activeId)) return { mount: s.mount, path: s.path, tabs, activeId: s.activeId };
  const ref = s.sheets.find((sh) => sh.id === s.activeId);
  // An active id the design does not list is not tabbable (nothing to label it with), but it is
  // still the active sheet, so the strip reports it rather than silently disagreeing with the canvas.
  if (!ref) return { mount: s.mount, path: s.path, tabs, activeId: s.activeId };
  return { mount: s.mount, path: s.path, tabs: [...tabs, { id: ref.id, name: ref.name || ref.id }], activeId: s.activeId };
}

// closeTab removes a tab and reports which sheet should become active as a result. Closing an
// INACTIVE tab changes nothing on the canvas (select ""); closing the ACTIVE one hands back the
// neighbour that slid into its place (or the new last tab when it was the rightmost), so the
// canvas never keeps rendering a sheet with no tab. The caller turns a non-empty select into the
// presenter intent, because showing a sheet is the presenter's decision, not the strip's.
export function closeTab(prev: TabStripState, id: string): { next: TabStripState; select: string } {
  const i = prev.tabs.findIndex((t) => t.id === id);
  if (i < 0) return { next: prev, select: "" };
  const tabs = prev.tabs.filter((t) => t.id !== id);
  if (prev.activeId !== id) return { next: { ...prev, tabs }, select: "" };
  const neighbour = tabs[Math.min(i, tabs.length - 1)];
  return { next: { ...prev, tabs, activeId: neighbour?.id ?? "" }, select: neighbour?.id ?? "" };
}

function TabStrip(props: { state: () => TabStripState; onSelect: (id: string) => void; onClose: (id: string) => void }) {
  return (
    <div class="sheet-tabs">
      <For each={props.state().tabs}>
        {(tab) => (
          <span class={`sheet-tab${props.state().activeId === tab.id ? " active" : ""}`}>
            <button type="button" class="sheet-tab-label" title={tab.name} onClick={() => props.onSelect(tab.id)}>
              {tab.name}
            </button>
            {/* The close affordance is hidden on the last remaining tab: closing it would leave a
                rendered canvas with an empty strip, a state with no way back except the Sheets panel. */}
            <Show when={props.state().tabs.length > 1}>
              <button type="button" class="sheet-tab-close" title={`close ${tab.name}`} onClick={() => props.onClose(tab.id)}>
                ×
              </button>
            </Show>
          </span>
        )}
      </For>
    </div>
  );
}

// SheetTabHandlers is the strip's single intent: the user wants a sheet shown. Closing a tab is
// view-local state, so it is not an intent — it only reaches the presenter when it implies a
// different sheet must be shown, and then it arrives as onSelect like any other navigation.
export interface SheetTabHandlers {
  onSelect: (sheetId: string) => void;
}

// sheetTabsIsland mounts the visited-sheet tab strip and returns its SheetsView, which the
// presenter pushes the open design's sheet state to (the same push the Sheets panel and the file
// tree receive — see ViewerPresenter's sheetNavs). Framework reactivity stays in this leaf
// (CONSTRAINTS C11).
export function sheetTabsIsland(
  el: HTMLElement,
  eventBus: EventBus | null,
  handlers: SheetTabHandlers,
): { island: SolidIsland; view: SheetsView } {
  const [state, setState] = signalView<TabStripState>(emptyTabStrip());
  const select = (id: string): void => {
    if (id !== state().activeId) handlers.onSelect(id);
  };
  const close = (id: string): void => {
    const { next, select: wanted } = closeTab(state(), id);
    setState(next);
    if (wanted) handlers.onSelect(wanted);
  };
  const island = new SolidIsland("sheet-tabs", el, () => <TabStrip state={state} onSelect={select} onClose={close} />, eventBus);
  return { island, view: { setState: (s) => setState(visitSheet(state(), s)) } };
}
