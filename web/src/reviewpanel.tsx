import { For, Show, createSignal } from "solid-js";
import { SolidIsland, signalView } from "@panyam/tsappkit-solid";
import type { EventBus } from "@panyam/tsappkit";
import {
  type ReviewState,
  type ReviewView,
  type ReviewRunView,
  type ReviewItemView,
  type Tally,
  areaTally,
  covered,
  emptyReview,
  outcomeClass,
  runTally,
  selectedRun,
  OUTCOME_LABEL,
} from "./review.js";

// tallyLine is the headline a reviewer reads first: coverage, then the split.
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

// runLabel names a stored run in the picker: when it ran, and what it scored.
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

// ReviewItemRow is one checklist item: its id, title, verdict, any note, and (for an item that
// fired) its findings as click-to-locate rows. The findings collapse by default because a broad rule
// can fire on hundreds of nets and an expanded list would bury the checklist itself.
//
// A finding locates by (kind, subject), the same path the query and coverage panels use, NOT by the
// per-instance net id the checks panel can use. The checks panel can be precise because it holds the
// findings it is locating; these arrive inside a stored document instead. Two nets sharing a name
// therefore highlight together here. Recorded in OUT_OF_SCOPE.md rather than papered over.
function ReviewItemRow(props: {
  item: ReviewItemView;
  onLocate: (kind: string, subject: string) => void;
}) {
  const [open, setOpen] = createSignal(false);
  const findings = () => props.item.findings;
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
              {(f) => (
                <li>
                  <button
                    type="button"
                    class="rv-finding"
                    title={`locate ${f.kind} ${f.subject}`}
                    onClick={() => props.onLocate(f.kind, f.subject)}
                  >
                    <span class="rv-finding-subject">{f.subject}</span>
                    <span class="rv-finding-message">{f.message}</span>
                  </button>
                </li>
              )}
            </For>
          </ul>
        </Show>
      </Show>
    </li>
  );
}

// ReviewPanel shows a project's checklist verdict for the open design: a run picker over the stored
// history, a create control, then the selected run's areas and per-item outcomes.
function ReviewPanel(props: {
  state: () => ReviewState;
  onSelectRun: (name: string) => void;
  onSelectChecklist: (ref: string) => void;
  onCreate: () => void;
  onLocate: (kind: string, subject: string) => void;
}) {
  const run = () => selectedRun(props.state());
  return (
    <div class="review">
      <Show
        when={props.state().storeConfigured}
        fallback={
          <div class="rv-empty">
            This server does not keep review runs. Start it with <code>--review-store &lt;dir&gt;</code> to
            run a checklist and keep the result.
          </div>
        }
      >
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
            disabled={props.state().running || !props.state().checklist}
            onClick={() => props.onCreate()}
          >
            {props.state().running ? "Running…" : "Run review"}
          </button>
        </div>

        <Show when={props.state().error}>
          <div class="rv-error" role="alert">{props.state().error}</div>
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
              <div class="rv-summary">{tallyLine(runTally(r()))}</div>
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
                        {(item) => <ReviewItemRow item={item} onLocate={props.onLocate} />}
                      </For>
                    </ul>
                  </div>
                )}
              </For>
            </div>
          )}
        </Show>
      </Show>
    </div>
  );
}

// reviewPanelIsland mounts the panel and returns its command-down view (setState). The handlers are
// the intents up: pick a stored run, pick a checklist, create a run, and locate a finding's entity.
// Same island shape as findingsPanelIsland / coveragePanelIsland.
export function reviewPanelIsland(
  el: HTMLElement,
  eventBus: EventBus | null,
  handlers: {
    onSelectRun: (name: string) => void;
    onSelectChecklist: (ref: string) => void;
    onCreate: () => void;
    onLocate: (kind: string, subject: string) => void;
  },
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
        onLocate={handlers.onLocate}
      />
    ),
    eventBus,
  );
  return { island, view: { setState } };
}
