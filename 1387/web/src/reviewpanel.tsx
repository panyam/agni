import { For, Show, createSignal } from "solid-js";
import type { FindingItem } from "./findings.js";
import { SolidIsland, signalView } from "@panyam/tsappkit-solid";
import type { EventBus } from "@panyam/tsappkit";
import type { WitnessValue } from "./query.js";
import { WitnessTree } from "./witnesstree.jsx";
import {
  type FindingExplain,
  type ReviewState,
  type ReviewView,
  type ReviewRunView,
  type ReviewItemView,
  type Tally,
  areaTally,
  covered,
  emptyReview,
  findingKey,
  outcomeClass,
  runTally,
  selectedRun,
  OUTCOME_LABEL,
} from "./review.js";

// tallyLine is the headline a reviewer reads first, coverage and then the split.
//
// Coverage leads because it is the question the outcome vocabulary exists to answer. "8 of 13
// covered" says a mechanism exists for eight items; the pass/fail split only means anything once you
// know how much of the checklist it is a split OF. A panel that opened with "3 pass, 2 fail" would
// read as a nearly-clean board even when most of the checklist was never mechanised.
function tallyLine(t: Tally): string {
  const parts = [`${t.pass} pass`, `${t.fail} fail`];
  if (t.provisional) parts.push(`${t.provisional} provisional`);
  if (t.needsDesignIntent) parts.push(`${t.needsDesignIntent} needs intent`);
  if (t.needsData) parts.push(`${t.needsData} needs data`);
  if (t.inconclusive) parts.push(`${t.inconclusive} inconclusive`);
  if (t.notApplicable || t.computedNA) parts.push(`${t.notApplicable + t.computedNA} n/a`);
  if (t.notAutomated) parts.push(`${t.notAutomated} not automated`);
  return `${covered(t)} of ${t.total} covered — ${parts.join(", ")}`;
}

// runLabel names a stored run in the picker by when it ran and what it scored.
function runLabel(run: ReviewRunView): string {
  const when = run.createdAt ? run.createdAt.replace("T", " ").replace("Z", " UTC") : "unknown time";
  const t = runTally(run);
  return `${when} — ${t.fail} fail, ${covered(t)}/${t.total} covered`;
}

// OutcomeChip renders one item's verdict. The class comes from outcomeClass, which gives `pass` its
// own style and everything else a distinct one, so an item that could not be evaluated never reads as
// a clean pass. The title carries the full wire value, since the chip text is abbreviated.
function OutcomeChip(props: { outcome: string }) {
  return (
    <span class={`rv-outcome ${outcomeClass(props.outcome)}`} title={props.outcome}>
      {OUTCOME_LABEL[props.outcome] ?? props.outcome}
    </span>
  );
}

