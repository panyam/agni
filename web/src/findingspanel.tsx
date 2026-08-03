import { For, Show, createSignal } from "solid-js";
import { SolidIsland, signalView } from "@panyam/tsappkit-solid";
import type { EventBus } from "@panyam/tsappkit";
import {
  type FindingsState,
  type FindingsView,
  type FindingGroupAxis,
  type FindingSortKey,
  type CollapsedFinding,
  collapseInstances,
  findingKey,
  groupFindings,
  sortFindings,
} from "./findings.js";

// GroupAxis extends the finding group axes with "none" — a flat sorted table with no group headers.
type GroupAxis = FindingGroupAxis | "none";

// GROUP_OPTIONS drives the "Group by" selector. "none" is a flat table; "entity"/"interface" label
// the kind/profile axes the way the retired report/findings panels did.
const GROUP_OPTIONS: { key: GroupAxis; label: string }[] = [
  { key: "none", label: "none" },
  { key: "rule", label: "rule" },
  { key: "severity", label: "severity" },
  { key: "category", label: "category" },
  { key: "kind", label: "entity" },
  { key: "profile", label: "interface" },
];

// SORT_COLS are the sortable table columns. sev sorts by severity rank (worst-first ascending).
const SORT_COLS: { key: FindingSortKey; label: string }[] = [
  { key: "severity", label: "sev" },
  { key: "subject", label: "subject" },
  { key: "rule", label: "rule" },
];

