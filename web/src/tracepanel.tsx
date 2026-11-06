import { For, Show, createMemo, createSignal, type Accessor, type Setter } from "solid-js";
import { SolidIsland, signalView } from "@panyam/tsappkit-solid";
import type { EventBus } from "@panyam/tsappkit";
import {
  type TraceState,
  type TraceView,
  type TraceEndItem,
  TraceOutcome,
  emptyTrace,
} from "./trace.js";
import { type TracePick, type TracePicks, emptyPicks, pickCandidates } from "./tracepick.js";
import { type WalkGoalItem, type WalkRequest, type WalkState, type WalkView, WALK_GOAL_COLOR, emptyWalk, hopColor, routeLine } from "./walk.js";

// WalkHandlers are the walk section's intents up. onWalk walks again with a changed request (wider,
// narrower, a part crossed), onLocate moves the canvas to one row, and onClear drops the walk.
export interface WalkHandlers {
  onWalk: (r: WalkRequest) => void;
  onLocate: (kind: "net" | "component", name: string, sheet: string) => void;
  onClear: () => void;
}

// GOAL_TITLES names each goal question as its button does.
const GOAL_TITLES: Record<string, string> = { "pull-up": "To the pull-up", rail: "To the rail", pin: "To the pin" };

// WalkGoalBar asks a goal question from a net the walk reached (agni issue 1340). The reader picks
// the net, or leaves the start's own, then asks for the pull-up, the rail, or a pin they type.
function WalkGoalBar(props: { walk: () => WalkState; onGoal: (kind: string, at: string, to: string) => void }) {
  const w = () => props.walk();
  const [at, setAt] = createSignal(w().request.goal?.at ?? "");
  const [to, setTo] = createSignal(w().request.goal?.to ?? "");
  let toInput: HTMLInputElement | undefined;
  const reached = () => w().hops.flatMap((h) => h.nets.filter((n) => !n.ended).map((n) => ({ name: n.name, hop: h.hop })));
  const ask = (kind: string) => props.onGoal(kind, at(), kind === "pin" ? (toInput?.value ?? to()).trim() : "");
  return (
    <div class="walk-goals">
      <label class="walk-goal-at">
        Ask from{" "}
        <select data-walk-at value={at()} onChange={(e) => setAt(e.currentTarget.value)}>
          <option value="">the start's own net</option>
          <For each={reached()}>
            {(n) => (
              <option value={n.name}>
                hop {n.hop}: {n.name}
              </option>
            )}
          </For>
        </select>
      </label>
      <button type="button" class="mode-btn" data-action="walk-goal" data-goal="pull-up" disabled={w().loading} onClick={() => ask("pull-up")}>
        {GOAL_TITLES["pull-up"]}
      </button>
      <button type="button" class="mode-btn" data-action="walk-goal" data-goal="rail" disabled={w().loading} onClick={() => ask("rail")}>
        {GOAL_TITLES.rail}
      </button>
      <span class="walk-goal-pin">
        <input
          ref={toInput}
          type="text"
          data-walk-goal-to
          placeholder="pin, e.g. J4.2"
          spellcheck={false}
          value={to()}
          onInput={(e) => setTo(e.currentTarget.value)}
          onKeyDown={(e) => e.key === "Enter" && ask("pin")}
        />
        <button type="button" class="mode-btn" data-action="walk-goal" data-goal="pin" disabled={w().loading} onClick={() => ask("pin")}>
          {GOAL_TITLES.pin}
        </button>
      </span>
    </div>
  );
}

