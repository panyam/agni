import { For, Show, createSignal } from "solid-js";
import { SolidIsland, signalView } from "@panyam/tsappkit-solid";
import type { EventBus } from "@panyam/tsappkit";
import { CLASS_LABELS, DIFF_COLORS, ITEM_CLASS_ORDER, itemId, itemPairs, type ChangedItem } from "./diff.js";
import { emptyDiffState, type DiffState } from "./diffpresenter.js";
import { SheetBadges } from "./sheetbadges.jsx";

// ChangesPanel is the WS9-006 changed-item list: every changed component and net, grouped by
// change class, with the per-field / connection-delta detail and, for multi-pair diffs,
// badges naming the sheet pairs the item appears on. Clicking a row asks the presenter to
// focus (emphasize + locate) that item; clicking its badge names the pair to show, like the
// findings panel's sheet badges. Collapse state is the panel's own; the items, pairs, and
// focus arrive pushed (command-down, C3).
function ChangesPanel(props: { state: () => DiffState; onSelect: (id: string, pair?: number) => void }) {
  const [collapsed, setCollapsed] = createSignal<Set<string>>(new Set());
  const toggleCollapse = (v: string) =>
    setCollapsed((c) => {
      const next = new Set(c);
      next.has(v) ? next.delete(v) : next.add(v);
      return next;
    });

  const groups = () => {
    const by = new Map<string, ChangedItem[]>();
    for (const it of props.state().items) (by.get(it.cls) ?? by.set(it.cls, []).get(it.cls)!).push(it);
    return ITEM_CLASS_ORDER.filter((cls) => by.has(cls)).map((cls) => [cls, by.get(cls)!] as const);
  };
  const badges = (it: ChangedItem) =>
    props.state().pairs.length > 1 ? itemPairs(it, props.state().pairs).map((i) => ({ i, name: props.state().pairs[i].name })) : [];

  return (
    <Show when={props.state().active} fallback={<div class="diff-empty">No comparison open.</div>}>
      <Show when={props.state().items.length > 0} fallback={<div class="diff-empty">No changes.</div>}>
        <ul class="finding-groups">
          <For each={groups()}>
            {([cls, items]) => (
              <li class="finding-group">
                <button type="button" class="finding-group-head" onClick={() => toggleCollapse(cls)}>
                  <span class="finding-group-twist">{collapsed().has(cls) ? "▸" : "▾"}</span>
                  <span class="diff-swatch" style={{ background: DIFF_COLORS[cls] }} />
                  <span class="finding-group-name">{CLASS_LABELS[cls] ?? cls}</span>
                  <span class="finding-group-badge">{items.length}</span>
                </button>
                <Show when={!collapsed().has(cls)}>
                  <ul class="finding-list">
                    <For each={items}>
                      {(it) => (
                        <li>
                          <button
                            type="button"
                            class={`finding chg${props.state().selected === itemId(it) ? " selected" : ""}`}
                            title={it.detail}
                            onClick={() => props.onSelect(itemId(it))}
                          >
                            <span class="diff-swatch" style={{ background: DIFF_COLORS[cls] }} />
                            <span class="finding-subject">{it.key}</span>
                            <span class="finding-rule">
                              {it.kind}
                              <SheetBadges
                                items={badges(it)}
                                label={(b) => b.name}
                                title={(b) => `show sheet pair ${b.name}`}
                                onSelect={(b) => props.onSelect(itemId(it), b.i)}
                              />
                            </span>
                            <Show when={it.detail}>
                              <span class="finding-msg">{it.detail}</span>
                            </Show>
                          </button>
                        </li>
                      )}
                    </For>
                  </ul>
                </Show>
              </li>
            )}
          </For>
        </ul>
      </Show>
    </Show>
  );
}

// ChangesView is the command-down surface the presenter's onState feeds (the same DiffState
// the chrome bar renders — one push, two panels).
export interface ChangesView {
  setState(s: DiffState): void;
}

// diffChangesPanelIsland mounts the changes panel; onSelect is the intent up (focus this
// item, optionally on a named sheet pair).
export function diffChangesPanelIsland(
  el: HTMLElement,
  eventBus: EventBus | null,
  handlers: { onSelect: (id: string, pair?: number) => void },
): { island: SolidIsland; view: ChangesView } {
  const [state, setState] = signalView<DiffState>(emptyDiffState());
  const island = new SolidIsland(
    "diff-changes",
    el,
    () => <ChangesPanel state={state} onSelect={handlers.onSelect} />,
    eventBus,
  );
  return { island, view: { setState } };
}