// ChecksPanel is the merged checks surface (WS9): it replaces the separate findings and report
// panels with one server-sourced, client-ordered table. The presenter pushes the flat finding list
// (from CheckDesign) plus the rule catalog summaries; the panel owns ALL ordering — a "Group by"
// axis, per-column sort, and the collapse of repeated findings — so the same data groups any way
// without a server round-trip. Checks are on-demand: the Run button triggers the run, and pending
// (selected-but-not-run rules) badges it. Everything shown is pushed (command-down, C3).
function ChecksPanel(props: {
  state: () => FindingsState;
  locateNote: () => string;
  onSelect: (subject: string, sheet?: string, netId?: string) => void;
  onRun: () => void;
}) {
  const [axis, setAxis] = createSignal<GroupAxis>("none");
  const [sortKey, setSortKey] = createSignal<FindingSortKey>("severity");
  const [sortDir, setSortDir] = createSignal<1 | -1>(1);
  const [collapsed, setCollapsed] = createSignal<Set<string>>(new Set());
  const [expanded, setExpanded] = createSignal<Set<string>>(new Set());

  const toggleGroup = (v: string) =>
    setCollapsed((c) => {
      const next = new Set(c);
      next.has(v) ? next.delete(v) : next.add(v);
      return next;
    });
  const toggleRow = (k: string) =>
    setExpanded((c) => {
      const next = new Set(c);
      next.has(k) ? next.delete(k) : next.add(k);
      return next;
    });
  // Clicking a sort column toggles direction if already active, else selects it ascending.
  const onSort = (key: FindingSortKey) => {
    if (sortKey() === key) setSortDir((d) => (d === 1 ? -1 : 1));
    else {
      setSortKey(key);
      setSortDir(1);
    }
  };
  const collapseSorted = (items: ReturnType<() => FindingsState>["findings"]) =>
    collapseInstances(sortFindings(items, sortKey(), sortDir()));

  // sections is the render model: either one unlabeled section (flat) or one per group value, each
  // carrying its collapsed+sorted rows and the total finding count (before collapse) for the badge.
  const sections = (): { value: string | null; rows: CollapsedFinding[]; count: number }[] => {
    const findings = props.state().findings;
    if (axis() === "none") return [{ value: null, rows: collapseSorted(findings), count: findings.length }];
    return groupFindings(findings, axis() as FindingGroupAxis).map(([value, items]) => ({
      value,
      rows: collapseSorted(items),
      count: items.length,
    }));
  };

  const runLabel = () => {
    const s = props.state();
    if (s.running) return "Running…";
    return s.pending > 0 ? `Run checks (${s.pending})` : "Run checks";
  };

  return (
    <div class="checks">
      <div class="checks-toolbar">
        <button type="button" class="checks-run" disabled={props.state().running} onClick={() => props.onRun()}>
          {runLabel()}
        </button>
        <label class="checks-groupby">
          Group by{" "}
          <select value={axis()} onChange={(e) => setAxis(e.currentTarget.value as GroupAxis)}>
            <For each={GROUP_OPTIONS}>{(o) => <option value={o.key}>{o.label}</option>}</For>
          </select>
        </label>
      </div>

      <Show when={props.locateNote() !== ""}>
        <div class="checks-locate-note" role="status">{props.locateNote()}</div>
      </Show>

      <Show when={props.state().ruleCount > 0} fallback={<div class="findings-empty">No rules selected.</div>}>
        <Show
          when={props.state().findings.length > 0}
          fallback={
            <div class="findings-empty">
              {props.state().pending > 0
                ? `Press Run checks to evaluate ${props.state().pending} rule${props.state().pending === 1 ? "" : "s"}.`
                : "No findings."}
            </div>
          }
        >
          <table class="checks-table">
            <thead>
              <tr>
                <th class="check-exp" />
                <For each={SORT_COLS}>
                  {(c) => (
                    <th class={`check-sort${sortKey() === c.key ? " active" : ""}`}>
                      <button type="button" onClick={() => onSort(c.key)}>
                        {c.label}
                        <Show when={sortKey() === c.key}>
                          <span class="check-sort-dir">{sortDir() === 1 ? "▲" : "▼"}</span>
                        </Show>
                      </button>
                    </th>
                  )}
                </For>
                <th>message</th>
              </tr>
            </thead>
            <tbody>
              <For each={sections()}>
                {(sec) => (
                  <>
                    <Show when={sec.value !== null}>
                      <tr class="check-group">
                        <td colspan={5}>
                          <button type="button" class="check-group-head" onClick={() => toggleGroup(sec.value!)}>
                            <span class="check-group-twist">{collapsed().has(sec.value!) ? "▸" : "▾"}</span>
                            <span class="check-group-name">{sec.value || "(none)"}</span>
                            <span class="finding-group-badge">{sec.count}</span>
                          </button>
                        </td>
                      </tr>
                    </Show>
                    <Show when={sec.value === null || !collapsed().has(sec.value!)}>
                      <For each={sec.rows}>{(row) => <Row row={row} state={props.state} onSelect={props.onSelect} expanded={expanded} toggleRow={toggleRow} summaries={() => props.state().ruleSummaries} />}</For>
                    </Show>
                  </>
                )}
              </For>
            </tbody>
          </table>
        </Show>
      </Show>
    </div>
  );
}