// WalkGoalAnswer shows a goal's answer below the walk. A route reads as one line, then each net with
// the part crossed into it. No route and a goal naming nothing read as two different things, as a
// trace's do.
function WalkGoalAnswer(props: { goal: WalkGoalItem; onLocate: WalkHandlers["onLocate"] }) {
  const g = () => props.goal;
  const what = () => (g().kind === "pin" ? `To ${g().to || "the pin"}` : (GOAL_TITLES[g().kind] ?? g().kind));
  return (
    <div class="walk-goal" data-goal-kind={g().kind} data-goal-outcome={g().outcome}>
      <div class="walk-goal-head">
        <span class="walk-swatch" style={{ background: WALK_GOAL_COLOR }} />
        {what()}, from {g().at}
      </div>
      <Show when={g().outcome === "no-route"}>
        <div class="trace-noroute">
          <div class="trace-headline">No route</div>
          <div class="trace-reason">{g().reason}</div>
        </div>
      </Show>
      <Show when={g().outcome === "unresolved"}>
        <div class="trace-unresolved">
          <div class="trace-headline">Cannot ask</div>
          <div class="trace-reason">{g().reason}</div>
        </div>
      </Show>
      <For each={g().routes}>
        {(r, i) => (
          <div class="walk-route" data-walk-route={i() + 1} data-ends-at={r.endsAt}>
            <div class="walk-route-line">{routeLine(r)}</div>
            <Show when={r.note}>
              <div class="walk-note">{r.note}</div>
            </Show>
            <ul class="walk-nets">
              <For each={r.nets}>
                {(net, j) => (
                  <li class="walk-net" data-walk-route-net={net}>
                    <button type="button" class="walk-net-name" onClick={() => props.onLocate("net", net, r.sheetIds[j()]?.[0] ?? "")}>
                      {net}
                    </button>
                    <Show when={j() > 0 && r.crossings[j() - 1]}>
                      {(c) => (
                        <span class="walk-entered">
                          {" "}through{" "}
                          <button type="button" class="walk-part" onClick={() => props.onLocate("component", c().refDes, r.sheetIds[j()]?.[0] ?? "")}>
                            {c().refDes}
                          </button>{" "}
                          pin {c().enterPin} to pin {c().exitPin}
                          {c().paired ? " (paired)" : ""}
                        </span>
                      )}
                    </Show>
                  </li>
                )}
              </For>
            </ul>
          </div>
        )}
      </For>
      <Show when={g().outcome === "routed"}>
        <div class="trace-summary">
          {g().routes.length} route{g().routes.length === 1 ? "" : "s"}, searched to {g().radius} {g().kind === "pull-up" ? "hops" : "crossings"} from {g().at}
        </div>
      </Show>
    </div>
  );
}

