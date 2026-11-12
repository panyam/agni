import { For, Show } from "solid-js";
import { SolidIsland, signalView } from "@panyam/tsappkit-solid";
import type { EventBus } from "@panyam/tsappkit";
import { type CompareOptions, type TablesSourceKey, type TablesState, type TablesView, emptyTables } from "./tables.js";

// LOCATABLE are the column kinds a cell click can find on the drawing. A pin needs its part as well
// as its name, and a table column carries only the cell, so pin columns stay text.
const LOCATABLE = new Set(["net", "component"]);

// SOURCES are the buttons that choose what the panel shows. The comparison's appear while one is open.
const SOURCES: { key: TablesSourceKey; label: string; title: string; compare: boolean }[] = [
  { key: "checks", label: "Checks", title: "The check run on screen, as its findings, verdicts and skipped rules", compare: false },
  { key: "compare", label: "Comparison", title: "What changed between the two revisions, a row per change", compare: true },
  { key: "compare-a", label: "Checks, old", title: "The checks run over the comparison's old side", compare: true },
  { key: "compare-b", label: "Checks, new", title: "The checks run over the comparison's new side", compare: true },
];

// TablesPanel renders the tables TableService.Tabulate projects from one answer (agni issue 898). It
// orders nothing itself: a header click asks for the engine's order, and Save CSV writes the engine's
// bytes, so what is on screen is what the CLI's csv holds.
function TablesPanel(props: {
  state: () => TablesState;
  onSource: (key: TablesSourceKey, compare: CompareOptions) => void;
  onSelect: (i: number) => void;
  onSort: (column: string) => void;
  onSave: () => void;
  onLocate: (kind: string, subject: string) => void;
}) {
  const active = () => props.state().tables[props.state().active];
  const arrow = (col: string) => (props.state().orderBy === col ? " ▲" : props.state().orderBy === `-${col}` ? " ▼" : "");
  // The options show only beside the comparison, so a change asks for the comparison again.
  const setCompare = (patch: Partial<CompareOptions>) => props.onSource("compare", { ...props.state().compare, ...patch });
  return (
    <div class="tables-panel">
      <div class="tables-sources" role="toolbar">
        <For each={SOURCES.filter((s) => !s.compare || props.state().compareOpen)}>
          {(s) => (
            <button
              type="button"
              class="tables-source"
              classList={{ active: props.state().source === s.key }}
              data-source={s.key}
              title={s.title}
              disabled={props.state().loading}
              onClick={() => props.onSource(s.key, props.state().compare)}
            >
              {s.label}
            </button>
          )}
        </For>
        <Show when={props.state().source === "compare"}>
          <label class="tables-option" title="Also pair a net renamed and changed slightly with its old self, as agni diff does when asked for near-renames">
            <input type="checkbox" class="tables-near-renames" checked={props.state().compare.nearRenames} onChange={(e) => setCompare({ nearRenames: e.currentTarget.checked })} />
            near-renames
          </label>
          <label class="tables-option" title="Also list every net that did not change">
            <input type="checkbox" class="tables-equal" checked={props.state().compare.equal} onChange={(e) => setCompare({ equal: e.currentTarget.checked })} />
            unchanged nets
          </label>
        </Show>
      </div>
      <Show when={props.state().source === ""}>
        <p class="tables-empty">Choose an answer to see it as the tables a csv carries.</p>
      </Show>
      <Show when={props.state().error !== ""}>
        <p class="tables-error">{props.state().error}</p>
      </Show>
      <Show when={props.state().tables.length > 0}>
        <div class="tables-head">
          <span class="tables-label">{props.state().label}</span>
          <For each={props.state().tables}>
            {(t, i) => (
              <button type="button" class="tables-tab" classList={{ active: i() === props.state().active }} data-table={t.name} onClick={() => props.onSelect(i())}>
                {t.name} <span class="tables-tab-count">{t.rows.length}</span>
              </button>
            )}
          </For>
          <button type="button" class="tables-save" title="Save this table as a csv file, the same file agni writes as csv" disabled={!active()} onClick={() => props.onSave()}>
            Save CSV
          </button>
        </div>
        <Show when={active()}>
          {(t) => (
            <>
              <table class="query-table tables-table">
                <thead>
                  <tr>
                    <For each={t().columns}>
                      {(c) => (
                        <th class="query-sortable" data-column={c.name} title={`Order by ${c.name}`} onClick={() => props.onSort(c.name)}>
                          {c.name}
                          {arrow(c.name)}
                        </th>
                      )}
                    </For>
                  </tr>
                </thead>
                <tbody>
                  <For each={t().rows}>
                    {(r) => (
                      <tr class="tables-row">
                        <For each={r.cells}>
                          {(cell, i) => {
                            // A row names its own cell's kind where the column names different kinds
                            // on different rows, such as a diff's subject (TableRow.cell_kinds).
                            const kind = r.cellKinds[i()] || t().columns[i()]?.kind || "";
                            return (
                              <td>
                                <Show when={cell !== "" && LOCATABLE.has(kind)} fallback={<>{cell}</>}>
                                  <button type="button" class="query-locate tables-locate" data-kind={kind} onClick={() => props.onLocate(kind, cell)}>
                                    {cell}
                                  </button>
                                </Show>
                              </td>
                            );
                          }}
                        </For>
                      </tr>
                    )}
                  </For>
                </tbody>
              </table>
              <div class="query-count tables-count">{t().rows.length} row(s)</div>
            </>
          )}
        </Show>
      </Show>
    </div>
  );
}

// tablesPanelIsland mounts the panel and returns its view, per
// docsite/content/architecture/web-client.md#wiring-a-new-panel. The TablesPresenter pushes state
// through view.setState and the handlers carry the reader's choices back to it.
export function tablesPanelIsland(
  el: HTMLElement,
  eventBus: EventBus | null,
  handlers: {
    onSource: (key: TablesSourceKey, compare: CompareOptions) => void;
    onSelect: (i: number) => void;
    onSort: (column: string) => void;
    onSave: () => void;
    onLocate: (kind: string, subject: string) => void;
  },
): { island: SolidIsland; view: TablesView } {
  const [state, setState] = signalView<TablesState>(emptyTables());
  const island = new SolidIsland(
    "tables",
    el,
    () => <TablesPanel state={state} onSource={handlers.onSource} onSelect={handlers.onSelect} onSort={handlers.onSort} onSave={handlers.onSave} onLocate={handlers.onLocate} />,
    eventBus,
  );
  return { island, view: { setState } };
}