// ReviewItemRow is one checklist item with its id, title, verdict, any note, and (for an item that
// fired) its findings as click-to-locate rows. The findings collapse by default because a broad rule
// can fire on hundreds of nets and an expanded list would bury the checklist itself.
//
// A finding locates by (kind, subject) on a sheet the server says it is drawn on, keeping a pin
// subject's pin (agni issue 1132), NOT by the per-instance net id the checks panel can use. The
// checks panel can be precise because it holds the findings it is locating; these arrive inside a
// stored document instead. Two nets sharing a name therefore highlight together here (agni issue 144).
//
// A finding of an inline-query item also opens onto how it holds (agni issue 1166), asked only when
// a reader opens it, as a query row is.
function ReviewItemRow(props: {
  run: string;
  item: ReviewItemView;
  explained: () => Record<string, FindingExplain> | undefined;
  onLocate: (f: FindingItem) => void;
  onExplain?: (item: string, index: number) => void;
  onLocateValue?: (v: WitnessValue, sheet?: string) => void;
}) {
  const [open, setOpen] = createSignal(false);
  const [shown, setShown] = createSignal<ReadonlySet<number>>(new Set());
  const findings = () => props.item.findings;
  const explains = () => props.item.query !== undefined && props.onExplain !== undefined;
  const toggleHow = (i: number) => {
    const next = new Set(shown());
    if (next.has(i)) next.delete(i);
    else {
      next.add(i);
      props.onExplain?.(props.item.id, i);
    }
    setShown(next);
  };
  const explained = (i: number) => props.explained()?.[findingKey(props.run, props.item.id, i)];
  return (
    <li class="rv-item">
      <div class="rv-item-head">
        <span class="rv-item-id">{props.item.id}</span>
        <span class="rv-item-title">{props.item.title}</span>
        <OutcomeChip outcome={props.item.outcome} />
      </div>
      <Show when={props.item.note}>
        <div class="rv-item-note">{props.item.note}</div>
      </Show>
      <Show when={findings().length > 0}>
        <button type="button" class="rv-findings-toggle" onClick={() => setOpen(!open())}>
          {open() ? "▾" : "▸"} {findings().length} finding{findings().length === 1 ? "" : "s"}
        </button>
        <Show when={open()}>
          <ul class="rv-findings">
            <For each={findings()}>
              {(f, i) => (
                <li>
                  <div class="rv-finding-row">
                    <Show when={explains()}>
                      <button
                        type="button"
                        class="rv-finding-how"
                        aria-expanded={shown().has(i())}
                        title="show how this finding holds"
                        onClick={() => toggleHow(i())}
                      >
                        {shown().has(i()) ? "▾" : "▸"}
                      </button>
                    </Show>
                    <button
                      type="button"
                      class="rv-finding"
                      title={`locate ${f.kind} ${f.subject}`}
                      onClick={() => props.onLocate(f)}
                    >
                      <span class="rv-finding-subject">{f.subject}</span>
                      <span class="rv-finding-message">{f.message}</span>
                    </button>
                  </div>
                  <Show when={shown().has(i())}>
                    <div class="rv-finding-derivation">
                      <Show
                        when={explained(i())}
                        fallback={<div class="rv-finding-note" role="status">Working out how this finding holds…</div>}
                      >
                        {(ex) => (
                          <>
                            <Show when={ex().loading}>
                              <div class="rv-finding-note" role="status">Working out how this finding holds…</div>
                            </Show>
                            <Show when={ex().note}>
                              <div class="rv-finding-note">{ex().note}</div>
                            </Show>
                            <For each={ex().rows}>
                              {(row) => (
                                <>
                                  <Show when={ex().rows.length > 1}>
                                    <div class="rv-finding-row-cells">{row.cells.join(", ")}</div>
                                  </Show>
                                  <WitnessTree
                                    items={row.witnesses}
                                    pick={(v, sheet) => props.onLocateValue?.(v, sheet)}
                                    isCurrent={() => false}
                                    isCurrentSheet={() => false}
                                  />
                                </>
                              )}
                            </For>
                          </>
                        )}
                      </Show>
                    </div>
                  </Show>
                </li>
              )}
            </For>
          </ul>
        </Show>
      </Show>
    </li>
  );
}

