import { For, Show } from "solid-js";
import { SolidIsland, signalView } from "@panyam/tsappkit-solid";
import type { BoardLayerRow, BoardStackView } from "@panyam/agni-client/gen/agni/v1/geom/geom_packed_pb";
import type { LayerPreset } from "./boardlayers.js";
import { TYPICAL_MM } from "./stacklayout.js";

// LayerControl is what a board layer list asks of its owner (agni 1351). A click on a row toggles
// it, a double-click shows it alone, and a preset button replaces the whole hidden set.
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

// BoardLayerList is a board's layer rows with the preset buttons above them. The Sheets panel shows
// it under the Board tile for the canvas, and a sheet view beside the canvas shows it in a popover
// over its own hidden set (agni issue 1401), so both lists behave alike. A row the board's stack
// names shows its thickness, and the board's whole stack follows the rows, dielectrics included,
// since a dielectric draws nothing and has no row to toggle (agni 1603).
export function BoardLayerList(props: {
  rows: () => BoardLayerRow[];
  hidden: () => ReadonlySet<string>;
  layers: LayerControl;
  stack?: () => BoardStackView | undefined;
  indent?: number;
}) {
  return (
    <div class="board-layers" style={props.indent ? { "padding-left": `${props.indent}px` } : undefined}>
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
        <For each={props.rows()}>
          {(r) => (
            <li>
              {/*
                One click handler for the whole row, the checkbox included, read by the browser's own
                click count. The list draws its rows again on every state push, so a dblclick
                handler or two handlers racing a redraw lost the second click (agni 1351). A key
                press on the checkbox arrives as a click with detail 0 and toggles.
              */}
              <div
                class="board-layer-row"
                title={`${r.name}: double-click to show it alone`}
                onClick={(e) => (e.detail >= 2 ? props.layers.onSolo(r.name) : props.layers.onToggle(r.name))}
              >
                <input type="checkbox" checked={!props.hidden().has(r.name)} aria-label={r.name} />
                <span class="board-layer-swatch" style={{ background: r.color }} />
                <span class="board-layer-name">{r.name}</span>
                <Show when={r.thickness}>
                  <span class="board-layer-thickness" data-thickness={r.thickness}>
                    {r.thickness}
                  </span>
                </Show>
              </div>
            </li>
          )}
        </For>
      </ul>
      <Show when={props.stack?.()?.rows.length}>
        <BoardStackTable stack={props.stack!()!} />
      </Show>
    </div>
  );
}

// BoardStackTable is the board's physical stack, top to bottom, then how the layers compare with
// the board thickness the file states. A row the file gives no thickness also says the typical
// thickness the stack view draws it at (agni issue 1593).
function BoardStackTable(props: { stack: BoardStackView }) {
  return (
    <div class="board-stack" data-board-stack>
      <div class="board-stack-title">Stack</div>
      <table class="board-stack-rows" aria-label="board layer stack">
        <tbody>
          <For each={props.stack.rows}>
            {(r) => (
              <tr data-kind={r.kind}>
                <td class="board-stack-name">{r.name}</td>
                <td class="board-stack-thickness" data-stack-thickness={r.thickness}>
                  {r.thickness}
                  <Show when={r.thicknessMm === undefined}>
                    <span
                      class="board-stack-default"
                      data-stack-default={String(typicalMm(r.kind))}
                      title="the stack view draws this layer at a typical thickness, since the file gives none"
                    >
                      {typicalMm(r.kind) > 0 ? ` (drawn at ${typicalMm(r.kind)} mm)` : " (drawn on the surface)"}
                    </span>
                  </Show>
                </td>
                <td class="board-stack-material">{r.material}</td>
              </tr>
            )}
          </For>
        </tbody>
      </table>
      <Show when={props.stack.summary}>
        <div class="board-stack-summary" data-disagrees={String(props.stack.disagrees)}>
          {props.stack.summary}
        </div>
      </Show>
    </div>
  );
}

function typicalMm(kind: string): number {
  return TYPICAL_MM[kind] ?? TYPICAL_MM.other;
}

// LayerListState is what a mounted list draws.
export interface LayerListState {
  rows: BoardLayerRow[];
  hidden: ReadonlySet<string>;
  stack?: BoardStackView;
}

// boardLayerListIsland mounts a list into an element that plain DOM code owns, such as a sheet
// view's popover, and returns the setter that redraws it.
export function boardLayerListIsland(el: HTMLElement, layers: LayerControl): { island: SolidIsland; setState: (s: LayerListState) => void } {
  const [state, setState] = signalView<LayerListState>({ rows: [], hidden: new Set() });
  const island = new SolidIsland(
    "board-layer-list",
    el,
    () => <BoardLayerList rows={() => state().rows} hidden={() => state().hidden} stack={() => state().stack} layers={layers} />,
    null,
  );
  return { island, setState };
}
