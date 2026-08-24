import { For, Show, createSignal, createEffect, createMemo } from "solid-js";
import { SolidIsland, signalView } from "@panyam/tsappkit-solid";
import type { EventBus } from "@panyam/tsappkit";
import {
  type RulesState,
  type RulesView,
  tagKeys,
  groupBy,
  filterRules,
  groupSelectState,
  toggleRule,
  toggleGroup,
} from "./rules.js";
import { type Bundle, allBundles, resolveBundle, upsertBundle, removeBundle, loadSaved, persistSaved } from "./bundles.js";
import { renderMarkdown } from "./markdown.js";

// RulesPanel is the catalog of what the engine can assert, with multi-select driving which checks
// run. The presenter pushes the catalog + active selection (RulesState); the panel holds its own
// ephemeral view knobs (group-by key, search, available-only) as local signals and emits
// onSelectionChange with the new set of selected names. Unavailable rules render greyed and cannot
// be selected (capability gating). Group-by is data-driven from the tag keys in the response.
function RulesPanel(props: { state: () => RulesState; onSelectionChange: (names: string[]) => void }) {
  const [groupKey, setGroupKey] = createSignal("category");
  const [search, setSearch] = createSignal("");
  const [availableOnly, setAvailableOnly] = createSignal(false);
  // expanded holds the rule names whose long-form detail is open (WS9-020) — ephemeral view
  // state like the group-by knob, so it lives here, not in the presenter.
  const [expanded, setExpanded] = createSignal<Set<string>>(new Set());
  const toggleDetail = (name: string) => {
    const next = new Set(expanded());
    if (next.has(name)) next.delete(name);
    else next.add(name);
    setExpanded(next);
  };

  // Bundle state is panel-local: the saved bundles (from localStorage), the currently applied
  // bundle name (a label, not a lock — the user may tweak checkboxes after), and the save input.
  const [saved, setSaved] = createSignal<Bundle[]>(loadSaved());
  const [bundle, setBundle] = createSignal("");
  const [saving, setSaving] = createSignal(false);
  const [newName, setNewName] = createSignal("");

  // applyBundle resolves the chosen bundle to rule names and fires the selection intent — the same
  // path a checkbox click takes, so the presenter re-runs the checks over it.
  const applyBundle = (name: string) => {
    setBundle(name);
    if (!name) return;
    const b = allBundles(saved()).find((x) => x.name === name);
    if (b) props.onSelectionChange(resolveBundle(props.state().rules, b));
  };
  const currentIsCustom = () => saved().some((b) => b.name === bundle());
  const deleteCurrent = () => {
    const next = removeBundle(saved(), bundle());
    setSaved(next);
    persistSaved(next);
    setBundle("");
  };
  const confirmSave = () => {
    const name = newName().trim();
    if (!name) {
      setSaving(false);
      return;
    }
    const next = upsertBundle(saved(), { name, rules: [...props.state().selected] });
    setSaved(next);
    persistSaved(next);
    setSaving(false);
    setBundle(name);
  };

  const rules = () => props.state().rules;
  const selected = createMemo(() => new Set(props.state().selected));
  const keys = createMemo(() => tagKeys(rules()));
  // effectiveKey falls back to the first available key if the chosen one is absent (e.g. a design
  // whose rules do not carry that tag), so the tree never renders one empty "(untagged)" bucket.
  const effectiveKey = () => (keys().includes(groupKey()) ? groupKey() : (keys()[0] ?? groupKey()));
  const visible = () => filterRules(rules(), { tagValues: {}, availableOnly: availableOnly(), search: search() });
  const groups = () => groupBy(visible(), effectiveKey());

  const selectRule = (r: Parameters<typeof toggleRule>[0]) => props.onSelectionChange(toggleRule(r, props.state().selected));
  const selectGroup = (g: Parameters<typeof toggleGroup>[0]) => props.onSelectionChange(toggleGroup(g, props.state().selected));

  return (
    <div class="rules">
      <div class="rules-bundles">
        <label class="rules-bundle-pick">
          Bundle{" "}
          <select value={bundle()} onChange={(e) => applyBundle(e.currentTarget.value)}>
            <option value="">— select —</option>
            <For each={allBundles(saved())}>{(b) => <option value={b.name}>{b.name}</option>}</For>
          </select>
        </label>
        <Show when={currentIsCustom()}>
          <button type="button" class="rules-bundle-del" title="delete this saved bundle" onClick={deleteCurrent}>
            ✕
          </button>
        </Show>
        <Show
          when={saving()}
          fallback={
            <button
              type="button"
              class="rules-bundle-savebtn"
              title="save the current selection as a bundle"
              onClick={() => {
                setNewName("");
                setSaving(true);
              }}
            >
              Save…
            </button>
          }
        >
          <input
            class="rules-bundle-name"
            type="text"
            placeholder="bundle name"
            autofocus
            value={newName()}
            onInput={(e) => setNewName(e.currentTarget.value)}
            onKeyDown={(e) => {
              if (e.key === "Enter") confirmSave();
              if (e.key === "Escape") setSaving(false);
            }}
          />
          <button type="button" onClick={confirmSave}>
            Save
          </button>
        </Show>
      </div>
      <div class="rules-toolbar">
        <label class="rules-groupby">
          Group by{" "}
          <select value={groupKey()} onChange={(e) => setGroupKey(e.currentTarget.value)}>
            <For each={keys()}>{(k) => <option value={k}>{k}</option>}</For>
          </select>
        </label>
        <input
          class="rules-search"
          type="search"
          placeholder="filter rules"
          value={search()}
          onInput={(e) => setSearch(e.currentTarget.value)}
        />
        <label class="rules-availonly">
          <input type="checkbox" checked={availableOnly()} onChange={(e) => setAvailableOnly(e.currentTarget.checked)} />
          available only
        </label>
      </div>
      <Show when={rules().length > 0} fallback={<div class="rules-empty">No rules.</div>}>
        <ul class="rule-groups">
          <For each={groups()}>
            {([value, group]) => {
              const selectedCount = () => group.filter((r) => selected().has(r.name)).length;
              const availableCount = () => group.filter((r) => r.available).length;
              const firedCount = () => group.reduce((n, r) => n + (props.state().fired[r.name] ?? 0), 0);
              return (
                <li class="rule-group">
                  <div class="rule-group-head">
                    <input
                      type="checkbox"
                      ref={(el) =>
                        createEffect(() => {
                          const st = groupSelectState(group, selected());
                          el.checked = st === "all";
                          el.indeterminate = st === "some";
                        })
                      }
                      onChange={() => selectGroup(group)}
                    />
                    <span class="rule-group-name">{value || "(untagged)"}</span>
                    <span class="rule-group-badge" title="fired / selected / available">
                      {firedCount()}/{selectedCount()}/{availableCount()}
                    </span>
                  </div>
                  <ul class="rule-list">
                    <For each={group}>
                      {(r) => (
                        <li class="rule-item">
                          <div class="rule-row">
                            <label
                              class={`rule sev-${r.severity}${r.available ? "" : " unavailable"}`}
                              title={r.available ? "" : r.unavailableReason}
                            >
                              <input
                                type="checkbox"
                                disabled={!r.available}
                                checked={selected().has(r.name)}
                                onChange={() => selectRule(r)}
                              />
                              <span class={`sev-dot sev-${r.severity}`} />
                              <span class="rule-name">{r.name}</span>
                            </label>
                            <Show when={r.detail !== ""}>
                              <button
                                type="button"
                                class={`rule-detail-toggle${expanded().has(r.name) ? " open" : ""}`}
                                title="what this rule checks and why"
                                onClick={() => toggleDetail(r.name)}
                              >
                                {expanded().has(r.name) ? "▾" : "▸"}
                              </button>
                            </Show>
                          </div>
                          <div class="rule-summary">{r.summary}</div>
                          <Show when={expanded().has(r.name)}>
                            <div class="rule-detail">
                              <Show when={r.impact !== ""}>
                                <p class="rule-impact">{r.impact}</p>
                              </Show>
                              <Show when={r.remedy !== ""}>
                                <p class="rule-remedy">{r.remedy}</p>
                              </Show>
                              <div class="rule-detail-md" innerHTML={renderMarkdown(r.detail)} />
                            </div>
                          </Show>
                        </li>
                      )}
                    </For>
                  </ul>
                </li>
              );
            }}
          </For>
        </ul>
      </Show>
    </div>
  );
}

// rulesPanelIsland mounts the panel and returns its command-down view. onSelectionChange is the
// intent up (the user changed which rules are selected), same island shape as findingsPanelIsland.
export function rulesPanelIsland(
  el: HTMLElement,
  eventBus: EventBus | null,
  handlers: { onSelectionChange: (names: string[]) => void },
): { island: SolidIsland; view: RulesView } {
  const [state, setState] = signalView<RulesState>({ rules: [], selected: [], fired: {} });
  const island = new SolidIsland(
    "rules",
    el,
    () => <RulesPanel state={state} onSelectionChange={handlers.onSelectionChange} />,
    eventBus,
  );
  return { island, view: { setState } };
}