// ReviewPanel shows a project's checklist verdict for the open design, with a run picker over the
// stored history, a create control, then the selected run's areas and per-item outcomes.
function ReviewPanel(props: {
  state: () => ReviewState;
  onSelectRun: (name: string) => void;
  onSelectChecklist: (ref: string) => void;
  onCreate: () => void;
  // onCancel stops the checklist run in flight (agni issue 918). Absent, the button stays disabled
  // while a run is under way.
  onCancel?: () => void;
  onLocate: (f: FindingItem) => void;
  // onExplain asks how one finding of an inline-query item holds (agni issue 1166), and
  // onLocateValue locates a value of the tree it answers. Absent, no finding offers to open.
  onExplain?: (item: string, index: number) => void;
  onLocateValue?: (v: WitnessValue, sheet?: string) => void;
  // onSave writes the shown run as the checklist page `agni review --format html` writes. Absent,
  // the panel offers no save.
  onSave?: () => void;
  staticHost: boolean;
}) {
  const run = () => selectedRun(props.state());
  return (
    <div class="review">
      {/* An engine that keeps no review store still runs a checklist (agni issue 931): the run is held
          by this page and lasts until the reader leaves it, which the note says. */}
      <Show when={!props.state().storeConfigured}>
        <Show
          when={!props.staticHost}
          fallback={<div class="rv-note">This demo keeps no review runs, so a run here lasts until you leave the page.</div>}
        >
          <div class="rv-note">
            This server keeps no review runs, so a run here lasts until you leave the page. Start it with{" "}
            <code>--review-store &lt;dir&gt;</code> to keep them.
          </div>
        </Show>
      </Show>
        <div class="rv-controls">
          <select
            class="rv-checklist"
            value={props.state().checklist}
            onChange={(e) => props.onSelectChecklist(e.currentTarget.value)}
            disabled={props.state().checklists.length === 0}
          >
            <Show
              when={props.state().checklists.length > 0}
              fallback={<option value="">no checklist beside this design</option>}
            >
              <For each={props.state().checklists}>
                {(c) => <option value={c.ref}>{c.label}</option>}
              </For>
            </Show>
          </select>
          <button
            type="button"
            class="rv-run"
            disabled={(props.state().running && !props.onCancel) || !props.state().checklist}
            onClick={() => (props.state().running ? props.onCancel?.() : props.onCreate())}
          >
            {props.state().running ? (props.onCancel ? "Cancel" : "Running…") : "Run review"}
          </button>
        </div>

        <Show when={props.state().error}>
          <div class="rv-error" role="alert">{props.state().error}</div>
        </Show>
        <Show when={props.state().running}>
          <div class="rv-running" role="status">Scoring the checklist…</div>
        </Show>
        <Show when={props.state().cancelled && !props.state().running}>
          <div class="rv-cancelled" role="status">Cancelled. The checklist was not scored.</div>
        </Show>
        <Show when={props.state().locateNote}>
          <div class="rv-locate-note" role="status">{props.state().locateNote}</div>
        </Show>

        <Show when={props.state().runs.length > 0}>
          <select
            class="rv-history"
            value={props.state().selected}
            onChange={(e) => props.onSelectRun(e.currentTarget.value)}
          >
            <For each={props.state().runs}>{(r) => <option value={r.name}>{runLabel(r)}</option>}</For>
          </select>
        </Show>

        <Show
          when={run()}
          fallback={
            <Show when={!props.state().error}>
              <div class="rv-empty">
                No review runs for this design yet. Pick a checklist and run one.
              </div>
            </Show>
          }
        >
          {(r) => (
            <div class="rv-run-body">
              <div class="rv-summary-row">
                <div class="rv-summary">{tallyLine(runTally(r()))}</div>
                <Show when={props.onSave}>
                  <button
                    type="button"
                    class="rv-save"
                    title="Save this run as the checklist page agni review --format html writes"
                    onClick={() => props.onSave?.()}
                  >
                    Save report
                  </button>
                </Show>
              </div>
              <Show when={props.state().saveNote}>
                <div class="rv-save-note" role="alert">{props.state().saveNote}</div>
              </Show>
              <div class="rv-provenance">
                {r().manifest}
                <Show when={r().producerVersion}>{` · ${r().producerVersion}`}</Show>
              </div>
              <For each={r().areas}>
                {(area) => (
                  <div class="rv-area">
                    <div class="rv-area-head">
                      <span class="rv-area-name">{area.name}</span>
                      <span class="rv-area-tally">{tallyLine(areaTally(area))}</span>
                    </div>
                    <ul class="rv-items">
                      <For each={area.items}>
                        {(item) => (
                          <ReviewItemRow
                            run={r().name}
                            item={item}
                            explained={() => props.state().explained}
                            onLocate={props.onLocate}
                            onExplain={props.onExplain}
                            onLocateValue={props.onLocateValue}
                          />
                        )}
                      </For>
                    </ul>
                  </div>
                )}
              </For>
            </div>
          )}
        </Show>
    </div>
  );
}

// reviewPanelIsland mounts the panel and returns its command-down view (setState). The handlers are
// the intents up (pick a stored run, pick a checklist, create a run, locate a finding's entity). The
// island shape is in docsite/content/architecture/web-client.md#wiring-a-new-panel.
export function reviewPanelIsland(
  el: HTMLElement,
  eventBus: EventBus | null,
  handlers: {
    onSelectRun: (name: string) => void;
    onSelectChecklist: (ref: string) => void;
    onCreate: () => void;
    onCancel?: () => void;
    onLocate: (f: FindingItem) => void;
    onExplain?: (item: string, index: number) => void;
    onLocateValue?: (v: WitnessValue, sheet?: string) => void;
    onSave?: () => void;
  },
  opts: { staticHost?: boolean } = {},
): { island: SolidIsland; view: ReviewView } {
  const [state, setState] = signalView<ReviewState>(emptyReview());
  const island = new SolidIsland(
    "review",
    el,
    () => (
      <ReviewPanel
        state={state}
        onSelectRun={handlers.onSelectRun}
        onSelectChecklist={handlers.onSelectChecklist}
        onCreate={handlers.onCreate}
        onCancel={handlers.onCancel}
        onLocate={handlers.onLocate}
        onExplain={handlers.onExplain}
        onLocateValue={handlers.onLocateValue}
        onSave={handlers.onSave}
        staticHost={opts.staticHost ?? false}
      />
    ),
    eventBus,
  );
  return { island, view: { setState } };
}
