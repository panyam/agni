import { For, Show, createMemo, createSignal } from "solid-js";
import { SolidIsland, signalView } from "@panyam/tsappkit-solid";
import type { EventBus } from "@panyam/tsappkit";
import type { OverviewState, OverviewView, SheetTile } from "./sheetoverview.js";
import { type SheetNode, sheetForest } from "./sheettree.js";
import type { LayerPreset } from "./boardlayers.js";

// SheetOverviewPanel is the birds-eye sheet list (WS9-025), one tile per sheet with its violation
// count and the shown sheet marked active. Clicking a tile navigates (the WS9-024 showSheet path).
// A hierarchical design's sheets nest under their parents, as the file tree nests them, and a
// parent folds (agni issue 1141). A flat design is one level, which draws as the plain list it was.
// BoardControl is the panel's part in attaching a board to the design (agni issue 1367). attached is
// the board the page attaches, "<mount>/<path>", or "" for the design's own.
export interface BoardControl {
  attached: string;
  onPick: () => void;
  onClear: () => void;
  // onSave writes the attached board into the design's design.yaml, or shows the text to write.
  onSave?: () => void;
}

// LayerControl is the panel's part in board layer visibility (agni 1351). The Board tile lists the
// board's layers while the board is shown. A click on a row toggles it and a double-click shows it
// alone.
export interface LayerControl {
  onToggle: (name: string) => void;
  onSolo: (name: string) => void;
  onPreset: (preset: LayerPreset) => void;
}

const PRESETS: { preset: LayerPreset; label: string; title: string }[] = [
  { preset: "all", label: "All", title: "show every layer" },
  { preset: "none", label: "None", title: "hide every layer" },
  { preset: "front", label: "Front", title: "show the front side: its layers, the edge, the through copper and the zones" },
  { preset: "back", label: "Back", title: "show the back side: its layers, the edge, the through copper and the zones" },
];

function BoardLayers(props: { state: () => OverviewState; layers: LayerControl; depth: number }) {
  const hidden = createMemo(() => new Set(props.state().hiddenLayers ?? []));
  return (
    <div class="board-layers" style={{ "padding-left": `${(props.depth + 1) * 12}px` }}>
      <div class="board-layer-presets" role="group" aria-label="board layer presets">
        <For each={PRESETS}>
          {(p) => (
            <button type="button" class="mode-btn board-layer-preset" title={p.title} onClick={() => props.layers.onPreset(p.preset)}>
              {p.label}
            </button>
          )}
        </For>
      </div>
      <ul class="board-layer-rows" aria-label="board layers">
        <For each={props.state().boardLayers ?? []}>
          {(r) => (
            <li>
              <label class="board-layer-row" title={`${r.name}: double-click to show it alone`} onDblClick={() => props.layers.onSolo(r.name)}>
                <input type="checkbox" checked={!hidden().has(r.name)} onChange={() => props.layers.onToggle(r.name)} />
                <span class="board-layer-swatch" style={{ background: r.color }} />
                <span class="board-layer-name">{r.name}</span>
              </label>
            </li>
          )}
        </For>
      </ul>
    </div>
  );
}

function SheetOverviewPanel(props: {
  state: () => OverviewState;
  onSelect: (sheetId: string) => void;
  onOpenBeside?: (sheetId: string) => void;
  board?: BoardControl;
  layers?: LayerControl;
}) {
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
          {/* Opens the sheet in a view of its own beside the canvas (agni issue 1326), only when asked. */}
          <Show when={props.onOpenBeside}>
            <button
              type="button"
              class="sheet-tile-beside"
              title={`open ${t.name} in a new view`}
              aria-label={`open ${t.name} in a new view`}
              onClick={() => props.onOpenBeside?.(t.id)}
            >
              ⧉
            </button>
          </Show>
        </div>
        <Show when={t.id === "board" && props.layers && props.state().activeId === "board" && (props.state().boardLayers ?? []).length > 0}>
          <BoardLayers state={props.state} layers={props.layers!} depth={p.depth} />
        </Show>
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
      <Show when={props.board}>
        {(b) => (
          <div class="sheet-board">
            <Show
              when={b().attached}
              fallback={
                <button type="button" class="mode-btn sheet-board-pick" title="attach a board this design does not declare" onClick={() => b().onPick()}>
                  Use a board…
                </button>
              }
            >
              <span class="sheet-board-attached" title="this page attaches this board; the design does not declare it">
                board: {b().attached}
              </span>
              <button type="button" class="mode-btn sheet-board-pick" onClick={() => b().onPick()}>
                Change…
              </button>
              <Show when={b().onSave}>
                <button type="button" class="mode-btn sheet-board-save" title="declare this board in the design's design.yaml" onClick={() => b().onSave?.()}>
                  Save to design.yaml
                </button>
              </Show>
              <button type="button" class="mode-btn sheet-board-clear" title="read the board the design declares, if any" onClick={() => b().onClear()}>
                Use the design's own
              </button>
            </Show>
          </div>
        )}
      </Show>
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
  handlers: { onSelect: (sheetId: string) => void; onOpenBeside?: (sheetId: string) => void; board?: BoardControl; layers?: LayerControl },
): { island: SolidIsland; view: OverviewView } {
  const [state, setState] = signalView<OverviewState>({ tiles: [], activeId: "", ruleCount: 0, ranCount: 0 });
  const island = new SolidIsland(
    "sheet-overview",
    el,
    () => <SheetOverviewPanel state={state} onSelect={handlers.onSelect} onOpenBeside={handlers.onOpenBeside} board={handlers.board} layers={handlers.layers} />,
    eventBus,
  );
  return { island, view: { setState } };
}