// Row renders one collapsed finding. A single-instance finding is a plain row; a multi-instance one
// (N findings that share every display field, e.g. duplicate-net-name over N same-named nets) gets a
// ×N expander whose sub-rows are the instances — each a click-to-locate target (distinct once they
// carry a per-instance identity; identical until WS9 Phase 2). The subject cell locates the entity;
// sheet badges navigate to a sheet, the findings idiom the query panel shares.
function Row(props: {
  row: CollapsedFinding;
  state: () => FindingsState;
  onSelect: (subject: string, sheet?: string, netId?: string) => void;
  expanded: () => Set<string>;
  toggleRow: (k: string) => void;
  summaries: () => Record<string, string>;
}) {
  const f = () => props.row.head;
  const multi = () => props.row.instances.length > 1;
  const key = () => findingKey(f());
  const open = () => props.expanded().has(key());
  const selected = () => props.state().selected === f().subject;
  // A single finding locates by its own net id (precise); a collapsed head locates by NAME (no id),
  // so clicking it highlights the whole family of same-named nets, while the instance sub-rows below
  // each locate their own net.
  // The per-instance locate id: a net id, or a bus id for a bus finding (WS7-042b). "" for a
  // collapsed multi-instance head (its instances each locate individually).
  const headNetId = () => (multi() ? "" : f().netId || f().busId);

  return (
    <>
      <tr class={`check-row sev-${f().severity}${selected() ? " selected" : ""}`}>
        <td class="check-exp">
          <Show when={multi()}>
            <button type="button" class={`check-exp-btn${open() ? " open" : ""}`} title="instances of this finding" onClick={() => props.toggleRow(key())}>
              <span class="check-exp-twist">{open() ? "▾" : "▸"}</span>
              <span class="check-count">×{props.row.instances.length}</span>
            </button>
          </Show>
        </td>
        <td class="check-sev">
          <span class={`sev-dot sev-${f().severity}`} title={f().severity} />
        </td>
        <td class="check-subject">
          <button type="button" class="check-locate" title={`locate ${f().kind} ${f().subject}`} onClick={() => props.onSelect(f().subject, undefined, headNetId())}>
            {f().subject}
            {f().pin ? `.${f().pin}` : ""}
          </button>
          <Show when={!multi()}>
            <For each={f().sheets}>
              {(b) => (
                <span
                  class="sheet-badge"
                  title={`show sheet ${b.name}`}
                  onClick={(e) => {
                    e.stopPropagation();
                    props.onSelect(f().subject, b.id, headNetId());
                  }}
                >
                  {b.name}
                </span>
              )}
            </For>
          </Show>
        </td>
        <td class="check-rule" title={props.summaries()[f().rule] ?? ""}>
          {f().rule}
        </td>
        <td class="check-msg" title={f().message}>
          {f().message}
        </td>
      </tr>
      <Show when={multi() && open()}>
        <For each={props.row.instances}>
          {(inst, i) => (
            <tr class="check-inst">
              <td class="check-exp" />
              <td class="check-sev" />
              <td class="check-subject" colspan={3}>
                <span class="check-inst-label">#{i() + 1}</span>
                <button type="button" class="check-locate" title={`locate ${inst.kind} ${inst.subject} (net ${inst.netId})`} onClick={() => props.onSelect(inst.subject, undefined, inst.netId || inst.busId)}>
                  {inst.subject}
                  {inst.pin ? `.${inst.pin}` : ""}
                </button>
                <For each={inst.sheets}>
                  {(b) => (
                    <span
                      class="sheet-badge"
                      title={`show sheet ${b.name}`}
                      onClick={(e) => {
                        e.stopPropagation();
                        props.onSelect(inst.subject, b.id, inst.netId || inst.busId);
                      }}
                    >
                      {b.name}
                    </span>
                  )}
                </For>
                <Show when={inst.sheets.length === 0 && inst.netId !== ""}>
                  <span class="check-inst-id" title={`net id ${inst.netId}`}>{inst.netId.slice(0, 6)}</span>
                </Show>
              </td>
            </tr>
          )}
        </For>
      </Show>
    </>
  );
}

// findingsPanelIsland mounts the merged checks panel and returns its command-down view. onSelect is
// the locate intent (a finding/instance clicked); onRun is the run intent (the Run button). The
// island id ("findings") and hole are unchanged, so the dock adopts the same server-rendered hole.
export function findingsPanelIsland(
  el: HTMLElement,
  eventBus: EventBus | null,
  handlers: { onSelect: (subject: string, sheet?: string, netId?: string) => void; onRun: () => void },
): { island: SolidIsland; view: FindingsView } {
  const [state, setState] = signalView<FindingsState>({
    findings: [],
    selected: "",
    ruleCount: 0,
    pending: 0,
    running: false,
    ruleSummaries: {},
  });
  const [locateNote, setLocateNote] = signalView<string>("");
  // A fresh check run clears any stale locate note from the previous selection.
  const setStateClearing = (s: FindingsState) => {
    setLocateNote("");
    setState(s);
  };
  const island = new SolidIsland(
    "findings",
    el,
    () => <ChecksPanel state={state} locateNote={locateNote} onSelect={handlers.onSelect} onRun={handlers.onRun} />,
    eventBus,
  );
  return { island, view: { setState: setStateClearing, setFindingLocateNote: setLocateNote } };
}