// WalkSection lists a walk hop by hop (agni issue 1341). Each hop is a group the reader can fold, in
// the shade the canvas lights it in. Each net says the part it was entered through, and a net where
// the walk stopped says why. The parts the walk did not cross come last, each with its reason and a
// button that crosses it.
function WalkSection(props: { walk: () => WalkState; handlers: WalkHandlers }) {
  const w = () => props.walk();
  const [folded, setFolded] = createSignal<Set<number>>(new Set());
  const toggle = (hop: number) =>
    setFolded((f) => {
      const next = new Set(f);
      if (next.has(hop)) next.delete(hop);
      else next.add(hop);
      return next;
    });
  const again = (change: Partial<WalkRequest>) => props.handlers.onWalk({ ...w().request, ...change });
  const nets = () => w().hops.reduce((n, h) => n + h.nets.length, 0);
  return (
    <div class="walk" data-walk-start={w().request.start}>
      <div class="walk-head">
        <span class="walk-title">
          Walk from <strong>{w().request.start}</strong>, {w().request.kind || "series"}, {w().request.hops} hop
          {w().request.hops === 1 ? "" : "s"}
        </span>
        <button type="button" class="mode-btn walk-narrower" data-action="walk-narrower" disabled={w().loading || w().request.hops <= 1} onClick={() => again({ hops: w().request.hops - 1 })}>
          Narrower
        </button>
        <button type="button" class="mode-btn walk-wider" data-action="walk-wider" disabled={w().loading} onClick={() => again({ hops: w().request.hops + 1 })}>
          Wider
        </button>
        <button type="button" class="mode-btn walk-clear" data-action="walk-clear" onClick={() => props.handlers.onClear()}>
          Clear
        </button>
      </div>
      <Show when={w().loading}>
        <div class="walk-note walk-loading">Walking…</div>
      </Show>
      <Show when={w().error}>
        <div class="trace-error">{w().error}</div>
      </Show>
      <Show when={w().reason}>
        <div class="trace-unresolved">
          <div class="trace-headline">Cannot walk</div>
          <div class="trace-reason">{w().reason}</div>
        </div>
      </Show>
      <Show when={!w().loading && !w().error && !w().reason}>
        <For each={w().hops}>
          {(h) => (
            <div class="walk-hop" data-hop-group={h.hop}>
              <button type="button" class="walk-hop-toggle" aria-expanded={!folded().has(h.hop)} onClick={() => toggle(h.hop)}>
                <span class="walk-swatch" style={{ background: hopColor(h.hop) }} />
                hop {h.hop}, {h.nets.length} net{h.nets.length === 1 ? "" : "s"}
              </button>
              <Show when={!folded().has(h.hop)}>
                <ul class="walk-nets">
                  <For each={h.nets}>
                    {(n) => (
                      <li class={`walk-net${n.ended ? " ended" : ""}`} data-walk-net={n.name} data-hop={n.hop}>
                        <button type="button" class="walk-net-name" onClick={() => props.handlers.onLocate("net", n.name, n.sheetIds[0] ?? "")}>
                          {n.name}
                        </button>
                        <span class="walk-entered">
                          {n.fromNet ? (
                            <>
                              {" "}through{" "}
                              <button type="button" class="walk-part" onClick={() => props.handlers.onLocate("component", n.through, n.sheetIds[0] ?? "")}>
                                {n.through}
                              </button>{" "}
                              pin {n.enterPin} to pin {n.exitPin}
                              {n.paired ? " (paired)" : ""}
                            </>
                          ) : (
                            <> from pin {n.exitPin}</>
                          )}
                          <Show when={n.alsoThrough.length > 0}>, also through {n.alsoThrough.join(", ")}</Show>
                        </span>
                        <Show when={n.ended}>
                          <span class="walk-ended">, ends here: {n.ended}</span>
                        </Show>
                        <Show when={n.parts.length > 0 || n.partsElided > 0}>
                          <div class="walk-parts">
                            <For each={n.parts}>
                              {(p) => (
                                <button type="button" class={`walk-part${p.cls === "test_point" ? " probe" : ""}`} onClick={() => props.handlers.onLocate("component", p.refDes, n.sheetIds[0] ?? "")}>
                                  {p.refDes}.{p.pin}
                                </button>
                              )}
                            </For>
                            <Show when={n.partsElided > 0}>
                              <span class="walk-more"> and {n.partsElided} more</span>
                            </Show>
                          </div>
                        </Show>
                      </li>
                    )}
                  </For>
                </ul>
              </Show>
            </div>
          )}
        </For>
        <For each={w().elided}>{(e) => <div class="walk-note">and {e.nets} more nets at hop {e.hop}, past the cap</div>}</For>
        <Show when={w().boundary.length > 0}>
          <div class="walk-boundary-head">not crossed</div>
          <ul class="walk-boundary">
            <For each={w().boundary}>
              {(b) => (
                <li data-walk-boundary={b.refDes}>
                  <button type="button" class="walk-part" onClick={() => props.handlers.onLocate("component", b.refDes, "")}>
                    {b.refDes}
                  </button>
                  <Show when={b.cls}>
                    <span class="trace-stub-class"> ({b.cls.replace(/_/g, " ")})</span>
                  </Show>{" "}
                  on {b.net}: {b.reason}{" "}
                  <button type="button" class="mode-btn walk-cross" data-action="walk-cross" onClick={() => again({ through: [...w().request.through, b.refDes] })}>
                    Cross
                  </button>
                </li>
              )}
            </For>
          </ul>
        </Show>
        <div class="trace-summary">
          {nets()} net{nets() === 1 ? "" : "s"} over {w().hops.length} hop{w().hops.length === 1 ? "" : "s"}, {w().boundary.length} part
          {w().boundary.length === 1 ? "" : "s"} not crossed
        </div>
        <WalkGoalBar walk={w} onGoal={(kind, at, to) => again({ goal: { kind, at, to } })} />
        <Show when={w().goal}>{(g) => <WalkGoalAnswer goal={g()} onLocate={props.handlers.onLocate} />}</Show>
      </Show>
    </div>
  );
}

