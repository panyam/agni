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
  handlers: { onTrace: (from: string, to: string) => void },
): { island: SolidIsland; view: TraceView & TracePickView } {
  const [state, setState] = signalView<TraceState>(emptyTrace());
  const [picks, setPicks] = createSignal<TracePicks>(emptyPicks());
  const [from, setFrom] = createSignal("");
  const [to, setTo] = createSignal("");
  const ends: TraceEnds = { text: { from, to }, set: { from: setFrom, to: setTo }, inputs: {} };
  const island = new SolidIsland(
    "trace",
    el,
    () => <TracePanel state={state} picks={picks} ends={ends} onTrace={handlers.onTrace} />,
    eventBus,
  );
  const setEnd = (end: End, text: string) => {
    ends.set[end](text);
    ends.inputs[end]?.focus();
  };
  return { island, view: { setState, setPicks, setEnd } };
}

// TracePickView is the pickers' half of the panel's view: the design's parts and pins, and a way to
// fill one endpoint from another panel.
export interface TracePickView {
  setPicks: (p: TracePicks) => void;
  setEnd: (end: End, text: string) => void;
}
