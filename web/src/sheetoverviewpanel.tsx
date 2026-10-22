import { For, Show, createMemo, createSignal } from "solid-js";
import { SolidIsland, signalView } from "@panyam/tsappkit-solid";
import type { EventBus } from "@panyam/tsappkit";
import type { OverviewState, OverviewView, SheetTile } from "./sheetoverview.js";
import { type SheetNode, sheetForest } from "./sheettree.js";

// SheetOverviewPanel is the birds-eye sheet list (WS9-025), one tile per sheet with its violation
// count and the shown sheet marked active. Clicking a tile navigates (the WS9-024 showSheet path).
// A hierarchical design's sheets nest under their parents, as the file tree nests them, and a
// parent folds (agni issue 1141). A flat design is one level, which draws as the plain list it was.
function SheetOverviewPanel(props: { state: () => OverviewState; onSelect: (sheetId: string) => void }) {
  const forest = createMemo(() => sheetForest(props.state().tiles));
  const [folded, setFolded] = createSignal(new Set<string>());
  const toggle = (id: string) =>
    setFolded((f) => {
      const next = new Set(f);
      if (!next.delete(id)) next.add(id);
      return next;
    });
  const counted = () => props.state().ruleCount > 0 && props.state().ranCount > 0;
  // nested is whether any sheet has children, so a flat list keeps no room for a fold arrow.
  const nested = createMemo(() => forest().some((n) => n.children.length > 0));

  const Tile = (p: { node: SheetNode<SheetTile>; depth: number }) => {
    const t = p.node.sheet;
    const parent = p.node.children.length > 0;
    return (
      <li>
        <div class="sheet-tile-row" style={{ "padding-left": `${p.depth * 12}px` }}>
          <Show when={parent} fallback={<Show when={nested()}><span class="sheet-tile-twist" /></Show>}>
            <button
              type="button"
              class="sheet-tile-twist"
              aria-label={folded().has(t.id) ? `show the sheets in ${t.name}` : `hide the sheets in ${t.name}`}
              aria-expanded={!folded().has(t.id)}
              onClick={() => toggle(t.id)}
            >
              {folded().has(t.id) ? "▸" : "▾"}
            </button>
          </Show>
          <button
            type="button"
            class={`sheet-tile${props.state().activeId === t.id ? " active" : ""}`}
            title={t.id !== t.name ? t.id : undefined}
            onClick={() => props.onSelect(t.id)}
          >
            <span class="sheet-tile-name">{t.name}</span>
            <Show
              when={props.state().ruleCount > 0}
              fallback={<span class="sheet-tile-count norules" title="no rules selected">—</span>}
            >
              {/*
                Two counts, because an inconclusive result is not a defect (agni issue 350). A
                zero draws nothing, and nothing is drawn before a run, since a green 0 there
                claimed clean over rules that never ran (agni issue 1132).
              */}
              <Show when={props.state().ranCount > 0}>
                <span class="sheet-tile-counts">
                  <Show when={t.count > 0}>
                    <span class="sheet-tile-count firing" title={defectsTitle(t)}>
                      {t.count}
                    </span>
                  </Show>
                  <Show when={t.unresolved > 0}>
                    <span class="sheet-tile-count unresolved" title={`${t.unresolved} inconclusive: the rule could not decide`}>
                      {t.unresolved}?
                    </span>
                  </Show>
                </span>
              </Show>
            </Show>
          </button>
        </div>
        <Show when={parent && !folded().has(t.id)}>
          <ul class="sheet-tiles">
            <For each={p.node.children}>{(c) => <Tile node={c} depth={p.depth + 1} />}</For>
          </ul>
        </Show>
      </li>
    );
  };

  return (
    <Show when={props.state().tiles.length > 0} fallback={<div class="findings-empty">No design open.</div>}>
      <ul class="sheet-tiles">
        <For each={forest()}>{(n) => <Tile node={n} depth={0} />}</For>
      </ul>
      {/* The pills are counts beside sheet names, which on a design whose sheets are named 1, 2, 3
          read as more numbering until something says what they are (agni issue 1142). */}
      <Show when={counted()}>
        <div class="sheet-tiles-legend">
          <span class="sheet-tile-count firing">n</span> defects, <span class="sheet-tile-count unresolved">n?</span> inconclusive. A
          finding on something drawn on several sheets counts on each.
        </div>
      </Show>
    </Show>
  );
}

// defectsTitle says what the red pill counts, and how many of those defects are drawn on this sheet
// alone rather than on a subject that runs across several (agni issue 1144).
export function defectsTitle(t: SheetTile): string {
  const head = `${t.count} defect${t.count === 1 ? "" : "s"} on this sheet`;
  if (t.local === t.count) return `${head}, all drawn here alone`;
  return `${head}, ${t.local} drawn here alone and ${t.count - t.local} also on other sheets`;
}

// sheetOverviewPanelIsland mounts the panel and returns its command-down view. Same island shape
// as controlBarIsland (docsite/content/architecture/web-client.md#wiring-a-new-panel).
export function sheetOverviewPanelIsland(
  el: HTMLElement,
  eventBus: EventBus | null,
  handlers: { onSelect: (sheetId: string) => void },
): { island: SolidIsland; view: OverviewView } {
  const [state, setState] = signalView<OverviewState>({ tiles: [], activeId: "", ruleCount: 0, ranCount: 0 });
  const island = new SolidIsland(
    "sheet-overview",
    el,
    () => <SheetOverviewPanel state={state} onSelect={handlers.onSelect} />,
    eventBus,
  );
  return { island, view: { setState } };
}