type End = "from" | "to";

// TraceEnds is the two endpoint fields' text, held by the island so the host can fill one ("trace
// from here" in Properties) as well as the reader.
interface TraceEnds {
  text: Record<End, Accessor<string>>;
  set: Record<End, Setter<string>>;
  inputs: Partial<Record<End, HTMLInputElement>>;
}

// endLabel is "U1.3 (SDA)", falling back to the bare pin where the source carries no pin names.
function endLabel(e: TraceEndItem): string {
  const pin = `${e.refDes}.${e.pin}`;
  return e.pinName ? `${pin} (${e.pinName})` : pin;
}

// TracePanel asks for two pins and shows the answer: the route as one line, then each net with what
// else sits on it, then what the answer rests on.
//
// The three outcomes render as three different things rather than one message with a status word.
// A route and a no-route are facts about the design, and a no-route names the nets it is about. An
// unresolved endpoint is a failed QUESTION, so it names the name that failed and never mentions
// connectivity, or a reader would go look at the board instead of at what they typed.
function TracePanel(props: {
  state: () => TraceState;
  walk: () => WalkState;
  walkHandlers: WalkHandlers;
  picks: () => TracePicks;
  ends: TraceEnds;
  onTrace: (from: string, to: string) => void;
}) {
  const ends = props.ends;
  // run reads the fields themselves, which hold whatever the reader typed or pasted however it got
  // there, rather than the text last seen by an input event.
  const run = () => {
    setOpen(null);
    const text = (end: End) => (ends.inputs[end]?.value ?? ends.text[end]()).trim();
    props.onTrace(text("from"), text("to"));
  };
  // open is the field whose picker is showing, and active the highlighted entry in it, -1 for none.
  // One list serves both fields and sits below them at the panel's width, inside the panel's own flow,
  // since a dropdown floating over a docked panel is clipped by the panel's edge.
  const [open, setOpen] = createSignal<End | null>(null);
  const [active, setActive] = createSignal(-1);
  const candidates = createMemo(() => {
    const end = open();
    return end ? pickCandidates(props.picks(), ends.text[end]()) : [];
  });
  // choose puts a pick in its field. A component lists its pins next, and a pin ends the pick and
  // moves on to the other field when that one is still empty.
  const choose = (end: End, p: TracePick) => {
    if (p.kind === "component") {
      ends.set[end](`${p.value}.`);
      setActive(-1);
      ends.inputs[end]?.focus();
      return;
    }
    ends.set[end](p.value);
    setOpen(null);
    const other: End = end === "from" ? "to" : "from";
    if (end === "from" && ends.text[other]() === "") ends.inputs[other]?.focus();
  };
  // A highlighted entry takes Enter. With none highlighted Enter traces what was typed, so a pin
  // pasted from a datasheet never has to be picked.
  const onKey = (end: End, e: KeyboardEvent) => {
    const n = candidates().length;
    if (e.key === "ArrowDown") {
      e.preventDefault();
      setOpen(end);
      setActive((i) => Math.min(i + 1, n - 1));
    } else if (e.key === "ArrowUp") {
      e.preventDefault();
      setActive((i) => Math.max(i - 1, -1));
    } else if (e.key === "Escape") {
      setOpen(null);
    } else if (e.key === "Enter") {
      const i = active();
      if (open() === end && i >= 0 && i < n) choose(end, candidates()[i]);
      else run();
    }
  };
  const field = (end: End, label: string, fallback: string) => {
    const example = () => props.picks().examples[end === "from" ? 0 : 1] || fallback;
    return (
      <input
        ref={(el) => (ends.inputs[end] = el)}
        class="trace-pin"
        type="text"
        role="combobox"
        aria-label={label}
        aria-autocomplete="list"
        aria-controls="trace-picks"
        aria-expanded={open() === end && candidates().length > 0}
        aria-activedescendant={open() === end && active() >= 0 ? `trace-pick-${active()}` : undefined}
        autocomplete="off"
        spellcheck={false}
        placeholder={`${end}, e.g. ${example()}`}
        value={ends.text[end]()}
        onInput={(e) => {
          ends.set[end](e.currentTarget.value);
          setOpen(end);
          setActive(-1);
        }}
        onFocus={() => {
          setOpen(end);
          setActive(-1);
        }}
        onBlur={() => setOpen((o) => (o === end ? null : o))}
        onKeyDown={(e) => onKey(end, e)}
      />
    );
  };
  const s = () => props.state();
  return (
    <div class="trace">
      <Show when={props.walk().ran}>
        <WalkSection walk={props.walk} handlers={props.walkHandlers} />
      </Show>
      <div class="trace-ask">
        {field("from", "Start pin", "U1.3")}
        {field("to", "End pin", "J1.1")}
        <button type="button" class="trace-run" disabled={s().loading} onClick={run}>
          {s().loading ? "Tracing…" : "Trace"}
        </button>
      </div>
      <Show when={open() && candidates().length > 0}>
        <ul class="trace-picks" id="trace-picks" role="listbox" aria-label={open() === "from" ? "Start pin" : "End pin"}>
          <For each={candidates()}>
            {(p, i) => (
              <li
                id={`trace-pick-${i()}`}
                role="option"
                class={`trace-pick ${p.kind}`}
                aria-selected={active() === i()}
                // mousedown rather than click, and the default prevented, so the field keeps focus and
                // its blur does not close the list before the choice lands.
                onMouseDown={(e) => {
                  e.preventDefault();
                  choose(open()!, p);
                }}
              >
                <span class="trace-pick-value">{p.value}</span>
                <Show when={p.detail}>
                  <span class="trace-pick-detail">{p.detail}</span>
                </Show>
              </li>
            )}
          </For>
        </ul>
      </Show>

      <Show when={s().error}>
        <div class="trace-error">{s().error}</div>
      </Show>

      <Show when={s().ran && !s().loading && !s().error}>
        <Show when={s().outcome === TraceOutcome.UNRESOLVED}>
          <div class="trace-unresolved">
            <div class="trace-headline">Cannot trace</div>
            <div class="trace-reason">{s().reason}</div>
            <div class="trace-note">
              Nothing was walked. This is a failed question rather than a disconnection.
            </div>
          </div>
        </Show>

        <Show when={s().outcome === TraceOutcome.NO_ROUTE}>
          <div class="trace-noroute">
            <div class="trace-headline">
              No route: {endLabel(s().from)} to {endLabel(s().to)}
            </div>
            <div class="trace-reason">{s().reason}</div>
            <div class="trace-note">
              The walk crosses resistors, inductors, ferrites and fuses. A capacitor is a DC block and
              is never crossed. A rail or plane may end a route and is never passed through.
            </div>
          </div>
        </Show>

        <Show when={s().outcome === TraceOutcome.ROUTED}>
          <div class="trace-headline">
            {[endLabel(s().from), ...s().crossings.map((c) => c.refDes), endLabel(s().to)].join(" → ")}
          </div>
          <ol class="trace-route">
            <For each={s().nets}>
              {(n, i) => (
                <>
                  <li class={`trace-net${n.busLike ? " bus-like" : ""}`}>
                    <span class="trace-net-name">{n.name}</span>
                    <Show when={n.busLike}>
                      <span class="trace-rail-note">rail or plane: a route may end here, never pass through</span>
                    </Show>
                    {/* The elided COUNT lives inside this list, so the gate admits a net whose
                        stubs were all dropped. The server caps at twelve and cannot produce that
                        today, but rendering nothing then would hide the count. */}
                    <Show when={n.stubs.length > 0 || n.stubsElided > 0}>
                      <ul class="trace-stubs">
                        <For each={n.stubs}>
                          {(st) => (
                            <li class={`trace-stub${st.cls === "test_point" ? " probe" : ""}`}>
                              {st.refDes}.{st.pin}
                              <Show when={st.cls}>
                                <span class="trace-stub-class"> ({st.cls.replace(/_/g, " ")})</span>
                              </Show>
                            </li>
                          )}
                        </For>
                        <Show when={n.stubsElided > 0}>
                          <li class="trace-stub elided">and {n.stubsElided} more</li>
                        </Show>
                      </ul>
                    </Show>
                  </li>
                  <Show when={i() < s().crossings.length}>
                    <li class="trace-cross">
                      cross {s().crossings[i()].refDes}
                      <Show when={s().crossings[i()].cls}>
                        <span class="trace-stub-class"> ({s().crossings[i()].cls.replace(/_/g, " ")})</span>
                      </Show>
                      <span class="trace-cross-pins">
                        {" "}
                        pin {s().crossings[i()].enterPin} to pin {s().crossings[i()].exitPin}
                      </span>
                    </li>
                  </Show>
                </>
              )}
            </For>
          </ol>
          <div class="trace-summary">
            {s().crossings.length} crossing{s().crossings.length === 1 ? "" : "s"}, {s().nets.length} net
            {s().nets.length === 1 ? "" : "s"}, searched to a radius of {s().radius}
          </div>
        </Show>
      </Show>

      <Show when={!s().ran}>
        <div class="trace-empty">
          Name two pins to follow a signal through the series parts between them.
        </div>
      </Show>
    </div>
  );
}

