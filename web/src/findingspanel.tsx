import { For, Show, createEffect, createMemo, createSignal } from "solid-js";
import { SolidIsland, signalView } from "@panyam/tsappkit-solid";
import type { EventBus } from "@panyam/tsappkit";
import { SheetBadges } from "./sheetbadges.jsx";
import {
  type FindingItem,
  type FindingsState,
  type FindingsView,
  type FindingGroupAxis,
  type FindingSortKey,
  type CollapsedFinding,
  collapseInstances,
  findingKey,
  groupFindings,
  sortFindings,
  tallySeverities,
  verdictSubjectLabel, type VerdictItem } from "./findings.js";

// GroupAxis adds "none" to the finding group axes, meaning a flat sorted table with no group headers.
type GroupAxis = FindingGroupAxis | "none";

// GROUP_OPTIONS drives the "Group by" selector. "entity" and "interface" label the kind and profile
// axes.
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

// ChecksPanel is the checks table (WS9). The presenter pushes the flat finding list from CheckDesign
// plus the rule catalog summaries, and the panel owns ALL ordering (group-by axis, per-column sort,
// collapse of repeated findings), so regrouping needs no server round-trip. Checks run on demand from
// the Run button, whose badge counts selected rules not yet run. Everything shown is pushed (C3).
function ChecksPanel(props: {
  state: () => FindingsState;
  locateNote: () => string;
  onSelect: (subject: string, sheet?: string, netId?: string) => void;
  // onLocateContext locates a CONTEXT entity, one a finding's message names but is not about (agni
  // issue 349). onSelect cannot, because its finding-by-subject lookup would not find it.
  onLocateContext: (kind: string, subject: string, pin: string) => void;
  onRun: () => void;
  // onSelectVerdict focuses a verdict by its derived id (see viewer.ts locateVerdict). A passing
  // verdict has no finding, so onSelect's lookup by subject cannot serve it.
  onSelectVerdict: (id: string) => void;
}) {
  // mode is panel-local view state picking which table shows. Two tables rather than one with extra
  // rows, because a findings row is a violation and anything counting rows would count passes as
  // defects.
  const [mode, setMode] = createSignal<"violations" | "considered">("violations");
  // A focused verdict switches the panel to the table that CONTAINS it, so a link naming a verdict
  // shows the explaining row and not only the canvas highlight. A row click is already in this mode,
  // so there it is a no-op. See
  // docsite/content/architecture/web-client.md#a-panel-that-works-on-click-can-still-be-broken-on-arrival.
  createEffect(() => {
    if (props.state().focusedVerdict) setMode("considered");
  });
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

  // sections is the render model, one unlabeled section when flat or one per group value, each with
  // its collapsed, sorted rows and badge counts. It MUST stay a memo over findings() alone: <For>
  // keys by reference and collapseSorted/groupFindings mint fresh objects, so recomputing on every
  // state push rebuilds the table on each selection and resets the scroll (agni issue 367,
  // docsite/content/architecture/web-client.md#traps-that-ship-green).
  const findings = createMemo(() => props.state().findings);
  // A section's badge counts DEFECTS, with inconclusive results shown beside it rather than inside
  // it (agni issue 350).
  const section = (value: string | null, items: FindingItem[]) => {
    const t = tallySeverities(items);
    return { value, rows: collapseSorted(items), count: t.total, unresolved: t.inconclusive };
  };
  const sections = createMemo((): { value: string | null; rows: CollapsedFinding[]; count: number; unresolved: number }[] => {
    const fs = findings();
    if (axis() === "none") return [section(null, fs)];
    return groupFindings(fs, axis() as FindingGroupAxis).map(([value, items]) => section(value, items));
  });

  const runLabel = () => {
    const s = props.state();
    if (s.running) return "Running…";
    if (s.catalogLoading) return "Loading rules…";
    return s.pending > 0 ? `Run checks (${s.pending})` : "Run checks";
  };

  return (
    <div class="checks">
      <div class="checks-toolbar">
        <button type="button" class="checks-run" disabled={props.state().running || props.state().catalogLoading} onClick={() => props.onRun()}>
          {runLabel()}
        </button>
        <label class="checks-groupby">
          Group by{" "}
          <select value={axis()} onChange={(e) => setAxis(e.currentTarget.value as GroupAxis)}>
            <For each={GROUP_OPTIONS}>{(o) => <option value={o.key}>{o.label}</option>}</For>
          </select>
        </label>
        <Show when={props.state().verdicts.length > 0}>
          <label class="checks-mode">
            <select value={mode()} onChange={(e) => setMode(e.currentTarget.value as "violations" | "considered")}>
              <option value="violations">Violations</option>
              <option value="considered">What was checked</option>
            </select>
          </label>
        </Show>
      </div>

      <Show when={mode() === "considered"}>
        <VerdictTable
          verdicts={() => props.state().verdicts}
          focused={() => props.state().focusedVerdict}
          onSelectVerdict={props.onSelectVerdict}
        />
      </Show>

      <Show when={props.locateNote() !== ""}>
        <div class="checks-locate-note" role="status">{props.locateNote()}</div>
      </Show>

      {/*
        Rules that could not run. Shown ABOVE the findings because it qualifies them (two findings
        from ten selected rules, six of which never ran, is a weaker claim than two from ten), and
        shown even with no findings, since "no findings" is the reading a gated rule silently
        produces.
      */}
      <Show when={props.state().skipped.length > 0}>
        <div class="checks-skipped" role="status">
          <div class="checks-skipped-head">
            {props.state().skipped.length} selected rule{props.state().skipped.length === 1 ? "" : "s"} could not run on this design
          </div>
          <ul class="checks-skipped-list">
            <For each={props.state().skipped}>
              {(sk) => (
                <li>
                  <span class="checks-skipped-rule">{sk.rule}</span>
                  <span class="checks-skipped-reason">{sk.reason}</span>
                </li>
              )}
            </For>
          </ul>
        </div>
      </Show>

      <Show
        when={props.state().ruleCount > 0}
        fallback={<div class="findings-empty">{props.state().catalogLoading ? "Loading rules…" : "No rules selected."}</div>}
      >
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
                            <Show when={sec.count > 0 || sec.unresolved === 0}>
                              <span class="finding-group-badge">{sec.count}</span>
                            </Show>
                            <Show when={sec.unresolved > 0}>
                              <span class="finding-group-unresolved" title={`${sec.unresolved} inconclusive: the rule could not decide`}>
                                {sec.unresolved}?
                              </span>
                            </Show>
                          </button>
                        </td>
                      </tr>
                    </Show>
                    <Show when={sec.value === null || !collapsed().has(sec.value!)}>
                      <For each={sec.rows}>{(row) => <Row row={row} state={props.state} onSelect={props.onSelect} onLocateContext={props.onLocateContext} expanded={expanded} toggleRow={toggleRow} summaries={() => props.state().ruleSummaries} />}</For>
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

// Row renders one collapsed finding. A multi-instance finding (N findings sharing every display
// field, e.g. duplicate-net-name over N same-named nets) gets a ×N expander whose sub-rows each
// locate their own instance by net or bus id. Sheet badges switch sheets, as in the query panel.
function Row(props: {
  row: CollapsedFinding;
  state: () => FindingsState;
  onSelect: (subject: string, sheet?: string, netId?: string) => void;
  onLocateContext: (kind: string, subject: string, pin: string) => void;
  expanded: () => Set<string>;
  toggleRow: (k: string) => void;
  summaries: () => Record<string, string>;
}) {
  const f = () => props.row.head;
  const multi = () => props.row.instances.length > 1;
  const key = () => findingKey(f());
  const open = () => props.expanded().has(key());
  const selected = () => props.state().selected === f().subject;
  // A single finding locates by its net id, or its bus id for a bus finding (WS7-042b). A collapsed
  // head passes "" and locates by NAME, highlighting every same-named net.
  const headNetId = () => (multi() ? "" : f().netId || f().busId);

  return (
    <>
      <tr class={`check-row sev-${f().severity}${f().inconclusive ? " inconclusive" : ""}${selected() ? " selected" : ""}`}>
        <td class="check-exp">
          <Show when={multi()}>
            <button type="button" class={`check-exp-btn${open() ? " open" : ""}`} title="instances of this finding" onClick={() => props.toggleRow(key())}>
              <span class="check-exp-twist">{open() ? "▾" : "▸"}</span>
              <span class="check-count">×{props.row.instances.length}</span>
            </button>
          </Show>
        </td>
        {/*
          An inconclusive result gets its own mark rather than a severity dot. It carries the
          severity the rule WOULD have reported, and a red dot would read as a defect beside a query
          panel calling the same finding "unresolved" (agni issue 350).
        */}
        <td class="check-sev">
          <Show
            when={f().inconclusive}
            fallback={<span class={`sev-dot sev-${f().severity}`} title={f().severity} />}
          >
            <span class="sev-unresolved" title={`inconclusive: the rule could not decide (would have been ${f().severity})`}>?</span>
          </Show>
        </td>
        <td class="check-subject">
          <button type="button" class="check-locate" title={`locate ${f().kind} ${f().subject}`} onClick={() => props.onSelect(f().subject, undefined, headNetId())}>
            {f().subject}
            {f().pin ? `.${f().pin}` : ""}
          </button>
          <Show when={!multi()}>
            <SheetBadges
              items={f().sheets}
              label={(b) => b.name}
              title={(b) => `show sheet ${b.name}`}
              onSelect={(b) => props.onSelect(f().subject, b.id, headNetId())}
            />
          </Show>
        </td>
        <td class="check-rule" title={props.summaries()[f().rule] ?? ""}>
          {f().rule}
        </td>
        <td class="check-msg" title={f().message}>
          {f().message}
          {/* The entities the message names but the finding is not ABOUT (agni issue 349). They go
              after the message so they do not read as a second subject. */}
          <For each={f().context}>
            {(c) => (
              <button
                type="button"
                class="check-context"
                title={`locate ${c.kind} ${c.subject} (${c.role})`}
                onClick={() => props.onLocateContext(c.kind, c.subject, c.pin)}
              >
                <span class="check-context-role">{c.role}</span>
                {c.subject}
                {c.pin ? `.${c.pin}` : ""}
              </button>
            )}
          </For>
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
                <SheetBadges
                  items={inst.sheets}
                  label={(b) => b.name}
                  title={(b) => `show sheet ${b.name}`}
                  onSelect={(b) => props.onSelect(inst.subject, b.id, inst.netId || inst.busId)}
                />
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

// VerdictTable is the considered set, one row per subject a rule was applied to, passes included.
// Outcome leads so the column can be scanned. Proof is the statement for a decided verdict and the
// reason for one the rule could not decide.
function VerdictTable(props: {
  verdicts: () => VerdictItem[];
  focused: () => string;
  onSelectVerdict: (id: string) => void;
}) {
  return (
    <table class="checks-table verdicts-table">
      <thead>
        <tr>
          <th>Outcome</th>
          <th>Rule</th>
          <th>Subject</th>
          <th>Proof</th>
        </tr>
      </thead>
      <tbody>
        <For each={props.verdicts()}>
          {(v) => (
            <tr
              class={`verdict-row outcome-${v.outcome}${props.focused() === v.id ? " selected" : ""}`}
              onClick={() => props.onSelectVerdict(v.id)}
            >
              <td class="verdict-outcome">{v.outcome}</td>
              <td class="verdict-rule">{v.rule}</td>
              <td class="verdict-subject">{verdictSubjectLabel(v)}</td>
              <td class="verdict-proof">{v.statement || v.reason}</td>
            </tr>
          )}
        </For>
      </tbody>
    </table>
  );
}

// findingsPanelIsland mounts the checks panel on the "findings" island and returns its command-down
// view (docsite/content/architecture/web-client.md#wiring-a-new-panel).
export function findingsPanelIsland(
  el: HTMLElement,
  eventBus: EventBus | null,
  handlers: {
    onSelect: (subject: string, sheet?: string, netId?: string) => void;
    onLocateContext: (kind: string, subject: string, pin: string) => void;
    onRun: () => void;
    onSelectVerdict: (id: string) => void;
  },
): { island: SolidIsland; view: FindingsView } {
  const [state, setState] = signalView<FindingsState>({
    findings: [],
    verdicts: [],
    focusedVerdict: "",
    selected: "",
    ruleCount: 0,
    pending: 0,
    running: false,
    catalogLoading: false,
    skipped: [],
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
    () => (
      <ChecksPanel
        state={state}
        locateNote={locateNote}
        onSelect={handlers.onSelect}
        onLocateContext={handlers.onLocateContext}
        onRun={handlers.onRun}
        onSelectVerdict={handlers.onSelectVerdict}
      />
    ),
    eventBus,
  );
  return { island, view: { setState: setStateClearing, setFindingLocateNote: setLocateNote } };
}