// tracePanelIsland mounts the panel and returns its command-down view. onTrace is the intent up,
// fired when the reader names two pins, and the presenter runs the walk and highlights the answer.
// setPicks hands the pickers the open design's parts and pins, and setEnd fills one field from
// outside the panel. Same island shape as coveragePanelIsland / findingsPanelIsland.
export function tracePanelIsland(
  el: HTMLElement,
  eventBus: EventBus | null,
  handlers: { onTrace: (from: string, to: string) => void; walk?: WalkHandlers },
): { island: SolidIsland; view: TraceView & TracePickView & WalkView } {
  const [state, setState] = signalView<TraceState>(emptyTrace());
  const [walk, setWalk] = createSignal<WalkState>(emptyWalk());
  const walkHandlers: WalkHandlers = handlers.walk ?? { onWalk: () => {}, onLocate: () => {}, onClear: () => {} };
  const [picks, setPicks] = createSignal<TracePicks>(emptyPicks());
  const [from, setFrom] = createSignal("");
  const [to, setTo] = createSignal("");
  const ends: TraceEnds = { text: { from, to }, set: { from: setFrom, to: setTo }, inputs: {} };
  const island = new SolidIsland(
    "trace",
    el,
    () => <TracePanel state={state} walk={walk} walkHandlers={walkHandlers} picks={picks} ends={ends} onTrace={handlers.onTrace} />,
    eventBus,
  );
  const setEnd = (end: End, text: string) => {
    ends.set[end](text);
    ends.inputs[end]?.focus();
  };
  return { island, view: { setState, setPicks, setEnd, setWalk } };
}

// TracePickView is the pickers' half of the panel's view: the design's parts and pins, and a way to
// fill one endpoint from another panel.
export interface TracePickView {
  setPicks: (p: TracePicks) => void;
  setEnd: (end: End, text: string) => void;
}
